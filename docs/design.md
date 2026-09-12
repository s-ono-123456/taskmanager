# task-dashboard 設計書（パイロット実装）

## 位置づけ

本ドキュメントはこのリポジトリ（`taskmanager`）の**現在の実装**を仕様としてまとめたものである。
2026-09-12に `/work`（メインリポジトリ）の `docker/task-dashboard/` から独立した別リポジトリ
`/work/public/taskmanager` へ移動した。
全体構想（JIRA2プロジェクト＋個人タスクをMattermost/メール/Zoomから自動収集し、JIRA自動起票・
完了候補提示まで行う）は `/work`（メインリポジトリ）の
`docs/adr/proposals/task-management-automation.md` を参照。
本サービスはそのうち「スキーマとダッシュボードUIのパイロット実装」に相当し、**外部通信は
一切行わない**（Mattermost/メール/Zoom/JIRA/Claude APIいずれにも接続しない。JIRA連携相当の
操作はすべてログ出力のみのスタブ）。

## 全体構成

```
(このリポジトリのルート)
├── build/Dockerfile          # python:3.13-slim + flask、templates同梱
├── compose/docker-compose.yml
├── db.py                     # SQLiteスキーマ定義・接続ヘルパー
├── seed.py                   # サンプルデータ投入（再実行可能・全件作り直し）
├── app.py                    # Flaskアプリ本体（ルーティング・業務ロジック）
├── templates/board.html      # 唯一のテンプレート（Kanbanボード＋モーダル2種）
└── docs/design.md            # 本書
```

技術スタック: Python標準ライブラリ（`sqlite3`）+ Flask + Jinja2 + Tailwind CSS（CDN）。
JS側もフレームワークなし（素のDOM操作・HTML5 Drag and Drop API・`<dialog>`要素）。

## データモデル（`db.py`）

| テーブル | 役割 | 主な列 |
|---|---|---|
| `messages` | 収集した生メッセージ（本パイロットではseed.pyの固定サンプルのみ） | `source`(mattermost/email/zoom), `channel_or_meeting`, `author`, `text`, `received_at`, `project_hint` |
| `candidates` | メッセージから抽出したタスク/完了候補（本パイロットでは表示画面からは未使用、将来のextractor実装用に器のみ用意） | `kind`(task/completion), `confidence`, `target`, `due_date`, `human_verdict` |
| `tasks` | ダッシュボードが実際に読み書きする本体 | `title`, `description`, `target`(jira_a/jira_b/personal), `status`(todo/in_progress/done), `jira_key`, `created_at`, `closed_at`, `last_synced_at`, `tracked`(0/1), `due_date`(YYYY-MM-DD、nullable) |
| `user_map` | 発言者⇔JIRAアカウントの対応（本パイロットでは表示画面からは未使用） | `source`, `source_user_id`, `jira_account_id`, `display_name` |

- `init_db()` は起動のたびに呼ばれ、`CREATE TABLE IF NOT EXISTS` に加えて旧スキーマ
  （`status`が`open`/`done`の2値だった時代のデータ）を`todo`へ寄せるマイグレーションを実行する。
- `tracked`（真偽値）は「非表示」の実体。削除ではなくこのフラグの反転のみで、行は物理的には
  常に残る。
- `due_date`は既存DBに対しては`init_db()`内で`PRAGMA table_info`を見て無ければ
  `ALTER TABLE ADD COLUMN`するマイグレーションで追加している（2026-09-12追加）。

## 画面仕様（`GET /` → `board.html`）

### レーン構成
- 4レーン固定: `未着手`(todo) / `進行中`(in_progress) / `確認中`(reviewing) / `完了`(done)。
  `status`列はCHECK制約の無い自由文字列のため、レーン追加は`app.py`の`STATUSES`/
  `STATUS_LABELS`と`board.html`の`accent`辞書に値を足すだけで済む。
- **完了レーンは直近7日以内に完了したタスクのみ表示**する
  （`DONE_LANE_WINDOW_DAYS = 7`、`is_recently_closed(closed_at)`で判定）。7日を超えたものは
  データとしては残るが、「非表示分も表示」をONにしても一切表示されない（無制限に膨らむのを防ぐ
  ための表示上のフィルタであり、削除ではない）。

### フィルタ（ツールバー）
- 対象(`target`)プルダウンと「非表示分も表示」(`show_untracked`)チェックボックスは、
  どちらも`onchange`で自フォームを即時submitする（絞り込みボタンは無い）。
- `show_untracked`がOFFの場合、SQLクエリ側で`tracked = 1`のみに絞り込む。ONの場合は
  `tracked`に関わらず全件取得したうえで、完了レーンの7日フィルタだけは常に適用される。

