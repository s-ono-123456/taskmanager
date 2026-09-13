package mattermost

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"taskmanager/internal/taskstore"
)

// Config はcollector/extractorの設定(環境変数から読み込む)。
type Config struct {
	BotToken      string
	ServerURL     string
	ChannelRoutes map[string]string // channelID -> project_hint(jira_a/jira_b/personal)
	LLMBaseURL    string
	LLMModel      string
}

// LoadConfigFromEnv は MATTERMOST_BOT_TOKEN / MATTERMOST_SERVER_URL /
// MATTERMOST_CHANNEL_ROUTES(例 "chID1:jira_a,chID2:jira_b,chID3:personal") /
// MATTERMOST_EXTRACTOR_LLM_URL / MATTERMOST_EXTRACTOR_LLM_MODEL を読み込む。
// MATTERMOST_BOT_TOKENが空の場合はconfigured=falseを返す(collectorを起動しない)。
func LoadConfigFromEnv() (cfg Config, configured bool, err error) {
	token := os.Getenv("MATTERMOST_BOT_TOKEN")
	if token == "" {
		return Config{}, false, nil
	}
	serverURL := strings.TrimSuffix(os.Getenv("MATTERMOST_SERVER_URL"), "/")
	if serverURL == "" {
		return Config{}, false, fmt.Errorf("MATTERMOST_SERVER_URLが未設定です")
	}
	routes, err := parseChannelRoutes(os.Getenv("MATTERMOST_CHANNEL_ROUTES"))
	if err != nil {
		return Config{}, false, err
	}
	if len(routes) == 0 {
		return Config{}, false, fmt.Errorf("MATTERMOST_CHANNEL_ROUTESが未設定です")
	}

	// ローカルLLM(llama-swap、docs/adr/complete/mattermost-extractor-llm-choice.md参照)。
	// host.docker.internalはcompose/docker-compose.ymlのextra_hosts設定で解決される。
	llmBaseURL := strings.TrimSuffix(os.Getenv("MATTERMOST_EXTRACTOR_LLM_URL"), "/")
	if llmBaseURL == "" {
		llmBaseURL = "http://host.docker.internal:8080"
	}
	llmModel := os.Getenv("MATTERMOST_EXTRACTOR_LLM_MODEL")
	if llmModel == "" {
		llmModel = "qwen3.8-flash-next-q5"
	}

	return Config{
		BotToken:      token,
		ServerURL:     serverURL,
		ChannelRoutes: routes,
		LLMBaseURL:    llmBaseURL,
		LLMModel:      llmModel,
	}, true, nil
}

// parseChannelRoutes は "chID1:jira_a,chID2:jira_b" 形式の文字列を
// map[channelID]project_hint にパースする。
func parseChannelRoutes(s string) (map[string]string, error) {
	routes := make(map[string]string)
	s = strings.TrimSpace(s)
	if s == "" {
		return routes, nil
	}
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return nil, fmt.Errorf("MATTERMOST_CHANNEL_ROUTESの形式が不正です: %q", pair)
		}
		routes[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return routes, nil
}

// PollInterval: collectorが常駐goroutineでMattermost APIをポーリングする間隔
// (docs/adr/complete/collection-trigger.md「収集トリガー方式」で採用したcron定期ポーリング、
// task-management-automation.mdの既存設計と同じ10分間隔)。
const PollInterval = 10 * time.Minute

// initialLookbackWindow: あるチャンネルの処理位置が1件も無い(初回ポーリング)場合、
// 直近この期間分から取得を開始する(全履歴を一度に取得しないため)。
const initialLookbackWindow = 24 * time.Hour

// StartCollectorLoop は常駐goroutineを起動する。起動直後に1回実行し、その後
// PollInterval間隔で繰り返す(internal/taskstore/rollover.goのStartRolloverLoopと同じ構造)。
//
// rollover.goと異なり、この初回実行は**goroutine内で非同期に**行う点に注意。
// LLM分類はスレッド単位で逐次実行され(実測、ローカルLLMで1スレッドあたり数秒〜)、
// 初回起動時に未処理分(最大initialLookbackWindow=24時間分)がまとまっていると
// 数分かかることが実機で確認された。ロールオーバーのような軽量なDB更新とは異なり、
// これを呼び出し元(main.go)をブロックして同期実行すると、その間HTTPサーバーが
// 起動できず、ダッシュボード自体が使えなくなってしまうため非同期にしている。
func StartCollectorLoop(ctx context.Context, db *sql.DB, cfg Config) {
	client := NewClient(cfg.ServerURL, cfg.BotToken)
	llm := NewLLMClient(cfg.LLMBaseURL, cfg.LLMModel)
	q := taskstore.New(db)

	go func() {
		runOnce(ctx, client, llm, db, q, cfg)

		ticker := time.NewTicker(PollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runOnce(ctx, client, llm, db, q, cfg)
			}
		}
	}()
}

