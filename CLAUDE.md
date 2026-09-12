# taskmanager (task-dashboard パイロット実装)

## プロジェクト概要

タスク管理自動化構想（JIRA2プロジェクト＋個人タスクをMattermost/メール/Zoomから
自動収集し、JIRA自動起票・完了候補提示まで行う）のうち、「スキーマとダッシュボード
UIのパイロット実装」に相当するリポジトリ。2026-09-12に`/work`（メインリポジトリ）の
`docker/task-dashboard/`からこの独立リポジトリへ切り出された。

**外部通信は一切行わない。** Mattermost/メール/Zoom/JIRA/Claude APIいずれにも
接続せず、JIRA連携相当の操作はすべて`stub_jira_transition()`によるログ出力のみ。

全体構想（本パイロットが将来どう拡張される想定か）は本リポジトリ内の
`docs/adr/proposals/task-management-automation.md`を参照（2026-09-12、ADR自体も
`/work`からこのリポジトリへ移動済み）。

## ディレクトリ構成

```
build/Dockerfile          # python:3.13-slim + flask、templates同梱
compose/docker-compose.yml
db.py                     # SQLiteスキーマ定義・接続ヘルパー
seed.py                   # サンプルデータ投入（再実行可能・全件作り直し）
app.py                    # Flaskアプリ本体（ルーティング・業務ロジック）
templates/board.html      # 唯一のテンプレート（Kanbanボード＋モーダル2種）
docs/design.md            # 詳細設計書（データモデル・画面仕様・ルート一覧を網羅）
```

## 技術スタック・依存関係

- Python標準ライブラリ（`sqlite3`）+ Flask + Jinja2 + Tailwind CSS（CDN読み込み）。
- JS側もフレームワークなし（素のDOM操作・HTML5 Drag and Drop API・`<dialog>`要素）。
- 依存パッケージは`flask`のみ。`pyproject.toml`/`requirements.txt`は無く、
  `build/Dockerfile`内で直接`pip install flask`している。
- リポジトリ直下に`.venv`は未整備（`app.py`冒頭コメントに`uv run python app.py`と
  あるが、uv管理ファイルは無いため、実行時は素の`python`/`pip`で構わない）。

## 実行方法

- ローカル実行: `python app.py`
  - 既定では`127.0.0.1`のみにバインド（外部公開しない前提）。
  - 環境変数`TASK_DASHBOARD_HOST`/`TASK_DASHBOARD_PORT`/`TASK_DASHBOARD_DB_PATH`/
    `TASK_DASHBOARD_AUTO_SEED`（`1`でDBが空の時のみ自動シード）で上書き可能。
- サンプルデータ投入: `python seed.py`（再実行すると全件作り直し）。
- Docker実行: `docker compose -f compose/docker-compose.yml up -d --build`
  - ポート8090、実データ(SQLite)は`/docker/task-dashboard/data`
    （このリポジトリの外・gitの管理対象外）にボリュームマウントされる。

## 誤解しやすい業務ルール（詳細は`docs/design.md`参照）

- **完了レーンは直近7日以内に完了(`closed_at`)したタスクのみ表示**する
  （`DONE_LANE_WINDOW_DAYS`）。7日を超えても データは残り続け、
  「非表示分も表示」をONにしても表示されない（削除ではなく表示上のフィルタ）。
- **「非表示」＝`tracked`フラグの反転のみ。** 削除機能は存在しない。
  非表示にする際、対象が`done`でなければ同時に`status=done`・`closed_at`を
  設定する（＝一旦完了扱いにする）。
- ドラッグ&ドロップのドロップ先レーン判定は**ポインタのx座標のみ**で行う
  （y座標・列の高さは考慮しない）。
- JIRA連携（`target=jira_a`/`jira_b`）タスクの状態変化時は`stub_jira_transition()`
  を呼ぶが、実際のHTTP通信は発生しない。

## 関連ドキュメント

- `docs/design.md`（本リポジトリ内） — データモデル・画面仕様・ルート一覧・
  デプロイ構成・既知の制限を網羅した詳細設計書。実装を変更する際は必ず参照し、
  変更があれば追記すること。
- `docs/adr/proposals/task-management-automation.md`（本リポジトリ内） — 全体構想のADR
  （データモデル・パイプライン全体像。まだ未実装のcollector/extractor/syncer/registrar
  含む）。
- 以下は`/work`（メインリポジトリ、このリポジトリとは別）側にあり、直接は参照不可:
  - `docs/work-log.md` / `docs/task-queue.md` — 進捗管理・作業経緯の記録
    （タスク管理は切り出し後もこのリポジトリではなく`/work`側で行う）
