# 論点: クローズ要求一覧の対象タスクAI推定

- 起票: 2026-09-13（`docs/adr/complete/mattermost-extractor-registration-flow.md`等の
  Mattermost extractor実装の続き。task-queue.md: `close-request-target-task-ai-suggest`）
- 論点: クローズ要求一覧（`docs/design/screen-close-requests.md`）で、`related_jira_key`が
  無い完了報告候補の対象タスク`<select>`を、人間が毎回手動で選ぶのではなく、AIが推定した
  デフォルト選択肢を出すには、どの段階でどう推定するか。
- 状態: **採用済み・実装完了（2026-09-13）**

## 背景

現行実装（`docs/adr/complete/completion-approval-ui.md`採用のC3案）は、`related_jira_key`が
無い候補について、`ListOpenTasksByTarget`で絞り込んだ未クローズタスク一覧を`<select>`に出し、
`<option value="">対象タスクを選択</option>`を初期値として人間に必ず選ばせている。ユーザーから
「対象タスクが自動的に選択されないのが微妙、AIで判断してデフォルト設定してほしい」との
要望があった。

`docs/adr/complete/auto-close-policy.md`で「自動クローズはせず、必ず人間の承認を挟む」方針が
既に決定済み。今回の変更は承認前の初期選択状態をAIが提案するのみで、承認操作自体は引き続き
人間が行うため、この方針とは矛盾しない。ただし同ADRの「JIRAの誤クローズは気づかれにくく
実害が大きい」というリスク認識は今回の設計でも考慮した。

## 方針案

| 案 | 内容 | コスト | リスク | 状態 |
|---|---|---|---|---|
| 案A | 抽出時（Mattermost extractorの`Classify`呼び出し内）でオープンタスク一覧をプロンプトに含めて推定させ、結果を`candidates`新規列(`suggested_task_id`)に保存する | 中（プロンプト拡張・DB列追加・sqlc再生成） | LLMの誤推定を人間がそのまま承認してしまう可能性（承認操作自体は人間が行うため許容） | **採用** |
| 案B | 承認画面表示時に文字列類似度等の軽量ヒューリスティックでその場計算（LLM呼び出し無し、DB変更無し） | 低 | 「AIで判断して」という要望から外れる、スレッド文脈が使えず要約テキストのみの単純比較になり精度が低い | 不採用 |
| 案C | 承認画面表示時に都度ローカルLLMへ問い合わせて推定 | 中 | ボード再描画のたびにLLM呼び出しが増え、GPU競合（ComfyUIと排他利用）・遅延が悪化する | 不採用 |

## 採用理由 / 検討経緯

grillingスキルで確認。「AIで判断して」という要望に忠実であり、スレッド全文という情報量が
最も多い文脈を使える点、既存のextractorパイプライン（スレッド単位で1回LLM呼び出し）に
オープンタスク一覧を足すだけで済み画面表示のたびの追加コストが発生しない点から案Aを採用した。

あわせて次の3点をユーザーに確認済み:

- **UI表現**: 「(AI推定)」等の特別なラベル・色分けはせず、`<select>`の初期選択状態として
  推定タスクを選んでおくだけにする（人間は変更可能、承認ボタンは引き続き必須の人間操作）。
- **確信が持てない場合**: LLMは空文字を返し、従来通りプレースホルダ「対象タスクを選択」の
  ままになる（既存の`assignee_raw`/`due_date`等と同じ「不明なら空欄」方針を踏襲）。
- **既存データへの遡及適用**: 実装時点で残っている未承認completion候補（2026-09-13時点で
  0件）へのバックフィルは行わず、今後新規に検知される分からのみ適用する。

## 関連する設計ドキュメント

- `docs/design/screen-close-requests.md`（本画面の仕様。実装後にこの機能を反映する）
- `docs/adr/complete/completion-approval-ui.md`（本画面の方式=C3案を採用した経緯）
- `docs/adr/complete/auto-close-policy.md`（「自動クローズしない」方針そのものの決定経緯）
- `docs/adr/complete/mattermost-extractor-registration-flow.md`・
  `docs/adr/complete/mattermost-extractor-llm-choice.md`（Mattermost extractorの既存設計）

## 結論

`internal/taskstore`（schema.sql/store.go/query.sql）・`internal/mattermost`（llm.go/collector.go）・
`internal/web`（kanban.go/board.html.tmpl）へ実装完了。`sqlc generate`→`go build`→`go vet`で
確認済み。実機（llama-swap）に対する一時的な検証テストで`related_task_id`が未クローズタスク
一覧の中から正しく推定されることを確認済み（確認後にテストは削除）。
