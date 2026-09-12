#!/usr/bin/env python3
"""タスク管理自動化パイロットのDBスキーマ・接続ヘルパー。

docs/adr/proposals/task-management-automation.md のデータモデルに対応するSQLiteスキーマ。
標準ライブラリのみ使用。外部通信は一切行わない。
"""
import os
import sqlite3

DB_PATH = os.environ.get(
    "TASK_DASHBOARD_DB_PATH",
    os.path.join(os.path.dirname(__file__), "task_dashboard.db"),
)

SCHEMA = """
CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT NOT NULL,                  -- mattermost / email / zoom
    source_id TEXT,
    channel_or_meeting TEXT,
    author TEXT,
    text TEXT NOT NULL,
    received_at TEXT NOT NULL,
    thread_id TEXT,
    project_hint TEXT                      -- jira_a / jira_b / personal / unknown
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
    human_verdict TEXT                     -- correct / false_positive / missed
);

CREATE TABLE IF NOT EXISTS tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_message_id INTEGER REFERENCES messages(id),
    title TEXT NOT NULL,
    description TEXT,
    target TEXT NOT NULL,                  -- jira_a / jira_b / personal
    status TEXT NOT NULL DEFAULT 'todo',   -- todo / in_progress / done
    jira_key TEXT,                         -- NULLなら個人タスク
    created_at TEXT NOT NULL,
    closed_at TEXT,
    last_synced_at TEXT,                   -- JIRA連携タスクのみ意味を持つ
    tracked INTEGER NOT NULL DEFAULT 1,    -- 0/1。falseで画面非表示・同期対象外
    due_date TEXT                          -- 期限(YYYY-MM-DD)、nullable
);

CREATE TABLE IF NOT EXISTS user_map (
    source TEXT NOT NULL,
    source_user_id TEXT NOT NULL,
    jira_account_id TEXT,
    display_name TEXT,
    PRIMARY KEY (source, source_user_id)
);
"""


def get_connection():
    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row
    conn.execute("PRAGMA foreign_keys = ON")
    return conn


def init_db():
    conn = get_connection()
    try:
        conn.executescript(SCHEMA)
        # 旧スキーマ(status='open'/'done'の2値)からのマイグレーション。
        # カンバン化(todo/in_progress/done の3値)に合わせて既存データを寄せる。
        conn.execute("UPDATE tasks SET status = 'todo' WHERE status = 'open'")
        # 既存DB(due_date列がまだ無いもの)へのマイグレーション。
        columns = [row["name"] for row in conn.execute("PRAGMA table_info(tasks)")]
        if "due_date" not in columns:
            conn.execute("ALTER TABLE tasks ADD COLUMN due_date TEXT")
        conn.commit()
    finally:
        conn.close()


if __name__ == "__main__":
    init_db()
    print(f"スキーマを初期化しました: {DB_PATH}")
