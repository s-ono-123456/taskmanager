# 論点: Mattermost連携の実装スコープ

- 起票: 2026-09-13 / タスク管理自動化構想の一部（`docs/design/task-management-automation.md` 参照）
- 論点: 「Mattermost連携機能」の要望に対し、`collector`/`extractor`/`registrar`のうちどこまでを
  今回実装するか。
- 状態: **採用済み・実装完了（2026-09-13）**

## 背景

`docs/design/task-management-automation.md`には収集(collector)→抽出(extractor、Claude API)→
登録(registrar、JIRA API)までの全体パイプラインが設計済みだが未実装。本リポジトリ
（taskmanager）は「外部通信は一切行わない（Mattermost/メール/Zoom/JIRA/Claude APIいずれにも
接続しない）」というCLAUDE.md/design.mdに明記の核となる方針を持つ。今回「Mattermost連携」を
実装することは、このうち「Mattermost」への接続に関して方針転換することを意味するが、
Claude API・JIRA APIへの接続可否は別問題として扱う必要がある。

## 方針案

| 案 | 内容 | コスト | リスク | 状態 |
|---|---|---|---|---|
| 案1 | 収集(collector)のみ: Mattermost APIでチャンネルをポーリングし`messages`テーブルへ保存するところまで。抽出・JIRA自動起票・完了候補推定は対象外 | 低〜中 | 蓄積したメッセージから先(タスク化)は当面手動のまま | **採用** |
| 案2 | 収集+抽出+登録のフルパイプライン(Claude API・JIRA API利用まで一括実装) | 高 | 一度に「外部通信ゼロ」「Claude APIには接続しない」の両方針を覆すことになり、検証範囲・障害時の切り分けが困難。認証情報もMattermost bot tokenに加えAnthropic APIキー・JIRA APIトークンが同時に必要になる | 不採用(将来の拡張候補) |

## 採用理由 / 検討経緯

grillingスキルでの確認により、段階的な範囲拡大（まず収集のみで安定稼働させ、抽出・自動登録は
別途着手）を採用した。将来的にAI（Claude API）を用いた完了判断・タスク抽出を追加する意向は
明示されており（Anthropic公式Go SDK `github.com/anthropics/anthropic-sdk-go` の存在を確認済み、
実装言語の選定は`docs/adr/complete/mattermost-collector-language.md`参照）、案2への拡張は
今回の実装（収集のみ）を土台に別途行う。

## 関連する設計ドキュメント

- `docs/design/task-management-automation.md`（全体パイプライン設計）
- `docs/adr/complete/collection-trigger.md`（収集トリガー方式=cron定期ポーリング、既に決定済み。
  今回の実装で実際に使用する）
