# データモデル（DB設計）

> 全体方針・技術スタックは`docs/design/design.md`、画面ごとの仕様は`docs/design/screen-*.md`を
> 参照。本書はDBスキーマ（`internal/taskstore/schema.sql`）自体の設計のみを扱う。

## テーブル一覧

| テーブル | 役割 | 主な列 |
|---|---|---|
| `messages` | Mattermost extractorがタスク化・候補化したメッセージのみを保持する（ノイズと判定された投稿は保存しない。`docs/adr/complete/mattermost-message-retention.md`参照）＋seed.goの固定サンプル | `source`(mattermost/email/zoom), `channel_or_meeting`, `author`, `text`, `received_at`, `project_hint`, `permalink_url`(元投稿URL、nullable) |
| `candidates` | メッセージから抽出したタスク/完了候補。`kind=task`はMattermost extractorが分類した新規タスク候補（`target`確定分は自動的に`tasks`へ登録され`human_verdict='auto_registered'`が設定される。不明分は「タスク候補一覧」画面で人間が承認/却下）、`kind=completion`は完了報告候補（「クローズ要求一覧」画面が参照。`related_jira_key`が無い場合、`suggested_task_id`にAI推定の対象タスクが入り`<select>`の初期選択肢になる。`docs/adr/complete/close-request-target-task-suggestion.md`参照） | `kind`(task/completion), `confidence`, `target`, `assignee_raw`, `due_date`, `summary`, `related_jira_key`, `human_verdict`(correct/false_positive/auto_registered等), `suggested_task_id`(nullable, FK→tasks.id) |
| `tasks` | ダッシュボードが実際に読み書きする本体 | `title`, `description`, `target`(jira_a/jira_b/personal), `status`(todo/in_progress/reviewing/done), `jira_key`, `created_at`, `closed_at`, `last_synced_at`, `tracked`(0/1), `due_date`(YYYY-MM-DD、nullable), `cycle_start_date`(所属週の月曜日、YYYY-MM-DD、nullable。NULL=バックログ), `priority`(highest/high/medium/low、デフォルトmedium) |
| `user_map` | 発言者⇔JIRAアカウントの対応（本パイロットでは表示画面からは未使用） | `source`, `source_user_id`, `jira_account_id`, `display_name` |
| `mattermost_channel_state` | Mattermost extractorがチャンネルごとにどこまで取得済みかを保持する（`messages`テーブルへの依存を無くしたカーソル管理、`docs/design/mattermost-extractor.md`参照） | `channel_id`(PK), `last_processed_at` |

## ER図

`internal/taskstore/schema.sql`（実装）と一致させたテーブル間のリレーション・列定義。

```mermaid
erDiagram
    MESSAGES ||--o{ CANDIDATES : "1メッセージから複数候補を抽出しうる"
    TASKS ||--o| CANDIDATES : "kind=taskが承認されて生成"
    TASKS ||--o{ CANDIDATES : "kind=completionが完了対象として参照"
    USER_MAP ||--o{ CANDIDATES : "assignee_rawの解決に参照"

    MESSAGES {
        int id PK
        string source "mattermost/email/zoom"
        string source_id
        string channel_or_meeting
        string author
        text text
        datetime received_at
        string thread_id
        string project_hint "jira_a/jira_b/personal/unknown"
        string permalink_url "nullable。元投稿URL"
    }

    CANDIDATES {
        int id PK
        int message_id FK
        string kind "task/completion"
        float confidence
        string target "jira_a/jira_b/personal/unknown"
        string assignee_raw
        string jira_account_id "nullable"
        date due_date "nullable"
        text summary
        string related_jira_key "nullable(completion用)"
        string human_verdict "nullable(correct/false_positive/missed)"
        int suggested_task_id FK "nullable(completion用、AI推定の対象タスク)"
    }

    TASKS {
        int id PK
        int source_message_id FK
        string title
        text description
        string target "jira_a/jira_b/personal"
        string status "todo/in_progress/reviewing/done"
        string jira_key "nullable(個人タスクはNULL)"
        datetime created_at
        datetime closed_at "nullable"
        datetime last_synced_at "nullable(JIRA連携タスクのみ、syncer更新時刻)"
        boolean tracked "default true。falseで画面非表示・同期対象外"
        date due_date "nullable。期限"
        date cycle_start_date "nullable。NULL=バックログ、値ありなら所属週の月曜日(週次サイクル)"
        string priority "highest/high/medium/low、デフォルトmedium"
    }

    USER_MAP {
        string source PK
        string source_user_id PK
        string jira_account_id
        string display_name
    }

    MATTERMOST_CHANNEL_STATE {
        string channel_id PK
        string last_processed_at "最後に取得した投稿のcreate_at(RFC3339)"
    }
```

