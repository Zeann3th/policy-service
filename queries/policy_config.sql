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

-- name: GetPolicyGroupID :one
SELECT
  id
FROM
  policy_groups
WHERE
  wallet_type = $1
  AND currency = $2
  AND tier = $3;

-- name: GetPoliciesByGroupID :many
SELECT
  p.id,
  p.policy_group_id,
  p.policy_type_id,
  pt.code AS policy_type,
  pt.unit_id,
  p.transaction_type,
  p.channel,
  p.direction,
  p.operator,
  p.value,
  p.created_at,
  p.updated_at
FROM
  policies p
  JOIN policy_types pt ON pt.id = p.policy_type_id
WHERE
  p.policy_group_id = $1
  AND pt.active = true
  AND EXISTS (SELECT 1 FROM units u WHERE u.id = pt.unit_id AND u.active = true)
ORDER BY
  pt.code,
  p.transaction_type,
  p.channel;

-- name: GetActiveCustomPolicies :many
SELECT
  cp.id,
  cp.wallet_id,
  cp.policy_type_id,
  pt.code AS policy_type,
  pt.unit_id,
  cp.transaction_type,
  cp.channel,
  cp.direction,
  cp.operator,
  cp.value,
  cp.effective_from,
  cp.effective_to,
  cp.reason,
  cp.created_by,
  cp.created_at,
  cp.updated_at
FROM
  custom_policies cp
  JOIN policy_types pt ON pt.id = cp.policy_type_id
WHERE
  cp.wallet_id = $1
  AND pt.active = true
  AND EXISTS (SELECT 1 FROM units u WHERE u.id = pt.unit_id AND u.active = true)
  AND cp.effective_from <= now()
  AND (
    cp.effective_to IS NULL
    OR cp.effective_to > now()
  )
ORDER BY
  pt.code,
  cp.transaction_type,
  cp.channel;
