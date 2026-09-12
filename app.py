#!/usr/bin/env python3
"""タスク管理自動化パイロットの管理画面（Flask）。

外部通信は一切行わない。JIRA連携タスクのクローズ操作は「本来ここでJIRA APIの
ステータス遷移を呼ぶ」ことが分かるよう stub_jira_transition() でログ出力するのみで、
実際のHTTPリクエストは送らない（本番実装時にJIRA REST API呼び出しへ差し替える想定）。

ローカル実行: uv run python app.py
（既定では127.0.0.1のみにバインドし、外部には公開しない）
Docker実行: compose/docker-compose.yml 参照
（環境変数 TASK_DASHBOARD_HOST 等でホスト/ポートを上書きする）
"""
import os
from datetime import datetime, timedelta, timezone

from flask import Flask, flash, redirect, render_template, request, url_for

from db import get_connection, init_db
from seed import seed

app = Flask(__name__)
# パイロット用の開発用シークレット。外部公開しない前提のためこの値のままでよい。
app.secret_key = "task-dashboard-pilot-dev-secret"

TARGETS = ["jira_a", "jira_b", "personal"]
STATUSES = ["todo", "in_progress", "reviewing", "done"]
STATUS_LABELS = {"todo": "未着手", "in_progress": "進行中", "reviewing": "確認中", "done": "完了"}
DONE_LANE_WINDOW_DAYS = 7  # 完了レーンに表示するのは直近この日数以内に完了したものだけ


def now_iso():
    return datetime.now(timezone.utc).isoformat(timespec="seconds")


def is_recently_closed(closed_at):
    """closed_atがDONE_LANE_WINDOW_DAYS日以内かどうか(完了レーンの表示絞り込み用)。"""
    if not closed_at:
        return False
    try:
        closed_dt = datetime.fromisoformat(closed_at)
    except ValueError:
        return False
    if closed_dt.tzinfo is None:
        closed_dt = closed_dt.replace(tzinfo=timezone.utc)
    return closed_dt >= datetime.now(timezone.utc) - timedelta(days=DONE_LANE_WINDOW_DAYS)


def stub_jira_transition(jira_key, action):
    """本来ここでJIRA REST APIを呼ぶ想定のスタブ。外部通信は行わない。"""
    print(f"[stub] JIRA API呼び出し想定: jira_key={jira_key} action={action}（実際の通信なし）", flush=True)


@app.route("/")
def list_tasks():
    target = request.args.get("target", "")
    show_untracked = request.args.get("show_untracked") == "1"

    query = """
        SELECT tasks.*,
               messages.source AS msg_source,
               messages.channel_or_meeting AS msg_channel,
               messages.author AS msg_author,
               messages.text AS msg_text,
               messages.received_at AS msg_received_at
        FROM tasks
        LEFT JOIN messages ON tasks.source_message_id = messages.id
        WHERE 1=1
    """
    params = []
    if target in TARGETS:
        query += " AND tasks.target = ?"
        params.append(target)
    if not show_untracked:
        query += " AND tasks.tracked = 1"
    query += " ORDER BY tasks.created_at DESC"

    conn = get_connection()
    try:
        tasks = conn.execute(query, params).fetchall()
    finally:
        conn.close()

    columns = {status: [] for status in STATUSES}
    for task in tasks:
        if task["status"] == "done" and not is_recently_closed(task["closed_at"]):
            # 完了レーンが際限なく膨らまないよう、直近7日以内に完了したものだけ表示する。
            continue
        columns.setdefault(task["status"], []).append(task)

    return render_template(
        "board.html",
        columns=columns,
        statuses=STATUSES,
        status_labels=STATUS_LABELS,
        targets=TARGETS,
        selected_target=target,
        show_untracked=show_untracked,
        done_window_days=DONE_LANE_WINDOW_DAYS,
        today=datetime.now(timezone.utc).date().isoformat(),
    )


@app.route("/tasks/<int:task_id>/edit", methods=["POST"])
def edit_task(task_id):
    conn = get_connection()
    try:
        task = conn.execute("SELECT * FROM tasks WHERE id = ?", (task_id,)).fetchone()
        if task is None:
            flash("タスクが見つかりません", "error")
            return redirect(url_for("list_tasks"))

        title = request.form.get("title", "").strip()
        description = request.form.get("description", "")
        target = request.form.get("target")
        status = request.form.get("status")
        due_date = request.form.get("due_date") or None
        if not title:
            flash("タイトルは必須です", "error")
            return redirect(url_for("list_tasks"))
        if target not in TARGETS:
            flash("対象プロジェクトの値が不正です", "error")
            return redirect(url_for("list_tasks"))
        if status not in STATUSES:
            flash("状態の値が不正です", "error")
            return redirect(url_for("list_tasks"))

        closed_at = task["closed_at"]
        if status == "done" and task["status"] != "done":
            closed_at = now_iso()
        elif status != "done":
            closed_at = None

        conn.execute(
            """UPDATE tasks SET title = ?, description = ?, target = ?,
               status = ?, closed_at = ?, due_date = ? WHERE id = ?""",
            (title, description, target, status, closed_at, due_date, task_id),
        )
        conn.commit()
        if task["jira_key"]:
            print(
                f"[stub] JIRA API呼び出し想定: jira_key={task['jira_key']} action=update_fields（実際の通信なし）",
                flush=True,
            )
        flash("更新しました", "success")
        return redirect(url_for("list_tasks"))
    finally:
        conn.close()


