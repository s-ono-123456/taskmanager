-- name: ListTasks :many
SELECT tasks.*,
       messages.source AS msg_source,
       messages.channel_or_meeting AS msg_channel,
       messages.author AS msg_author,
       messages.text AS msg_text,
       messages.received_at AS msg_received_at
FROM tasks
LEFT JOIN messages ON tasks.source_message_id = messages.id
WHERE (sqlc.narg('target') IS NULL OR tasks.target = sqlc.narg('target'))
  AND (sqlc.arg('show_untracked') OR tasks.tracked = 1)
ORDER BY tasks.created_at DESC;

-- name: GetTask :one
SELECT * FROM tasks WHERE id = ?;

-- name: CreateTask :one
INSERT INTO tasks
  (source_message_id, title, description, target, status, jira_key,
   created_at, closed_at, last_synced_at, tracked, due_date, cycle_start_date, priority)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateTask :exec
UPDATE tasks SET title = ?, description = ?, target = ?,
                 status = ?, closed_at = ?, due_date = ?, priority = ? WHERE id = ?;

-- name: UpdateTaskStatus :exec
UPDATE tasks SET status = ?, closed_at = ? WHERE id = ?;

-- name: UpdateTaskStatusAndCycle :exec
UPDATE tasks SET status = ?, closed_at = ?, cycle_start_date = ? WHERE id = ?;

-- name: RolloverCycles :execrows
UPDATE tasks
SET cycle_start_date = sqlc.arg('new_monday')
WHERE cycle_start_date IS NOT NULL
  AND cycle_start_date <> sqlc.arg('new_monday')
  AND status <> 'done';

-- name: SetTaskUntrackedKeepStatus :exec
UPDATE tasks SET tracked = 0 WHERE id = ?;

-- name: SetTaskUntrackedAsDone :exec
UPDATE tasks SET tracked = 0, status = 'done', closed_at = ? WHERE id = ?;

-- name: SetTaskTrackedAndSynced :exec
UPDATE tasks SET tracked = 1, last_synced_at = ? WHERE id = ?;

-- name: SetTaskTracked :exec
UPDATE tasks SET tracked = 1 WHERE id = ?;

-- name: CountTasks :one
SELECT COUNT(*) FROM tasks;

-- name: DeleteAllCandidates :exec
DELETE FROM candidates;

-- name: DeleteAllTasks :exec
DELETE FROM tasks;

-- name: DeleteAllMessages :exec
DELETE FROM messages;

-- name: DeleteAllUserMap :exec
DELETE FROM user_map;

-- name: InsertMessage :one
INSERT INTO messages (source, source_id, channel_or_meeting, author, text, received_at, thread_id, project_hint)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: InsertCandidate :exec
INSERT INTO candidates (message_id, kind, confidence, target, assignee_raw, jira_account_id, due_date, summary, related_jira_key, human_verdict)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: InsertUserMap :exec
INSERT INTO user_map (source, source_user_id, jira_account_id, display_name)
VALUES (?, ?, ?, ?);

-- name: ListPendingCompletionCandidates :many
SELECT candidates.*,
       messages.source AS msg_source,
       messages.channel_or_meeting AS msg_channel,
       messages.author AS msg_author,
       messages.text AS msg_text,
       messages.received_at AS msg_received_at
FROM candidates
LEFT JOIN messages ON candidates.message_id = messages.id
WHERE candidates.kind = 'completion'
  AND (candidates.human_verdict IS NULL OR candidates.human_verdict = '')
ORDER BY candidates.id;

-- name: GetCandidate :one
SELECT * FROM candidates WHERE id = ?;

-- name: UpdateCandidateVerdict :exec
UPDATE candidates SET human_verdict = ? WHERE id = ?;

-- name: GetTaskByJiraKey :one
SELECT * FROM tasks WHERE jira_key = ? LIMIT 1;

-- name: ListOpenTasksByTarget :many
SELECT * FROM tasks
WHERE target = ? AND status != 'done' AND tracked = 1
ORDER BY created_at DESC;

-- name: MessageExistsBySourceID :one
SELECT EXISTS(SELECT 1 FROM messages WHERE source = ? AND source_id = ?);

-- name: LastMessageReceivedAt :one
SELECT CAST(COALESCE(MAX(received_at), '') AS TEXT) FROM messages WHERE source = 'mattermost' AND channel_or_meeting = ?;
