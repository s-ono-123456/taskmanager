# taskmanager (task-dashboard パイロット実装)

タスク管理自動化構想（JIRA2プロジェクト＋個人タスクをMattermost/メール/Zoomから
自動収集し、JIRA自動起票・完了候補提示まで行う）のうち、「スキーマとダッシュボード
UIのパイロット実装」に相当するリポジトリ。2026-09-12に`/work`（メインリポジトリ）の
`docker/task-dashboard/`からこの独立リポジトリへ切り出された。

**外部通信は原則行わない。** メール/Zoom/JIRA/Claude APIいずれにも接続せず、JIRA連携
相当の操作はすべて`stubJiraTransition()`によるログ出力のみ。唯一の例外として、
Mattermost collector（`internal/mattermost/`、収集のみ。2026-09-13追加）は実際に
Mattermost APIへポーリング接続し、`messages`テーブルへ実データを保存する
（認証情報未設定なら起動しない。詳細は後述）。

主な機能: カンバンボード（4ステータス列×「今週/バックログ」2スイムレーン、
ドラッグ&ドロップ、優先度バッジ）、週次サイクルの自動繰り越し、クローズ要求一覧、
Mattermost collector（収集のみ）。

全体構想（本パイロットが将来どう拡張される想定か）は`docs/design/task-management-automation.md`
を参照。各設計判断の経緯は`docs/adr/proposals/`・`docs/adr/complete/`配下の論点ファイルを参照。

## 技術スタック

- Go標準ライブラリの`net/http`（Go 1.22+の`ServeMux`）+ `html/template`。追加の
  ルーターフレームワークは使わない。
- SQL: [sqlc](https://sqlc.dev/)で`internal/taskstore/query.sql`から型安全なGoコードを
  生成する。SQLは`.sql`ファイル、ロジックはGoファイル、という分離を最重視している。
- DBドライバ: `modernc.org/sqlite`（cgo不要）。`CGO_ENABLED=0`でビルドでき、実行イメージを
  `distroless/static-debian12`にできる（最終イメージ30MB台）。
- フロントエンド: htmx（vendor同梱）+ Tailwind CSS（CDN読み込み）。JSはドラッグ&ドロップ・
  モーダル開閉のみ素のDOM操作。

2026-09-12に、Python/Flask実装からGo + sqlc + htmxへ全面移行した。「軽量さ」と
「SQLがロジックから分離されたわかりやすさ」を重視した選択（経緯は`docs/work-log.md`参照）。

## ディレクトリ構成

```
build/Dockerfile               # マルチステージ(golang:1.25-alpine builder → distroless/static-debian12)
compose/docker-compose.yml
go.mod / go.sum
main.go                        # エントリポイント(DB初期化・マイグレーション・自動シード・サーバー起動)
internal/taskstore/            # データ層(SQLとロジックの分離を最重視)
  schema.sql                    # スキーマ定義(go:embed、起動時DDLにも使う)
  query.sql                     # sqlc用の名前付きクエリ
  sqlc.yaml
  db.go / models.go / query.sql.go  # sqlc生成コード(コミットしない。ビルド時に生成)
  store.go                      # DB接続・起動時マイグレーション
  seed.go                       # サンプルデータ投入(再実行可能・全件作り直し)
  rollover.go                   # 週次サイクルの自動繰り越し(常駐goroutine)
internal/mattermost/           # Mattermost collector(収集のみ。外部通信の唯一の例外)
  client.go                     # net/httpのみの最小限のRESTクライアント
  collector.go                  # 設定読み込み・常駐goroutineでのポーリング
internal/web/                  # HTTPハンドラ・業務ロジック
  handlers.go / kanban.go / render.go
  templates/board.html.tmpl     # 唯一のテンプレート(html/template、go:embed)
static/htmx.min.js             # htmx本体(vendor同梱、CDN不使用)
static/board.js                # ボード画面のJS(board.html.tmplから分離)
docs/design/design.md          # 全体方針（位置づけ・技術スタック・ルート一覧の索引等）
docs/design/data-model.md      # DB設計
docs/design/screen-board.md    # 画面設計: カンバンボード
docs/design/screen-close-requests.md  # 画面設計: クローズ要求一覧
```

## 実行方法

**`internal/taskstore/db.go`/`models.go`/`query.sql.go`はsqlc生成コードでコミットしない
（`.gitignore`対象）。ビルド前に必ず`sqlc generate`を実行すること。**

- sqlcによるコード生成（この環境にGo/sqlcがローカルインストールされていない場合、
  `docker run`経由で行う方針）:
  ```bash
  docker run --rm --user "$(id -u):$(id -g)" -v "$(pwd)/internal/taskstore:/src" -w /src \
    sqlc/sqlc generate
  ```
- ローカルビルド（同じくDocker経由）:
  ```bash
  docker run --rm --user "$(id -u):$(id -g)" -e HOME=/tmp -v "$(pwd):/src" -w /src \
    golang:1.25-alpine go build -o /src/.build/task-dashboard .
  ```
  - `--user`を付けないと生成物がroot所有になるので必ず付けること。
  - 実行時は環境変数`TASK_DASHBOARD_HOST`（既定127.0.0.1）/`TASK_DASHBOARD_PORT`
    （既定5000）/`TASK_DASHBOARD_DB_PATH`/`TASK_DASHBOARD_AUTO_SEED`
    （`1`でDBが空の時のみ自動シード）で上書き可能。
- Docker実行: `docker compose -f compose/docker-compose.yml up -d --build`
  - `build/Dockerfile`内で`sqlc generate`を自動実行してからビルドするため、事前の
    手動生成は不要。
  - ポート8090、実データ(SQLite)は`/docker/task-dashboard/data`
    （このリポジトリの外・gitの管理対象外）にボリュームマウントされる。
- Mattermost collector（任意）: `MATTERMOST_BOT_TOKEN`/`MATTERMOST_SERVER_URL`/
  `MATTERMOST_CHANNEL_ROUTES`（例: `channelID1:jira_a,channelID2:jira_b`）を設定すると
  実際にMattermost APIをポーリングして`messages`テーブルへ保存する（10分間隔）。
  未設定ならcollectorは起動しない。値はリポジトリに書かず、ホストのシェル環境変数か
  `compose/.env`（`.gitignore`対象）で渡す（`docker-compose.yml`は`${VAR:-}`で展開する）。

## 関連ドキュメント

- `docs/design/design.md` — 全体方針（位置づけ・技術スタック・ルート一覧の索引・デプロイ構成・
  既知の制限）。実装を変更する際は必ず参照すること。
- `docs/design/data-model.md` — DB設計（テーブル定義・マイグレーション）。
- `docs/design/screen-board.md` — 画面設計: カンバンボード画面。
- `docs/design/screen-close-requests.md` — 画面設計: クローズ要求一覧画面。
- `docs/adr/proposals/`・`docs/adr/complete/` — 全体構想のADR。意思決定の経緯（案の比較・
  採用理由）を論点ごとのファイルに分けて記録している（実装まで完了したものは`complete/`）。
- `docs/design/task-management-automation.md` — 全体構想のうち、まだ未実装の
  extractor/syncer/registrar/digest（およびメール/Zoom collector）を含む確定設計
  （データモデル・パイプライン全体像。Mattermost collectorは実装済み）。
- `docs/work-log.md` — 完了した作業の経緯・学んだこと（Go+sqlc+htmxへの移行判断など）。
