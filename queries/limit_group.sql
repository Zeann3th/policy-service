-- name: CreateLimitGroup :one
INSERT INTO limit_groups (wallet_type, currency, tier)
VALUES (sqlc.arg(wallet_type), upper(CAST(sqlc.arg(currency) AS text)), sqlc.arg(tier)) RETURNING *;

-- name: GetLimitGroupByID :one
SELECT * FROM limit_groups WHERE id = $1;

-- name: DeleteLimitGroup :one
DELETE FROM limit_groups WHERE id = $1 RETURNING *;