### カード
- クリックすると編集モーダル（詳細表示＋編集を兼ねる）を開く。カード上のdata属性
  （`data-title`等）からモーダルへ値を流し込む方式で、サーバー往復なしに即座に開く。
- `draggable="true"`。ドラッグ&ドロップで別レーンへ移動できる（後述）。
- レイアウトは2行構成（タイトル行＋対象タグ(`personal`/`jira_a`/`jira_b`)/非表示バッジ/期限を
  まとめたメタ行）＋右側のアイコンボタンのみ（旧: 下部に罫線区切りの文字ボタン行があったが
  高さ短縮のため廃止）。JIRA連携有無や個人タスクか否かは対象タグだけで表せるため、「個人タスク」
  「JIRA: <キー>」という冗長なテキスト表示はカードからは削除した（詳細は編集モーダルの
  「JIRA連携」欄で確認できる）。「非表示」/「再表示」操作は、カード右側の目のアイコン
  （表示中→目に斜線、非表示中→目、インラインSVG・外部依存なし）のみで行う。カード全体
  クリック＝編集モーダル、アイコンクリック＝非表示切替、に統合されている。
- 非表示（`tracked = 0`）のカードは、点線枠・半透明・「非表示中」バッジで通常カードと
  視覚的に区別される。
- `due_date`が設定されているタスクは日付(`YYYY-MM-DD`)のみのバッジを表示する（「期限:」の
  ラベルは冗長なため付けていない）。`due_date`が今日より前かつ`status != 'done'`の場合は
  赤系の配色で強調する（期限切れの視認性向上）。

### ドラッグ&ドロップ
- 個々の列（`.column`）ではなく`document`全体に`dragover`/`drop`リスナーを張り、
  **ポインタのx座標がどの列の左右範囲(`getBoundingClientRect`)に入っているかだけ**で
  ドロップ先レーンを決定する（y座標・列の高さは一切考慮しない）。列の枠の下（余白部分）に
  ドロップしても、横方向にその列の範囲内であれば移動が成立する。
- ドロップ確定時は`POST /tasks/<id>/move`をfetchで呼び、成功後に`location.reload()`で
  再描画する。

### モーダル（2種、ともに標準`<dialog>`要素、外部ライブラリ不使用）
- **編集モーダル(`#edit-modal`)**: カードクリックで開く。表示順は上から
  (1) タイトル・説明の編集フォーム、
  (2) 期限(`due_date`、`<input type="date">`)・対象・状態を1行3カラムのグリッドにまとめた
  コンパクトな編集フォーム、
  (3) 読み取り専用の詳細情報（ID・JIRA連携キー・表示状態・作成日時・完了日時・最終同期日時）、
  (4) 元発言（あれば。source/channel/author/text/受信日時をmessagesとのJOINで取得）
  という「編集項目を上・参照専用の詳細情報を下」の構成にした「詳細表示＋編集」のハイブリッドUI
  (2026-09-12、ユーザー要望により編集項目を上部にまとめる形へ並び替え)。
- **新規タスクモーダル(`#new-task-modal`)**: 「+新規タスク」ボタンで開く。タイトル・説明・期限・
  対象のみの簡易フォーム。作成時は常に`status=todo`・`jira_key=NULL`・`tracked=1`。
- 両モーダルとも背景（backdrop）クリックで閉じる（`event.target === dialog`判定）。

### 自動リフレッシュ
- 45秒間隔(`AUTO_REFRESH_INTERVAL_MS`)で`location.reload()`する`setInterval`を仕込んでいる。
  ただし編集モーダル・新規タスクモーダルのいずれかが開いている間(`dialog.open`)はスキップし、
  入力中の内容が消えないようにしている。
- 追加した経緯: 本ダッシュボードをOrca（IDE）の内蔵ブラウザタブで開きっぱなしにして日常的に
  眺める運用を想定しており、タブを開いたままでも他の変更（他ワークツリーでの操作等）が
  自動的に画面へ反映されるようにするため。

### 通知（flashメッセージ）
- `get_flashed_messages`によるフラッシュメッセージは、通常のドキュメントフローには置かず
  `</body>`直前に`position: fixed; inset-x-0; bottom-0`のコンテナとして配置している。これにより
  通知が表示されてもボード本体のレイアウト位置がずれない。外側コンテナは`pointer-events-none`、
  個々のメッセージは`pointer-events-auto`とし、通知が無い領域のクリックを妨げないようにしている。

## ルート一覧（`app.py`）

