package taskstore

import (
	"context"
	"database/sql"
	"fmt"
)

type seedMessage struct {
	source           string
	sourceID         string
	channelOrMeeting string
	author           string
	text             string
	receivedAt       string
	threadID         string
	projectHint      string
}

type seedCandidate struct {
	msgIdx         int
	kind           string
	confidence     float64
	target         string
	assigneeRaw    string
	jiraAccountID  string
	dueDate        string
	summary        string
	relatedJiraKey string
	humanVerdict   string
}

type seedTask struct {
	msgIdx         int // -1ならNULL
	title          string
	description    string
	target         string
	status         string
	jiraKey        string
	createdAt      string
	closedAt       string
	lastSyncedAt   string
	tracked        int64
	dueDate        string
	cycleStartDate string // ""ならバックログ、値ありなら所属週の月曜日(YYYY-MM-DD)
}

type seedUserMap struct {
	source        string
	sourceUserID  string
	jiraAccountID string
	displayName   string
}

var seedMessages = []seedMessage{
	{"mattermost", "post-1001", "#project-a", "tanaka",
		"来週までにAPI仕様書をまとめてもらえますか？",
		"2026-09-08T10:00:00", "", "jira_a"},
	{"mattermost", "post-1002", "#project-b", "suzuki",
		"ログイン画面のエラーのバグ修正の件、対応完了しました",
		"2026-09-10T15:30:00", "", "jira_b"},
	{"email", "msg-2001", "inbox/personal", "yamada@example.com",
		"資料のレビューをお願いします（個人依頼）",
		"2026-09-09T09:15:00", "", "personal"},
	{"zoom", "meeting-3001", "定例会議 2026-09-10", "(meeting summary)",
		"Next steps: ログ基盤の調査を進める / UIレビューを来週までに完了する",
		"2026-09-10T18:00:00", "", "jira_a"},
	{"mattermost", "post-1003", "#project-a", "tanaka",
		"API仕様書のドラフトを完成させました。ご確認お願いします",
		"2026-09-12T14:00:00", "", "jira_a"},
	{"email", "msg-2002", "inbox/personal", "yamada@example.com",
		"経費精算の提出、終わりました",
		"2026-09-12T16:00:00", "", "personal"},
}

var seedCandidates = []seedCandidate{
	{0, "task", 0.9, "jira_a", "tanaka", "tanaka.k", "2026-09-19",
		"API仕様書をまとめる", "", "correct"},
	{1, "completion", 0.85, "jira_b", "suzuki", "suzuki.m", "",
		"バグ修正完了報告", "PROJB-42", "correct"},
	{2, "task", 0.7, "personal", "yamada@example.com", "", "2026-09-15",
		"資料レビュー", "", "correct"},
	{3, "task", 0.6, "jira_a", "", "", "",
		"ログ基盤の調査", "", ""},
	{3, "task", 0.4, "unknown", "", "", "",
		"UIレビューを来週までに完了する", "", ""},
	{4, "completion", 0.8, "jira_a", "tanaka", "tanaka.k", "",
		"API仕様書ドラフト完成の報告", "PROJA-101", ""},
	{5, "completion", 0.65, "personal", "yamada@example.com", "", "",
		"経費精算完了の報告", "", ""},
}

var seedTasks = []seedTask{
	{0, "API仕様書をまとめる", "元発言: post-1001（#project-a）", "jira_a", "todo",
		"PROJA-101", "2026-09-08T10:05:00", "", "2026-09-12T09:00:00", 1, "2026-09-10", "2026-09-07"},
	{1, "バグ修正: ログイン画面のエラー", "元発言: post-1002（#project-b）", "jira_b", "done",
		"PROJB-42", "2026-09-05T11:00:00", "2026-09-10T15:35:00", "2026-09-12T09:00:00", 1, "", ""},
	{2, "資料レビュー", "依頼元: yamada@example.com", "personal", "in_progress",
		"", "2026-09-09T09:20:00", "", "", 1, "2026-09-20", "2026-09-07"},
	{-1, "旧: サーバー証明書更新", "過去に完了・追跡除外済みの例", "jira_a", "done",
		"PROJA-88", "2026-08-01T09:00:00", "2026-08-20T17:00:00", "2026-08-21T09:00:00", 0, "", ""},
	{-1, "個人: 経費精算", "個人タスクの例", "personal", "todo",
		"", "2026-09-11T08:00:00", "", "", 1, "", ""},
	{-1, "個人: 昔のメモ整理", "完了済み・追跡除外の個人タスクの例", "personal", "done",
		"", "2026-07-01T09:00:00", "2026-07-05T09:00:00", "", 0, "", ""},
}

