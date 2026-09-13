# 画面設計: クローズ要求一覧画面

> 全体方針・技術スタックは`docs/design/design.md`、DBスキーマは`docs/design/data-model.md`を
> 参照。本書はツールバーの「クローズ要求」ボタンから開くモーダル画面
> （`internal/web/templates/board.html.tmpl`の`close-requests-modal`）の仕様を扱う。

## 位置づけ

タスク管理自動化構想の全体設計（`docs/design/automation-roadmap.md`）のADR
「完了候補の承認UI」（`docs/adr/complete/completion-approval-ui.md`）で採用案C3として
決めた実装。`candidates`テーブル（`kind='completion'`かつ
`human_verdict`が未設定の行）を一覧表示し、ツールバーの「クローズ要求」ボタン（承認待ち件数
バッジ付き）からモーダルで開く。

## 一覧の取得

`ListPendingCompletionCandidates`で`kind='completion' AND (human_verdict IS NULL OR
human_verdict = '')`の行を取得する（`internal/web/kanban.go`の`LoadCloseRequests()`）。
`related_jira_key`が空の候補には、`target`が一致し`status != 'done' AND tracked = 1`の
タスク一覧（`ListOpenTasksByTarget`）を選択肢として付加する。`candidates.suggested_task_id`
（Mattermost extractorが抽出時にローカルLLMで推定した対象タスク、下記「対象タスクのAI推定」
参照）がこの一覧に含まれていれば、`<select>`の初期選択肢として提示する。

## 承認・却下の業務ルール（`handleApproveCandidate`/`handleRejectCandidate`）

- **承認（`related_jira_key`あり）**: サーバー側で`GetTaskByJiraKey`により対象タスクを
  自動解決する。見つからなければエラートースト。
- **承認（`related_jira_key`なし）**: フォームの`task_id`（画面上の`<select>`でユーザーが
  選んだタスクID）を使う。未選択ならエラートースト。`<select>`の初期値はAI推定
  （下記「対象タスクのAI推定」参照）だが、あくまでデフォルト値であり人間が変更・確認した上で
  承認操作を行う（自動クローズはしない、`docs/adr/complete/auto-close-policy.md`参照）。
- **承認の効果**: 対象タスクを`status='done'`に更新し、`closedAtForTransition`で`closed_at`
  を設定（`closeTask()`関数、`edit`/`move`ハンドラと共通ロジック）。JIRA連携タスクなら
  `stubJiraTransition(jiraKey, "close_via_completion_candidate")`を呼ぶ（実通信なし）。
  `candidates.human_verdict`を`'correct'`に更新する。
- **却下の効果**: `candidates.human_verdict`を`'false_positive'`に更新するのみ。対象タスクは
  一切変更しない。

## 対象タスクのAI推定

`candidates.suggested_task_id`（nullable、`tasks.id`参照）は、Mattermost extractorが投稿を
抽出・分類する時点（`internal/mattermost/collector.go`の`collectChannel`）でローカルLLMに
推定させ書き込む。`related_jira_key`が無い`kind=completion`の候補について、`channel_id`に
対応する`project_hint`の未クローズタスク一覧（`ListOpenTasksByTarget`、id+titleのみ）を
スレッド全文とあわせて同じ`Classify`呼び出しのプロンプトに渡し、対象タスクを一意に推定
できた場合のみそのidを返させる（`internal/mattermost/llm.go`）。確信が持てない場合は
空文字を返させ、`suggested_task_id`はNULLのままになる。LLMが一覧に無いidを返した場合は
`internal/mattermost/collector.go`の`suggestedTaskID()`で無効値として破棄する
（ハルシネーション対策）。

推定はあくまで`<select>`の初期選択状態を提案するのみで、承認操作自体は引き続き人間が行う
（自動クローズ方針は変更しない）。特別なラベル表示等は行わず、人間が選んだ場合と見た目上の
区別はしない。既存の未承認候補への遡及適用（バックフィル）は行わない。
詳細な検討経緯は`docs/adr/complete/close-request-target-task-suggestion.md`参照。

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

## 実データの投入元

Mattermost extractor（`internal/mattermost/`、`docs/design/data-model.md`「Mattermost
extractor」節参照）が、Mattermostの発言をローカルLLMで分類し、完了報告と判定したものを
`kind=completion`の候補として`candidates`へ書き込む。これにより本画面に初めて実データが
投入されるようになった（それ以前は`internal/taskstore/seed.go`のサンプルデータのみで
動作確認していた）。メール/Zoom収集は未実装のため、それらのソースからの完了報告候補は
引き続き投入されない。

## 関連ドキュメント

- `docs/design/design.md` — 全体方針・技術スタック・ルート一覧の索引・既知の制限。
- `docs/design/data-model.md` — `candidates`テーブルのスキーマ詳細。
- `docs/design/screen-board.md` — メイン画面（カンバンボード）の仕様。
- `docs/adr/complete/completion-approval-ui.md` — 本画面の方式（C3案）を採用した経緯。
- `docs/adr/complete/auto-close-policy.md` — 「自動クローズしない」方針そのものの決定経緯。
