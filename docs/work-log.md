# ワークログ

一区切りごとに、コミット前にセッションで作業したこと・学んだことを整理してここに書く。
新しい区切りは **新しいセクションとして先頭（最新）に追加** する。

---

## 2026-09-12 / Python/FlaskからGo + sqlc + htmxへの全面移行

### やったこと

- 前セッションのWeb調査（言語・フレームワーク選定）を受け、ユーザーが「軽量さ」
  「SQLがロジックから分離されたわかりやすさ」を重視することを確認したうえで、
  Go（標準`net/http`/`html/template`）+ sqlc + htmxへの全面移行を実施した。
  プランモードで移行計画を作成し、Plan agentによるレビューを経てユーザー承認を得てから
  実装した（詳細は`/home/s-ono/.claude/plans/federated-inventing-widget.md`参照、
  ローカルのプランファイルのため本リポジトリ外）。
- この環境にGo/sqlcがローカルインストールされていなかった（Dockerのみ利用可）ため、
  `docker run`経由でビルド・sqlcコード生成を行う方針で進めた。
- 実装したもの:
  - `internal/taskstore/schema.sql`・`query.sql`・`sqlc.yaml`（sqlc設定）、
    sqlc生成コード（`db.go`/`models.go`/`query.sql.go`）
  - `internal/taskstore/store.go`（DB接続・起動時マイグレーション）、`seed.go`
    （サンプルデータ投入）
  - `internal/web/kanban.go`（業務ロジック集約）・`handlers.go`（5ルート）・
    `render.go`（テンプレートレンダリング）
  - `internal/web/templates/board.html.tmpl`（html/template + htmx属性）
  - `main.go`（エントリポイント）、`static/htmx.min.js`（vendor同梱）
  - `build/Dockerfile`（マルチステージ: golang:1.25-alpine → distroless/static-debian12）
- htmx化の方針: フォームPOST（編集/新規作成/移動/非表示切替）はすべて`hx-post`+
  「ボード全体フラグメントの返却」に統一し、Flask版のsession flashは
  `hx-swap-oob`によるトースト通知に置き換えた。
- Dockerイメージビルド・起動・curl/ブラウザでの全機能動作確認を行い、最後に旧Python
  実装（`app.py`/`db.py`/`seed.py`/`templates/board.html`）を削除し、`docs/design.md`・
  `CLAUDE.md`を新構成に合わせて更新した。

### 発生した問題と解決

1. **`modernc.org/sqlite`最新版がGo 1.25以上を要求**: 当初`golang:1.24-alpine`で
   `go get`したところ`requires go >= 1.25.0`でエラー。`golang:1.25-alpine`
   （Go 1.25.14）に切り替えて解決。以降Dockerfileも1.25系で統一した。
2. **Go 1.22+ ServeMuxのパターン競合**: `mux.Handle("/static/", ...)`と
   `mux.HandleFunc("GET /", ...)`が「どちらがより具体的か曖昧」として起動時panicに
   なった。`GET /static/`とメソッドを明示して解決。
3. **フィルタ状態とフォーム自身のフィールド名の衝突（重要）**: 編集/新規作成フォームは
   タスクの対象を表す`name="target"`フィールドを持つが、当初は現在の表示フィルタ
   （同じく`target`という名前）を`hx-include="#filter-state"`で素直に含めていたため、
   POSTボディに同名キーが2つ存在してしまい、`r.FormValue("target")`が意図しない方の値を
   返すバグを作り込んだ（ブラウザ実機テストで発覚：バリデーションエラー時に他のカードが
   消える現象として現れた）。フィルタ側を`filter_target`/`filter_show_untracked`という
   専用キー名にし、`hx-vals="js:{...filterStateVals()}"`（JS関数でツールバーの現在値を
   読んで返す）に切り替えて解決。
4. **`hx-vals`のjs:記法**: `hx-vals="js:filterStateVals()"`（関数呼び出しのみ）は
   `SyntaxError: Unexpected token '}'`で失敗する。`js:`以降はオブジェクトリテラルとして
   評価されるため、`js:{...filterStateVals()}`とスプレッド構文で包む必要がある。
5. **`hx-on::after-request`が発火しない**: 属性自体は正しい記法（htmx 2.0.4で
   `hx-on::after-request`は`htmx:afterRequest`の省略形として文書化されている）だが、
   実機では発火が確認できなかった。原因の切り分けに`javascript_tool`でイベントリスナーを
   仕込んでのデバッグが有効だった。最終的に`document.body`に張ったグローバルな
   `htmx:afterRequest`リスナー（`ev.detail.elt`で対象フォームを判定）に置き換えて解決。
