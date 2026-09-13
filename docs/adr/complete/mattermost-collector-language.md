# 論点: Mattermost collectorの実装言語

- 起票: 2026-09-13 / タスク管理自動化構想の一部（`docs/design/automation-roadmap.md` 参照）
- 論点: Mattermost collector（`docs/adr/complete/mattermost-collector-scope.md`で決定した
  「収集のみ」スコープ）をGoとPythonのどちらで実装するか。
- 状態: **採用済み・実装完了（2026-09-13）**

## 背景

当時の設計（現`docs/design/mail-zoom-pipeline.md`「技術スタック」節）はcollector/extractor/
registrar全体をPython（`requests`でMattermost/JIRA/Zoom API呼び出し、`anthropic` SDKでClaude
API呼び出し）で実装する想定だった。taskmanagerリポジトリの既存実装（ダッシュボード）はGo + sqlc + htmxのみで、
Pythonの実行環境は現状このリポジトリに存在しない。

## 方針案

| 案 | 内容 | コスト | リスク | 状態 |
|---|---|---|---|---|
| 案1 | Python（既存設計通り。`requests`でMattermost REST APIを直接叩く） | 中（Python/uv環境の新規導入、既存Goコードとの連携（`messages`テーブルへの書き込み）を別言語間でどう繋ぐか設計が必要） | 単一バイナリ・軽量という既存ダッシュボードの方針から外れ、リポジトリ内に2言語のビルド・実行環境が併存することになる | 不採用 |
| 案2 | Go（Mattermost公式Goクライアント`github.com/mattermost/mattermost/server/public/model`の`Client4`を使用） | 低（`GetPostsSince`による差分ポーリングが可能。既存の`internal/taskstore`の`InsertMessage`(sqlc生成)をそのまま再利用でき、`rollover.go`と同じ「常駐goroutine+time.Ticker」パターンを踏襲できる） | 特になし | **採用** |

当初「Python採用の理由は抽出(extractor)でのAnthropic SDK利用にある」と考えていたが、
Anthropicは公式Go SDK（`github.com/anthropics/anthropic-sdk-go`、Go 1.24+対応。本リポジトリの
ビルドは`golang:1.25-alpine`のため利用可能）も提供しており、この前提は誤りだったことを
確認した（ユーザーからの指摘により発覚）。将来extractorをAI(Claude API)で実装する際も
Goで完結でき、Pythonを新規に持ち込む技術的必然性は無い。

## 採用理由 / 検討経緯

今回のスコープ（収集のみ、`mattermost-collector-scope.md`参照）ではAnthropic SDKを使わない
ため、そもそもPython優位の理由が最初から無かった。加えてAnthropic公式Go SDKの存在により、
将来extractorを追加する場合もGoで完結見込みであるため、単一言語・単一バイナリという既存
ダッシュボードの方針に合わせ**案2（Go）**を採用した。

## 関連する設計ドキュメント

- `docs/design/design.md`（技術スタック、常駐処理の方針）
- `docs/design/mail-zoom-pipeline.md`（技術スタック節。メール/Zoom分はPython想定のまま、
  Mattermost分はGo採用済み）
- `docs/design/data-model.md`（「Mattermost extractor」節、Go実装の確定仕様）
