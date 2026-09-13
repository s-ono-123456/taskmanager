# 論点: ダッシュボードの実装技術

- 起票: 2026-09-12 / タスク管理自動化構想の一部（`docs/design/automation-roadmap.md` 参照）
- 論点: タスク管理ダッシュボード（Webアプリ）をどの技術スタックで実装するか。
- 状態: **採用済み（D2、2026-09-12、パイロット実装で確定）**

## 方針案

| 案 | 内容 | コスト | リスク | 状態 |
|---|---|---|---|---|
| D1 | FastAPI＋簡易フロントエンド（Python） | 低（自動化パイプライン本体と同じPython資産・ライブラリを共有できる） | 実行環境が重くなりがち（イメージサイズ・依存関係） | 不採用 |
| D2 | Go（標準`net/http`のServeMux + `html/template`）+ [sqlc](https://sqlc.dev/) + [htmx](https://htmx.org/) | 中（Python資産とは別に言語・ツールチェインを増やすことになる） | 自動化パイプライン（collector/extractor/syncer/registrar）とダッシュボードで実装言語が分かれる | **採用**（2026-09-12） |

## 採用理由 / 検討経緯

パイロット実装（スキーマ＋ダッシュボードUI）を進める中で、「軽量さ」（単一バイナリ・最終
イメージ30MB台）と「SQLとロジックの分離」を重視してD2（Go + sqlc + htmx）を採用した。
自動化パイプライン（collector/extractor/syncer/registrar）とは実装言語が分かれる形になるが、
ダッシュボード単体としての軽量さ・保守性を優先した。

当初はPython/Flaskで実装し、その後Goへ全面移行している。経緯の詳細は
`/work/public/taskmanager/docs/work-log.md`を参照。

## 関連する設計ドキュメント

- `docs/design/design.md`（Go + sqlc + htmxによるパイロット実装の詳細設計）
