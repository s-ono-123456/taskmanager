// Package mattermost はMattermost APIをポーリングし、ローカルLLMで取得時に分類、
// target確定分は自動でタスク登録するcollector兼extractorを実装する
// （docs/adr/complete/mattermost-collector-scope.md・mattermost-message-retention.md参照）。
//
// Mattermost公式Go SDK(github.com/mattermost/mattermost/server/public/model)は使わず、
// 標準ライブラリのnet/httpのみで最小限のRESTクライアントを実装している。外部依存を
// 追加するより、本リポジトリの既存方針（追加のフレームワーク・ライブラリを増やさない
// 軽量さ）に合うため（docs/adr/complete/mattermost-collector-language.md参照）。
package mattermost

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"
)

// Client はMattermost REST API v4への最小限のアクセスを提供する。
type Client struct {
	baseURL string
	token   string
	http    *http.Client

	// channelTeamName はチャンネルID->チーム名(URLスラッグ)のキャッシュ。
	// パーマリンク組み立て(PermalinkURL)のために解決する。常駐goroutine1つのみが
	// このClientを使うため、ロックは設けていない。
	channelTeamName map[string]string
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL:         baseURL,
		token:           token,
		http:            &http.Client{Timeout: 30 * time.Second},
		channelTeamName: make(map[string]string),
	}
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

// GetThread はpostIDが属するスレッド全文(ルート投稿+返信)を取得する。
// GET /api/v4/posts/{postID}/thread。レスポンス形状はGetPostsSinceと同じ(order+posts map)。
// 投稿がスレッドに属さない(返信が無い)場合でも、その投稿自身を含む1件のスレッドとして返る。
func (c *Client) GetThread(ctx context.Context, postID string) ([]Post, error) {
	var list postList
	if err := c.doGet(ctx, "/api/v4/posts/"+postID+"/thread", &list); err != nil {
		return nil, err
	}
	posts := make([]Post, 0, len(list.Order))
	for _, id := range list.Order {
		if p, ok := list.Posts[id]; ok {
			posts = append(posts, p)
		}
	}
	sort.Slice(posts, func(i, j int) bool { return posts[i].CreateAt < posts[j].CreateAt })
	return posts, nil
}

type channel struct {
	TeamID string `json:"team_id"`
}

type team struct {
	Name string `json:"name"`
}

// teamNameForChannel はchannelIDが属するチームの名前(URLスラッグ)を解決する。
// チャンネル->チームID(GET /api/v4/channels/{channelID})、チームID->チーム名
// (GET /api/v4/teams/{teamID})の2段階。結果はClientの生存期間中キャッシュする。
func (c *Client) teamNameForChannel(ctx context.Context, channelID string) (string, error) {
	if name, ok := c.channelTeamName[channelID]; ok {
		return name, nil
	}
	var ch channel
	if err := c.doGet(ctx, "/api/v4/channels/"+channelID, &ch); err != nil {
		return "", fmt.Errorf("get channel: %w", err)
	}
	var t team
	if err := c.doGet(ctx, "/api/v4/teams/"+ch.TeamID, &t); err != nil {
		return "", fmt.Errorf("get team: %w", err)
	}
	c.channelTeamName[channelID] = t.Name
	return t.Name, nil
}

// PermalinkURL はchannelID内のpostIDへのMattermostパーマリンクURLを組み立てる。
// {serverURL}/{teamName}/pl/{postID} の形式。
func (c *Client) PermalinkURL(ctx context.Context, channelID, postID string) (string, error) {
	teamName, err := c.teamNameForChannel(ctx, channelID)
	if err != nil {
		return "", err
	}
	return c.baseURL + "/" + teamName + "/pl/" + postID, nil
}