var seedUserMaps = []seedUserMap{
	{"mattermost", "tanaka", "tanaka.k", "田中"},
	{"mattermost", "suzuki", "suzuki.m", "鈴木"},
	{"email", "yamada@example.com", "", "山田"},
}

func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullFloat(f float64) sql.NullFloat64 {
	return sql.NullFloat64{Float64: f, Valid: true}
}

func nullInt(i int64, valid bool) sql.NullInt64 {
	return sql.NullInt64{Int64: i, Valid: valid}
}

// Seed はサンプルデータを投入する(seed.pyのseed()相当)。
// 外部通信は一切行わない。実行するたびに既存データを消して作り直す(冪等)。
func Seed(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	q := New(tx)

	if err := q.DeleteAllCandidates(ctx); err != nil {
		return fmt.Errorf("delete candidates: %w", err)
	}
	if err := q.DeleteAllTasks(ctx); err != nil {
		return fmt.Errorf("delete tasks: %w", err)
	}
	if err := q.DeleteAllMessages(ctx); err != nil {
		return fmt.Errorf("delete messages: %w", err)
	}
	if err := q.DeleteAllUserMap(ctx); err != nil {
		return fmt.Errorf("delete user_map: %w", err)
	}

	messageIDs := make([]int64, len(seedMessages))
	for i, m := range seedMessages {
		id, err := q.InsertMessage(ctx, InsertMessageParams{
			Source:           m.source,
			SourceID:         nullStr(m.sourceID),
			ChannelOrMeeting: nullStr(m.channelOrMeeting),
			Author:           nullStr(m.author),
			Text:             m.text,
			ReceivedAt:       m.receivedAt,
			ThreadID:         nullStr(m.threadID),
			ProjectHint:      nullStr(m.projectHint),
		})
		if err != nil {
			return fmt.Errorf("insert message %d: %w", i, err)
		}
		messageIDs[i] = id
	}

	for i, c := range seedCandidates {
		if err := q.InsertCandidate(ctx, InsertCandidateParams{
			MessageID:      messageIDs[c.msgIdx],
			Kind:           c.kind,
			Confidence:     nullFloat(c.confidence),
			Target:         nullStr(c.target),
			AssigneeRaw:    nullStr(c.assigneeRaw),
			JiraAccountID:  nullStr(c.jiraAccountID),
			DueDate:        nullStr(c.dueDate),
			Summary:        nullStr(c.summary),
			RelatedJiraKey: nullStr(c.relatedJiraKey),
			HumanVerdict:   nullStr(c.humanVerdict),
		}); err != nil {
			return fmt.Errorf("insert candidate %d: %w", i, err)
		}
	}

	for i, t := range seedTasks {
		var sourceMessageID sql.NullInt64
		if t.msgIdx >= 0 {
			sourceMessageID = nullInt(messageIDs[t.msgIdx], true)
		}
		if _, err := q.CreateTask(ctx, CreateTaskParams{
			SourceMessageID: sourceMessageID,
			Title:           t.title,
			Description:     nullStr(t.description),
			Target:          t.target,
			Status:          t.status,
			JiraKey:         nullStr(t.jiraKey),
			CreatedAt:       t.createdAt,
			ClosedAt:        nullStr(t.closedAt),
			LastSyncedAt:    nullStr(t.lastSyncedAt),
			Tracked:         t.tracked,
			DueDate:         nullStr(t.dueDate),
			CycleStartDate:  nullStr(t.cycleStartDate),
		}); err != nil {
			return fmt.Errorf("insert task %d: %w", i, err)
		}
	}

	for i, u := range seedUserMaps {
		if err := q.InsertUserMap(ctx, InsertUserMapParams{
			Source:        u.source,
			SourceUserID:  u.sourceUserID,
			JiraAccountID: nullStr(u.jiraAccountID),
			DisplayName:   nullStr(u.displayName),
		}); err != nil {
			return fmt.Errorf("insert user_map %d: %w", i, err)
		}
	}

	return tx.Commit()
}
