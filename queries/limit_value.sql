-- name: CreateLimitValue :one
INSERT INTO limit_values
  (limit_group_id, limit_type_id, transaction_type, channel, operator, value)
VALUES
  (sqlc.arg(limit_group_id), sqlc.arg(limit_type_id), sqlc.arg(transaction_type),
   sqlc.arg(channel), sqlc.arg(operator), sqlc.arg(value))
RETURNING *;

-- name: GetLimitValueByID :one
SELECT * FROM limit_values WHERE limit_group_id = sqlc.arg(limit_group_id) AND id = sqlc.arg(id);

-- name: GetLimitValueByIDForUpdate :one
SELECT * FROM limit_values WHERE limit_group_id = sqlc.arg(limit_group_id) AND id = sqlc.arg(id) FOR UPDATE;

-- name: ListLimitValues :many
SELECT * FROM limit_values
WHERE limit_group_id = sqlc.arg(limit_group_id)
ORDER BY id LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountLimitValues :one
SELECT count(*) FROM limit_values WHERE limit_group_id = $1;

-- name: UpdateLimitValue :one
UPDATE limit_values
SET operator = sqlc.arg(operator), value = sqlc.arg(value), updated_at = now()
WHERE limit_group_id = sqlc.arg(limit_group_id) AND id = sqlc.arg(id)
RETURNING *;

-- name: DeleteLimitValue :one
DELETE FROM limit_values WHERE limit_group_id = sqlc.arg(limit_group_id) AND id = sqlc.arg(id) RETURNING *;
