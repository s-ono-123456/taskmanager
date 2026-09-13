-- タスク管理自動化パイロットのDBスキーマ。
-- docs/adr/proposals/task-management-automation.md のデータモデルに対応するSQLiteスキーマ。
-- go:embed でそのまま読み込み、起動時のCREATE TABLE IF NOT EXISTS実行にも使う
-- (sqlcの型推論と起動時DDLの単一のソース)。

CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT NOT NULL,                  -- mattermost / email / zoom
    source_id TEXT,
    channel_or_meeting TEXT,
    author TEXT,
    text TEXT NOT NULL,
    received_at TEXT NOT NULL,
    thread_id TEXT,
    project_hint TEXT,                     -- jira_a / jira_b / personal / unknown
    permalink_url TEXT                     -- 元投稿へのパーマリンク(nullable。Mattermost等)
);

-- Mattermost collectorがチャンネルごとにどこまで取得済みかを保持する状態テーブル。
-- messagesテーブルへの依存を無くすため(タスク化・候補化されなかった投稿は保持しない方針、
-- docs/adr/proposals/mattermost-message-retention.md参照)、分類結果に関わらず取得できた
-- 投稿の最大create_atで更新する。
CREATE TABLE IF NOT EXISTS mattermost_channel_state (
    channel_id TEXT PRIMARY KEY,
    last_processed_at TEXT NOT NULL        -- 最後に取得した投稿のcreate_at(RFC3339)
);

CREATE TABLE IF NOT EXISTS candidates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id INTEGER NOT NULL REFERENCES messages(id),
    kind TEXT NOT NULL,                    -- task / completion
    confidence REAL,
    target TEXT,                           -- jira_a / jira_b / personal / unknown
    assignee_raw TEXT,
    jira_account_id TEXT,
    due_date TEXT,
    summary TEXT,
    related_jira_key TEXT,
    human_verdict TEXT,                    -- correct / false_positive / missed
    suggested_task_id INTEGER REFERENCES tasks(id) -- kind=completionでrelated_jira_keyが
                                            -- 無い場合にLLMが推定した対象タスク(nullable、
                                            -- docs/adr/proposals/close-request-target-task-suggestion.md参照)
);

CREATE TABLE IF NOT EXISTS tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_message_id INTEGER REFERENCES messages(id),
    title TEXT NOT NULL,
    description TEXT,
    target TEXT NOT NULL,                  -- jira_a / jira_b / personal
    status TEXT NOT NULL DEFAULT 'todo',   -- todo / in_progress / reviewing / done
    jira_key TEXT,                         -- NULLなら個人タスク
    created_at TEXT NOT NULL,
    closed_at TEXT,
    last_synced_at TEXT,                   -- JIRA連携タスクのみ意味を持つ
    tracked INTEGER NOT NULL DEFAULT 1,    -- 0/1。falseで画面非表示・同期対象外
    due_date TEXT,                         -- 期限(YYYY-MM-DD)、nullable
    cycle_start_date TEXT,                 -- 所属する週の月曜日(YYYY-MM-DD)。NULL=バックログ
    priority TEXT NOT NULL DEFAULT 'medium' -- 優先度(highest/high/medium/low)
);

CREATE TABLE IF NOT EXISTS user_map (
    source TEXT NOT NULL,
    source_user_id TEXT NOT NULL,
    jira_account_id TEXT,
    display_name TEXT,
    PRIMARY KEY (source, source_user_id)
);
