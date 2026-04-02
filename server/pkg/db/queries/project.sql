-- name: ListProjectsByWorkspace :many
SELECT * FROM project
WHERE workspace_id = $1
ORDER BY position ASC, created_at ASC;

-- name: GetProject :one
SELECT * FROM project WHERE id = $1;

-- name: GetProjectInWorkspace :one
SELECT * FROM project
WHERE id = $1 AND workspace_id = $2;

-- name: CreateProject :one
INSERT INTO project (workspace_id, name, position)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateProject :one
UPDATE project SET
    name = COALESCE(sqlc.narg('name'), name),
    position = COALESCE(sqlc.narg('position'), position),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteProject :exec
DELETE FROM project WHERE id = $1;

-- name: CountIssuesByProject :one
SELECT count(*)::int4 FROM issue WHERE project_id = $1;

-- name: GetFirstProjectInWorkspace :one
SELECT * FROM project
WHERE workspace_id = $1
ORDER BY position ASC, created_at ASC
LIMIT 1;
