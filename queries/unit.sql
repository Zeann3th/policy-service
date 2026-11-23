-- name: CreateUnit :one
INSERT INTO units (code, description, active)
VALUES (upper(CAST(sqlc.arg(code) AS text)), sqlc.narg(description), sqlc.arg(active)) RETURNING *;

-- name: GetUnitByID :one
SELECT * FROM units WHERE id = $1;

-- name: GetUnitByIDForUpdate :one
SELECT * FROM units WHERE id = $1 FOR UPDATE;

-- name: ListUnits :many
SELECT * FROM units ORDER BY id LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountUnits :one
SELECT count(*) FROM units;

-- name: UpdateUnit :one
UPDATE units SET description = sqlc.narg(description), active = sqlc.arg(active), updated_at = now()
WHERE id = sqlc.arg(id) RETURNING *;

-- name: DeleteUnit :one
DELETE FROM units WHERE id = $1 RETURNING *;