| メソッド/パス | 概要 |
|---|---|
| `GET /` | ボード表示。`target`・`show_untracked`をクエリパラメータで受け取る |
| `POST /tasks/<id>/edit` | 編集モーダルからの保存。title/target/statusを検証し更新（due_dateは未入力ならNULLとして保存）。`status`が`done`へ/から変化する際は`closed_at`をその場で設定/クリアする |
| `POST /tasks/new` | 新規タスク作成（due_date任意）。target が jira_a/jira_b の場合はJIRA起票スタブのログのみ出力（実通信なし） |
| `POST /tasks/<id>/move` | ドラッグ&ドロップからの状態変更（`status`をJSON body or formで受け取る）。JIRA連携タスクなら`stub_jira_transition()`を呼ぶ |
| `POST /tasks/<id>/track` | 「非表示」/「再表示」ボタン。後述の業務ルール参照 |

## 「非表示」の業務ルール（`toggle_track`）

- **非表示にする（`tracked: 1→0`）**: 対象タスクが`status != 'done'`の場合、同時に
  `status = 'done'`・`closed_at = 現在時刻`を設定する（＝一旦完了扱いにする）。既に`done`だった
  場合は`tracked`のみ変更し、元の`closed_at`は上書きしない。
- **再表示する（`tracked: 0→1`）**: `jira_key`がある（JIRA連携）タスクの場合のみ
  `stub_jira_transition(jira_key, "resync")`を呼び、`last_synced_at`を現在時刻に更新する
  （実際のJIRA API通信はしない）。個人タスクの場合は`tracked`を戻すのみ。
- 「削除」に相当する機能は存在しない。過去に「追跡から外す(削除)」と「追跡しない(非表示)」の
  2つがあり紛らわしかったため、削除機能は廃止し「非表示」に一本化した（データは常に残る）。

## デプロイ構成

- `build/Dockerfile`: `python:3.13-slim` + `pip install flask`。`db.py`/`seed.py`/`app.py`/
  `templates/`をコピー。`EXPOSE 8090`、`CMD ["python", "app.py"]`。
- `compose/docker-compose.yml`: サービス名`task-dashboard`。ビルドコンテキストはリポジトリ
  ルート（`..`）、`build/Dockerfile`参照。ポート`8090:8090`。
  実データ(SQLite)は`/docker/task-dashboard/data`（gitの外側）にボリュームマウントする。
- 環境変数: `TASK_DASHBOARD_HOST` / `TASK_DASHBOARD_PORT` / `TASK_DASHBOARD_DB_PATH` /
  `TASK_DASHBOARD_AUTO_SEED`（`1`ならDBが空の場合のみ`seed.py`を自動実行。実連携を組み込んだら
  `0`にする想定）。

## 日常運用（現時点の方針）

- 常時ブラウザを開いて確認する運用は「開き忘れる」ため定着しないという課題があり、日常
  よく使うツール（Orca＋Claude Code、Mattermost）側に寄せる方法を検討した。
- Orcaのプラグイン機構（カスタムパネル）を調査したが、パネル(iframe)はCSPで外部ネットワーク
  アクセスが完全に遮断されており、本ダッシュボードのデータをライブ表示する用途には使えないと
  判明（詳細は`docs/work-log.md`の2026-09-12エントリ参照）。
- 代わりに**Orcaの内蔵ブラウザ（ワークツリーごとのChromiumタブ）で本URLを開いておく**運用を
  採用し、上記の自動リフレッシュを追加した。内蔵ブラウザのタブはワークツリー単位でスコープ
  されるため、別ワークツリーに切り替えると見えなくなる制約は残るが、まずはこの運用で試すことに
  している。

## 既知の制限・今後の課題

- `collector`/`extractor`/`syncer`/`registrar`（Mattermost/JIRA/Zoom/Claude APIとの実連携）は
  未実装。着手にはMattermost botトークン・JIRA APIトークン・Zoom Server-to-Server OAuthアプリの
  準備、`project_routing`/`user_map`の初期データ整備が必要（ユーザー側準備待ち）。
- `candidates`・`user_map`テーブルは器のみ用意されており、画面・業務ロジックからは未使用。
- 認証・アクセス制御は無い（`app.secret_key`もパイロット用固定値）。外部公開しない前提
  （既定では`127.0.0.1`バインド、Docker運用時もLAN内利用を想定）。

## 関連ドキュメント

以下はすべて `/work`（メインリポジトリ、このリポジトリとは別）側にある。

- `docs/adr/proposals/task-management-automation.md` — 全体構想のADR（データモデル・パイプライン全体像）
- `docs/work-log.md` — 実装の経緯・判断理由（`task-management-automation`セクション）
- `docs/task-queue.md` — 残タスク・進捗管理
