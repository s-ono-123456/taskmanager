package mattermost

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// LLMClient はOpenAI互換のchat completions APIを持つローカルLLM
// （このホストで稼働中のllama-swap、docs/adr/complete/mattermost-extractor-llm-choice.md参照）
// への最小限のクライアント。Claude API等の外部LLMには接続しない。
type LLMClient struct {
	baseURL string
	model   string
	http    *http.Client
}

func NewLLMClient(baseURL, model string) *LLMClient {
	// ローカルLLMはコールドロード(モデル切替)に数十秒かかりうるため、余裕を持ったタイムアウトにする。
	return &LLMClient{baseURL: baseURL, model: model, http: &http.Client{Timeout: 90 * time.Second}}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatCompletionRequest struct {
	Model          string         `json:"model"`
	Messages       []chatMessage  `json:"messages"`
	Temperature    float64        `json:"temperature"`
	ResponseFormat responseFormat `json:"response_format"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// ClassifyResult はLLMが1投稿について返す分類結果(candidatesテーブルの列に対応)。
type ClassifyResult struct {
	PostID         string  `json:"post_id"`
	Kind           string  `json:"kind"` // "task" / "completion" / "none"
	Confidence     float64 `json:"confidence"`
	Target         string  `json:"target"` // "jira_a" / "jira_b" / "personal" / "unknown"
	AssigneeRaw    string  `json:"assignee_raw"`
	DueDate        string  `json:"due_date"`
	Summary        string  `json:"summary"`
	RelatedJiraKey string  `json:"related_jira_key"`
	// RelatedTaskID: kind=completionでRelatedJiraKeyが空の場合に、OpenTaskで渡した
	// 一覧の中から対象タスクを一意に推定できればそのid(文字列)、確信が持てなければ空文字
	// (docs/adr/proposals/close-request-target-task-suggestion.md参照)。
	RelatedTaskID string `json:"related_task_id"`
}

// OpenTask はClassifyに渡す「未クローズタスク一覧」1件分(対象タスク推定の手がかり)。
type OpenTask struct {
	ID    int64
	Title string
}

type classifyResponseBody struct {
	Results []ClassifyResult `json:"results"`
}

// MinCandidateConfidence: この値未満のconfidenceは、kindの値に関わらず
// (誤検知抑制のため)呼び出し元で破棄する目安として使う。
const MinCandidateConfidence = 0.3

// Classify はスレッド全文(threadTranscript、時系列に整形済みのプレーンテキスト)を文脈として、
// targetPostIDsに列挙した投稿(スレッド内の新着分)それぞれをkind/target等に分類する。
// projectHintはそのチャンネルに設定されたtarget(jira_a/jira_b/personal)の手がかりとして
// プロンプトに含める(本文の内容と矛盾する場合は本文を優先させる、
// docs/design/mail-zoom-pipeline.md「抽出・分類」節の方針を踏襲)。
// openTasksはprojectHintに紐づく未クローズタスク一覧で、kind=completionと判定した投稿の
// 対象タスク推定(RelatedTaskID)の手がかりとして渡す
// (docs/adr/complete/close-request-target-task-suggestion.md参照)。
func (c *LLMClient) Classify(ctx context.Context, threadTranscript string, targetPostIDs []string, projectHint string, openTasks []OpenTask) ([]ClassifyResult, error) {
	system := `あなたはMattermostの発言を分類するアシスタントです。渡されたスレッドの文脈を踏まえ、
指定された投稿IDそれぞれについて、次のJSON形式で厳密に回答してください（説明文は不要、JSONのみ）:
{"results":[{"post_id":"...","kind":"task|completion|none","confidence":0.0から1.0,
"target":"jira_a|jira_b|personal|unknown","assignee_raw":"","due_date":"YYYY-MM-DD or 空文字",
"summary":"","related_jira_key":"kind=completionでJIRAキーが本文に明示されている場合のみ、無ければ空文字",
"related_task_id":"kind=completionでrelated_jira_keyが空の場合のみ、open_tasksの中から対象を
一意に推定できればそのid、確信が持てなければ空文字"}]}

kind=taskは新しい依頼・やるべきことの発生、kind=completionは既存作業の完了報告、
どちらでもなければkind=noneとしてください。targetはchannel_hintを手がかりにしてよいですが、
本文の内容と矛盾する場合は本文を優先し、確信が持てなければunknownにしてください。
断定できない項目は空文字のままにし、推測で埋めないでください。related_task_idはopen_tasksに
列挙されていないidを返してはいけません。`

	openTasksText := "（未クローズタスクなし）"
	if len(openTasks) > 0 {
		var b strings.Builder
		for _, t := range openTasks {
			fmt.Fprintf(&b, "id=%d: %s\n", t.ID, t.Title)
		}
		openTasksText = b.String()
	}

	user := fmt.Sprintf(
		"channel_hint(target): %s\n\n--- スレッド全文(時系列) ---\n%s\n\n--- 分類対象の投稿ID ---\n%s\n\n--- open_tasks(%sの未クローズタスク一覧) ---\n%s",
		projectHint, threadTranscript, joinIDs(targetPostIDs), projectHint, openTasksText,
	)

	reqBody := chatCompletionRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature:    0.1,
		ResponseFormat: responseFormat{Type: "json_object"},
	}

	buf, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("chat completions API returned status %d: %s", resp.StatusCode, string(b))
	}

	var completion chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&completion); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("chat completions API returned no choices")
	}

	var body classifyResponseBody
	if err := json.Unmarshal([]byte(completion.Choices[0].Message.Content), &body); err != nil {
		return nil, fmt.Errorf("parse classification JSON: %w (content=%q)", err, completion.Choices[0].Message.Content)
	}
	return body.Results, nil
}

func joinIDs(ids []string) string {
	out := ""
	for i, id := range ids {
		if i > 0 {
			out += ", "
		}
		out += id
	}
	return out
}
