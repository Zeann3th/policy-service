-- e2e seed: policy groups per wallet type and tier, system limits / fees /
-- waivers / discounts / tax, and custom policies for half of 1000 wallets.
-- Idempotent: safe to run on every `docker compose up`.
--
-- Wallet scheme, shared with e2e/k6/lib.js (keep both in sync):
--   id    e2e-w-0001 .. e2e-w-1000
--   type  MERCHANT when i % 10 = 0, else PERSONAL
--   tier  T1 when i % 3 = 1, T2 when i % 3 = 2, T3 when i % 3 = 0
--   custom policies for i <= 500, profile = i % 5 (see below)
--   currency VND

CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Uses only the baseline catalog (units AMOUNT / COUNT / BALANCE and the
-- seeded policy types). Each policy reads as one row:
--   unit      what a LIMIT / WAIVER measures, or what an AUXILIARY cap counts
--             (BALANCE limits are served for the accounting service, not
--             enforced on reserve)
--   base      what an AUXILIARY percentage is of (NULL = flat amount)
--   operator  how a LIMIT compares (NULL for WAIVER / AUXILIARY)
--   cap       most an AUXILIARY policy charges per window (NULL = no cap)

INSERT INTO policy_groups (wallet_type, currency, tier)
SELECT w, 'VND', t
FROM unnest(ARRAY['PERSONAL', 'MERCHANT']) w, unnest(ARRAY['T1', 'T2', 'T3']) t
ON CONFLICT (wallet_type, currency, tier) DO NOTHING;

-- System policies. factor scales TXN_MAX / TOTAL_MAX by tier (T1 1x, T2 5x,
-- T3 20x); merchants receive 10x more per day.
WITH g AS (
  SELECT id, wallet_type, tier,
         CASE tier WHEN 'T1' THEN 1 WHEN 'T2' THEN 5 ELSE 20 END AS factor
  FROM policy_groups WHERE currency = 'VND'
), rules (code, transaction_type, direction, window_type, unit, base, operator, value, cap, only_tier, only_type) AS (
  VALUES
    -- code              txn type    side   window     unit       base       op     value                cap   tier  type
    -- sender (OUT)
    ('COIN_MIN',          'ALL',      'OUT', 'NONE',    'BALANCE', NULL,      'GTE', 0::numeric,          NULL::numeric, NULL, NULL),
    ('TXN_MIN',           'ALL',      'OUT', 'NONE',    'AMOUNT',  NULL,      'GTE', 1000,                NULL, NULL, NULL),
    ('TXN_MAX',           'ALL',      'OUT', 'NONE',    'AMOUNT',  NULL,      'LTE', 20000000,            NULL, NULL, NULL),
    ('TOTAL_MAX',         'ALL',      'OUT', 'DAILY',   'AMOUNT',  NULL,      'LTE', 100000000,           NULL, NULL, NULL),
    ('TOTAL_MAX',         'ALL',      'OUT', 'MONTHLY', 'AMOUNT',  NULL,      'LTE', 1000000000,          NULL, NULL, NULL),
    ('TOTAL_MAX',         'ALL',      'OUT', 'DAILY',   'COUNT',   NULL,      'LTE', 2000,                NULL, NULL, NULL),
    ('NO_FEE_TRANSACTION','TRANSFER', 'OUT', 'MONTHLY', 'COUNT',   NULL,      NULL,  5,                   NULL, NULL, NULL),
    ('TXN_FEE',           'TRANSFER', 'OUT', 'NONE',    NULL,      NULL,      NULL,  1100,                NULL, NULL, NULL),
    ('TXN_FEE',           'PAYMENT',  'OUT', 'NONE',    NULL,      'AMOUNT',  NULL,  0.5,                 NULL, NULL, NULL),
    ('TXN_DISCOUNT',      'ALL',      'OUT', 'NONE',    NULL,      'FEE',     NULL,  10,                  NULL, 'T3', NULL),
    ('TXN_TAX',           'ALL',      'OUT', 'NONE',    NULL,      'NET_FEE', NULL,  10,                  NULL, NULL, NULL),
    -- receiver (IN)
    ('COIN_MAX',          'ALL',      'IN',  'NONE',    'BALANCE', NULL,      'LTE', 100000000000,        NULL, NULL, NULL),
    ('TOTAL_MAX',         'ALL',      'IN',  'DAILY',   'AMOUNT',  NULL,      'LTE', 500000000,           NULL, NULL, NULL),
    ('TXN_FEE',           'PAYMENT',  'IN',  'NONE',    NULL,      'AMOUNT',  NULL,  1,                   NULL, NULL, 'MERCHANT')
)
INSERT INTO policies (policy_group_id, policy_type_id, transaction_type, channel, direction, window_type, unit_id, base, operator, value, cap)
SELECT g.id, pt.id, r.transaction_type, 'ALL', r.direction, r.window_type, u.id, r.base, r.operator,
       CASE
         WHEN r.code IN ('TXN_MAX', 'TOTAL_MAX')
           THEN r.value * g.factor * CASE WHEN r.direction = 'IN' AND g.wallet_type = 'MERCHANT' THEN 10 ELSE 1 END
         ELSE r.value
       END,
       r.cap