6. **`event.detail.successful`は使えない設計だった**: バリデーションエラー時も
   HTTP 200を返す設計（htmxが4xx/5xxを自動スワップしないため）にした結果、
   `htmx:afterRequest`の`event.detail.successful`は常に`true`になり、
   「エラー時はモーダルを開いたままにする」判定に使えなかった。レスポンスヘッダー
   `X-Toast-Category`（`success`/`error`）を追加し、`event.detail.xhr.getResponseHeader(...)`
   で判定する方式に変更して解決。
7. **Dockerで生成したファイルがroot所有になる**: `docker run`だけだとsqlc生成物・
   ビルド成果物がroot所有になりホスト側から編集できなくなった。
   `--user "$(id -u):$(id -g)"`を付けて回避（以降すべてのdocker run呼び出しに付与）。
8. **ブラウザ操作テストでの誤検知**: デバッグ中、`triple_click`のつもりが
   `left_click`になっていて既存タイトルの前later後にスペースが挿入されただけ（trimしても
   空にならず「成功」してしまう）というテスト側のミスで、一時的に実装バグと誤認した。
   `javascript_tool`で直接DOM操作・イベント発火して切り分けたことで、実装側は正しく
   動いていることを確認できた。

### 学んだこと・今後の参考

- **htmxの`hx-on::`記法や`hx-vals`のjs:記法は、ドキュメント通りに書いても実機で
  発火しない/構文エラーになるケースがあるため、実装したら必ずブラウザの実機で
  console/networkを確認すること。** 特に`hx-on::after-request`のような属性ベースの
  イベントハンドラは、疑わしい場合はグローバルな`addEventListener`に置き換えると
  デバッグ・保守がしやすい。
- **「常に200を返す」設計にすると、htmxの`event.detail.successful`（HTTPステータス
  ベースの判定）は使えなくなる。** 成功/失敗をアプリケーション側で判定させたい場合は
  レスポンスヘッダーやレスポンスボディ内の目印（data属性等）を自前で用意する必要がある。
- **同じ意味を持たない同名の`name`属性が1つのPOSTボディに混在する設計（今回は
  「表示フィルタのtarget」と「タスクの対象のtarget」）は事故りやすい。** htmxの
  `hx-include`は素直にDOM要素をそのまま含めるため、名前が衝突する場合は片方を別名にする
  （今回は`filter_`接頭辞）か、明示的にオブジェクトを組み立てる`hx-vals`を使うこと。
- Dockerイメージサイズ: 旧Python版（`python:3.13-slim`+flask）は150〜200MB程度に対し、
  Go版（`distroless/static-debian12`+単一バイナリ）は34.5MB。「軽量さ」というユーザーの
  重視軸に対して定量的に効果が出た。
- Go/sqlcをホストにインストールせず`docker run`経由で開発する場合、モジュール
  キャッシュがコンテナ間で共有されないため`go build`のたびに依存を再ダウンロードする
  （数秒〜十数秒のオーバーヘッド）。頻繁にビルドし直す場合はDockerボリュームで
  `/go/pkg/mod`をキャッシュすると速くなる（今回は都度ダウンロードのままで進めたが、
  今後の改善余地）。

---

## 2026-09-12 / 言語・フレームワーク選定に関するWeb調査（コード変更なし）

### やったこと

- ユーザーから「現状Flaskを使っているが、言語・フレームワークを問わず最適な選択肢は
  何か、最新情報をWebで調査してほしい」と依頼を受けた。
- 1回目の調査（TechEmpowerベンチマーク傾向、FastAPI/Flask/Django比較、htmx等）では
  「Python生態系内でFlaskのままで良い/FastAPIは過剰」という、Pythonに限定した結論を
  出してしまい、ユーザーから「Pythonである必要はない、言語不問と言ったはず」と
  指摘を受けた。
