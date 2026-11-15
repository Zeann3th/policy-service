-- name: CreatePolicy :one
INSERT INTO policies
  (policy_group_id, policy_type_id, transaction_type, channel, direction, operator, value)
VALUES
  (sqlc.arg(policy_group_id), sqlc.arg(policy_type_id), sqlc.arg(transaction_type),
   sqlc.arg(channel), sqlc.arg(direction), sqlc.arg(operator), sqlc.arg(value))
RETURNING *;

-- name: GetPolicyByID :one
SELECT * FROM policies WHERE policy_group_id = sqlc.arg(policy_group_id) AND id = sqlc.arg(id);

-- name: GetPolicyByIDForUpdate :one
SELECT * FROM policies WHERE policy_group_id = sqlc.arg(policy_group_id) AND id = sqlc.arg(id) FOR UPDATE;

-- name: ListPolicies :many
SELECT * FROM policies
WHERE policy_group_id = sqlc.arg(policy_group_id)
ORDER BY id LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountPolicies :one
SELECT count(*) FROM policies WHERE policy_group_id = $1;

-- name: UpdatePolicy :one
UPDATE policies
SET operator = sqlc.arg(operator), value = sqlc.arg(value), updated_at = now()
WHERE policy_group_id = sqlc.arg(policy_group_id) AND id = sqlc.arg(id)
RETURNING *;

-- name: DeletePolicy :one
DELETE FROM policies WHERE policy_group_id = sqlc.arg(policy_group_id) AND id = sqlc.arg(id) RETURNING *;
