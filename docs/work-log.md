# ワークログ

一区切りごとに、コミット前にセッションで作業したこと・学んだことを整理してここに書く。
新しい区切りは **新しいセクションとして先頭（最新）に追加** する。

---

## 2026-09-13 / data-model.mdとtask-management-automation.mdのER図重複を解消

### やったこと

- ユーザーから「`docs/design/data-model.md`になぜER図が載っていないのか、
  `docs/design/task-management-automation.md`と内容が重複しているので正しく振り分けて」
  という指摘を受けた。確認したところ、両ファイルとも同一のDBスキーマ（`messages`/
  `candidates`/`tasks`/`user_map`）を別の形式（テーブル一覧 vs mermaid ER図）で
  説明しており、`task-management-automation.md`側のER図は実装済みスキーマ
  （`internal/taskstore/schema.sql`）と完全に一致するものだった（「本ERはそれと
  一致させてある」と明記されていた）ため、正真正銘の重複だった。
- `docs/design/data-model.md`（DB設計）にER図（mermaid）を移設し、テーブル一覧（役割の
  概要）とER図（列定義・リレーション詳細）の2段構成にした。スキーマ自体に関する
  補足（`tasks.jira_key`の意味、`project_routing`がER図に含まれない理由）もこちらへ移した。
- `docs/design/task-management-automation.md`側のER図・スキーマ補足は削除し、
  `docs/design/data-model.md`への参照に置き換えた。ただし「まだ実装されていない
  collector/extractor視点でのデータの使われ方」（`project_hint`をLLMへのコンテキストとして
  使う、`user_map`未整備時の日次まとめでの確認等）はスキーマ定義そのものではなく
  pipeline固有の振る舞いのため、こちらに残した。

### 学んだこと・注意点

- 「同じ情報を異なる見た目（テーブル vs 図）で書く」ことと「同じ情報を異なるファイルに
  重複して書く」ことは別問題。前者は1ファイル内なら概要と詳細の使い分けとして妥当だが、
  後者は今回のように「片方だけ更新されて食い違う」リスクを生む。ADR分割・design分割の
  作業直後は特に、「このセクションは本当にこのファイル固有の情報か、他のファイルに
  同じ情報が無いか」を見出し単位で洗い出す確認が要る。

---

## 2026-09-13 / ADRの「索引ファイル」方式を廃止し、フラットな観点ファイル名に統一

### やったこと

- 直前のセッションで`task-management-automation`のADRを索引ファイル＋論点ファイル
  （`task-management-automation--a-...`等）に分割していたが、ユーザーから連続して
  次の指摘を受けた:
  1. 「索引ファイルは必要か？必要な部分があれば観点を切り出してADRファイルにして」
     → 索引ファイルの中身を精査したところ、「完了判定は自動クローズしない」という
     決定事項が論点化されず`背景・現状`に埋もれたまま残っていた。これを新しい論点
     ファイル（`auto-close-policy.md`）として切り出した。
  2. 「論点が別れているならまとめる意味もないでしょう」→ 索引ファイル自体
     （`docs/adr/proposals/task-management-automation.md`）を`git rm`で削除。
  3. 「読みにくいだけなのでファイル名もaとかbとかtask-management-automationとか
     つけなくていい」→ 全ファイルを`task-management-automation--{a..g}-*.md`という
     命名から、内容だけを表す平易な名前（`personal-task-store.md`等）へ
     `git mv`でリネームした。
- 最終的なファイル構成: `docs/adr/complete/`に`personal-task-store.md`・
  `completion-approval-ui.md`・`dashboard-tech.md`・`status-granularity.md`・
  `delete-vs-hide.md`・`auto-close-policy.md`（実装済み6件）、
  `docs/adr/proposals/`に`collection-trigger.md`・`data-handling-policy.md`
  （決定済みだが実装が無い2件）。
- 各ファイルの冒頭メタ情報は「起票: 2026-09-12 / タスク管理自動化構想の一部
  （`docs/design/task-management-automation.md` 参照）」という形に統一し、索引ファイルへの
  参照を削除した。同じ構想に属することの一覧性は、`docs/design/task-management-automation.md`
  が各所から各論点ファイルへリンクする形で担保する。
- `docs/design/task-management-automation.md`・`screen-close-requests.md`・
  `data-model.md`・`design.md`・README.md・CLAUDE.md・`docs/session-context.md`・
  `docs/adr/README.md`（本リポジトリ、および`/work/docs/adr/README.md`）の参照箇所を
  すべて新しいファイルパスに更新した。あわせて、旧A〜Gのアルファベット表記に依存していた
  リンクテキスト（`[論点A]`等）も、内容を表す文言に置き換えた（`completion-approval-ui.md`
  等ファイル内部のC1/C2/C3のような単一ファイル内の選択肢ラベルはそのまま残した。これは
  ファイル間の命名規則とは別物のため）。
