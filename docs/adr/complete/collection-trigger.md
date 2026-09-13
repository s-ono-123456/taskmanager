# 論点: 収集トリガー方式

- 起票: 2026-09-12 / タスク管理自動化構想の一部（`docs/design/automation-roadmap.md` 参照）
- 論点: Mattermost/メールからのタスク自動収集を、どのタイミング・方式でトリガーするか。
- 状態: **採用済み・実装完了（B1、2026-09-12決定・2026-09-13にMattermost collectorで実装。
  `internal/mattermost.PollInterval = 10 * time.Minute`）**

## 方針案

| 案 | 内容 | コスト | リスク | 状態 |
|---|---|---|---|---|
| B1 | cronでの定期ポーリング（Mattermost API/IMAPを5〜15分間隔で取得） | 低 | リアルタイム性はやや低い | **採用**（2026-09-12） |
| B2 | Mattermost outgoing webhook等でのイベント即時受信 | 中（受信用エンドポイントが必要） | 常時稼働の受信サービスが必要になる | 不採用 |

## 採用理由 / 検討経緯

タスク管理用途ではリアルタイム性より実装・運用コストの低さを優先し、B1を採用した。
常時稼働の受信サービスを持たずに済むため、Dockerコンテナ＋cronという既存の定期実行環境
だけで完結できる。

## 関連する設計ドキュメント

- `docs/design/data-model.md`（「Mattermost extractor」節、Mattermost分は実装済み）
- `docs/design/mail-zoom-pipeline.md`（収集: メール/Zoomの各仕様、未実装）