// runOnce は設定済みチャンネルごとに、新着投稿の取得→分類→登録/保留を行う。
// チャンネル単位でエラーが起きても他のチャンネルの処理は継続する。
func runOnce(ctx context.Context, client *Client, llm *LLMClient, db *sql.DB, q *taskstore.Queries, cfg Config) {
	for channelID, projectHint := range cfg.ChannelRoutes {
		if err := collectChannel(ctx, client, llm, db, q, channelID, projectHint); err != nil {
			log.Printf("[mattermost-extractor] channel=%s エラー: %v", channelID, err)
		}
	}
}

// collectChannel は1チャンネル分の「取得→スレッド単位でグループ化→分類→登録/破棄」を行う
// (docs/adr/proposals/mattermost-extractor-batching.md参照)。GPU競合の回避は行わない
// (docs/adr/proposals/mattermost-extractor-llm-choice.md参照、許容する方針)。
func collectChannel(ctx context.Context, client *Client, llm *LLMClient, db *sql.DB, q *taskstore.Queries, channelID, projectHint string) error {
	sinceMS, err := cursorForChannel(ctx, q, channelID)
	if err != nil {
		return fmt.Errorf("cursor取得: %w", err)
	}

	posts, err := client.GetPostsSince(ctx, channelID, sinceMS)
	if err != nil {
		return fmt.Errorf("投稿取得: %w", err)
	}
	if len(posts) == 0 {
		return nil
	}

	// 分類対象(本文が空でないもの)をスレッドキーでグループ化する。
	groups := make(map[string][]Post)
	var maxCreateAt int64
	for _, p := range posts {
		if p.CreateAt > maxCreateAt {
			maxCreateAt = p.CreateAt
		}
		if p.Message == "" {
			continue // システムメッセージ等はスキップ(cursorは進めるが分類しない)。
		}
		groups[threadKey(p)] = append(groups[threadKey(p)], p)
	}

	// kind=completionの対象タスク推定(docs/adr/proposals/close-request-target-task-suggestion.md参照)
	// に使う未クローズタスク一覧。チャンネル(=projectHint)ごとに1回だけ取得しスレッドグループ間で使い回す。
	openTaskRows, err := q.ListOpenTasksByTarget(ctx, projectHint)
	if err != nil {
		return fmt.Errorf("未クローズタスク一覧取得: %w", err)
	}
	openTasks := make([]OpenTask, len(openTaskRows))
	for i, t := range openTaskRows {
		openTasks[i] = OpenTask{ID: t.ID, Title: t.Title}
	}

	usernameCache := make(map[string]string)
	registered, pending := 0, 0

	for key, newPosts := range groups {
		thread, err := client.GetThread(ctx, key)
		if err != nil {
			log.Printf("[mattermost-extractor] channel=%s thread=%s のスレッド取得に失敗（スキップ）: %v", channelID, key, err)
			continue
		}
		resolveUsernames(ctx, client, usernameCache, thread)

		transcript := formatTranscript(thread, usernameCache)
		targetIDs := make([]string, len(newPosts))
		postByID := make(map[string]Post, len(newPosts))
		for i, p := range newPosts {
			targetIDs[i] = p.ID
			postByID[p.ID] = p
		}

		results, err := llm.Classify(ctx, transcript, targetIDs, projectHint, openTasks)
		if err != nil {
			log.Printf("[mattermost-extractor] channel=%s thread=%s のLLM分類に失敗（スキップ）: %v", channelID, key, err)
			continue
		}

		for _, result := range results {
			p, ok := postByID[result.PostID]
			if !ok {
				continue // LLMが対象外のIDを返した場合は無視。
			}
			if result.Kind == "" || result.Kind == "none" || result.Confidence < MinCandidateConfidence {
				continue // 破棄(保存しない)。
			}
			autoRegistered, err := registerCandidate(ctx, client, db, q, channelID, projectHint, p, result, openTasks)
			if err != nil {
				log.Printf("[mattermost-extractor] channel=%s post=%s の登録に失敗: %v", channelID, p.ID, err)
				continue
			}
			if autoRegistered {
				registered++
			} else {
				pending++
			}
		}
	}

	if registered > 0 || pending > 0 {
		log.Printf("[mattermost-extractor] channel=%s 自動登録%d件・確認待ち候補%d件", channelID, registered, pending)
	}

	// 分類結果に関わらず、取得できた全投稿の最大create_atでcursorを進める
	// (LLM分類に失敗した投稿も再分類はしない、という割り切り)。
	if err := q.UpsertMattermostChannelState(ctx, taskstore.UpsertMattermostChannelStateParams{
		ChannelID:       channelID,
		LastProcessedAt: time.UnixMilli(maxCreateAt).UTC().Format(time.RFC3339),
	}); err != nil {
		return fmt.Errorf("cursor更新: %w", err)
	}
	return nil
}