- `messages.project_hint`は収集元の設定（`project_routing`）から機械的に付与する「対象
  プロジェクトの手がかり」。`project_routing`自体はDBではなく設定ファイルで管理するため、
  ER図には含めていない（Mattermost分は環境変数`MATTERMOST_CHANNEL_ROUTES`で代替、
  `docs/design/mattermost-extractor.md`参照。メール/Zoom分は`docs/design/mail-zoom-pipeline.md`
  参照）。
- `tasks.jira_key`はJIRA起票済みなら値あり、個人タスクはNULLのまま自前ストアの実体となる。

## 週次サイクル（Cycles）

Linear風の「今週/バックログ」区分を`tasks.cycle_start_date`のみで表現し、独立した
Cycleテーブルは持たない（過去サイクルの履歴参照は要件外のため。比較検討の経緯は
`docs/adr/complete/cycle-data-model.md`参照）。`internal/taskstore/rollover.go`が
週境界（月曜0:00 JST）を跨いだ未完了タスクの`cycle_start_date`を現在の週の月曜日へ
書き換える（常駐goroutine＋起動時キャッチアップ実行、外部通信なし。詳細は
`docs/design/screen-board.md`「週次繰り越し」節、実行方式の検討経緯は
`docs/adr/complete/cycle-rollover-execution.md`参照）。

## 優先度（priority）

`tasks.priority`（highest/high/medium/low、デフォルトmedium）はLinear風の優先度概念を取り込んだ
もので、`target`（personal/jira_a/jira_b）問わず全タスク共通。カード上のバッジ表示にのみ使い、
並び順・レーン構造には影響しない（比較検討の経緯は`docs/adr/complete/task-priority-field.md`
参照）。JIRA連携タスクの優先度もローカル表示専用でJIRAへは書き込まない（`stubJiraTransition`と
同様、実同期はしない）。

## マイグレーション・実装上の注意

- `internal/taskstore/store.go`の`InitSchema()`は起動のたびに呼ばれ、`schema.sql`
  （go:embed）を実行した後、旧スキーマ（`status`が`open`/`done`の2値だった時代のデータ）を
  `todo`へ寄せるマイグレーションと、`due_date`/`cycle_start_date`/`priority`/
  `messages.permalink_url`列が無ければ`ALTER TABLE`で追加するマイグレーションを実行する
  （Flask版の`db.py`の`init_db()`と同内容＋各列追加分。`priority`は`DEFAULT 'medium'`付きの
  ため既存行にも自動的に値が入る）。`mattermost_channel_state`テーブルは新規テーブルのため
  `CREATE TABLE IF NOT EXISTS`のみで追加され、マイグレーション不要。
- `tracked`（真偽値、SQLite上は0/1の整数）は「非表示」の実体。削除ではなくこのフラグの
  反転のみで、行は物理的には常に残る（業務ルールの詳細は`docs/design/screen-board.md`参照）。
- クエリは`internal/taskstore/query.sql`に集約されており、`sqlc generate`で
  `query.sql.go`（型安全なGo関数）を生成する。生成コードは手で編集しない
  （`// Code generated by sqlc. DO NOT EDIT.`）。**生成コード（`db.go`/`models.go`/
  `query.sql.go`）はコミットせず、ローカルにも置かない方針**（`.gitignore`対象）。
  ビルド前に必ず`sqlc generate`を実行する必要がある（`build/Dockerfile`はビルド
  ステージ内で自動実行するため、Docker運用では意識不要。手順は`docs/design/design.md`の
  「開発時のビルド方法」参照）。

## 関連ドキュメント

- `docs/design/design.md` — 全体方針・技術スタック・デプロイ構成。
- `docs/design/screen-board.md` / `docs/design/screen-close-requests.md` — 各画面が
  このデータモデルをどう読み書きするか。
- `docs/design/automation-roadmap.md` — 全体構想・背景・リスク・ロードマップ。
- `docs/design/mattermost-extractor.md` — 実装済みのMattermost collector/extractorの確定仕様
  （本書のER図・スキーマ定義は重複させず本書のみに置く）。
- `docs/design/mail-zoom-pipeline.md` — 未実装のメール/Zoom分での`messages`/`candidates`の
  使われ方。
