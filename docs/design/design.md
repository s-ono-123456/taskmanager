# task-dashboard 設計書（パイロット実装・全体方針）

> 本書は**全体方針**（位置づけ・技術スタック・デプロイ構成・ルート一覧の索引・既知の制限）
> を扱う。**DB設計**（テーブル定義）は`docs/design/data-model.md`、**画面設計**は
> `docs/design/screen-board.md`（カンバンボード画面）・
> `docs/design/screen-close-requests.md`（クローズ要求一覧画面）を参照。

## 位置づけ

本ドキュメントはこのリポジトリ（`taskmanager`）の**現在の実装**を仕様としてまとめたものである。
2026-09-12に `/work`（メインリポジトリ）の `docker/task-dashboard/` から独立した別リポジトリ
`/work/public/taskmanager` へ移動し、全体構想のADRもこのリポジトリへ移動した
（`docs/adr/proposals/`・`docs/adr/complete/`配下、後述の「関連ドキュメント」参照）。
本サービスはそのうち「スキーマとダッシュボードUIのパイロット実装」に相当し、**外部通信は
原則行わない**（メール/Zoom/JIRA/Claude APIいずれにも接続しない。JIRA連携相当の操作はすべて
ログ出力のみのスタブ）。2026-09-13、唯一の例外としてMattermost collector（収集のみ、
`internal/mattermost/`）を追加した。実際にMattermost APIをポーリングして`messages`テーブルへ
実データを保存するが、抽出(extractor)・JIRA自動起票(registrar)・digestは対象外で、これらは
引き続き未実装（比較検討の経緯は`docs/adr/complete/mattermost-collector-scope.md`参照）。
まだ実装していない`extractor`/`syncer`/`registrar`/`digest`部分の確定設計は
`docs/design/task-management-automation.md`にまとめている（本書はダッシュボードUI側のみを
扱い、重複させない）。

**2026-09-12、Python/Flask実装からGo + sqlc + htmxへ全面移行した。** 「軽量さ」（単一バイナリ
配布・依存の少なさ）と「ソースコードのわかりやすさ」（特にSQLがロジックから分離されている
こと）を重視した結果の選択（経緯は`docs/work-log.md`参照）。業務ルール・画面仕様・挙動は
移行前のFlask版から原則変更していない（唯一の意図的な変更は後述の「編集モーダルの
バリデーションエラー時にモーダルを開いたままにする」UX改善）。

## 全体構成

```
(このリポジトリのルート)
├── build/Dockerfile            # マルチステージ(golang:1.25-alpine builder → distroless/static-debian12)
├── compose/docker-compose.yml
├── go.mod / go.sum
├── main.go                     # エントリポイント(DB初期化・マイグレーション・自動シード・HTTPサーバー起動)
├── internal/taskstore/         # データ層(SQLとロジックの分離を最重視)
│   ├── schema.sql               # スキーマ定義(go:embedで起動時DDLにも使う)
│   ├── query.sql                # sqlc用の名前付きクエリ
│   ├── sqlc.yaml
│   ├── (db.go / models.go / query.sql.go: sqlc生成コード。コミットせず、ビルド時に生成)
│   ├── store.go                 # DB接続・起動時マイグレーション
│   ├── seed.go                  # サンプルデータ投入(再実行可能・全件作り直し)
│   └── rollover.go              # 週次サイクルの自動繰り越し(常駐goroutine)
├── internal/mattermost/         # Mattermost collector(収集のみ、外部通信の唯一の例外)
│   ├── client.go                 # net/httpのみの最小限のRESTクライアント
│   └── collector.go              # 設定読み込み・常駐goroutineでのポーリング
├── internal/web/                # HTTPハンドラ・業務ロジック
│   ├── handlers.go               # 7ルートのハンドラ
│   ├── kanban.go                 # 業務ロジック集約(closed_at計算・7日フィルタ・ラベル等)
│   ├── render.go                 # テンプレートレンダリング
│   └── templates/board.html.tmpl # 唯一のテンプレート(html/template、go:embed)
├── static/
│   ├── htmx.min.js              # htmx本体(vendor同梱。外部通信ゼロの原則に合わせCDN不使用)
│   └── board.js                 # ボード画面のJS(board.html.tmplから分離)
└── docs/design/
    ├── design.md                 # 本書(全体方針)
    ├── data-model.md             # DB設計
    ├── screen-board.md           # 画面設計: カンバンボード
    └── screen-close-requests.md  # 画面設計: クローズ要求一覧
```

