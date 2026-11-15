-- name: CreateUserLimit :one
INSERT INTO user_limits
  (wallet_id, limit_type_id, transaction_type, channel, operator, value,
   effective_from, effective_to, reason, created_by)
VALUES
  (sqlc.arg(wallet_id), sqlc.arg(limit_type_id), sqlc.arg(transaction_type), sqlc.arg(channel),
   sqlc.arg(operator), sqlc.arg(value), COALESCE(CAST(sqlc.narg(effective_from) AS timestamptz), now()), sqlc.narg(effective_to),
   sqlc.narg(reason), sqlc.arg(created_by))
RETURNING *;

-- name: GetUserLimitByID :one
SELECT * FROM user_limits WHERE wallet_id = sqlc.arg(wallet_id) AND id = sqlc.arg(id);

-- name: GetUserLimitByIDForUpdate :one
SELECT * FROM user_limits WHERE wallet_id = sqlc.arg(wallet_id) AND id = sqlc.arg(id) FOR UPDATE;

-- name: ListUserLimits :many
SELECT * FROM user_limits
WHERE wallet_id = sqlc.arg(wallet_id)
ORDER BY id LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountUserLimits :one
SELECT count(*) FROM user_limits WHERE wallet_id = $1;

-- name: UpdateUserLimit :one
UPDATE user_limits
SET operator = sqlc.arg(operator), value = sqlc.arg(value),
    effective_from = sqlc.arg(effective_from), effective_to = sqlc.narg(effective_to),
    reason = sqlc.narg(reason), updated_at = now()
WHERE wallet_id = sqlc.arg(wallet_id) AND id = sqlc.arg(id)
RETURNING *;

-- name: DeleteUserLimit :one
DELETE FROM user_limits WHERE wallet_id = sqlc.arg(wallet_id) AND id = sqlc.arg(id) RETURNING *;