- グローバル共有スキル`~/.claude/skills/adr/SKILL.md`を全面改修し、「索引ファイル」の概念を
  撤廃した。`<slug>`は常に単一の論点を表す平易な名前とし、タスクIDや連番接頭辞は付けない。
  広い構想の一部であることは各ファイルの起票行に一文で示すのみとし、まとめるための索引は
  作らない方針に統一した。`docs/adr/README.md`（本リポジトリ、`/work`双方）も同様に修正。

### 学んだこと・注意点

- 「論点ごとに1ファイルへ分割する」という要望を実現する際、安易に「索引ファイル」という
  レイヤーを追加すると、論点ファイルが自己完結していればいるほど索引の存在価値が薄れ、
  むしろ二重管理・リンク切れの温床になる。ユーザーからの指摘の通り、分割後に「まとめる
  ファイル」が本当に必要かどうかは都度疑うべきだった。
- ファイル命名規則を変更する（索引方式の導入、その後の全面撤廃）ような可逆性の低い意思決定は、
  一度で確定させようとせず、実際に手を動かしてユーザーに見せながら早めにフィードバックを
  得るとよい。今回は同一セッション内で3段階の指摘を受けて都度手戻りが発生したが、
  「索引ファイルを作る」という最初の設計判断自体をもっと慎重に（他の選択肢と比較して）
  検討していれば、手戻りを減らせた可能性がある。
- ファイル名からタスクIDやアルファベット接頭辞を除去する場合、ファイル名だけでなく
  Markdownリンクの**表示テキスト**（`[論点A]`のような）にも同じ接頭辞が embedded
  されていないか確認する必要がある。`grep -rn "論点[A-Z]"`のような広めのパターンで
  横断的に洗い出すと漏れが減る。

---

## 2026-09-13 / ADR論点のうち実装完了分をdocs/adr/complete/へ移動

### やったこと

- ユーザーから「ADRのうち、実装まで完了したものはcompleteに入れて」という依頼を受けた。
  `task-management-automation`索引配下の論点A〜Gはいずれも採用/決定は済んでいたが、
  「意思決定が済んでいるか」ではなく「対応する実装が実際に完了しているか」を基準に
  判定した:
  - 論点A（個人タスク格納先、A2）: `tasks`テーブル（`jira_key`がNULLの行）として実装済み → 移動。
  - 論点C（完了候補承認UI、C3）: 「クローズ要求一覧」画面として実装済み → 移動。
  - 論点D（ダッシュボード実装技術、D2）: パイロット全体がGo+sqlc+htmxで実装済み → 移動。
  - 論点E（status粒度、E2）・論点F（削除vs非表示、F2）: いずれもパイロット実装に反映済み → 移動。
  - 論点B（収集トリガー、B1）・論点G（情報取り扱い方針、G1）: 決定はしているが、対応する
    collector/extractorが未実装のため実装物が無い → `docs/adr/proposals/`に残置。
- `git mv`で5ファイル（`task-management-automation--{a,c,d,e,f}-*.md`）を
  `docs/adr/proposals/`から`docs/adr/complete/`へ移動。
- 索引ファイル（`docs/adr/proposals/task-management-automation.md`）の論点一覧テーブル・
  冒頭の状態行・本文中のリンクを、移動後のパス（`../complete/...`）と実装状況の注記
  （実装済み/未実装）に合わせて更新した。
- `docs/design/task-management-automation.md`・`docs/design/screen-close-requests.md`
  内の該当ファイルへのリンク（`[論点A](../adr/proposals/...)`等）も、移動先
  （`../adr/complete/...`）に追従させた。`grep`で`proposals/`配下への古いリンクが
  残っていないことを確認した。

### 学んだこと・注意点

- ADRスキルの`complete`は本来「意思決定が決着したか」を基準にしているが、今回ユーザーは
  それとは異なる「実装まで終わっているか」という基準を明示的に指定した。同じ`complete`
  という操作でも、呼び出しごとに判定基準が変わりうるため、機械的にスキルのデフォルト基準
  だけで判断せず、その場の指示を優先する必要がある。
- 論点ファイルを`proposals/`→`complete/`へ移動すると、他の設計ドキュメントからの相対パス
  リンクが壊れる。移動前に`grep -rn`で参照元を洗い出し、移動後にリンク切れが無いことを
  確認する、という手順が有効だった。

---

