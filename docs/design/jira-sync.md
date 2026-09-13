# JIRA同期方針（syncer、未実装）

> 全体像・背景は`docs/design/automation-roadmap.md`を参照（本書では重複させない）。

## 位置づけ

本書は、タスク管理自動化構想のうち**まだ実装されていない、JIRA連携タスクの同期処理
（syncer）**の確定設計をまとめたものである。

## データの実体・同期方針

- **JIRA連携タスク**（`tasks.jira_key`が値を持つ行）は**JIRAが唯一の正**。ローカルの
  `title`/`description`/`status`等はJIRAの状態を映した**キャッシュ**として扱い、崩れてもJIRAから
  作り直せるものと位置づける。
- **syncer**（cron、収集と同程度の間隔・目安10〜15分毎）が追跡中の`jira_key`一覧をJIRA API
  （バッチ取得: JQL `key in (...)`）で問い合わせ、ローカルの`tasks`行へ反映し
  `last_synced_at`を更新する。
- ダッシュボードは通常ローカルDBを読んで高速表示し、「今すぐ更新」操作で対象タスクのみ即時に
  JIRAへ再取得できるようにする。
- ダッシュボードからの編集（title/description/due_date等）はローカルのみで完結させず、JIRA API
  へ書き込んだ上でローカルキャッシュも更新する（write-through。ローカルとJIRAの内容が
  ズレないようにする）。
- **個人タスク**（`jira_key`がNULLの行）は他に正となる外部システムが存在しないため、ローカルDBが
  そのまま正データとなる（syncer・write-through の対象外）。

`tasks.jira_key`はJIRA起票済みなら値あり、個人タスクはNULLのまま自前ストアの実体となる
（`docs/design/data-model.md`のER図参照）。

## 追跡フラグ（`tracked`）との関係

[「追跡除外」操作の範囲](../adr/complete/delete-vs-hide.md)で採用した方針により、
`tasks.tracked`（デフォルト`true`）を`false`にすると、UI上の非表示に加えて次の効果を持つ
（UI側の業務ルール・実装詳細は`docs/design/screen-board.md`「非表示」節参照。本書では
syncerとの関係のみ扱う）。

- `tracked=false`のJIRA連携タスクはsyncerの同期対象から外れる（無駄なJIRA API呼び出しを
  減らせる）。syncer自体が未実装のため現時点では効果を持たないが、実装時にこの条件を
  組み込む。
- 完了候補の対象特定（「未クローズタスク一覧」から対象を推定する処理）の候補からも除外する
  （関係ない古いタスクが完了候補として誤って挙がるのを防ぐ）。Mattermost分は
  `ListOpenTasksByTarget`が`tracked = 1`条件で既に絞り込んでおり実装済み
  （`docs/design/screen-close-requests.md`参照）。

## 関連ドキュメント

- `docs/design/automation-roadmap.md` — 全体構想・背景・リスク・ロードマップ。
- `docs/design/data-model.md` — `tasks`テーブルのスキーマ（`jira_key`/`last_synced_at`/`tracked`）。
- `docs/design/screen-board.md` — 「非表示」のUI側業務ルール（`tracked`フラグの反転）。
- `docs/adr/complete/delete-vs-hide.md` — 「追跡除外」操作の範囲を決定した経緯。
