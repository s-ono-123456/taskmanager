# 日次まとめ（digest、未実装）

> 全体像・背景は`docs/design/automation-roadmap.md`を参照（本書では重複させない）。

## 位置づけ

本書は、タスク管理自動化構想のうち**まだ実装されていない、Mattermostへの日次まとめ投稿
（digest）**の確定設計をまとめたものである。クローズ候補の承認はダッシュボード側
（クローズ要求一覧、[完了候補の承認UI](../adr/complete/completion-approval-ui.md)で採用した
C3）で行うため、digestには含めない。

## 構成

1メッセージの中で以下をカテゴリ分けして提示する。

1. 新規登録済みタスク（JIRA/個人、当日分）
2. 対象不明のため保留中のタスク候補（人間の判定待ち）
3. 完了報告はあったが対象タスク不明（手動対応が必要。メール/Zoom分。Mattermost分は
   `related_jira_key`の有無に関わらずクローズ要求一覧に表示するため対象外、
   `docs/design/screen-close-requests.md`参照）

`user_map`（発言者⇔JIRAアカウントの対応）が未整備の担当者は、抽出時にassignee未設定のまま
登録し、この日次まとめで人間に確認する想定。

## 関連ドキュメント

- `docs/design/automation-roadmap.md` — 全体構想・背景・リスク・ロードマップ。
- `docs/design/mail-zoom-pipeline.md` — 日次まとめに集約される、メール/Zoom分の登録・
  完了候補提示の確定設計。
- `docs/design/screen-close-requests.md` — クローズ候補の承認UI（digestとは切り離されている）。
- `docs/adr/complete/completion-approval-ui.md` — クローズ候補の承認方式（C3）の決定経緯。
