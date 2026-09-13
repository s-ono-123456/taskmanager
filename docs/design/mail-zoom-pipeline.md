# メール/Zoom収集・抽出・登録パイプライン（未実装）

> 全体像・背景・リスクは`docs/design/automation-roadmap.md`を参照（本書では重複させない）。
> Mattermost分は実装済みのため対象外（`docs/design/mattermost-extractor.md`・
> `docs/design/screen-close-requests.md`参照）。

## 位置づけ

本書は、タスク管理自動化構想のうち**まだ実装されていない、メール/Zoom分のcollector→
extractor→registrar→完了候補提示**の確定設計をまとめたものである。Mattermost分は実装済み
（`internal/mattermost/`）だが方式が異なる部分があり、その差異は
`docs/design/automation-roadmap.md`「全体パイプライン」節を参照。

## 収集（収集源ごと）

トリガー方式は[収集トリガー方式](../adr/complete/collection-trigger.md)で
採用したcron定期ポーリング（Mattermost分は`internal/mattermost.PollInterval`=10分間隔で
実装済み、`docs/design/mattermost-extractor.md`参照）。

- **メール**: IMAPで監視対象フォルダ/ラベルをcronポーリング。フォルダごとに`project_hint`を設定。
- **Zoom**: cronでZoom API（Server-to-Server OAuth）を使い、終了済み会議一覧を取得。Zoom AI
  Companionの会議要約API（overview・next steps/action items）を取得し、要約全文を1件の
  `messages`（source=zoom）として保存する。会議トピック名・シリーズ名から`project_hint`を推定
  する。Zoom自身が抽出したaction itemsをextractorの入力に含めることで、ゼロから発言を解析する
  より精度を上げやすい。

`messages.project_hint`は収集元の設定（`project_routing`）から機械的に付与する「対象
プロジェクトの手がかり」。`project_routing`自体はDBではなく設定ファイルで管理するため、
`docs/design/data-model.md`のER図には含めていない（Mattermost分は`MATTERMOST_CHANNEL_ROUTES`
環境変数でチャンネルごとに指定する方式で実装済み。メール/Zoom分は同様にフォルダ/会議トピック
単位の設定を別途用意する）。

## 抽出・分類

外部LLM APIへの送信可否は[外部LLM API送信可否](../adr/proposals/data-handling-policy.md)
で確認済み（メール/Zoom想定、Claude API等の外部LLM向け。Mattermost分はこの確認の対象外で
別途ローカルLLMを使う）。

`messages`1件ごとにClaude APIで構造化抽出し`candidates`へ格納する。
`kind/confidence/target/assignee_raw/due_date/summary/related_jira_key`を構造化JSONで抽出する。
`target`は`project_hint`を手がかりに使うが、本文の内容と矛盾する場合は本文を優先し
`confidence`を下げさせる。不明な項目は断定させず`unknown`/空文字のままにする
（複数プロジェクトが混在するチャンネル/メールフォルダでは`project_hint`だけでは判定が
弱いため、本文内容を優先しつつconfidenceで保留に倒す設計としている）。

## 登録（Registrar）

- **JIRA登録**: `target`が`jira_a`/`jira_b`で確定した候補はJIRA REST API
  （`POST /rest/api/2/issue`）で起票。descriptionに元発言/メール/会議へのリンクと引用を残す。
  `assignee`が解決できていれば設定し、できていなければ未設定で起票（担当者未設定の起票がある旨
  は日次まとめ（`docs/design/digest.md`）に含める）。
- **対象不明（`target=unknown`）**: 自動登録せず、`candidates`に保留のまま残し、日次まとめ
  （`docs/design/digest.md`）経由で人間に確認させる。

## 完了候補提示・クローズ

完了報告らしき発言を検知した場合、対象タスクの特定を行う。

1. 発言内にJIRAキー（例: `PROJ-123`）が明示されていればそれをそのまま`related_jira_key`とする。
2. 明示がない場合、「未クローズタスク一覧」（`tracked=1`のもののみ、`docs/design/jira-sync.md`
   「追跡フラグとの関係」参照）をLLMに提示して対象を推定させる（Mattermost分は実装済みの
   同種のロジックがあり、`docs/design/screen-close-requests.md`「対象タスクのAI推定」参照。
   メール/Zoom分の実装時、同じ方式を踏襲するか改めて検討する）。

クローズ候補（`candidates`のうち`kind=completion`かつ`human_verdict`が未設定の行）は、
ダッシュボードの「クローズ要求一覧」画面に随時蓄積して表示する（Mattermost分と同じ画面・
同じ業務ルールを使う想定、`docs/design/screen-close-requests.md`参照。本書では重複させない）。

## 技術スタック

未実装のメール/Zoom collector・extractor・registrarについては、引き続き以下の想定
（Python、リポジトリの既存方針どおりルートの`.venv`/uv環境を使用、`sqlite3`標準
ライブラリ、`requests`でJIRA/Zoom各API呼び出し（ZoomはServer-to-Server OAuth）、`anthropic`
SDKでClaude API呼び出し、Dockerコンテナ＋cronで定期実行）を置くが、Mattermost collectorの
実装経験（`docs/adr/complete/mattermost-collector-language.md`参照）を踏まえるとこれらも
Goで実装できる可能性があり、着手時に改めて技術選定を見直す余地がある。

## 関連ドキュメント

- `docs/design/automation-roadmap.md` — 全体構想・背景・リスク・ロードマップ。
- `docs/design/data-model.md` — DBスキーマ・ER図。
- `docs/design/mattermost-extractor.md` — 実装済みのMattermost collector/extractorの確定仕様。
- `docs/design/jira-sync.md` — JIRA同期方針（syncer、未実装）。
- `docs/design/digest.md` — 日次まとめ（digest、未実装）。
- `docs/design/screen-close-requests.md` / `docs/design/screen-task-candidates.md` —
  ダッシュボード側の承認UI（Mattermost分と共用想定）。
- `docs/adr/proposals/data-handling-policy.md` — 外部LLM API送信可否の決定経緯。
- `docs/adr/complete/collection-trigger.md` — 収集トリガー方式の決定経緯。
