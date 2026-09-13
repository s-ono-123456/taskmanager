# 論点: タスクの進捗管理粒度（`tasks.status`）

- 起票: 2026-09-12 / タスク管理自動化構想の一部（`docs/design/task-management-automation.md` 参照）
- 論点: `tasks.status`をどの粒度で持つか（進捗の途中経過を画面上でどこまで区別するか）。
- 状態: **採用済み（E2、2026-09-12、パイロット実装で確定）**

## 方針案

| 案 | 内容 | コスト | リスク | 状態 |
|---|---|---|---|---|
| E1 | `open`/`done`の2値 | 低 | 「着手中」「レビュー中」等の途中経過を画面上で区別できない | 不採用 |
| E2 | `todo`/`in_progress`/`reviewing`/`done`の4値カンバン | 低（値の種類が増えるのみ、移行コストは小さい） | 状態遷移のバリエーションが増える分、ロジック（`closed_at`設定タイミング等）がやや複雑になる | **採用**（2026-09-12） |

## 採用理由 / 検討経緯

画面をカンバン形式にし、ドラッグ&ドロップで状態遷移できるようにするため、2値ではなく
4値のカンバン粒度（E2）を採用した。状態遷移パターンが増える分のロジックの複雑さは、
`closed_at`の設定/クリアを共通関数（`closedAtForTransition`）に集約することで吸収している。

## 関連する設計ドキュメント

- `docs/design/design.md`（`internal/web/kanban.go`の`Statuses`/`StatusLabels`、
  `closedAtForTransition`の実装）
- `docs/design/task-management-automation.md`（データモデルER図の`tasks.status`）