## 2026-09-13 / docs/design/design.mdをDB設計・画面設計・全体方針設計に分割

### やったこと

- ユーザーから「DB設計、画面設計（画面ごと）、全体方針設計に分けてファイルを分けて」という
  依頼を受けた。複数ファイルへの再構成のためプランモードで方針を確認してから実施した。
- ADR分割時と同じ考え方（既存の参照パスは変えない）を踏襲し、`docs/design/design.md`は
  パスを維持したまま中身を「全体方針設計」（位置づけ・技術スタック・デプロイ構成・
  ルート一覧の索引・既知の制限）に絞った。
- 新設: `docs/design/data-model.md`（DB設計、テーブル定義・マイグレーション）、
  `docs/design/screen-board.md`（画面設計: カンバンボード画面。レーン/フィルタ/カード/
  編集・新規作成モーダル/D&D/「非表示」の業務ルール）、
  `docs/design/screen-close-requests.md`（画面設計: クローズ要求一覧画面。承認/却下の
  業務ルール）。タスク編集・新規作成モーダルはボード画面と不可分なため
  `screen-board.md`にまとめ、独立したボタン・モーダル・ルートを持つクローズ要求一覧は
  別ファイルに分けた。
- `design.md`のルート一覧テーブルは索引として残し、各ルートの詳細な業務ルールは対応する
  画面ファイル側にのみ書く形にして重複を無くした。
- README.md・CLAUDE.md・docs/session-context.mdの参照箇所を、新ファイルへのリンクを
  含む形に更新した（`docs/design/design.md`自体への既存参照はパス不変のため書き換え不要）。
- 途中、ユーザーから動作確認のためのアプリ起動を依頼された。最初host上で直接バイナリを
  実行したところ「Dockerで動かして」と指摘され、次に`docker run`で手動起動したところ
  「Dockerfileとかcomposeとかに設定を入れてよ」と指摘されたため、最終的に
  `docker compose -f compose/docker-compose.yml up -d --build`でプロジェクト所定の
  compose定義通りに起動した。`/docker/task-dashboard/data`に前回セッションの古いDB
  （seed.go変更前のデータ、クローズ要求候補なし）が残っていたため、一時コンテナ
  （`docker run --rm -v ... alpine rm`）でDBファイルのみ削除し、再起動でauto-seedが
  効くようにした（root所有でmvが使えなかったため）。

### 学んだこと・注意点

- 動作確認の「動かして」という依頼は、環境によって期待するものが違う
  （ホスト直接実行／`docker run`手動／プロジェクトの`docker compose`定義通り、の3通り）。
  今回は指示を受けるたびに一段階ずつ正しい方法に近づいた形になったが、最初から
  「このリポジトリの正式なDocker運用方法（`docker compose -f compose/docker-compose.yml
  up -d --build`）」を使うべきだった。次回、既にcompose定義があるリポジトリで動作確認を
  頼まれたときは、まずそちらを使う。
- `TASK_DASHBOARD_AUTO_SEED=1`はDBが空の場合のみ有効なため、既存のボリューム
  （`/docker/task-dashboard/data`）に前回データが残っていると、コード変更後の新しい
  サンプルデータが反映されない。distroless実行イメージにはシェルが無くコンテナ内から
  直接削除できないため、別の一時コンテナ（busybox系イメージ）で同じボリュームをマウントして
  ファイル操作する、という回避策が有効だった。

### 未解決事項

- `/docker/task-dashboard/data`配下は今回のセッションでroot所有のまま運用しており、
  ホスト側（非root）から直接編集・削除ができない状態が続いている。今後同様の作業が
  頻発するようであれば、パーミッションの見直しをユーザーに相談してもよいかもしれない。

---

## 2026-09-13 / 「クローズ要求一覧」画面を実装（ADR論点C3）

### やったこと

- ユーザーから「その画面を作って」という依頼（直前にADR論点CをC3採用へ訂正した流れ）を受け、
  影響範囲が複数ファイルにまたがる新機能のためプランモードで方針を確認してから実装した。
- `internal/taskstore/query.sql`に`ListPendingCompletionCandidates`
  （`candidates.kind='completion' AND human_verdict未設定`を`messages`とJOINして取得）・
  `GetCandidate`・`UpdateCandidateVerdict`・`GetTaskByJiraKey`・`ListOpenTasksByTarget`を追加。
- `internal/taskstore/seed.go`に承認待ちの完了報告候補を2件追加した（1件は
  `related_jira_key="PROJA-101"`で対象タスクが一意に決まるケース、1件は`target=personal`で
  `related_jira_key`が無く、画面上で対象タスクを選ぶケース）。
