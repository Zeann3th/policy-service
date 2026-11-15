-- name: CreateCustomPolicy :one
INSERT INTO custom_policies
  (wallet_id, policy_type_id, transaction_type, channel, direction, window_type, operator, value,
   effective_from, effective_to, reason, created_by)
VALUES
  (sqlc.arg(wallet_id), sqlc.arg(policy_type_id), sqlc.arg(transaction_type), sqlc.arg(channel), sqlc.arg(direction), sqlc.arg(window_type),
   sqlc.arg(operator), sqlc.arg(value), COALESCE(CAST(sqlc.narg(effective_from) AS timestamptz), now()), sqlc.narg(effective_to),
   sqlc.narg(reason), sqlc.arg(created_by))
RETURNING *;

-- name: GetCustomPolicyByID :one
SELECT * FROM custom_policies WHERE wallet_id = sqlc.arg(wallet_id) AND id = sqlc.arg(id);

-- name: GetCustomPolicyByIDForUpdate :one
SELECT * FROM custom_policies WHERE wallet_id = sqlc.arg(wallet_id) AND id = sqlc.arg(id) FOR UPDATE;

-- name: ListCustomPolicies :many
SELECT * FROM custom_policies
WHERE wallet_id = sqlc.arg(wallet_id)
ORDER BY id LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountCustomPolicies :one
SELECT count(*) FROM custom_policies WHERE wallet_id = $1;

-- name: UpdateCustomPolicy :one
UPDATE custom_policies
SET operator = sqlc.arg(operator), value = sqlc.arg(value),
    effective_from = sqlc.arg(effective_from), effective_to = sqlc.narg(effective_to),
    reason = sqlc.narg(reason), updated_at = now()
WHERE wallet_id = sqlc.arg(wallet_id) AND id = sqlc.arg(id)
RETURNING *;

-- name: DeleteCustomPolicy :one
DELETE FROM custom_policies WHERE wallet_id = sqlc.arg(wallet_id) AND id = sqlc.arg(id) RETURNING *;