- 指摘を受けて追加調査（Go/SQLite/htmxを組んだ社内ツール構成の事例、
  modernc.org/sqlite等cgo-freeドライバの動向）を行い、結論を修正した。
  - taskmanagerの性質（単一プロセス・SQLite・外部通信なし・少人数向け社内ツール・
    Docker配布）を踏まえると、パフォーマンスではなく「配布の軽さ」「依存の少なさ」
    「型安全性」の観点でGo（標準ライブラリの`net/http`/`html/template` +
    cgo不要の`modernc.org/sqlite` + htmx）が有利という結論に至った。
  - 現在の`python:3.13-slim`+`pip install flask`という配布形態に対し、Goなら
    単一の静的バイナリ配布ができ、Dockerイメージを大幅に軽量化できる点を優位性として
    提示した。
  - 一方で、既存の`app.py`/`db.py`/`seed.py`/`templates/board.html`の書き直しコストと
    チームのPython習熟度とのトレードオフがあるため、実際に移行するかどうかの判断は
    ユーザーに委ねた。
- コード変更・依存追加は行っていない（調査・提案のみ）。

### 学んだこと・今後の参考

- 「言語・フレームワーク問わず」という依頼に対しては、既存実装の言語（Python）を
  起点にその生態系内だけで比較して結論を出すと、依頼の趣旨を外してしまう。
  他言語（特にGo/Rust等コンパイル言語）も実際に候補として本気で比較検討すること。
- パフォーマンス比較だけで技術選定を判断せず、「実際のワークロード（低トラフィックの
  社内SSRツール）に効果があるか」を基準に評価する必要がある。今回の場合は
  パフォーマンスより「配布のシンプルさ・依存の少なさ・型安全性」が決め手になった。

---

## 2026-09-12 / CLAUDE.md作成とセッション運用ファイルの導入

### やったこと

- ユーザーから「CLAUDE.mdを作って」と依頼を受け、`docs/design.md`・`app.py`・
  `compose/docker-compose.yml`・`build/Dockerfile`の内容を確認したうえで、プロジェクト概要・
  ディレクトリ構成・技術スタック・実行方法・誤解しやすい業務ルール（完了レーンの7日フィルタ、
  「非表示」=`tracked`フラグの反転のみ、ドラッグ&ドロップのx座標判定等）をまとめた
  `CLAUDE.md`を新規作成した。
- 作業中、別の並行セッション（claude-code, session_01T56nUYxWvzCPeWVjchgWhS）が
  `/work`側の`docs/adr/proposals/task-management-automation.md`をこのリポジトリへ移動し、
  `docs/design.md`・`CLAUDE.md`の関連ドキュメント参照を更新したうえでコミット
  （`1619439`）していたことに気付いた。そのコミットメッセージには「進捗管理
  (work-log/task-queue)は引き続き`/work`側で行う」と明記されていた。
- ユーザーから続けて「(session-context.md/task-queue.md/work-log.mdを)作成して」と
  依頼されたが、上記の他セッションの方針（`/work`側で進捗管理を継続）と矛盾する可能性が
  あったため、`AskUserQuestion`でユーザーに方針を確認。「taskmanager単独で運用開始する」
  という回答を得た。
- `/work/docs/session-context.md`・`/work/docs/task-queue.md`・`/work/docs/work-log.md`の
  フォーマット（冒頭の注意書き・アクティブセッション表・4セクション構成のタスクキュー・
  運用ルール9項目・日付見出し形式のワークログ）を参考に、taskmanager版として
  `docs/session-context.md`・`docs/task-queue.md`・`docs/work-log.md`を新規作成した。
  taskmanagerはまだ並列衝突の実績が少ないため、運用ルールの説明文は`/work`版より
  コンパクトにしたが、規則の骨子（全文上書き禁止・状態はセクションのみで表現・IDは
  kebab-caseスラッグ等）は同一にした。
- `CLAUDE.md`にも「セッションコンテキスト・タスクキュー・ワークログ」章を追記し、
  3ファイルの役割分担・作業開始時の手順・コミット前チェックリストを明記した。あわせて
  「関連ドキュメント」章末尾にあった「タスク管理は`/work`側で行う」という記述を、
  今回の方針転換に合わせて修正した。

### 学んだこと・判断の理由

- 独立したリポジトリへの切り出し後も、切り出し前の`/work`側の運用方針（進捗管理は
  `/work`側で継続）を前提に動くセッションと、リポジトリ単体での運用開始を望むユーザーの
  意向がすれ違う場面があり得る。CLAUDE.mdやタスクキューの記述が「いつ・誰の判断で」
  書かれたものかが分かるようにしておく（コミットメッセージ・work-logへの経緯記録）ことが、
  こうした食い違いに気づく手がかりになった。
- 結果（他セッションのコミットが既に存在すること）だけを見て自分の作業をそのまま進めず、
  方針の矛盾に気付いた時点で一度ユーザーに確認してから進めた。