- `internal/web/kanban.go`に`CloseRequest`/`OpenTaskOption`ビューモデル、
  `LoadCloseRequests()`（`related_jira_key`が無い候補には`ListOpenTasksByTarget`で選択肢を
  付加）、`closeTask()`（既存の`closedAtForTransition`・`stubJiraTransition`を再利用する
  クローズ処理の共通化）を追加し、`BoardData`に`CloseRequests`を追加した。
- `internal/web/handlers.go`に`POST /candidates/{id}/approve`・
  `POST /candidates/{id}/reject`を追加。承認は`related_jira_key`があれば自動解決、
  無ければフォームの`task_id`（`<select>`でユーザーが選択）を使う。
- `internal/web/templates/board.html.tmpl`にツールバーの「クローズ要求」ボタン（件数バッジ）・
  `close-requests-modal`・一覧表示テンプレートを追加。一覧本体（`close-requests-container`）
  と件数バッジ（`close-requests-count`）は、既存のトーストと同じ`hx-swap-oob="true"`パターンで
  ボード操作のたびに再描画するようにした（モーダル自体は`#board`外の静的要素なので、承認/却下
  後も開いたままになり複数件を続けて処理できる）。
- `static/board.js`にモーダルの開閉処理（新規タスクモーダルと同じパターン）を追加。
- 動作確認: Docker経由で`sqlc generate`→`go build`が通ることを確認した後、ローカルで
  サーバーを起動しブラウザ（claude-in-chrome）で一連の操作を確認した。
  - `related_jira_key`ありの候補を承認 → 対象タスク（API仕様書をまとめる/PROJA-101）が
    完了レーンへ移動、トースト表示、一覧から消え、バッジが2→1に更新されることを確認。
  - `related_jira_key`なしの候補で`<select>`から対象タスク（個人: 経費精算）を選び承認
    → そのタスクが完了することを確認（ネイティブ`<select>`はcomputerツールのキー操作では
    選択できなかったため、javascript_toolで`value`をセットし`change`イベントを発火させて
    選択した）。
  - モーダルが承認後も開いたままであること、背景クリックで閉じられること、既存の編集
    モーダル等が影響を受けていないことを確認。
  - 検証後、生成物（`sqlc generate`の出力3ファイル・`.build/`）と一時DBは削除済み
    （コミットしない方針を維持）。
- ドキュメント更新: `docs/design/design.md`にルート一覧2行・「クローズ要求一覧」の業務ルール
  節・「既知の制限」の実態修正を追加。`docs/design/task-management-automation.md`と
  ADR論点Cファイルの「まだ実装していない」という記述を実装済みに更新。CLAUDE.mdの
  「誤解しやすい業務ルール」に、対象タスク未確定時は人間が画面上で選ぶ点を追記。

### 学んだこと・注意点

- ネイティブ`<select>`要素はブラウザ拡張のcomputerツール（クリック+矢印キー）では選択操作が
  反映されないことがあった。`javascript_tool`で`element.value`をセットし`change`イベントを
  発火させる方法で確実にテストできた。
- `candidates`のような「将来のパイプライン用に用意されていたが実データが無いテーブル」に
  UIを先行実装する場合、`seed.go`にデモ用データを追加しないと機能の動作確認自体ができない
  （既存の`tasks`同様、パイロット全体が「実装が先、実データ投入は後」という順序で育っている）。

---

## 2026-09-13 / ADR論点C: C2「検討中」表記の誤りを修正しC3採用として確定

### やったこと

- 直前のセッションで、ユーザーが「Mattermostを経由するより、今のWEBアプリにクローズ要求
  一覧を作成し…」と提案した際、これを「まだ採用可否未定の追加案（検討中）」として
  `task-management-automation--c-completion-approval-ui.md`に記録していたが、ユーザーから
  「私はC3にしろと言ったと思うんだけど」と指摘を受けた。提案ではなく採用指示だったと判断し、
  以下を修正した。
- 論点Cファイル: C2を「不採用（2026-09-12採用→2026-09-13にC3へ変更）」、C3を「採用
  （2026-09-13）」に変更。「採用理由/検討経緯」節も「検討中」のトーンから「C2→C3へ変更した
  理由」の記述に書き換えた。
- ADR索引ファイル: 冒頭の状態行・論点一覧表・「想定される次の一手」（「論点Cの再検討で結論を
  出す」という項目を「クローズ要求一覧画面を実装する」という実装タスクに変更）を更新。
