# 論点: Mattermost extractorで使うAI(LLM)の選定

- 起票: 2026-09-13 / タスク管理自動化構想の一部（`docs/design/automation-roadmap.md` 参照）
- 論点: Mattermost収集メッセージを取得時に分類・タスク化するextractorで、どのLLMに接続するか。
- 状態: **採用済み・実装完了（2026-09-13）**

## 背景

当時の設計（現`docs/design/mail-zoom-pipeline.md`「抽出・分類」節）はextractorをClaude API
（Anthropic）で実装する想定だったが、CLAUDE.md/design.mdには「Claude APIへは引き続き一切接続しない」という方針が
明記されている。一方、このホスト環境には`/work/docker/llama-swap/`で構築済みのローカルLLM
基盤（llama-swap、OpenAI互換API、ポート8080、host network）が既に稼働しており、
`qwen3.8-27b`/`qwen3.8-flash-next`/`qwen3.8-flash-next-iq2`/`qwen3.8-flash-next-q5`の
4モデルが利用可能（`curl http://127.0.0.1:8080/v1/models`で確認済み）。ただし、この基盤は
ComfyUI（画像/動画生成）と同一GPUを排他利用しており（`exclusive: true`）、どちらかを起動すると
他方が強制停止される。

## 方針案

| 案 | 内容 | コスト | リスク | 状態 |
|---|---|---|---|---|
| 案1 | Claude API（既存設計通り） | 低（設計済み） | 「Claude APIには接続しない」という明記済みの方針をさらに転換する必要がある。外部通信・APIコストも発生 | 不採用 |
| 案2 | ローカルLLM（llama-swap経由） | 低〜中（OpenAI互換APIのため既存のGoコードで対応しやすいが、GPU競合の考慮が要る） | ComfyUI生成ジョブと同一GPUを共有するため、抽出処理実行時に生成ジョブが強制停止されうる | **採用** |

GPU競合について: 「ComfyUI稼働中は抽出処理をスキップする」という回避策も検討したが、
ユーザーの判断で**許容する（回避ロジックは実装しない）**ことになった。

## 採用理由 / 検討経緯

ユーザーから「Claude APIではなくローカルLLMに接続する」という明確な意向が示された。これにより
「Claude APIには接続しない」という既存方針を維持したまま、AIによる分類・抽出を実現できる。
使用モデルはデフォルト`qwen3.8-flash-next-q5`（呼び出し時点で既にロード済みのため追加の
コールドロードが発生しない）とし、環境変数`MATTERMOST_EXTRACTOR_LLM_MODEL`で変更可能にする。
GPU競合（ComfyUIジョブの強制停止）は許容することとした。

## 関連する設計ドキュメント

- `docs/design/mail-zoom-pipeline.md`（技術スタック節、メール/Zoom分のextractor実装方式）
- `docs/design/mattermost-extractor.md`（実装済みのLLM選定結果）