// threadKey はpが属するスレッドのルート投稿IDを返す(返信ならroot_id、ルート投稿なら自身のID)。
func threadKey(p Post) string {
	if p.RootID != "" {
		return p.RootID
	}
	return p.ID
}

// resolveUsernames はpostsの投稿者名のうちcacheに無いものだけ解決する
// (ポーリングサイクル内でのAPI呼び出し削減)。個別の解決失敗はログのみでベストエフォート継続する。
func resolveUsernames(ctx context.Context, client *Client, cache map[string]string, posts []Post) {
	for _, p := range posts {
		if _, ok := cache[p.UserID]; ok {
			continue
		}
		name, err := client.GetUsername(ctx, p.UserID)
		if err != nil {
			log.Printf("[mattermost-extractor] user=%s の名前解決に失敗（未設定のまま処理）: %v", p.UserID, err)
			name = ""
		}
		cache[p.UserID] = name
	}
}

// formatTranscript はスレッドの投稿群を時系列のプレーンテキストに整形する
// (LLMへのプロンプトに含める文脈)。
func formatTranscript(posts []Post, usernames map[string]string) string {
	var b strings.Builder
	for _, p := range posts {
		author := usernames[p.UserID]
		if author == "" {
			author = "(不明なユーザー)"
		}
		ts := time.UnixMilli(p.CreateAt).UTC().Format("2006-01-02 15:04")
		fmt.Fprintf(&b, "[%s] %s (post_id=%s): %s\n", ts, author, p.ID, p.Message)
	}
	return b.String()
}

// suggestedTaskID はLLMが返したrelated_task_id(文字列)を検証しsql.NullInt64へ変換する。
// openTasksに実在しないid・空文字・数値変換できない値はすべて無効として扱う(LLMの
// ハルシネーション対策。プロンプトでも一覧外のidを返さないよう指示しているが、
// ここでも防御的に検証する)。
func suggestedTaskID(raw string, openTasks []OpenTask) sql.NullInt64 {
	if raw == "" {
		return sql.NullInt64{}
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return sql.NullInt64{}
	}
	for _, t := range openTasks {
		if t.ID == id {
			return sql.NullInt64{Int64: id, Valid: true}
		}
	}
	return sql.NullInt64{}
}

// isConfirmedTarget はtargetがjira_a/jira_b/personalのいずれか(自動登録可能な確定値)かを返す。
// internal/web.isValidTargetと同じ値だが、パッケージをまたいで共有しない方針
// (internal/mattermostはinternal/webに依存させない)のためここで独自に定義する。
func isConfirmedTarget(target string) bool {
	return target == "jira_a" || target == "jira_b" || target == "personal"
}

// stubJiraTransition は本来ここでJIRA REST APIを呼ぶ想定のスタブ。外部通信は行わない
// (internal/web.stubJiraTransitionと同じ役割・同じログ文言をパッケージ内に複製している)。
func stubJiraTransition(jiraKey, action string) {
	fmt.Printf("[stub] JIRA API呼び出し想定: jira_key=%s action=%s（実際の通信なし）\n", jiraKey, action)
}