- `docs/design/task-management-automation.md`: C3採用に合わせて設計そのものを更新
  （C2前提だった「日次まとめ→Mattermost→リアクション承認」の記述・全体パイプライン図を、
  「クローズ候補はダッシュボードのクローズ要求一覧画面に表示し、ユーザーが任意タイミングで
  承認する」という設計に書き換え。digestの構成からクローズ候補の項目を削除。管理画面との
  関係図・技術スタック節等、関連箇所を一通り更新）。

### 学んだこと・注意点

- ユーザーが「〜すると良いと思う」という柔らかい言い回しで発言しても、文脈上は実質的な
  決定・指示であることがある。今回は「ADRの承認UIですが」という書き出しで既存の採用方針
  そのものに言及していたため、「新しい代替案の提示」ではなく「既存方針の変更指示」と読むべき
  だった。ADRのような意思決定記録では、ユーザー発言の言い回しの柔らかさだけで
  「検討中」に倒さず、文脈（既存の採用済み方針に対する言及かどうか）を踏まえて判断する。
- ADRの採用結果を変更する際は、対応する`docs/design/`側の設計（パイプライン図・関連節）も
  同時に書き換えないと、ADRとdesignの内容が食い違ったままになる。役割分担を導入した直後
  だったため、両方を追随させる意識が特に重要だった。

---

## 2026-09-13 / CLAUDE.mdに「設計を検討するときはgrillingスキルを使う」ルールを追記

### やったこと

- ユーザーから「設計を検討するときはgrillingスキルで詳細をしっかり詰めることを
  CLAUDE.mdに明記してほしい」と直接依頼を受けた。単純な追記だが、CLAUDE.mdの
  ファイル編集ルールに従いプランモードで内容を確認してから実施した。
- `CLAUDE.md`に新規セクション「## 設計を検討するとき」を追加。複数方針の比較検討・
  設計上の決断を行う際は`grilling`スキルで前提・トレードオフを問い詰めてから結論を
  出すこと、経緯は`docs/adr/README.md`の運用に従いADRとして記録することを明記した。
  配置は「## 関連ドキュメント」の直後、「## セッションコンテキスト・タスクキュー・
  ワークログ」の直前。

### 学び・メモ

- taskmanager側のCLAUDE.mdには元々ADR運用ルールへの参照（`docs/adr/README.md`）は
  あったが、「設計検討時にgrillingスキルを使う」という指示は無かった。`/work/CLAUDE.md`
  側の「## 設計・方針の検討資料（ADR）を扱うとき」セクションと役割が近いが、grilling
  スキルは「問い詰めて詳細を詰める」フェーズ、ADRスキルは「経緯を記録する」フェーズと
  役割が異なるため、taskmanager側では別セクションとして追加した。

---

## 2026-09-13 / ADRを論点ごとに分割し、確定設計をdocs/design/へ切り出し。adrスキルも改修

### やったこと

- ユーザーから「ADRが長くなりすぎている。要点（論点）ごとに1ファイルにしてほしい。また、
  設計とADRの区別がついていない。設計はdesignフォルダに、ADRは設計に至った経緯をポイント
  ごとに1ファイル作成すべき。この修正と同時にadrスキルの内容も修正してほしい」という依頼を
  受けた。グローバル共有スキル（`~/.claude/skills/adr/SKILL.md`、`/work`配下の全リポジトリ
  共通）の改修を伴う影響範囲の広い変更のため、プランモードで具体案を作成しユーザー承認を
  得てから実施した（Plan agentに設計案を検討させ、フラット命名方式
  `docs/adr/proposals/<task-slug>--<point-slug>.md`＋索引ファイル
  `docs/adr/proposals/<task-slug>.md`という構成を採用）。
- `~/.claude/skills/adr/SKILL.md`を改修: 「ADRは意思決定の経緯（案の比較・採用理由）のみ、
  確定仕様はdocs/design/側」という役割分担を明記し、論点が複数ある検討を「索引ファイル＋
  論点ファイル」に分割できる仕組み（`create`/`update`/`complete`の分岐、新規`split`
  サブコマンド）を追加した。既存の単一ファイルADRは索引判定マーカー
  （`- 種別: 索引（複数論点方式）`）が無ければ従来通り動くため、無改修で後方互換を保っている。
- taskmanagerに`docs/adr/README.md`を新設（新規約の説明。従来このファイルが存在せず、
  `adr`スキルの「共通の前提」が読むべきものが無い状態だった）。
- `docs/adr/proposals/task-management-automation.md`を索引ファイルへ書き換え、論点A〜F
  （個人タスク格納先/収集トリガー/完了候補承認UI/ダッシュボード実装技術/status粒度/
  削除vs非表示）に加え、埋もれていた「外部LLM API送信可否」を論点Gとして独立させ、
  計7つの論点ファイル（`task-management-automation--{a..g}-*.md`）に分割した。各ファイルは
  方針案テーブル＋採用理由のみを持ち、確定仕様は含めない。
