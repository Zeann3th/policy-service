-- name: HasLimitTypes :one
SELECT
  EXISTS (
    SELECT
      1
    FROM
      limit_types
  );

-- name: GetLimitGroupID :one
SELECT
  id
FROM
  limit_groups
WHERE
  wallet_type_code = $1
  AND currency_code = $2
  AND tier_code = $3;

-- name: GetLimitValuesByGroupID :many
SELECT
  limit_group_id,
  limit_type_code,
  transaction_type_code,
  channel_code,
  operator,
  value,
  created_at,
  updated_at
FROM
  limit_values
WHERE
  limit_group_id = $1
ORDER BY
  limit_type_code,
  transaction_type_code,
  channel_code;