@app.route("/tasks/new", methods=["POST"])
def new_task():
    title = request.form.get("title", "").strip()
    description = request.form.get("description", "")
    target = request.form.get("target")
    due_date = request.form.get("due_date") or None
    if not title:
        flash("タイトルは必須です", "error")
        return redirect(url_for("list_tasks"))
    if target not in TARGETS:
        flash("対象プロジェクトの値が不正です", "error")
        return redirect(url_for("list_tasks"))

    conn = get_connection()
    try:
        conn.execute(
            """INSERT INTO tasks
               (source_message_id, title, description, target, status, jira_key,
                created_at, tracked, due_date)
               VALUES (NULL, ?, ?, ?, 'todo', NULL, ?, 1, ?)""",
            (title, description, target, now_iso(), due_date),
        )
        conn.commit()
    finally:
        conn.close()

    if target in ("jira_a", "jira_b"):
        print(
            f"[stub] JIRA API呼び出し想定: jira_key=(未発行) action=create target={target}"
            "（実際の通信なし。手動追加のためJIRA起票は未実施）",
            flush=True,
        )
    flash(f"タスクを追加しました: {title}", "success")
    return redirect(url_for("list_tasks"))


@app.route("/tasks/<int:task_id>/move", methods=["POST"])
def move_task(task_id):
    status = request.form.get("status") or (request.get_json(silent=True) or {}).get("status")
    if status not in STATUSES:
        flash("状態の値が不正です", "error")
        return redirect(url_for("list_tasks"))

    conn = get_connection()
    try:
        task = conn.execute("SELECT * FROM tasks WHERE id = ?", (task_id,)).fetchone()
        if task is None:
            flash("タスクが見つかりません", "error")
            return redirect(url_for("list_tasks"))

        if task["jira_key"]:
            stub_jira_transition(task["jira_key"], f"move_to_{status}")

        closed_at = task["closed_at"]
        if status == "done" and task["status"] != "done":
            closed_at = now_iso()
        elif status != "done":
            closed_at = None

        conn.execute(
            "UPDATE tasks SET status = ?, closed_at = ? WHERE id = ?",
            (status, closed_at, task_id),
        )
        conn.commit()
        flash(f"「{task['title']}」を{STATUS_LABELS[status]}に移動しました", "success")
        return redirect(url_for("list_tasks"))
    finally:
        conn.close()


@app.route("/tasks/<int:task_id>/track", methods=["POST"])
def toggle_track(task_id):
    conn = get_connection()
    try:
        task = conn.execute("SELECT * FROM tasks WHERE id = ?", (task_id,)).fetchone()
        if task is None:
            flash("タスクが見つかりません", "error")
            return redirect(url_for("list_tasks"))

        new_tracked = 0 if task["tracked"] else 1

        if new_tracked == 0:
            # 非表示にする時点で一旦完了扱いにする(既に完了済みなら完了日時は変更しない)。
            if task["status"] == "done":
                conn.execute("UPDATE tasks SET tracked = 0 WHERE id = ?", (task_id,))
            else:
                conn.execute(
                    "UPDATE tasks SET tracked = 0, status = 'done', closed_at = ? WHERE id = ?",
                    (now_iso(), task_id),
                )
            flash("非表示にしました(完了扱いにしました)", "success")
        elif task["jira_key"]:
            # 再表示のタイミングでJIRAとの再同期を行う想定のスタブ(実際の通信なし)。
            stub_jira_transition(task["jira_key"], "resync")
            conn.execute(
                "UPDATE tasks SET tracked = 1, last_synced_at = ? WHERE id = ?",
                (now_iso(), task_id),
            )
            flash("再表示し、JIRAと再同期しました", "success")
        else:
            conn.execute("UPDATE tasks SET tracked = 1 WHERE id = ?", (task_id,))
            flash("再表示しました", "success")

        conn.commit()
        return redirect(request.referrer or url_for("list_tasks"))
    finally:
        conn.close()


if __name__ == "__main__":
    init_db()

    auto_seed = os.environ.get("TASK_DASHBOARD_AUTO_SEED", "1") == "1"
    if auto_seed:
        conn = get_connection()
        try:
            (task_count,) = conn.execute("SELECT COUNT(*) FROM tasks").fetchone()
        finally:
            conn.close()
        if task_count == 0:
            seed()

    host = os.environ.get("TASK_DASHBOARD_HOST", "127.0.0.1")
    port = int(os.environ.get("TASK_DASHBOARD_PORT", "5000"))
    debug = os.environ.get("TASK_DASHBOARD_DEBUG", "0") == "1"
    app.run(host=host, port=port, debug=debug)