- 元のADRに同居していた確定仕様（全体パイプライン図・データモデルER図・データの実体/同期
  方針・追跡フラグ・収集/抽出/登録/完了候補提示・クローズ/管理画面/digest構成・リスク・
  自動化パイプライン側の技術スタック）を、新設した`docs/design/task-management-automation.md`
  へ移した。パイロット実装済みで`docs/design/design.md`が既にカバーしている範囲（ダッシュ
  ボードの詳細仕様）は重複記述せず、リンクのみにした。
- `docs/design/design.md`・`CLAUDE.md`・`README.md`・`docs/session-context.md`の関連ドキュメント
  参照を更新。
- `/work/docs/adr/README.md`（**別リポジトリ**）にも、新規約を「今後使える選択肢」として
  追記した（`/work`配下の既存6件のADRは調査・実験ログ的な内容で対応するdesign文書を持たず、
  遡っての分割・移行は行っていない）。

### 学んだこと・注意点

- ADR/designの役割分担を明確化する際、索引ファイルのパスを従来の`<task-slug>.md`のまま
  保つ（論点ファイルは`<task-slug>--<point-slug>.md`という別名で追加する）ことで、
  README/CLAUDE.md等の既存の参照パスを一切変更せずに済んだ。パス変更を伴う再構成では、
  「どのパスが外部から参照されているか」を先に把握し、可能な限りそのパスを不変に保つ設計
  にすると移行コストが下がる。
- グローバル共有スキルの改修は`/work`配下の全リポジトリに影響するため、変更前にプランモードで
  具体案を提示しユーザー承認を得た。特に既存資産（`/work`側の既存6件のADR）を壊さない後方
  互換性の設計（索引マーカーの有無で新旧を判定）を最初に決めたことで、遡及的な移行作業を
  スコープ外にできた。

---

## 2026-09-13 / ADRに論点C3を追加し、パイロット実装の決定事項を反映

### やったこと

- ユーザーから、完了候補の承認UI（論点C、採用済みはC2＝Mattermost日次まとめ＋リアクション
  承認）について、「ダッシュボードWebアプリ内にクローズ要求一覧画面を新設し、DBに蓄積して
  ユーザーが任意タイミングでまとめて承認する」という代替案（C3）の提案を受け、`adr`スキル経由で
  `docs/adr/proposals/task-management-automation.md`の論点Cテーブルに追記した（状態は
  「検討中・採用可否未決定」）。あわせてC2/C3の比較・ハイブリッド案の考察を追記した。
- 続けてユーザーから「（直前のコミットで書いた）パイロット実装との差分について、差分としてで
  はなく、その方針を選んだという書き方でADRに反映してほしい」という指摘を受けた。直前の
  コミットで追加していた「パイロット実装との差分（2026-09-13時点）」という付記的なセクション
  （ADRの記述とパイロット実装の実態が食い違っている、という書き方）を撤去し、代わりに
  論点A/B/Cと同じ形式で新たに論点D（ダッシュボードの実装技術: FastAPI案 vs 採用したGo+sqlc+
  htmx）・論点E（`tasks.status`の粒度: open/done案 vs 採用したtodo/in_progress/reviewing/done
  の4値カンバン）・論点F（追跡除外操作の範囲: 完全削除案 vs 採用した`tracked`フラグの反転の
  みという設計）を新設し、「決定済みの方針」として本文中の該当箇所（ER図、管理画面の削除・
  技術アクセス節等）を直接書き換えた。
- 冒頭メタ情報・「ユーザー判断の反映」節にも論点D/E/Fの採用結果を追記し、ADR全体を通して
  「ADRが未確定のまま実装が先行して食い違いが生じた」という印象ではなく「検討の結果この方針を
  採用した」という一貫したトーンになるよう整えた。

### 学んだこと・注意点

- 実装が先行したあとにADRを追いつかせる場合、「実装 vs 設計の差分」という報告調の書き方は、
  読み手に「設計が形骸化している」という印象を与えやすい。ADRの目的が「決定の記録」である以上、
  たとえ決定の時系列が実装後だったとしても、既存の論点テーブル（案の列挙→採用/不採用）と同じ
  形式に落とし込んで書く方が、ドキュメントとしての一貫性・可読性が高い。

---

## 2026-09-13 / design.md/ADRを現在の実装内容に合わせて最新化

### やったこと

