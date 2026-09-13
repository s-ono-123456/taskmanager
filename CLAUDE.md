# taskmanager (task-dashboard パイロット実装)

## プロジェクト概要

タスク管理自動化構想（JIRA2プロジェクト＋個人タスクをMattermost/メール/Zoomから
自動収集し、JIRA自動起票・完了候補提示まで行う）のうち、「スキーマとダッシュボード
UIのパイロット実装」に相当するリポジトリ。**外部通信は原則行わない**が、唯一の例外として
Mattermost collector兼extractor（`internal/mattermost/`。2026-09-13追加、同日にAI分類機能を
拡張）は実際にMattermost APIへポーリング接続し、取得した投稿をこのホスト上の
ローカルLLM（llama-swap、外部ではない）で分類する（Bot Token等の認証情報はユーザーが
環境変数で設定、未設定なら起動しない）。メール/Zoom/JIRA/Claude APIへは引き続き一切
接続せず、JIRA連携相当の操作はすべて`stubJiraTransition()`によるログ出力のみ。

プロジェクト概要・技術スタック・ディレクトリ構成・実行方法は`README.md`を参照。
詳細設計は`docs/design/`配下に分割している: 全体方針は`docs/design/design.md`、
DB設計は`docs/design/data-model.md`、画面設計は`docs/design/screen-board.md`
（カンバンボード）・`docs/design/screen-close-requests.md`（クローズ要求一覧）・
`docs/design/screen-task-candidates.md`（タスク候補一覧）を参照。

## 誤解しやすい業務ルール（詳細は`docs/design/screen-board.md`・`docs/design/screen-close-requests.md`参照）

- **完了レーンは直近7日以内に完了(`closed_at`)したタスクのみ表示**する
  （`DoneLaneWindowDays`）。7日を超えても データは残り続け、
  「非表示分も表示」をONにしても表示されない（削除ではなく表示上のフィルタ）。
- **「非表示」＝`tracked`フラグの反転のみ。** 削除機能は存在しない。
  非表示にする際、対象が`done`でなければ同時に`status=done`・`closed_at`を
  設定する（＝一旦完了扱いにする）。
- ボードは4ステータス列×「今週/バックログ」2スイムレーンの2軸グリッド（Cycles機能）。
  ドラッグ&ドロップのドロップ先判定は**y座標でスイムレーン行、x座標でステータス列**を
  2段階で判定する（列の高さは考慮しない点は変更なし）。今週⇔バックログの切替はD&Dのみで、
  タスクの`cycle_start_date`（所属週の月曜日、NULL=バックログ）で表現する。週境界（月曜0:00
  JST）で未完了タスクは常駐goroutineにより自動的に次週へ繰り越される。
- JIRA連携（`target=jira_a`/`jira_b`）タスクの状態変化時は`stubJiraTransition()`
  を呼ぶが、実際のHTTP通信は発生しない。
- POST操作（編集/新規作成/移動/非表示切替）はhtmx化されており、成功・失敗いずれも
  HTTP 200で「ボードフラグメント＋トースト」を返す。成功/失敗はレスポンスヘッダー
  `X-Toast-Category`で判定している（htmxは4xx/5xxを自動スワップしないため）。
- 編集・新規作成フォームがフィルタ状態（`target`/`show_untracked`）をhx-valsで送る際は
  `filter_target`/`filter_show_untracked`という専用キー名を使う。フォーム自身の
  `target`（タスクの対象）と名前が衝突するのを避けるため。
- 「クローズ要求一覧」（`candidates.kind=completion`の承認/却下）で、候補に
  `related_jira_key`が無い場合、対象タスクは**画面上の`<select>`で人間が選ぶ**
  （承認時はサーバー側で自動解決しない。JIRAの誤クローズは実害が大きいため、人間の
  明示的な承認操作は必ず必要）。Mattermost extractorが抽出時にローカルLLMで対象タスクを
  推定し（`candidates.suggested_task_id`）、その推定が未クローズタスク一覧に含まれていれば
  `<select>`の初期選択肢として提示するが、あくまでデフォルト値であり人間は自由に選び直せる
  （`docs/adr/complete/close-request-target-task-suggestion.md`参照）。
