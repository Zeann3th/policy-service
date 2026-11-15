-- name: CreateLimitType :one
INSERT INTO limit_types (code, description, unit_id, active)
VALUES (upper(CAST(sqlc.arg(code) AS text)), sqlc.narg(description), sqlc.arg(unit_id), sqlc.arg(active)) RETURNING *;

-- name: GetLimitTypeByID :one
SELECT * FROM limit_types WHERE id = $1;

-- name: GetLimitTypeByIDForUpdate :one
SELECT * FROM limit_types WHERE id = $1 FOR UPDATE;

-- name: ListLimitTypes :many
SELECT * FROM limit_types ORDER BY id LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountLimitTypes :one
SELECT count(*) FROM limit_types;

-- name: UpdateLimitType :one
UPDATE limit_types
SET description = sqlc.narg(description), active = sqlc.arg(active), updated_at = now()
WHERE id = sqlc.arg(id) RETURNING *;

-- name: DeleteLimitType :one
DELETE FROM limit_types WHERE id = $1 RETURNING *;