- ユーザーから直接依頼で、`docs/design/design.md`（旧`docs/design.md`）と
  `docs/adr/proposals/task-management-automation.md`を、リポジトリの現在の実装内容
  （Go + sqlc + htmx版）に合わせて見直した。
- `docs/design.md`が既に`docs/design/design.md`へ未コミットのまま移動済み（内容は
  移動前と同一）だったため、この移動を完了させ、`README.md`・`CLAUDE.md`・
  `docs/session-context.md`内の参照パスも`docs/design/design.md`に更新した。
  `docs/work-log.md`内の過去ログの参照は履歴として書き換えていない。
- ソース（`main.go`/`internal/web/*.go`/`internal/taskstore/*`/`static/board.js`/
  `board.html.tmpl`）を通読し、design.mdの記述との突合を行った結果、以下の齟齬を発見・修正:
  - 「既知の制限」節の「JS無効時はフォームの通常送信（303リダイレクト）にフォールバックする」
    という記述が誤り（Flask版由来の記述がGo移行時に更新されずそのまま残っていたと見られる）。
    実際にはhtmx経由かどうかをハンドラ側で判別しておらず、JS無効時も常にボード＋トースト
    フラグメント（`<html>`/`<head>`を含まない）を返すため、フォーム送信後はヘッダー・
    ツールバーを失った見た目になる。実態に合わせて記述を修正した。
  - `internal/taskstore/store.go`の`OpenDB()`が`db.SetMaxOpenConns(1)`でSQLiteの
    同時書き込み制限に対応している点が未記載だったため追記した。
- ADR（全体構想）側は、パイロット実装（スキーマ＋ダッシュボードUI）が完了した現状を
  反映する新規節「パイロット実装との差分」を追加。当初案のFastAPIではなくGoを採用した点、
  `tasks.status`が2値(open/done)ではなく4値カンバンに拡張されている点、`due_date`列が
  追加されている点、ADRが想定していた「個人タスクの完全削除」機能は実装されておらず
  `tracked`フラグの反転による非表示のみが実装されている点、Basic認証が未実装である点を明記。
  「想定される次の一手」の管理画面実装タスクも完了済みである旨に更新。
- 冒頭のメタ情報にあった「タスク管理は`/work`側のtask-queue.mdで追跡」という記述は、
  2026-09-12にタスク管理をこのリポジトリ単体に切り替えた運用（CLAUDE.md参照）と矛盾していた
  ため削除した。

### 学んだこと・注意点

- ドキュメントとソースの突合は、ディレクトリツリーや業務ルールの説明だけでなく、
  「既知の制限」のような細部の記述も含めて全ハンドラ・テンプレート・JSを実際に読んで
  裏取りする必要がある（Flask版からの移行時に更新されずに残っていた記述が1件見つかった）。
- `docs/design.md → docs/design/design.md`のような未コミットのファイル移動が前セッションで
  中途半端な状態（session-context.md/work-log.mdへの記録なし）で残っていることがある。
  今回のように別セッションが引き継ぐ場合は、`git status`で移動・削除の有無を確認し、
  参照元ファイルの更新も含めて完了させるとよい。

---

## 2026-09-12 / board.html.tmpl内のJSをstatic/board.jsへ分離

### やったこと

- ユーザーから直接依頼で、`internal/web/templates/board.html.tmpl`内に埋め込まれていた
  `<script>...</script>`(カードクリック・モーダル開閉・ドラッグ&ドロップ・htmxイベント
  連携・自動リフレッシュのJS一式)を`static/board.js`として分離した。
- JS内にGoテンプレートの`{{...}}`構文が含まれていないことを確認したうえで、中身を
  そのまま`static/board.js`へ移動し、テンプレート側は`<script src="/static/board.js">`
  に置き換えた。`main.go`の`//go:embed static`は`static`ディレクトリ全体を対象にして
  いるため、追加のコード変更なしに`board.js`も配信対象になる。
- 動作確認: sqlc generate → go build → ローカル起動 → ブラウザで編集モーダル・
  ドラッグ&ドロップ（列移動）・非表示切替・トースト通知が正しく動作することを確認。
  最後に`build/Dockerfile`単体でのビルド（sqlc-genステージ込み）も通ることを確認した。
- `README.md`・`docs/design.md`のディレクトリ構成に`static/board.js`を追記した。

### 学んだこと・今後の参考

- 特になし（Go側のコード変更を伴わない、テンプレートからのJS切り出しのみの
  シンプルなリファクタリング）。

---

## 2026-09-12 / sqlc生成コードをコミット・ローカル保存しない方式へ変更

### やったこと

