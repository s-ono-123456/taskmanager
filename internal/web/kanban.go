// Package web はタスク管理ダッシュボードのHTTPハンドラ・業務ロジックを持つ。
package web

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"taskmanager/internal/taskstore"
)

var Targets = []string{"jira_a", "jira_b", "personal"}

var Statuses = []string{"todo", "in_progress", "reviewing", "done"}

var StatusLabels = map[string]string{
	"todo":        "未着手",
	"in_progress": "進行中",
	"reviewing":   "確認中",
	"done":        "完了",
}

// DoneLaneWindowDays: 完了レーンに表示するのは直近この日数以内に完了したものだけ。
const DoneLaneWindowDays = 7

// Lanes: D&Dのスイムレーン行。"this_week"=今週 / "backlog"=バックログ。
// Cycleは独立テーブルを持たず、tasks.cycle_start_dateの有無のみで表現する
// (docs/adr/proposals/cycle-data-model.md参照)。
var Lanes = []string{"this_week", "backlog"}

var LaneLabels = map[string]string{
	"this_week": "今週",
	"backlog":   "バックログ",
}

// Priorities: 優先度4段階（並び順は高い順）。全target共通、カード上のバッジ表示のみに使い
// レーン構造・並び順には影響しない（docs/adr/proposals/task-priority-field.md参照）。
var Priorities = []string{"highest", "high", "medium", "low"}

var PriorityLabels = map[string]string{
	"highest": "最高",
	"high":    "高",
	"medium":  "中",
	"low":     "低",
}

func isValidTarget(t string) bool {
	for _, v := range Targets {
		if v == t {
			return true
		}
	}
	return false
}

func isValidStatus(s string) bool {
	for _, v := range Statuses {
		if v == s {
			return true
		}
	}
	return false
}

func isValidCycle(c string) bool {
	return c == "this_week" || c == "backlog"
}

func isValidPriority(p string) bool {
	for _, v := range Priorities {
		if v == p {
			return true
		}
	}
	return false
}

// cycleStartDateForLane はD&Dで指定されたレーン名から、保存すべきcycle_start_date
// (週の月曜日、バックログならNULL)を計算する(closedAtForTransitionと同様、業務ルールを
// この層に集約する)。
func cycleStartDateForLane(lane string) sql.NullString {
	if lane != "this_week" {
		return sql.NullString{}
	}
	return sql.NullString{String: taskstore.CurrentWeekMonday(time.Now()), Valid: true}
}

// laneForCard はカードが所属するスイムレーンを返す。判定はcycle_start_dateの有無のみ
// (NULL=backlog/値あり=this_week)。done化したタスクのcycle_start_dateはロールオーバー
// 対象外で据え置かれるため、doneタスクも元のスイムレーンに残り続ける。
func laneForCard(c Card) string {
	if c.CycleStartDate == "" {
		return "backlog"
	}
	return "this_week"
}

// nowISO は現在時刻をUTCのRFC3339文字列で返す(新規書き込み用)。
func nowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// parseFlexibleTime はRFC3339("...Z"/"+09:00"等)と、タイムゾーン情報を持たない
// "2026-09-08T10:00:00"形式(seed.pyの固定サンプルに存在する)の両方を解釈する。
// naive(タイムゾーンなし)な場合はUTCとみなす(Python版のis_recently_closedと同じ扱い)。
func parseFlexibleTime(s string) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), true
	}
	if t, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
		return t.UTC(), true
	}
	return time.Time{}, false
}

// isRecentlyClosed はclosedAtがDoneLaneWindowDays日以内かどうか(完了レーンの表示絞り込み用)。
func isRecentlyClosed(closedAt sql.NullString, now time.Time) bool {
	if !closedAt.Valid || closedAt.String == "" {
		return false
	}
	t, ok := parseFlexibleTime(closedAt.String)
	if !ok {
		return false
	}
	return !t.Before(now.AddDate(0, 0, -DoneLaneWindowDays))
}

// isOverdue はdue_dateが今日より前かどうかを文字列比較(YYYY-MM-DDの辞書順=日付順)で判定する。
func isOverdue(dueDate, today, status string) bool {
	if dueDate == "" || status == "done" {
		return false
	}
	return dueDate < today
}

