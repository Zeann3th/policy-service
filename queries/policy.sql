-- name: CreatePolicy :one
INSERT INTO policies
  (policy_group_id, policy_type_id, transaction_type, channel, direction, window_type, is_enforced, unit_id, base, operator,
   value, cap, effective_from, effective_to)
VALUES
  (sqlc.arg(policy_group_id), sqlc.arg(policy_type_id), sqlc.arg(transaction_type),
   sqlc.arg(channel), sqlc.arg(direction), sqlc.arg(window_type), sqlc.arg(is_enforced), sqlc.narg(unit_id), sqlc.narg(base),
   sqlc.narg(operator), sqlc.arg(value), sqlc.narg(cap),
   COALESCE(CAST(sqlc.narg(effective_from) AS timestamptz), now()), sqlc.narg(effective_to))
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
SET is_enforced = sqlc.arg(is_enforced), base = sqlc.narg(base), operator = sqlc.narg(operator), value = sqlc.arg(value), cap = sqlc.narg(cap),
    effective_from = sqlc.arg(effective_from), effective_to = sqlc.narg(effective_to), updated_at = now()
WHERE policy_group_id = sqlc.arg(policy_group_id) AND id = sqlc.arg(id)
RETURNING *;

-- name: DeletePolicy :one
DELETE FROM policies WHERE policy_group_id = sqlc.arg(policy_group_id) AND id = sqlc.arg(id) RETURNING *;
