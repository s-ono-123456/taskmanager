# 画面設計: クローズ要求一覧画面

> 全体方針・技術スタックは`docs/design/design.md`、DBスキーマは`docs/design/data-model.md`を
> 参照。本書はツールバーの「クローズ要求」ボタンから開くモーダル画面
> （`internal/web/templates/board.html.tmpl`の`close-requests-modal`）の仕様を扱う。

## 位置づけ

タスク管理自動化構想の全体設計（`docs/design/task-management-automation.md`）でADR論点C3
として採用した「完了候補の承認UI」の実装。`candidates`テーブル（`kind='completion'`かつ
`human_verdict`が未設定の行）を一覧表示し、ツールバーの「クローズ要求」ボタン（承認待ち件数
バッジ付き）からモーダルで開く。

## 一覧の取得

`ListPendingCompletionCandidates`で`kind='completion' AND (human_verdict IS NULL OR
human_verdict = '')`の行を取得する（`internal/web/kanban.go`の`LoadCloseRequests()`）。
`related_jira_key`が空の候補には、`target`が一致し`status != 'done' AND tracked = 1`の
タスク一覧（`ListOpenTasksByTarget`）を選択肢として付加する。

## 承認・却下の業務ルール（`handleApproveCandidate`/`handleRejectCandidate`）

- **承認（`related_jira_key`あり）**: サーバー側で`GetTaskByJiraKey`により対象タスクを
  自動解決する。見つからなければエラートースト。
- **承認（`related_jira_key`なし）**: フォームの`task_id`（画面上の`<select>`でユーザーが
  選んだタスクID）を使う。未選択ならエラートースト（LLMによる自動推定は
  collector/extractor未実装のため行わず、人間が選ぶ形で代替している）。
- **承認の効果**: 対象タスクを`status='done'`に更新し、`closedAtForTransition`で`closed_at`
  を設定（`closeTask()`関数、`edit`/`move`ハンドラと共通ロジック）。JIRA連携タスクなら
  `stubJiraTransition(jiraKey, "close_via_completion_candidate")`を呼ぶ（実通信なし）。
  `candidates.human_verdict`を`'correct'`に更新する。
- **却下の効果**: `candidates.human_verdict`を`'false_positive'`に更新するのみ。対象タスクは
  一切変更しない。

## 画面の更新方式

一覧（`close-requests-container`）とツールバーの件数バッジ（`close-requests-count`）は
どちらも、トーストと同じ`hx-swap-oob="true"`パターンでボード操作のたびに再描画される
（`boardAndToast`テンプレート）。モーダル本体は`#board`の外にある静的な`<dialog>`要素の
ため、承認/却下後も開いたままになり、複数件を続けて処理できる（自動で閉じる対象には
含めていない）。

## 関連ルート（`internal/web/handlers.go`）

| メソッド/パス | 概要 |
|---|---|
| `POST /candidates/{id}/approve` | 「承認」ボタン。上記の業務ルール参照 |
| `POST /candidates/{id}/reject` | 「却下」ボタン。`candidates.human_verdict`を`false_positive`にするのみ |

## 現状の制約

collector/extractorが未実装のため、実運用では`candidates`テーブルに`kind=completion`の
データが投入されず、この画面は空のままになる。現状は`internal/taskstore/seed.go`の
サンプルデータでのみ動作確認できる。

## 関連ドキュメント

- `docs/design/design.md` — 全体方針・技術スタック・ルート一覧の索引・既知の制限。
- `docs/design/data-model.md` — `candidates`テーブルのスキーマ詳細。
- `docs/design/screen-board.md` — メイン画面（カンバンボード）の仕様。
- `docs/adr/proposals/task-management-automation--c-completion-approval-ui.md` — 本画面の
  方式（C3）を採用した経緯。