技術スタック:
- 言語: Go（標準ライブラリの`net/http`（Go 1.22+の`ServeMux`、パスパラメータ対応）+
  `html/template`）。追加のルーターフレームワークは使わない。
- SQL: [sqlc](https://sqlc.dev/)。`internal/taskstore/query.sql`にクエリを書き、
  型安全なGoコード（`query.sql.go`等）を生成する。SQLとロジックをファイルで完全に分離する
  ことを最重視している。
- DBドライバ: `modernc.org/sqlite`（cgo不要の純Go実装）。`CGO_ENABLED=0`でビルドできるため、
  実行イメージを`distroless/static-debian12`にでき、最終イメージは30MB台まで軽量化できる
  （旧Python版は`python:3.13-slim`ベースで150〜200MB程度）。SQLiteは複数コネクションからの
  同時書き込みに弱いため、`internal/taskstore/store.go`の`OpenDB()`で
  `db.SetMaxOpenConns(1)`により最大コネクション数を1に制限している（Python版の単一
  コネクション運用と同等の安全性を保つため）。
- フロントエンド: [htmx](https://htmx.org/)（vendor同梱、`static/htmx.min.js`）+ Tailwind CSS
  （引き続きCDN読み込み）。JSはドラッグ&ドロップ・モーダル開閉など最小限のみ素のDOM操作で
  実装。
- 開発環境: このリポジトリを触る環境にGo/sqlcがローカルインストールされていない場合、
  `docker run`経由でビルド・コード生成を行う（後述「開発時のビルド方法」参照）。

## ルート一覧（`internal/web/handlers.go`）

各ルートの詳細な業務ルールは、対応する画面設計ドキュメント（`docs/design/screen-board.md`・
`docs/design/screen-close-requests.md`）を参照。

| メソッド/パス | 概要 |
|---|---|
| `GET /` | ボード表示。`target`・`show_untracked`をクエリパラメータで受け取る |
| `POST /tasks/new` | 新規タスク作成（due_date任意、priorityは未指定なら`medium`）。target が jira_a/jira_b の場合はJIRA起票スタブのログのみ出力（実通信なし） |
| `POST /tasks/{id}/edit` | 編集モーダルからの保存。title/target/status/priorityを検証し更新（due_dateは未入力ならNULLとして保存）。`status`が`done`へ/から変化する際は`closed_at`をその場で設定/クリアする |
| `POST /tasks/{id}/move` | ドラッグ&ドロップからの状態変更。`status`に加え`cycle`(`this_week`/`backlog`)も受け取り両方を更新する。JIRA連携タスクなら`stubJiraTransition()`を呼ぶ |
| `POST /tasks/{id}/track` | 「非表示」/「再表示」ボタン。後述の業務ルール参照 |
| `POST /candidates/{id}/approve` | クローズ要求一覧の「承認」ボタン。後述の業務ルール参照 |
| `POST /candidates/{id}/reject` | クローズ要求一覧の「却下」ボタン。`candidates.human_verdict`を`false_positive`にするのみ |
| `GET /static/` | htmx.min.js等の静的ファイル配信（go:embed） |

## デプロイ構成

- `build/Dockerfile`: マルチステージビルド。
  - builderステージ: `golang:1.25-alpine`で`CGO_ENABLED=0 GOOS=linux go build`。
  - 実行ステージ: `gcr.io/distroless/static-debian12`（rootで実行。既存のホスト側ボリューム
    `/docker/task-dashboard/data`のパーミッションがroot実行前提のため、nonrootタグは
    使わない）。`templates/`・`static/`はGoバイナリに`go:embed`済みのため、実行ステージへの
    `COPY`は不要（バイナリ単体をコピーするのみ）。
  - 最終イメージサイズは30MB台（旧Python版は150〜200MB程度）。
- `compose/docker-compose.yml`: サービス名`task-dashboard`。ビルドコンテキストはリポジトリ
  ルート（`..`）、`build/Dockerfile`参照。ポート`8090:8090`。
  実データ(SQLite)は`/docker/task-dashboard/data`（gitの外側）にボリュームマウントする
  （変更なし）。
- 環境変数: `TASK_DASHBOARD_HOST` / `TASK_DASHBOARD_PORT` / `TASK_DASHBOARD_DB_PATH` /
  `TASK_DASHBOARD_AUTO_SEED`（`1`ならDBが空の場合のみ自動シード実行。実連携を組み込んだら
  `0`にする想定）。環境変数名・意味はFlask版から変更していない。
- Mattermost collector用の環境変数（すべて未設定ならcollectorは起動しない）:
  `MATTERMOST_BOT_TOKEN` / `MATTERMOST_SERVER_URL`（例: `https://mattermost.example.com`） /
  `MATTERMOST_CHANNEL_ROUTES`（例: `channelID1:jira_a,channelID2:jira_b,channelID3:personal`。
  MattermostのチャンネルIDと`project_hint`の対応をカンマ区切りで指定）。実際のトークン値は
  リポジトリ・ドキュメントに書かない。`compose/docker-compose.yml`は
  `${MATTERMOST_BOT_TOKEN:-}`のようにDocker Composeの変数展開でこれらを参照しているため、
  値は**ホスト側のシェル環境変数**（例: `export MATTERMOST_BOT_TOKEN=... && docker compose up -d`）
  か、**`compose/.env`ファイル**（`docker compose`が自動読み込みする。`.gitignore`で
  除外済みのためコミットされない）のどちらかに置けばよい。


## 開発時のビルド方法

このリポジトリを触る環境にGo/sqlcがローカルインストールされていない場合、`docker run`経由で
ビルド・コード生成を行う（ホストへのインストールは行わない方針）。

**sqlc生成コード（`internal/taskstore/db.go`/`models.go`/`query.sql.go`）はコミットせず、
ローカルにも置かない。** `go build`の前に必ず`sqlc generate`を実行すること（順序が逆だと
生成物が無くビルドに失敗する）。

```bash
# 1. sqlcによるコード生成(internal/taskstore/query.sql・schema.sqlから)
docker run --rm --user "$(id -u):$(id -g)" -v "$(pwd)/internal/taskstore:/src" -w /src \
  sqlc/sqlc generate

# 2. ビルド
docker run --rm --user "$(id -u):$(id -g)" -e HOME=/tmp -v "$(pwd):/src" -w /src \
  golang:1.25-alpine go build -o /src/.build/task-dashboard .
```

`--user "$(id -u):$(id -g)"`を付けないと生成物がroot所有になり、ホスト側から編集できなく
なるので必ず付けること。`build/Dockerfile`はこの2ステップをビルドステージとして
自動実行するため、`docker compose up --build`等のDocker運用では意識不要。

## 常駐処理（週次ロールオーバー）

本アプリで初めて、アプリ独自の常駐goroutineを導入した（`internal/taskstore/rollover.go`の
`StartRolloverLoop`）。Linear風の「今週/バックログ」区分（Cycles機能）における週次繰り越しを
15分間隔のtickerで自動実行する。`main.go`の起動シーケンスは
`InitSchema`→（AUTO_SEEDなら`Seed`）→**起動時ロールオーバーを1回同期実行**→常駐goroutine起動
→HTTPサーバー起動、の順。現状`main.go`にはグレースフルシャットダウンの仕組みが元々無い
（`http.ListenAndServe`のエラーは`log.Fatal`で即終了し、`defer db.Close()`は実質到達しない）
ため、今回追加した常駐goroutineも`context.Background()`を渡すだけのシンプルな
fire-and-forget実装としている（将来グレースフルシャットダウンを実装する際はこのgoroutineの
終了待ちも合わせて設計する必要がある）。詳細な業務ルールは`docs/design/screen-board.md`
「週次繰り越し」節、実行方式の比較検討の経緯は`docs/adr/complete/cycle-rollover-execution.md`
を参照。

## 既知の制限・今後の課題

- Mattermost collector（`internal/mattermost/`）は収集のみ実装済み。`extractor`
  （メッセージからタスク候補への抽出、Claude API使用）・`syncer`（JIRA実同期）・`registrar`
  （JIRA自動起票）・`digest`（日次まとめ投稿）は未実装。着手にはJIRA APIトークン・
  `project_routing`/`user_map`の初期データ整備が必要（ユーザー側準備待ち）。メール・Zoom収集も
  未実装（着手にはIMAP認証情報・Zoom Server-to-Server OAuthアプリの準備が必要）。
- `user_map`テーブル、および`candidates`テーブルのうち`kind=task`の候補は器のみ用意されており
  画面・業務ロジックからは未使用（extractor未実装のため実データは投入されない）。
  `kind=completion`の候補は「クローズ要求一覧」画面が参照するが、extractorが
  無いため実運用ではこのテーブルにデータが投入されず、画面は空のままになる
  （現状は`seed.go`のサンプルデータでのみ動作確認できる）。Mattermost collectorが保存する
  `messages`テーブルの実データも、extractor未実装のため`candidates`へは変換されない。
- 認証・アクセス制御は無い。外部公開しない前提（既定では`127.0.0.1`バインド、Docker運用時も
  LAN内利用を想定）。
- JS無効時、編集/新規作成フォーム・非表示切替ボタンは通常のHTMLフォーム送信（トップレベル
  ナビゲーション）にフォールバックする。**ただしハンドラ側はhtmx経由かどうかを判別せず、
  常にボード＋トーストのHTMLフラグメント（`boardAndToast`テンプレート、`<html>`/`<head>`を
  含まないフラグメント）を返す**ため、JS無効時にフォーム送信すると、ページ全体がこの
  フラグメントに置き換わり、Tailwind CSS等を読み込むヘッダーやツールバーが失われた見た目に
  なる（機能的にはタスクの作成・更新自体は成功する）。セッション機構を持たないため、
  この経路ではトースト通知も次回操作まで残らない。

## 関連ドキュメント

- `docs/design/data-model.md`（このリポジトリ内） — DB設計（テーブル定義・マイグレーション）。
- `docs/design/screen-board.md`（このリポジトリ内） — 画面設計: カンバンボード画面
  （レーン・フィルタ・カード・編集/新規作成モーダル・D&D・「非表示」の業務ルール）。
- `docs/design/screen-close-requests.md`（このリポジトリ内） — 画面設計: クローズ要求一覧画面
  （承認/却下の業務ルール）。
- `docs/adr/proposals/`・`docs/adr/complete/`（このリポジトリ内） — 全体構想のADR。
  意思決定の経緯（案の比較・採用理由）を論点ごとのファイルに分けて記録している
  （実装まで完了したものは`complete/`、決定のみで実装が無いものは`proposals/`）。
- `docs/design/task-management-automation.md`（このリポジトリ内） — 全体構想のうち、
  まだ実装していない`collector`/`extractor`/`syncer`/`registrar`/`digest`部分の確定設計
  （データモデルER図・パイプライン全体像）。
- `docs/work-log.md` — Go+sqlc+htmxへの移行の経緯・判断理由