// closedAtForTransition はstatus変化に応じてclosed_atをどう設定/クリアするかを決める
// (edit・moveの両ハンドラで共通のロジック)。
func closedAtForTransition(oldStatus, newStatus string, oldClosedAt sql.NullString) sql.NullString {
	if newStatus == "done" && oldStatus != "done" {
		return sql.NullString{String: nowISO(), Valid: true}
	}
	if newStatus != "done" {
		return sql.NullString{}
	}
	return oldClosedAt
}

// stubJiraTransition は本来ここでJIRA REST APIを呼ぶ想定のスタブ。外部通信は行わない。
func stubJiraTransition(jiraKey, action string) {
	fmt.Printf("[stub] JIRA API呼び出し想定: jira_key=%s action=%s（実際の通信なし）\n", jiraKey, action)
}

func nullStrIfNotEmpty(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// Card はテンプレート・ハンドラで扱いやすいよう、sql.NullStringを素の文字列に変換した
// カード1件分のビューモデル。
type Card struct {
	ID             int64
	Title          string
	Description    string
	Target         string
	Status         string
	JiraKey        string
	Tracked        bool
	CreatedAt      string
	ClosedAt       string
	LastSyncedAt   string
	DueDate        string
	CycleStartDate string // ""ならバックログ、値ありなら所属週の月曜日(YYYY-MM-DD)
	Priority       string // highest/high/medium/low
	MsgSource      string
	MsgChannel     string
	MsgAuthor      string
	MsgText        string
	MsgReceivedAt  string
}

func cardFromRow(row taskstore.ListTasksRow) Card {
	return Card{
		ID:             row.ID,
		Title:          row.Title,
		Description:    row.Description.String,
		Target:         row.Target,
		Status:         row.Status,
		JiraKey:        row.JiraKey.String,
		Tracked:        row.Tracked != 0,
		CreatedAt:      row.CreatedAt,
		ClosedAt:       row.ClosedAt.String,
		LastSyncedAt:   row.LastSyncedAt.String,
		DueDate:        row.DueDate.String,
		CycleStartDate: row.CycleStartDate.String,
		Priority:       row.Priority,
		MsgSource:      row.MsgSource.String,
		MsgChannel:     row.MsgChannel.String,
		MsgAuthor:      row.MsgAuthor.String,
		MsgText:        row.MsgText.String,
		MsgReceivedAt:  row.MsgReceivedAt.String,
	}
}

// OpenTaskOption はクローズ要求一覧の<select>に表示する、対象候補となるタスク1件分。
type OpenTaskOption struct {
	ID    int64
	Title string
}

// CloseRequest はクローズ要求一覧に表示する完了報告候補(candidates.kind=completion)
// 1件分のビューモデル。
type CloseRequest struct {
	ID             int64
	Summary        string
	Target         string
	Confidence     float64
	RelatedJiraKey string
	MsgSource      string
	MsgChannel     string
	MsgAuthor      string
	MsgText        string
	MsgReceivedAt  string
	// OpenTasks はRelatedJiraKeyが空(対象タスクを一意に特定できない)の場合のみ、
	// 選択肢として使う候補タスク一覧。
	OpenTasks []OpenTaskOption
}

func closeRequestFromRow(row taskstore.ListPendingCompletionCandidatesRow) CloseRequest {
	return CloseRequest{
		ID:             row.ID,
		Summary:        row.Summary.String,
		Target:         row.Target.String,
		Confidence:     row.Confidence.Float64,
		RelatedJiraKey: row.RelatedJiraKey.String,
		MsgSource:      row.MsgSource.String,
		MsgChannel:     row.MsgChannel.String,
		MsgAuthor:      row.MsgAuthor.String,
		MsgText:        row.MsgText.String,
		MsgReceivedAt:  row.MsgReceivedAt.String,
	}
}

// LoadCloseRequests は承認待ちの完了報告候補(candidates.kind=completion、
// human_verdict未設定)を取得する。related_jira_keyが無い候補には、対象を絞り込むための
// 未クローズタスク一覧(ListOpenTasksByTarget)を付加する(論点C3: 人間がダッシュボード上で
// 対象タスクを選ぶ)。
func LoadCloseRequests(ctx context.Context, q *taskstore.Queries) ([]CloseRequest, error) {
	rows, err := q.ListPendingCompletionCandidates(ctx)
	if err != nil {
		return nil, fmt.Errorf("list pending completion candidates: %w", err)
	}

	openTasksByTarget := make(map[string][]OpenTaskOption)
	requests := make([]CloseRequest, 0, len(rows))
	for _, row := range rows {
		cr := closeRequestFromRow(row)
		if cr.RelatedJiraKey == "" && cr.Target != "" {
			opts, ok := openTasksByTarget[cr.Target]
			if !ok {
				taskRows, err := q.ListOpenTasksByTarget(ctx, cr.Target)
				if err != nil {
					return nil, fmt.Errorf("list open tasks by target: %w", err)
				}
				opts = make([]OpenTaskOption, len(taskRows))
				for i, t := range taskRows {
					opts[i] = OpenTaskOption{ID: t.ID, Title: t.Title}
				}
				openTasksByTarget[cr.Target] = opts
			}
			cr.OpenTasks = opts
		}
		requests = append(requests, cr)
	}
	return requests, nil
}

// closeTask はタスクをdoneにし、JIRA連携タスクならstubJiraTransitionを呼ぶ
// (クローズ要求承認時の共通処理。edit/moveハンドラと同じclosedAtForTransitionを再利用する)。
func closeTask(ctx context.Context, q *taskstore.Queries, task taskstore.Task) error {
	closedAt := closedAtForTransition(task.Status, "done", task.ClosedAt)
	if err := q.UpdateTaskStatus(ctx, taskstore.UpdateTaskStatusParams{
		Status:   "done",
		ClosedAt: closedAt,
		ID:       task.ID,
	}); err != nil {
		return fmt.Errorf("update task status: %w", err)
	}
	if task.JiraKey.Valid {
		stubJiraTransition(task.JiraKey.String, "close_via_completion_candidate")
	}
	return nil
}

// BoardFilter はGET /・各POST操作で共有するフィルタ状態。
type BoardFilter struct {
	Target        string
	ShowUntracked bool
}

// Toast は操作結果として画面に表示する通知1件分(Flask版のflashメッセージ相当)。
type Toast struct {
	Category string // "success" or "error"
	Message  string
}

// BoardData はボード全体(フルページ・フラグメント両方)の描画に必要な情報。
type BoardData struct {
	Columns        map[string]map[string][]Card // lane -> status -> cards
	Lanes          []string
	LaneLabels     map[string]string
	Statuses       []string
	StatusLabels   map[string]string
	Priorities     []string
	PriorityLabels map[string]string
	Targets        []string
	SelectedTarget string
	ShowUntracked  bool
	DoneWindowDays int
	Today          string
	Toast          *Toast
	CloseRequests  []CloseRequest
}

// LoadBoardData はGET /のフィルタ取得・グルーピング・7日フィルタ適用ロジックを、
// POST操作後の再描画とも共通化するためにまとめた関数。
func LoadBoardData(ctx context.Context, q *taskstore.Queries, filter BoardFilter) (BoardData, error) {
	var targetParam any
	target := filter.Target
	if isValidTarget(target) {
		targetParam = target
	} else {
		targetParam = nil
		target = ""
	}

	rows, err := q.ListTasks(ctx, taskstore.ListTasksParams{
		Target:        targetParam,
		ShowUntracked: filter.ShowUntracked,
	})
	if err != nil {
		return BoardData{}, fmt.Errorf("list tasks: %w", err)
	}

	columns := make(map[string]map[string][]Card, len(Lanes))
	for _, lane := range Lanes {
		columns[lane] = make(map[string][]Card, len(Statuses))
		for _, s := range Statuses {
			columns[lane][s] = []Card{}
		}
	}

	now := time.Now().UTC()
	for _, row := range rows {
		card := cardFromRow(row)
		if card.Status == "done" && !isRecentlyClosed(row.ClosedAt, now) {
			// 完了レーンが際限なく膨らまないよう、直近7日以内に完了したものだけ表示する。
			continue
		}
		lane := laneForCard(card)
		columns[lane][card.Status] = append(columns[lane][card.Status], card)
	}

	closeRequests, err := LoadCloseRequests(ctx, q)
	if err != nil {
		return BoardData{}, err
	}

	return BoardData{
		Columns:        columns,
		Lanes:          Lanes,
		LaneLabels:     LaneLabels,
		Statuses:       Statuses,
		StatusLabels:   StatusLabels,
		Priorities:     Priorities,
		PriorityLabels: PriorityLabels,
		Targets:        Targets,
		SelectedTarget: target,
		ShowUntracked:  filter.ShowUntracked,
		DoneWindowDays: DoneLaneWindowDays,
		Today:          now.Format("2006-01-02"),
		CloseRequests:  closeRequests,
	}, nil
}
