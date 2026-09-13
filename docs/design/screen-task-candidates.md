# 画面設計: タスク候補一覧画面

> 全体方針・技術スタックは`docs/design/design.md`、DBスキーマは`docs/design/data-model.md`を
> 参照。本書はツールバーの「タスク候補」ボタンから開くモーダル画面
> （`internal/web/templates/board.html.tmpl`の`task-candidates-modal`）の仕様を扱う。
> 「クローズ要求一覧」画面（`docs/design/screen-close-requests.md`）と対になる画面で、
> 実装パターンもほぼ同一（コピーして`kind`・登録処理を差し替えたもの）。

## 位置づけ

Mattermost extractor（`docs/design/mattermost-extractor.md`、比較検討の経緯は
`docs/adr/complete/mattermost-extractor-registration-flow.md`参照）が分類した`kind=task`の
候補のうち、`target`（personal/jira_a/jira_b）を自動確定できなかったもの
（`target=unknown`、または信頼度が低いもの）を一覧表示し、人間が対象を選んで承認/却下する。
`target`が確定した候補はこの画面を経由せず、Mattermost extractorがその場で自動的に
`tasks`へ登録する（`candidates.human_verdict='auto_registered'`が設定され、本画面には
現れない）。

## 一覧の取得

`ListPendingTaskCandidates`で`kind='task' AND (human_verdict IS NULL OR human_verdict = '')`の
行を取得する（`internal/web/kanban.go`の`LoadTaskCandidates()`）。クローズ要求一覧と異なり、
「未クローズタスク一覧」のような付加取得は行わない（新規タスクを作るだけで既存タスクとの
紐付けは不要なため）。

## 承認・却下の業務ルール（`handleApproveTaskCandidate`/`handleRejectTaskCandidate`）

- **承認**: 画面上の`<select name="target">`（新規作成モーダルと同じ選択肢:
  personal/jira_a/jira_b）でユーザーが選んだ値を使い、`createTaskFromCandidate()`で新規タスクを
  作成する（`SourceMessageID`に候補の`message_id`を設定するため、作成後のタスクは編集モーダルの
  「元発言」表示でMattermostの元投稿・URLを確認できる）。未選択ならエラートースト。
  クローズ要求一覧と異なり、`related_jira_key`による自動解決の分岐は無い（既存タスクの特定では
  なく新規作成のため、常にユーザーの対象選択が必要）。
- **承認の効果**: `tasks`へ新規行を作成（`status='todo'`、`priority='medium'`固定）。対象が
  jira_a/jira_bなら`stubJiraTransition("(未発行)", "create_via_task_candidate")`を呼ぶ
  （実通信なし）。`candidates.human_verdict`を`'correct'`に更新する。
- **却下の効果**: `candidates.human_verdict`を`'false_positive'`に更新するのみ。タスクは
  作成しない。

## 画面の更新方式

クローズ要求一覧と同じパターン: 一覧（`task-candidates-container`）とツールバーの件数バッジ
（`task-candidates-count`）は`hx-swap-oob="true"`でボード操作のたびに再描画される
（`boardAndToast`テンプレート）。モーダルは承認/却下後も自動で閉じず、複数件を続けて処理できる。

## 関連ルート（`internal/web/handlers.go`）

| メソッド/パス | 概要 |
|---|---|
| `POST /task-candidates/{id}/approve` | 「承認」ボタン。上記の業務ルール参照 |
| `POST /task-candidates/{id}/reject` | 「却下」ボタン。`candidates.human_verdict`を`false_positive`にするのみ |

## 関連ドキュメント

- `docs/design/design.md` — 全体方針・技術スタック・ルート一覧の索引・既知の制限。
- `docs/design/data-model.md` — `candidates`テーブルのスキーマ詳細。
- `docs/design/mattermost-extractor.md` — Mattermost extractorの分類・登録フロー。
- `docs/design/screen-board.md` — メイン画面（カンバンボード）の仕様。
- `docs/design/screen-close-requests.md` — 対になる画面（クローズ要求一覧、`kind=completion`用）。
- `docs/adr/complete/mattermost-extractor-registration-flow.md` — 自動登録/人間承認の切り分けを
  採用した経緯。
