-- name: CreatePolicyType :one
INSERT INTO policy_types (code, description, kind, active)
VALUES (upper(CAST(sqlc.arg(code) AS text)), sqlc.narg(description), sqlc.arg(kind), sqlc.arg(active))
RETURNING *;

-- name: GetPolicyTypeByID :one
SELECT * FROM policy_types WHERE id = $1;

-- name: GetPolicyTypeByIDForUpdate :one
SELECT * FROM policy_types WHERE id = $1 FOR UPDATE;

-- name: ListPolicyTypes :many
SELECT * FROM policy_types ORDER BY id LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountPolicyTypes :one
SELECT count(*) FROM policy_types;

-- name: UpdatePolicyType :one
UPDATE policy_types
SET description = sqlc.narg(description), active = sqlc.arg(active), updated_at = now()
WHERE id = sqlc.arg(id) RETURNING *;

-- name: DeletePolicyType :one
DELETE FROM policy_types WHERE id = $1 RETURNING *;