// registerCandidate はLLMの分類結果1件を保存する。target確定のkind=taskは自動的に
// tasksへ登録し(戻り値true)、それ以外(kind=completion、またはkind=taskでtarget未確定)は
// candidatesに保留のまま残す(戻り値false。既存の「クローズ要求一覧」・新設の
// 「タスク候補一覧」がそれぞれ表示する。docs/adr/proposals/mattermost-extractor-registration-flow.md参照)。
//
// メッセージ保存・候補保存・(自動登録時の)タスク作成は1トランザクションで行う。
// 分けて実行すると、途中でプロセスが中断した場合にメッセージだけが保存され候補が
// 作られない「孤立レコード」が発生し、MessageExistsBySourceIDの重複防止チェックに
// より二度と再分類されなくなる不具合があったため(実データで1件発生を確認、
// docs/work-log.md参照)。
func registerCandidate(ctx context.Context, client *Client, db *sql.DB, q *taskstore.Queries, channelID, projectHint string, p Post, result ClassifyResult, openTasks []OpenTask) (autoRegistered bool, err error) {
	exists, err := q.MessageExistsBySourceID(ctx, taskstore.MessageExistsBySourceIDParams{
		Source:   "mattermost",
		SourceID: sql.NullString{String: p.ID, Valid: true},
	})
	if err != nil {
		return false, fmt.Errorf("重複確認: %w", err)
	}
	if exists {
		return false, nil
	}

	permalink, err := client.PermalinkURL(ctx, channelID, p.ID)
	if err != nil {
		log.Printf("[mattermost-extractor] post=%s のパーマリンク解決に失敗（URL無しで保存）: %v", p.ID, err)
		permalink = ""
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("トランザクション開始: %w", err)
	}
	defer tx.Rollback()
	txq := q.WithTx(tx)

	msgID, err := txq.InsertMessage(ctx, taskstore.InsertMessageParams{
		Source:           "mattermost",
		SourceID:         sql.NullString{String: p.ID, Valid: true},
		ChannelOrMeeting: sql.NullString{String: channelID, Valid: true},
		Text:             p.Message,
		ReceivedAt:       time.UnixMilli(p.CreateAt).UTC().Format(time.RFC3339),
		ThreadID:         sql.NullString{String: p.RootID, Valid: p.RootID != ""},
		ProjectHint:      sql.NullString{String: projectHint, Valid: true},
		PermalinkUrl:     sql.NullString{String: permalink, Valid: permalink != ""},
	})
	if err != nil {
		return false, fmt.Errorf("メッセージ保存: %w", err)
	}

	autoRegister := result.Kind == "task" && isConfirmedTarget(result.Target)
	var verdict sql.NullString
	if autoRegister {
		verdict = sql.NullString{String: "auto_registered", Valid: true}
	}

	if err := txq.InsertCandidate(ctx, taskstore.InsertCandidateParams{
		MessageID:       msgID,
		Kind:            result.Kind,
		Confidence:      sql.NullFloat64{Float64: result.Confidence, Valid: true},
		Target:          sql.NullString{String: result.Target, Valid: result.Target != ""},
		AssigneeRaw:     sql.NullString{String: result.AssigneeRaw, Valid: result.AssigneeRaw != ""},
		DueDate:         sql.NullString{String: result.DueDate, Valid: result.DueDate != ""},
		Summary:         sql.NullString{String: result.Summary, Valid: result.Summary != ""},
		RelatedJiraKey:  sql.NullString{String: result.RelatedJiraKey, Valid: result.RelatedJiraKey != ""},
		HumanVerdict:    verdict,
		SuggestedTaskID: suggestedTaskID(result.RelatedTaskID, openTasks),
	}); err != nil {
		return false, fmt.Errorf("候補保存: %w", err)
	}

	if autoRegister {
		if _, err := txq.CreateTask(ctx, taskstore.CreateTaskParams{
			SourceMessageID: sql.NullInt64{Int64: msgID, Valid: true},
			Title:           result.Summary,
			Target:          result.Target,
			Status:          "todo",
			CreatedAt:       time.Now().UTC().Format(time.RFC3339),
			Tracked:         1,
			DueDate:         sql.NullString{String: result.DueDate, Valid: result.DueDate != ""},
			Priority:        "medium",
		}); err != nil {
			return false, fmt.Errorf("タスク自動登録: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("トランザクションコミット: %w", err)
	}

	if autoRegister && (result.Target == "jira_a" || result.Target == "jira_b") {
		stubJiraTransition("(未発行)", "create_via_mattermost_extractor")
	}
	return autoRegister, nil
}

// cursorForChannel はそのチャンネルの次回取得開始位置(Unixミリ秒)を返す。
// mattermost_channel_stateに記録が無ければ(初回ポーリング)直近initialLookbackWindow前を返す。
func cursorForChannel(ctx context.Context, q *taskstore.Queries, channelID string) (int64, error) {
	last, err := q.GetMattermostChannelState(ctx, channelID)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Now().Add(-initialLookbackWindow).UnixMilli(), nil
	}
	if err != nil {
		return 0, err
	}
	t, err := time.Parse(time.RFC3339, last)
	if err != nil {
		return 0, fmt.Errorf("last_processed_atの解析に失敗(%q): %w", last, err)
	}
	return t.UnixMilli(), nil
}
