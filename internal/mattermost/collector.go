package mattermost

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"taskmanager/internal/taskstore"
)

// Config はcollectorの設定(環境変数から読み込む)。
type Config struct {
	BotToken      string
	ServerURL     string
	ChannelRoutes map[string]string // channelID -> project_hint(jira_a/jira_b/personal)
}

// LoadConfigFromEnv はMATTERMOST_BOT_TOKEN / MATTERMOST_SERVER_URL /
// MATTERMOST_CHANNEL_ROUTES(例 "chID1:jira_a,chID2:jira_b,chID3:personal")を読み込む。
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
	return Config{BotToken: token, ServerURL: serverURL, ChannelRoutes: routes}, true, nil
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
// (docs/adr/proposals/collection-trigger.md「収集トリガー方式」で採用したcron定期ポーリング、
// task-management-automation.mdの既存設計と同じ10分間隔)。
const PollInterval = 10 * time.Minute

// initialLookbackWindow: あるチャンネルの投稿が1件もmessagesテーブルに無い(初回ポーリング)場合、
// 直近この期間分から取得を開始する(全履歴を一度に取得しないため)。
const initialLookbackWindow = 24 * time.Hour

// StartCollectorLoop は常駐goroutineを起動する。起動直後に1回同期実行し、その後
// PollInterval間隔で繰り返す(internal/taskstore/rollover.goのStartRolloverLoopと同じ構造)。
func StartCollectorLoop(ctx context.Context, db *sql.DB, cfg Config) {
	client := NewClient(cfg.ServerURL, cfg.BotToken)
	q := taskstore.New(db)

	runOnce(ctx, client, q, cfg)

	ticker := time.NewTicker(PollInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runOnce(ctx, client, q, cfg)
			}
		}
	}()
}

// runOnce は設定済みチャンネルごとに、前回のcursor(そのチャンネルの最新received_at、
// 無ければ直近24時間前)以降の投稿を取得しmessagesテーブルへ保存する。
// チャンネル単位でエラーが起きても他のチャンネルの処理は継続する。
func runOnce(ctx context.Context, client *Client, q *taskstore.Queries, cfg Config) {
	for channelID, projectHint := range cfg.ChannelRoutes {
		if err := collectChannel(ctx, client, q, channelID, projectHint); err != nil {
			log.Printf("[mattermost-collector] channel=%s エラー: %v", channelID, err)
		}
	}
}

func collectChannel(ctx context.Context, client *Client, q *taskstore.Queries, channelID, projectHint string) error {
	sinceMS, err := cursorForChannel(ctx, q, channelID)
	if err != nil {
		return fmt.Errorf("cursor取得: %w", err)
	}

	posts, err := client.GetPostsSince(ctx, channelID, sinceMS)
	if err != nil {
		return fmt.Errorf("投稿取得: %w", err)
	}

	usernameCache := make(map[string]string)
	saved := 0
	for _, p := range posts {
		if p.Message == "" {
			continue // システムメッセージ等、本文が無い投稿はスキップ。
		}
		exists, err := q.MessageExistsBySourceID(ctx, taskstore.MessageExistsBySourceIDParams{
			Source:   "mattermost",
			SourceID: sql.NullString{String: p.ID, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("重複確認(post=%s): %w", p.ID, err)
		}
		if exists {
			continue
		}

		author, ok := usernameCache[p.UserID]
		if !ok {
			author, err = client.GetUsername(ctx, p.UserID)
			if err != nil {
				log.Printf("[mattermost-collector] user=%s の名前解決に失敗（未設定のまま保存）: %v", p.UserID, err)
				author = ""
			}
			usernameCache[p.UserID] = author
		}

		if _, err := q.InsertMessage(ctx, taskstore.InsertMessageParams{
			Source:           "mattermost",
			SourceID:         sql.NullString{String: p.ID, Valid: true},
			ChannelOrMeeting: sql.NullString{String: channelID, Valid: true},
			Author:           sql.NullString{String: author, Valid: author != ""},
			Text:             p.Message,
			ReceivedAt:       time.UnixMilli(p.CreateAt).UTC().Format(time.RFC3339),
			ThreadID:         sql.NullString{String: p.RootID, Valid: p.RootID != ""},
			ProjectHint:      sql.NullString{String: projectHint, Valid: true},
		}); err != nil {
			return fmt.Errorf("保存(post=%s): %w", p.ID, err)
		}
		saved++
	}
	if saved > 0 {
		log.Printf("[mattermost-collector] channel=%s %d件のメッセージを保存しました", channelID, saved)
	}
	return nil
}

// cursorForChannel はそのチャンネルの次回取得開始位置(Unixミリ秒)を返す。
// messagesテーブルに既存データがあればその最新received_atを、無ければ
// (初回ポーリング)直近initialLookbackWindow前を返す。
func cursorForChannel(ctx context.Context, q *taskstore.Queries, channelID string) (int64, error) {
	last, err := q.LastMessageReceivedAt(ctx, sql.NullString{String: channelID, Valid: true})
	if err != nil {
		return 0, err
	}
	if last == "" {
		return time.Now().Add(-initialLookbackWindow).UnixMilli(), nil
	}
	t, err := time.Parse(time.RFC3339, last)
	if err != nil {
		return 0, fmt.Errorf("received_atの解析に失敗(%q): %w", last, err)
	}
	return t.UnixMilli(), nil
}
