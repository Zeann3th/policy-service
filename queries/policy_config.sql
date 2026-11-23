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
  pt.kind,
  p.transaction_type,
  p.channel,
  p.direction,
  p.window_type,
  p.unit,
  p.base,
  p.operator,
  p.value,
  p.effective_from,
  p.effective_to,
  p.created_at,
  p.updated_at
FROM
  policies p
  JOIN policy_types pt ON pt.id = p.policy_type_id
WHERE
  p.policy_group_id = $1
  AND p.effective_from <= now()
  AND (p.effective_to IS NULL OR p.effective_to > now())
  AND pt.active = true
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
  pt.kind,
  cp.transaction_type,
  cp.channel,
  cp.direction,
  cp.window_type,
  cp.unit,
  cp.base,
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
  AND cp.effective_from <= now()
  AND (
    cp.effective_to IS NULL
    OR cp.effective_to > now()
  )
ORDER BY
  pt.code,
  cp.transaction_type,
  cp.channel;

-- name: GetNextPolicyStart :one
-- The next time a policy of the group becomes effective; bounds the cache.
SELECT effective_from
FROM policies
WHERE policy_group_id = $1 AND effective_from > now()
ORDER BY effective_from
LIMIT 1;
