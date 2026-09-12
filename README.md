# taskmanager (task-dashboard パイロット実装)

タスク管理自動化構想（JIRA2プロジェクト＋個人タスクをMattermost/メール/Zoomから
自動収集し、JIRA自動起票・完了候補提示まで行う）のうち、「スキーマとダッシュボード
UIのパイロット実装」に相当するリポジトリ。2026-09-12に`/work`（メインリポジトリ）の
`docker/task-dashboard/`からこの独立リポジトリへ切り出された。

**外部通信は一切行わない。** Mattermost/メール/Zoom/JIRA/Claude APIいずれにも
接続せず、JIRA連携相当の操作はすべて`stubJiraTransition()`によるログ出力のみ。

全体構想（本パイロットが将来どう拡張される想定か）は
`docs/adr/proposals/task-management-automation.md`を参照。

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
internal/web/                  # HTTPハンドラ・業務ロジック
  handlers.go / kanban.go / render.go
  templates/board.html.tmpl     # 唯一のテンプレート(html/template、go:embed)
static/htmx.min.js             # htmx本体(vendor同梱、CDN不使用)
static/board.js                # ボード画面のJS(board.html.tmplから分離)
docs/design.md                 # 詳細設計書（データモデル・画面仕様・ルート一覧を網羅）
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

## 関連ドキュメント

- `docs/design.md` — データモデル・画面仕様・ルート一覧・デプロイ構成・既知の制限を
  網羅した詳細設計書。実装を変更する際は必ず参照すること。
- `docs/adr/proposals/task-management-automation.md` — 全体構想のADR（データモデル・
  パイプライン全体像。まだ未実装のcollector/extractor/syncer/registrar含む）。
- `docs/work-log.md` — 完了した作業の経緯・学んだこと（Go+sqlc+htmxへの移行判断など）。
