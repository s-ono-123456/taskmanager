# taskmanager (task-dashboard パイロット実装)

## プロジェクト概要

タスク管理自動化構想（JIRA2プロジェクト＋個人タスクをMattermost/メール/Zoomから
自動収集し、JIRA自動起票・完了候補提示まで行う）のうち、「スキーマとダッシュボード
UIのパイロット実装」に相当するリポジトリ。2026-09-12に`/work`（メインリポジトリ）の
`docker/task-dashboard/`からこの独立リポジトリへ切り出された。

**外部通信は一切行わない。** Mattermost/メール/Zoom/JIRA/Claude APIいずれにも
接続せず、JIRA連携相当の操作はすべて`stubJiraTransition()`によるログ出力のみ。

全体構想（本パイロットが将来どう拡張される想定か）は本リポジトリ内の
`docs/adr/proposals/task-management-automation.md`を参照（2026-09-12、ADR自体も
`/work`からこのリポジトリへ移動済み）。

**2026-09-12、Python/Flask実装からGo + sqlc + htmxへ全面移行した。** 「軽量さ」と
「SQLがロジックから分離されたわかりやすさ」を重視した選択（経緯は`docs/work-log.md`参照）。
以下はGo版の構成。詳細は`docs/design.md`を参照。

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
  db.go / models.go / query.sql.go  # sqlc生成コード(コミット済み・手編集しない)
  store.go                      # DB接続・起動時マイグレーション
  seed.go                       # サンプルデータ投入(再実行可能・全件作り直し)
internal/web/                  # HTTPハンドラ・業務ロジック
  handlers.go / kanban.go / render.go
  templates/board.html.tmpl     # 唯一のテンプレート(html/template、go:embed)
