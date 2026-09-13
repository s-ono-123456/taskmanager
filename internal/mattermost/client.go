// Package mattermost はMattermost APIをポーリングしてmessagesテーブルへ保存する
// collector（収集のみ、抽出・JIRA自動起票は対象外）を実装する。
//
// Mattermost公式Go SDK(github.com/mattermost/mattermost/server/public/model)は使わず、
// 標準ライブラリのnet/httpのみで最小限のRESTクライアントを実装している。今回必要なAPIは
// 「チャンネルの投稿をsince指定で取得」「ユーザー名解決」の2つのみで、外部依存を追加する
// より、本リポジトリの既存方針（追加のフレームワーク・ライブラリを増やさない軽量さ）に合う
// ため（docs/adr/proposals/mattermost-collector-language.md参照）。
package mattermost

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// Client はMattermost REST API v4への最小限のアクセスを提供する。
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{baseURL: baseURL, token: token, http: &http.Client{}}
}

// Post はMattermost APIのPostオブジェクトのうち、collectorが使うフィールドのみ。
type Post struct {
	ID        string `json:"id"`
	CreateAt  int64  `json:"create_at"` // Unixミリ秒
	UserID    string `json:"user_id"`
	ChannelID string `json:"channel_id"`
	RootID    string `json:"root_id"` // スレッド返信の場合、親投稿のID
	Message   string `json:"message"`
}

type postList struct {
	Order []string        `json:"order"`
	Posts map[string]Post `json:"posts"`
}

func (c *Client) doGet(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("mattermost API %s returned status %d", path, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// GetPostsSince はchannelID内の、sinceMS(Unixミリ秒)より後に作成された投稿を古い順で返す。
// GET /api/v4/channels/{channelID}/posts?since=<sinceMS>
func (c *Client) GetPostsSince(ctx context.Context, channelID string, sinceMS int64) ([]Post, error) {
	path := "/api/v4/channels/" + channelID + "/posts?since=" + strconv.FormatInt(sinceMS, 10)
	var list postList
	if err := c.doGet(ctx, path, &list); err != nil {
		return nil, err
	}
	// list.Order は新しい順。古い順(投稿された順)に並べ替えて返す。
	posts := make([]Post, 0, len(list.Order))
	for i := len(list.Order) - 1; i >= 0; i-- {
		if p, ok := list.Posts[list.Order[i]]; ok {
			posts = append(posts, p)
		}
	}
	return posts, nil
}

type user struct {
	Username string `json:"username"`
}

// GetUsername はuserIDに対応するMattermostユーザー名を取得する。
// GET /api/v4/users/{userID}
func (c *Client) GetUsername(ctx context.Context, userID string) (string, error) {
	var u user
	if err := c.doGet(ctx, "/api/v4/users/"+userID, &u); err != nil {
		return "", err
	}
	return u.Username, nil
}