FROM g
CROSS JOIN rules r
JOIN policy_types pt ON pt.code = r.code
LEFT JOIN units u ON u.code = r.unit
WHERE (r.only_tier IS NULL OR r.only_tier = g.tier)
  AND (r.only_type IS NULL OR r.only_type = g.wallet_type)
ON CONFLICT (policy_group_id, policy_type_id, transaction_type, channel, direction, window_type, unit_id) DO NOTHING;

-- Custom policies for wallets 1..500, by profile = i % 5:
--   0 TIGHT     daily OUT total 2,000,000 (overrides system; produces REJECTED)
--   1 VIP       daily OUT total 10B and per-txn max 500M (overrides system)
--   2 FEE_FREE  1,000,000 free TRANSFERs a month (overrides the waiver), and
--               1% cashback on PAYMENT, at most 50,000 a month
--   3 PROMO     flat fee 2,000 with 50% discount (fee override + new discount)
--   4 APP_CAP   extra APP-channel daily cap 5,000,000 and 50 txns/day
-- Every 50th wallet also has an expired override that must be ignored.
WITH w AS (
  SELECT i, format('e2e-w-%s', lpad(i::text, 4, '0')) AS wallet_id
  FROM generate_series(1, 500) i
), customs (profile, code, transaction_type, channel, direction, window_type, unit, base, operator, value, cap) AS (
  VALUES
    -- profile code               txn type    channel side   window     unit      base      op     value               cap
    (0, 'TOTAL_MAX',              'ALL',      'ALL',  'OUT', 'DAILY',   'AMOUNT', NULL,     'LTE', 2000000::numeric,   NULL::numeric),
    (1, 'TOTAL_MAX',              'ALL',      'ALL',  'OUT', 'DAILY',   'AMOUNT', NULL,     'LTE', 10000000000,        NULL),
    (1, 'TXN_MAX',                'ALL',      'ALL',  'OUT', 'NONE',    'AMOUNT', NULL,     'LTE', 500000000,          NULL),
    (2, 'NO_FEE_TRANSACTION',     'TRANSFER', 'ALL',  'OUT', 'MONTHLY', 'COUNT',  NULL,     NULL,  1000000,            NULL),
    (2, 'TXN_DISCOUNT',           'PAYMENT',  'ALL',  'OUT', 'MONTHLY', 'AMOUNT', 'AMOUNT', NULL,  1,                  50000),
    (3, 'TXN_FEE',                'TRANSFER', 'ALL',  'OUT', 'NONE',    NULL,     NULL,     NULL,  2000,               NULL),
    (3, 'TXN_DISCOUNT',           'ALL',      'ALL',  'OUT', 'NONE',    NULL,     'FEE',    NULL,  50,                 NULL),
    (4, 'TOTAL_MAX',              'ALL',      'APP',  'OUT', 'DAILY',   'AMOUNT', NULL,     'LTE', 5000000,            NULL),
    (4, 'TOTAL_MAX',              'ALL',      'ALL',  'OUT', 'DAILY',   'COUNT',  NULL,     'LTE', 50,                 NULL)
)
INSERT INTO custom_policies (wallet_id, policy_type_id, transaction_type, channel, direction, window_type, unit_id, base, operator, value, cap, effective_from, reason, created_by)
SELECT w.wallet_id, pt.id, c.transaction_type, c.channel, c.direction, c.window_type, u.id, c.base, c.operator, c.value, c.cap,
       now() - interval '1 day', format('e2e profile %s', c.profile), 'e2e-seed'
FROM w
JOIN customs c ON c.profile = w.i % 5
JOIN policy_types pt ON pt.code = c.code
LEFT JOIN units u ON u.code = c.unit
ON CONFLICT (wallet_id, policy_type_id, transaction_type, channel, direction, window_type, unit_id) DO NOTHING;

-- Expired: would block everything if it were applied.
INSERT INTO custom_policies (wallet_id, policy_type_id, transaction_type, channel, direction, window_type, unit_id, operator, value, effective_from, effective_to, reason, created_by)
SELECT format('e2e-w-%s', lpad(i::text, 4, '0')), pt.id, 'ALL', 'ALL', 'OUT', 'NONE', u.id, 'LTE', 1,
       now() - interval '30 days', now() - interval '1 day', 'e2e expired', 'e2e-seed'
FROM generate_series(50, 500, 50) i
JOIN policy_types pt ON pt.code = 'TXN_MAX'
JOIN units u ON u.code = 'AMOUNT'
ON CONFLICT (wallet_id, policy_type_id, transaction_type, channel, direction, window_type, unit_id) DO NOTHING;

SELECT
  (SELECT count(*) FROM policy_groups) AS groups,
  (SELECT count(*) FROM policies) AS policies,
  (SELECT count(DISTINCT wallet_id) FROM custom_policies) AS custom_wallets,
  (SELECT count(*) FROM custom_policies) AS custom_policies;
