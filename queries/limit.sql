-- name: GetUnits :many
SELECT
  id,
  code,
  active,
  created_at,
  updated_at
FROM
  units
ORDER BY
  code;

-- name: GetLimitGroupID :one
SELECT
  id
FROM
  limit_groups
WHERE
  wallet_type = $1
  AND currency = $2
  AND tier = $3;

-- name: GetLimitValuesByGroupID :many
SELECT
  lv.id,
  lv.limit_group_id,
  lv.limit_type_id,
  lt.code AS limit_type,
  lt.unit_id,
  lv.transaction_type,
  lv.channel,
  lv.operator,
  lv.value,
  lv.created_at,
  lv.updated_at
FROM
  limit_values lv
  JOIN limit_types lt ON lt.id = lv.limit_type_id
WHERE
  lv.limit_group_id = $1
  AND lt.active = true
  AND EXISTS (SELECT 1 FROM units u WHERE u.id = lt.unit_id AND u.active = true)
ORDER BY
  lt.code,
  lv.transaction_type,
  lv.channel;

-- name: GetActiveUserLimits :many
SELECT
  ul.id,
  ul.wallet_id,
  ul.limit_type_id,
  lt.code AS limit_type,
  lt.unit_id,
  ul.transaction_type,
  ul.channel,
  ul.operator,
  ul.value,
  ul.effective_from,
  ul.effective_to,
  ul.reason,
  ul.created_by,
  ul.created_at,
  ul.updated_at
FROM
  user_limits ul
  JOIN limit_types lt ON lt.id = ul.limit_type_id
WHERE
  ul.wallet_id = $1
  AND lt.active = true
  AND EXISTS (SELECT 1 FROM units u WHERE u.id = lt.unit_id AND u.active = true)
  AND ul.effective_from <= now()
  AND (
    ul.effective_to IS NULL
    OR ul.effective_to > now()
  )
ORDER BY
  lt.code,
  ul.transaction_type,
  ul.channel;