- 前セッションでGo+sqlc+htmxへ移行した際、sqlc生成コード（`internal/taskstore/db.go`/
  `models.go`/`query.sql.go`）はコミットする方針としていたが、ユーザーから
  「ビルド時生成にしてソースコード自体を管理しないようにできるか」と問われ、
  実現方法とトレードオフ（Go単体ビルドができなくなる／レビューで生成結果が見えなくなる、
  という代償と、リポジトリの純粋なソース量が減るという利点）を説明したうえで、
  ユーザーが変更を選択した。
- 対応内容:
  - `.gitignore`に生成物3ファイルを追加し、`git rm --cached`で追跡から除外。
  - ユーザーからの追加指示「コミットしないではなく、ローカルにもファイルとして
    残さないでください」を受け、生成物をローカルからも削除した。
  - `build/Dockerfile`に`sqlc-gen`ステージを追加し、`golang`ビルドステージへ
    生成済み`.go`ファイルをCOPYする形にした。
  - `README.md`・`docs/design.md`の該当記述（ディレクトリ構成の説明・開発時の
    ビルド手順）を「コミット済み」から「コミットしない・ビルド時生成」に更新し、
    手順の順序（sqlc generateが先、goビルドが後）も明記した。
- 動作確認: ローカルで`sqlc generate`→`go build`の順で通ることを確認した後、
  生成物を再度削除し、`docker build`単体（`build/Dockerfile`のsqlc-genステージ込み）
  でクリーンな状態からビルド・起動・HTTPアクセスまで通ることを確認した。
  イメージサイズは変更前と同じ34.5MB。

### 発生した問題と解決

- **`sqlc/sqlc`公式イメージにはシェルが無い**: Dockerfileで`RUN sqlc generate`
  （shell形式、内部的に`/bin/sh -c`を使う）と書いたところ
  `stat /bin/sh: no such file or directory`で失敗した。exec形式`RUN ["sqlc", "generate"]`
  に変更しても今度は`executable file not found in $PATH`で失敗。
  `docker inspect sqlc/sqlc --format '{{json .Config.Entrypoint}}'`で確認すると
  ENTRYPOINTが`/workspace/sqlc`という絶対パスだったため、`RUN`命令はENTRYPOINTを
  引き継がないことを踏まえ`RUN ["/workspace/sqlc", "generate"]`とフルパス指定して解決。

### 学んだこと・今後の参考

- **distroless/シェル無しベースの公式イメージをDockerfileの`RUN`で使う場合、
  shell形式は使えず、exec形式でもコマンド名だけでは`$PATH`解決に失敗することがある。**
  `docker inspect --format '{{json .Config.Entrypoint}}'`でそのイメージの実行ファイルの
  絶対パスを確認し、`RUN ["/絶対パス/コマンド", "引数"]`の形で呼び出すのが確実。
- 生成コードをコミットするかどうかは「Go単体でビルドできる利便性」と
  「リポジトリの純粋性・生成物の差分をコミット履歴に残さない」のトレードオフであり、
  唯一の正解はない。今回はユーザーの「軽量さ・わかりやすさ」重視の意向に沿って
  後者を選んだ。

---

## 2026-09-12 / README.md新規作成とCLAUDE.mdの重複内容整理

### やったこと

- ユーザーからの直接依頼で、README.mdが存在しなかったリポジトリにREADME.mdを
  新規作成した。内容はCLAUDE.md冒頭にあった「プロジェクト概要」「ディレクトリ構成」
  「技術スタック・依存関係」「実行方法」をベースに再編したもの。
- CLAUDE.mdから上記の一般情報セクション（ディレクトリ構成・技術スタック・実行方法）を
  削除し、「プロジェクト概要」も外部通信なし・パイロット位置づけの核心のみ残して
  簡潔化した。詳細はREADME.mdを見るよう文中で誘導する形にした。
- CLAUDE.mdの「誤解しやすい業務ルール」「セッションコンテキスト・タスクキュー・
  ワークログ」等、AIエージェント特有の運用ルールはそのまま残した（一般向けREADMEの
  粒度ではなく、CLAUDE.md固有の価値が高いと判断）。
- 「関連ドキュメント」セクションの先頭にREADME.mdへの参照を追加した。

### 学んだこと・判断理由

- CLAUDE.mdとREADME.mdの役割分担は「READMEは人間の開発者が最初に読む一般情報、
  CLAUDE.mdはAIエージェントが実装時に踏み外しやすい注意点・運用ルールに専念する」
  という切り分けが妥当と判断した。docs/design.mdは既存の詳細設計書としてそのまま
  独立させ、README/CLAUDE.md双方から参照する形を維持した。

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
