#!/usr/bin/env python3
"""パイロット用のサンプルデータ投入スクリプト。

外部通信は一切行わない（Mattermost/メール/Zoom/JIRA/Claude APIのいずれにも接続しない）。
実行するたびに既存データを消して作り直す（再現可能な状態にするため）。
"""
from db import get_connection, init_db

MESSAGES = [
    # (source, source_id, channel_or_meeting, author, text, received_at, thread_id, project_hint)
    ("mattermost", "post-1001", "#project-a", "tanaka",
     "来週までにAPI仕様書をまとめてもらえますか？",
     "2026-09-08T10:00:00", None, "jira_a"),
    ("mattermost", "post-1002", "#project-b", "suzuki",
     "ログイン画面のエラーのバグ修正の件、対応完了しました",
     "2026-09-10T15:30:00", None, "jira_b"),
    ("email", "msg-2001", "inbox/personal", "yamada@example.com",
     "資料のレビューをお願いします（個人依頼）",
     "2026-09-09T09:15:00", None, "personal"),
    ("zoom", "meeting-3001", "定例会議 2026-09-10", "(meeting summary)",
     "Next steps: ログ基盤の調査を進める / UIレビューを来週までに完了する",
     "2026-09-10T18:00:00", None, "jira_a"),
]

CANDIDATES = [
    # (message_idx, kind, confidence, target, assignee_raw, jira_account_id, due_date, summary, related_jira_key, human_verdict)
    (0, "task", 0.9, "jira_a", "tanaka", "tanaka.k", "2026-09-19",
     "API仕様書をまとめる", None, "correct"),
    (1, "completion", 0.85, "jira_b", "suzuki", "suzuki.m", None,
     "バグ修正完了報告", "PROJB-42", "correct"),
    (2, "task", 0.7, "personal", "yamada@example.com", None, "2026-09-15",
     "資料レビュー", None, "correct"),
    (3, "task", 0.6, "jira_a", None, None, None,
     "ログ基盤の調査", None, None),
    (3, "task", 0.4, "unknown", None, None, None,
     "UIレビューを来週までに完了する", None, None),
]

TASKS = [
    # (source_message_idx, title, description, target, status, jira_key,
    #  created_at, closed_at, last_synced_at, tracked, due_date)
    (0, "API仕様書をまとめる", "元発言: post-1001（#project-a）", "jira_a", "todo",
     "PROJA-101", "2026-09-08T10:05:00", None, "2026-09-12T09:00:00", 1, "2026-09-10"),
    (1, "バグ修正: ログイン画面のエラー", "元発言: post-1002（#project-b）", "jira_b", "done",
     "PROJB-42", "2026-09-05T11:00:00", "2026-09-10T15:35:00", "2026-09-12T09:00:00", 1, None),
    (2, "資料レビュー", "依頼元: yamada@example.com", "personal", "in_progress",
     None, "2026-09-09T09:20:00", None, None, 1, "2026-09-20"),
    (None, "旧: サーバー証明書更新", "過去に完了・追跡除外済みの例", "jira_a", "done",
     "PROJA-88", "2026-08-01T09:00:00", "2026-08-20T17:00:00", "2026-08-21T09:00:00", 0, None),
    (None, "個人: 経費精算", "個人タスクの例", "personal", "todo",
     None, "2026-09-11T08:00:00", None, None, 1, None),
    (None, "個人: 昔のメモ整理", "完了済み・追跡除外の個人タスクの例", "personal", "done",
     None, "2026-07-01T09:00:00", "2026-07-05T09:00:00", None, 0, None),
]

USER_MAP = [
    # (source, source_user_id, jira_account_id, display_name)
    ("mattermost", "tanaka", "tanaka.k", "田中"),
    ("mattermost", "suzuki", "suzuki.m", "鈴木"),
    ("email", "yamada@example.com", None, "山田"),  # JIRAアカウント未解決の例
]


def seed():
    init_db()
    conn = get_connection()
    try:
        conn.execute("DELETE FROM candidates")
        conn.execute("DELETE FROM tasks")
        conn.execute("DELETE FROM messages")
        conn.execute("DELETE FROM user_map")

        message_ids = []
        for source, source_id, channel_or_meeting, author, text, received_at, thread_id, project_hint in MESSAGES:
            cur = conn.execute(
                """INSERT INTO messages
                   (source, source_id, channel_or_meeting, author, text, received_at, thread_id, project_hint)
                   VALUES (?, ?, ?, ?, ?, ?, ?, ?)""",
                (source, source_id, channel_or_meeting, author, text, received_at, thread_id, project_hint),
            )
            message_ids.append(cur.lastrowid)

        for (msg_idx, kind, confidence, target, assignee_raw, jira_account_id,
             due_date, summary, related_jira_key, human_verdict) in CANDIDATES:
            conn.execute(
                """INSERT INTO candidates
                   (message_id, kind, confidence, target, assignee_raw, jira_account_id,
                    due_date, summary, related_jira_key, human_verdict)
                   VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)""",
                (message_ids[msg_idx], kind, confidence, target, assignee_raw, jira_account_id,
                 due_date, summary, related_jira_key, human_verdict),
            )

        for (msg_idx, title, description, target, status, jira_key,
             created_at, closed_at, last_synced_at, tracked, due_date) in TASKS:
            source_message_id = message_ids[msg_idx] if msg_idx is not None else None
            conn.execute(
                """INSERT INTO tasks
                   (source_message_id, title, description, target, status, jira_key,
                    created_at, closed_at, last_synced_at, tracked, due_date)
                   VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)""",
                (source_message_id, title, description, target, status, jira_key,
                 created_at, closed_at, last_synced_at, tracked, due_date),
            )

        for source, source_user_id, jira_account_id, display_name in USER_MAP:
            conn.execute(
                """INSERT INTO user_map (source, source_user_id, jira_account_id, display_name)
                   VALUES (?, ?, ?, ?)""",
                (source, source_user_id, jira_account_id, display_name),
            )

        conn.commit()
    finally:
        conn.close()


if __name__ == "__main__":
    seed()
    print("サンプルデータを投入しました（外部通信なし）")
