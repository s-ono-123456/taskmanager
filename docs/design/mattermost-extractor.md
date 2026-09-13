# Mattermost extractor（収集+ローカルLLMによる取得時分類・自動タスク登録）

> DBスキーマ・ER図は`docs/design/data-model.md`を参照（本書では重複させない）。全体構想
> における位置づけは`docs/design/automation-roadmap.md`参照。

## 位置づけ

「外部通信は原則行わない」という本パイロットの既定方針を、Mattermostに限り覆し、実際に
Mattermost APIをポーリングして取得した投稿を**取得時に即座にローカルLLMで分類し**、
意味のあるものだけを残す（`internal/mattermost/`。当初は「収集のみ」で実装したが、
実運用開始後「メッセージを全部保存するだけでは意味がない」との指摘を受けて拡張した。
比較検討の経緯は`docs/adr/complete/mattermost-collector-scope.md`・`mattermost-collector-language.md`・
`mattermost-extractor-llm-choice.md`・`mattermost-extractor-registration-flow.md`・
`mattermost-extractor-batching.md`・`mattermost-message-retention.md`参照）。

メール/Zoom分の同等機能（未実装）は`docs/design/mail-zoom-pipeline.md`を参照。Mattermost分は
そちらの構想とは異なる方式（取得時に都度分類、ローカルLLM使用）で実装済み。

## 仕様

- **収集**: `internal/mattermost/collector.go`の常駐goroutineが**10分間隔**
  （`PollInterval`定数）でポーリングする（トリガー方式の比較検討経緯は
  `docs/adr/complete/collection-trigger.md`参照）。監視対象チャンネル（複数可）とその
  `project_hint`（jira_a/jira_b/personal）は環境変数`MATTERMOST_CHANNEL_ROUTES`
  （例: `chID1:jira_a,chID2:jira_b`）でチャンネルごとに指定する。
- **AI**: Claude API等の外部LLMには接続せず、このホスト上に既に稼働しているローカルLLM
  （llama-swap、OpenAI互換API）を使う。デフォルトモデルは`qwen3.8-flash-next-q5`、
  環境変数`MATTERMOST_EXTRACTOR_LLM_URL`（既定`http://host.docker.internal:8080`）・
  `MATTERMOST_EXTRACTOR_LLM_MODEL`で変更可能。ComfyUIと同一GPUを排他利用しており、
  抽出処理実行時にComfyUI生成ジョブが強制停止されうるが、これは許容する方針（回避ロジックは
  実装しない）。
- **バッチ化**: 新着投稿をスレッド単位でグループ化し、同一スレッド内に複数の新着があれば
  スレッド全文（`GET /api/v4/posts/{postID}/thread`）を文脈としてまとめて1回のLLM呼び出しで
  分類する（JSON配列で結果を受け取る）。
- **登録フロー**: `kind=task`かつ`target`が確定（`jira_a`/`jira_b`/`personal`のいずれか）
  していれば、その場で`tasks`へ自動登録する（`candidates.human_verdict`は
  `'auto_registered'`）。`target`不明の`kind=task`は「タスク候補一覧」画面
  （`docs/design/screen-task-candidates.md`参照）で人間が承認/却下する。`kind=completion`は
  既存の「クローズ要求一覧」画面が参照する。
- **対象タスクのAI推定**: `kind=completion`かつ`related_jira_key`が本文から抽出できない
  候補について、同じ`Classify`呼び出しに`project_hint`の未クローズタスク一覧(id+title)を
  あわせて渡し、対象タスクを一意に推定できれば`candidates.suggested_task_id`に保存する
  （確信が持てなければ空のまま）。クローズ要求一覧の`<select>`の初期選択肢として使うのみで、
  承認操作自体は引き続き人間が行う（自動クローズはしない）。
  `docs/adr/complete/close-request-target-task-suggestion.md`参照。
- **保存方針**: `kind=none`または信頼度がしきい値未満（`MinCandidateConfidence`、目安0.3）の
  投稿は`messages`テーブルに一切保存しない（破棄）。候補化・タスク化された投稿のみ保存し、
  `permalink_url`も記録することで、既存の「元発言」表示（`tasks.source_message_id`経由の
  JOIN、編集モーダル）でURL・本文をそのまま確認できる。confidenceの値自体はローカルLLMの
  自己申告値であり、別途こちら側で検証・補正するロジックは無い。
- **カーソル管理**: `messages`テーブルへの依存をやめ、`mattermost_channel_state`テーブルで
  チャンネルごとの最終処理位置（取得できた投稿の最大`create_at`、分類結果に関わらず更新）を
  保持する。
- **原子性**: `messages`保存・`candidates`保存・(自動登録時の)`tasks`作成は`registerCandidate`
  内で1トランザクション（`db.BeginTx`+`WithTx`）にまとめている。分けて実行すると、途中で
  処理が中断された場合に`messages`だけが保存され`candidates`が作られない孤立レコードが生じ、
  重複防止チェック（`MessageExistsBySourceID`）により二度と再分類されなくなる不具合が
  実データで発生したための対応（`docs/work-log.md` 2026-09-13「Mattermost extractorの
  孤立メッセージ不具合を修正」参照）。
- **起動シーケンス**: 初回キャッチアップ実行は`main.go`の起動処理をブロックしないよう
  **非同期（goroutine内）**で行う。ローカルLLMでのスレッド単位の分類は逐次実行のため、
  未処理分がまとまっていると実測で数分単位の時間がかかることがあり、Cyclesの
  ロールオーバーのような軽量な同期実行には適さないため。
- 重複防止は`source`+`source_id`（MattermostのPost ID）の存在チェックで行う。

## 関連ドキュメント

- `docs/design/data-model.md` — `messages`/`candidates`/`mattermost_channel_state`の
  スキーマ・ER図。
- `docs/design/automation-roadmap.md` — 全体構想における位置づけ。
- `docs/design/mail-zoom-pipeline.md` — メール/Zoom分の同等機能（未実装、方式が異なる）。
- `docs/design/screen-close-requests.md` / `docs/design/screen-task-candidates.md` —
  抽出結果を承認するダッシュボード側の画面仕様。
- `docs/adr/complete/mattermost-collector-scope.md` / `mattermost-collector-language.md` /
  `mattermost-extractor-llm-choice.md` / `mattermost-extractor-registration-flow.md` /
  `mattermost-extractor-batching.md` / `mattermost-message-retention.md` /
  `close-request-target-task-suggestion.md` — 各設計判断の経緯。