- **優先度(`priority`)は表示専用**（最高/高/中/低、デフォルト「中」）。カード上のバッジ
  表示のみで、並び順（`created_at DESC`固定）・レーン/ステータス構造には一切影響しない。
  編集・新規作成モーダルで変更可能だが、ドラッグ&ドロップ（`/tasks/{id}/move`）では
  変更されない。
- **Mattermost extractorは収集+ローカルLLMでの取得時分類+確定target分の自動タスク登録**
  （`internal/mattermost/`）。`kind=task`かつ`target`確定なら即`tasks`へ自動登録
  （`candidates.human_verdict='auto_registered'`）、`target`不明なら「タスク候補一覧」画面へ、
  `kind=completion`なら既存の「クローズ要求一覧」画面へ。`kind=none`・低信頼度の投稿は
  `messages`テーブルにすら保存しない（破棄）。`MATTERMOST_BOT_TOKEN`が未設定の環境では
  起動自体しない。ComfyUIと同一GPUを排他利用しており、抽出処理実行時にComfyUI生成ジョブが
  強制停止されうるが許容する方針（回避ロジックなし）。

## 関連ドキュメント

- `README.md`（本リポジトリ内） — プロジェクト概要・技術スタック・ディレクトリ構成・
  実行方法。
- `docs/design/design.md`（本リポジトリ内） — 全体方針（位置づけ・技術スタック・
  ルート一覧の索引・デプロイ構成・既知の制限）。実装を変更する際は必ず参照し、
  変更があれば追記すること。
- `docs/design/data-model.md`（本リポジトリ内） — DB設計（テーブル定義・マイグレーション）。
- `docs/design/screen-board.md`（本リポジトリ内） — 画面設計: カンバンボード画面。
- `docs/design/screen-close-requests.md`（本リポジトリ内） — 画面設計: クローズ要求一覧画面。
- `docs/design/screen-task-candidates.md`（本リポジトリ内） — 画面設計: タスク候補一覧画面。
- `docs/adr/proposals/`・`docs/adr/complete/`（本リポジトリ内） — 全体構想のADR。
  意思決定の経緯（案の比較・採用理由）を論点ごとのファイルに分けて記録している
  （実装まで完了したものは`complete/`）。
- `docs/design/automation-roadmap.md`（本リポジトリ内） — タスク管理自動化構想の全体像・
  背景・リスク・ロードマップ（Mattermost collector兼extractorは実装済み）。
- `docs/design/mail-zoom-pipeline.md` / `docs/design/jira-sync.md` / `docs/design/digest.md`
  （本リポジトリ内） — まだ未実装のメール/Zoom collector・extractor・registrar・syncer
  （実JIRA通信）・digestそれぞれの確定設計。
- `docs/adr/README.md`（本リポジトリ内） — ADRとdocs/design/の役割分担・ファイル構成の
  運用ルール。
- `docs/session-context.md` / `docs/task-queue.md` / `docs/work-log.md`（本リポジトリ内） —
  進捗管理・作業経緯の記録。2026-09-12より、タスク管理は`/work`側ではなくこのリポジトリ
  単体で行う運用に変更した（詳細は次の「セッションコンテキスト・タスクキュー・ワークログ」章）。

## 設計を検討するとき

複数の実装方針を比較検討する、または設計上の決断を行う場合は、`grilling`スキルを使い、
前提・トレードオフ・想定していないケースなどをユーザー自身に対して容赦なく問い詰めて
詳細をしっかり詰めてから結論を出すこと。方針の比較検討の経緯は`docs/adr/README.md`の
運用ルールに従いADRとして記録する。

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