static/htmx.min.js             # htmx本体(vendor同梱、CDN不使用)
docs/design.md                 # 詳細設計書（データモデル・画面仕様・ルート一覧を網羅）
```

## 技術スタック・依存関係

- Go標準ライブラリの`net/http`（Go 1.22+の`ServeMux`）+ `html/template`。追加の
  ルーターフレームワークは使わない。
- SQL: [sqlc](https://sqlc.dev/)で`internal/taskstore/query.sql`から型安全なGoコードを
  生成する。SQLは`.sql`ファイル、ロジックはGoファイル、という分離を最重視している。
- DBドライバ: `modernc.org/sqlite`（cgo不要）。`CGO_ENABLED=0`でビルドでき、実行イメージを
  `distroless/static-debian12`にできる（最終イメージ30MB台）。
- フロントエンド: htmx（vendor同梱）+ Tailwind CSS（CDN読み込み、変更なし）。JSは
  ドラッグ&ドロップ・モーダル開閉のみ素のDOM操作。
- **この環境にGo/sqlcがローカルインストールされていない場合、`docker run`経由で
  ビルド・コード生成を行う**（ホストへのインストールは行わない方針）。コマンド例は
  `docs/design.md`の「開発時のビルド方法」を参照。

## 実行方法

- ローカルビルド（Docker経由）:
  `docker run --rm --user "$(id -u):$(id -g)" -e HOME=/tmp -v "$(pwd):/src" -w /src golang:1.25-alpine go build -o /src/.build/task-dashboard .`
  - `--user`を付けないと生成物がroot所有になるので必ず付けること。
  - 実行時は環境変数`TASK_DASHBOARD_HOST`（既定127.0.0.1）/`TASK_DASHBOARD_PORT`
    （既定5000）/`TASK_DASHBOARD_DB_PATH`/`TASK_DASHBOARD_AUTO_SEED`
    （`1`でDBが空の時のみ自動シード）で上書き可能。
- sqlcによるクエリ再生成（`query.sql`変更時）:
  `docker run --rm --user "$(id -u):$(id -g)" -v "$(pwd)/internal/taskstore:/src" -w /src sqlc/sqlc generate`
- Docker実行: `docker compose -f compose/docker-compose.yml up -d --build`
  - ポート8090、実データ(SQLite)は`/docker/task-dashboard/data`
    （このリポジトリの外・gitの管理対象外）にボリュームマウントされる。

## 誤解しやすい業務ルール（詳細は`docs/design.md`参照）

- **完了レーンは直近7日以内に完了(`closed_at`)したタスクのみ表示**する
  （`DoneLaneWindowDays`）。7日を超えても データは残り続け、
  「非表示分も表示」をONにしても表示されない（削除ではなく表示上のフィルタ）。
- **「非表示」＝`tracked`フラグの反転のみ。** 削除機能は存在しない。
  非表示にする際、対象が`done`でなければ同時に`status=done`・`closed_at`を
  設定する（＝一旦完了扱いにする）。
- ドラッグ&ドロップのドロップ先レーン判定は**ポインタのx座標のみ**で行う
  （y座標・列の高さは考慮しない）。
- JIRA連携（`target=jira_a`/`jira_b`）タスクの状態変化時は`stubJiraTransition()`
  を呼ぶが、実際のHTTP通信は発生しない。
- POST操作（編集/新規作成/移動/非表示切替）はhtmx化されており、成功・失敗いずれも
  HTTP 200で「ボードフラグメント＋トースト」を返す。成功/失敗はレスポンスヘッダー
  `X-Toast-Category`で判定している（htmxは4xx/5xxを自動スワップしないため）。
- 編集・新規作成フォームがフィルタ状態（`target`/`show_untracked`）をhx-valsで送る際は
  `filter_target`/`filter_show_untracked`という専用キー名を使う。フォーム自身の
  `target`（タスクの対象）と名前が衝突するのを避けるため。

## 関連ドキュメント

- `docs/design.md`（本リポジトリ内） — データモデル・画面仕様・ルート一覧・
  デプロイ構成・既知の制限を網羅した詳細設計書。実装を変更する際は必ず参照し、
  変更があれば追記すること。
- `docs/adr/proposals/task-management-automation.md`（本リポジトリ内） — 全体構想のADR
  （データモデル・パイプライン全体像。まだ未実装のcollector/extractor/syncer/registrar
  含む）。
- `docs/session-context.md` / `docs/task-queue.md` / `docs/work-log.md`（本リポジトリ内） —
  進捗管理・作業経緯の記録。2026-09-12より、タスク管理は`/work`側ではなくこのリポジトリ
  単体で行う運用に変更した（詳細は次の「セッションコンテキスト・タスクキュー・ワークログ」章）。

## セッションコンテキスト・タスクキュー・ワークログ

このリポジトリを複数セッション（claude-code/opencode等）が並行して触ることがあるため、
作業状態を3ファイルに役割分担して記録し、セッション間のコンテキスト引き継ぎ・作業の
重複防止に利用する。

- **`docs/session-context.md`**: プロジェクト概要と、「今アクティブなセッションが
  何をしているか」（アクティブセッション表）のみを記録する。完了した作業の経緯は書かない。
- **`docs/task-queue.md`**: 次にやるべきタスクを一元管理する（4セクション:
  ユーザーの判断・実施待ち／未着手／進行中／低優先度）。状態は行が属するセクションのみで
  表し、行内に別途「状態」欄は設けない。
- **`docs/work-log.md`**: 完了した作業の経緯・学んだことを記録する。新しい区切りは
  新しいセクションとして先頭（最新）に追加する。

**並列セッション対応のため、session-context.md・task-queue.md は全文上書きしない。**
自分が追加した行、または自分が担当している行のみを更新する。他セッションが書いた行は
編集・削除しない（内容に疑問があれば新しい行として書き添える）。

**作業開始時（最初の実装ツール呼び出しの前に必ず実行する）**:
1. `docs/session-context.md`と`docs/task-queue.md`を読み、他セッションの活動状況・
   既存タスクの状況を把握する。
2. 自分がこれから着手しようとしている範囲が、他のアクティブセッションの対象範囲と
   重なっていないか確認する。重複するなら別のタスクを優先するか、ユーザーに確認する。
3. `docs/session-context.md`のアクティブセッション表に自分の行を追加する
   （セッションID=ツール種別@着手時刻、作業内容、対象ファイル、更新時刻）。
   時刻は`date "+%Y-%m-%d %H:%M"`で取得した値を`YYYY-MM-DD HH:MM`形式で使う。

**コミット直前の必須チェックリスト**:
- [ ] `docs/session-context.md`の自分のアクティブセッション行を最新化（完了していれば削除）
- [ ] `docs/task-queue.md`で自分が担当していたタスクの状態を更新（完了なら行を削除）
- [ ] `docs/work-log.md`に今回のセッションで作業したこと・学んだことを追記
