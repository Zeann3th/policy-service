-- name: CreatePolicyGroup :one
INSERT INTO policy_groups (wallet_type, currency, tier)
VALUES (sqlc.arg(wallet_type), upper(CAST(sqlc.arg(currency) AS text)), sqlc.arg(tier)) RETURNING *;

-- name: GetPolicyGroupByID :one
SELECT * FROM policy_groups WHERE id = $1;

-- name: DeletePolicyGroup :one
DELETE FROM policy_groups WHERE id = $1 RETURNING *;

-- name: ListPolicyGroups :many
SELECT * FROM policy_groups
ORDER BY wallet_type, currency, tier LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountPolicyGroups :one
SELECT count(*) FROM policy_groups;
