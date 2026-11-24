-- updated_at stays NULL until the first real UPDATE
CREATE OR REPLACE FUNCTION set_updated_at () RETURNS trigger AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END
$$ LANGUAGE plpgsql;

-- Catalog tables (units, policy_types) are metadata: names the policies
-- refer to. Everything the engine reads is on the policy row (kind, role,
-- is_enforced, unit, base, operator, window, value, cap). Every policy is
-- returned as config (GET /v1/policies); the engine (reserve) evaluates
-- only rows with is_enforced, and does nothing with a unit it has no
-- measure for. COIN_MIN / COIN_MAX rows are config for the accounting
-- service (is_enforced false).
CREATE TABLE units (
  id bigserial PRIMARY KEY,
  code text NOT NULL UNIQUE,
  description text,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz
);

INSERT INTO units (code, description) VALUES
  ('AMOUNT', 'Money: the transaction amount, or the amount charged'),
  ('COUNT', 'Number of transactions'),
  ('BALANCE', 'Wallet balance; enforced by the accounting service, not on reserve');

-- A policy type is a label: a code and a description. What a policy does is
-- on the policy row, so a new type needs no code change.
CREATE TABLE policy_types (
  id bigserial PRIMARY KEY,
  code text NOT NULL UNIQUE,
  description text,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz
);

INSERT INTO policy_types (code, description) VALUES
  ('COIN_MIN', 'Minimum balance a wallet must keep after a transaction'),
  ('COIN_MAX', 'Maximum balance a wallet may hold after a transaction'),
  ('TXN_MIN', 'Minimum of a single transaction'),
  ('TXN_MAX', 'Maximum of a single transaction'),
  ('TOTAL_MAX', 'Maximum total over a window'),
  ('NO_FEE_TRANSACTION', 'First N transactions per window are free'),
  ('TXN_FEE', 'Fee per transaction'),
  ('TXN_DISCOUNT', 'Discount off the fee or the amount'),
  ('TXN_TAX', 'Tax per transaction');

CREATE TABLE policy_groups (
  id bigserial PRIMARY KEY,
  wallet_type text NOT NULL,
  currency varchar(3) NOT NULL, -- ISO 4217
  tier text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz,
  UNIQUE (wallet_type, currency, tier)
);

-- policies and custom_policies share one id space, so a policy id
-- recorded in usage_ledger never collides across the two tables.
CREATE SEQUENCE policy_id_seq;

-- A policy row is a rule: everything the engine reads (checked by the
-- kind's strategy in the application):
--   kind       LIMIT | WAIVER | AUXILIARY: the strategy that evaluates it
--   role       AUXILIARY only: FEE | DISCOUNT | TAX, how the charge applies
--   is_enforced  false: config only, the engine skips it
--   LIMIT      unit, operator
--   WAIVER     unit; free while window usage < value (always with NONE)
--   AUXILIARY  role; base NULL (flat value) or AMOUNT | FEE | NET_FEE (value
--              is a percentage of it); with a window, cap (in unit) limits
--              what is charged per window
-- window_type: NONE = per transaction, nothing accumulates; DAILY / MONTHLY
-- (UTC calendar); LIFETIME = accumulates and never resets.
-- value and cap are never negative: the role decides the sign.
-- policy_type_id / transaction_type / channel / direction / window_type /
-- unit_id are the identity; they, kind and role are immutable: changing one
-- means a new row.
-- direction: ALL | OUT | IN; operator: EQ | NEQ | GT | GTE | LT | LTE
CREATE TABLE policies (
  id bigint PRIMARY KEY DEFAULT nextval('policy_id_seq'),
  policy_group_id bigint NOT NULL REFERENCES policy_groups (id) ON DELETE CASCADE,
  policy_type_id bigint NOT NULL REFERENCES policy_types (id) ON DELETE CASCADE,
  transaction_type text NOT NULL DEFAULT 'ALL',
  channel text NOT NULL DEFAULT 'ALL',
  direction text NOT NULL DEFAULT 'ALL',
  window_type text NOT NULL DEFAULT 'NONE',
  kind text NOT NULL,
  role text, -- AUXILIARY only
  is_enforced boolean NOT NULL DEFAULT true,
  unit_id bigint REFERENCES units (id) ON DELETE RESTRICT,
  base text, -- AUXILIARY only; NULL: flat
  operator text, -- LIMIT only
  value numeric(24, 4) NOT NULL,
  cap numeric(24, 4), -- AUXILIARY with a window: most charged per window, in unit
  effective_from timestamptz NOT NULL DEFAULT now(),
  effective_to timestamptz, -- NULL: effective with no end
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz
);

CREATE UNIQUE INDEX policies_identity_idx ON policies (
  policy_group_id,
  policy_type_id,
  transaction_type,
  channel,
  direction,
  window_type,
  unit_id
) NULLS NOT DISTINCT;

CREATE INDEX policies_group_page_idx ON policies (policy_group_id, id);

-- resolve: a group's policies effective now
CREATE INDEX policies_group_effective_idx ON policies (policy_group_id, effective_from, effective_to);

CREATE TABLE custom_policies (
  id bigint PRIMARY KEY DEFAULT nextval('policy_id_seq'),
  wallet_id text NOT NULL,
  policy_type_id bigint NOT NULL REFERENCES policy_types (id) ON DELETE CASCADE,
  transaction_type text NOT NULL DEFAULT 'ALL',
  channel text NOT NULL DEFAULT 'ALL',
  direction text NOT NULL DEFAULT 'ALL',
  window_type text NOT NULL DEFAULT 'NONE',
  kind text NOT NULL,
  role text, -- AUXILIARY only
  is_enforced boolean NOT NULL DEFAULT true,
  unit_id bigint REFERENCES units (id) ON DELETE RESTRICT,
  base text, -- AUXILIARY only; NULL: flat
  operator text, -- LIMIT only
  value numeric(24, 4) NOT NULL,
  cap numeric(24, 4), -- AUXILIARY with a window: most charged per window, in unit
  effective_from timestamptz NOT NULL DEFAULT now(),
  effective_to timestamptz,
  reason text,
  created_by text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz
);

CREATE UNIQUE INDEX custom_policies_identity_idx ON custom_policies (
  wallet_id,
  policy_type_id,
  transaction_type,
  channel,
  direction,
  window_type,
  unit_id
) NULLS NOT DISTINCT;

CREATE INDEX custom_policies_wallet_page_idx ON custom_policies (wallet_id, id);

-- resolve: a wallet's custom policies effective now
CREATE INDEX custom_policies_wallet_effective_idx ON custom_policies (wallet_id, effective_from, effective_to);

CREATE INDEX custom_policies_expiry_idx ON custom_policies (effective_to)
WHERE
  effective_to IS NOT NULL;

-- ---------------------------------------------------------------------
-- read-only views per kind, for people: each shows only the columns its
-- kind uses, with type and unit codes. The service reads the tables.
-- ---------------------------------------------------------------------

-- LIMIT: the measured unit, over the window, must satisfy operator value.
CREATE VIEW limit_policies AS
SELECT
  p.id,
  p.policy_group_id,
  pt.code AS policy_type,
  p.transaction_type,
  p.channel,
  p.direction,
  p.window_type,
  u.code AS unit,
  p.operator,
  p.value,
  p.is_enforced,
  p.effective_from,
  p.effective_to
FROM
  policies p
  JOIN policy_types pt ON pt.id = p.policy_type_id
  LEFT JOIN units u ON u.id = p.unit_id
WHERE
  p.kind = 'LIMIT';

CREATE VIEW custom_limit_policies AS
SELECT
  p.id,
  p.wallet_id,
  pt.code AS policy_type,
  p.transaction_type,
  p.channel,
  p.direction,
  p.window_type,
  u.code AS unit,
  p.operator,
  p.value,
  p.is_enforced,
  p.effective_from,
  p.effective_to,
  p.reason,
  p.created_by
FROM
  custom_policies p
  JOIN policy_types pt ON pt.id = p.policy_type_id
  LEFT JOIN units u ON u.id = p.unit_id
WHERE
  p.kind = 'LIMIT';

-- WAIVER: the fee is waived while usage of unit in the window, before the
-- transaction, is below value.
CREATE VIEW waiver_policies AS
SELECT
  p.id,
  p.policy_group_id,
  pt.code AS policy_type,
  p.transaction_type,
  p.channel,
  p.direction,
  p.window_type,
  u.code AS unit,
  p.value,
  p.is_enforced,
  p.effective_from,
  p.effective_to
FROM
  policies p
  JOIN policy_types pt ON pt.id = p.policy_type_id
  LEFT JOIN units u ON u.id = p.unit_id
WHERE
  p.kind = 'WAIVER';

CREATE VIEW custom_waiver_policies AS
SELECT
  p.id,
  p.wallet_id,
  pt.code AS policy_type,
  p.transaction_type,
  p.channel,
  p.direction,
  p.window_type,
  u.code AS unit,
  p.value,
  p.is_enforced,
  p.effective_from,
  p.effective_to,
  p.reason,
  p.created_by
FROM
  custom_policies p
  JOIN policy_types pt ON pt.id = p.policy_type_id
  LEFT JOIN units u ON u.id = p.unit_id
WHERE
  p.kind = 'WAIVER';

-- AUXILIARY: role charges value, flat or as a percentage of base; when
-- capped, window_type / unit / cap say how much is charged per window.
CREATE VIEW auxiliary_policies AS
SELECT
  p.id,
  p.policy_group_id,
  pt.code AS policy_type,
  p.transaction_type,
  p.channel,
  p.direction,
  p.role,
  p.base,
  p.value,
  p.window_type,
  u.code AS unit,
  p.cap,
  p.is_enforced,
  p.effective_from,
  p.effective_to
FROM
  policies p
  JOIN policy_types pt ON pt.id = p.policy_type_id
  LEFT JOIN units u ON u.id = p.unit_id
WHERE
  p.kind = 'AUXILIARY';

CREATE VIEW custom_auxiliary_policies AS
SELECT
  p.id,
  p.wallet_id,
  pt.code AS policy_type,
  p.transaction_type,
  p.channel,
  p.direction,
  p.role,
  p.base,
  p.value,
  p.window_type,
  u.code AS unit,
  p.cap,
  p.is_enforced,
  p.effective_from,
  p.effective_to,
  p.reason,
  p.created_by
FROM
  custom_policies p
  JOIN policy_types pt ON pt.id = p.policy_type_id
  LEFT JOIN units u ON u.id = p.unit_id
WHERE
  p.kind = 'AUXILIARY';

-- ---------------------------------------------------------------------
-- audit logs: native range partitions by month via pg_partman
-- partition key (updated_at) must be part of the primary key
-- ---------------------------------------------------------------------
CREATE TABLE policy_audit_logs (
  id bigserial,
  entity text NOT NULL, -- unit | policy_type | policy_group | policy | custom_policy
  entity_key text NOT NULL,
  action text NOT NULL, -- INSERT | UPDATE | DELETE
  old_value jsonb,
  new_value jsonb,
  updated_by text NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now(), -- partition key, must be NOT NULL
  PRIMARY KEY (id, updated_at)
)
PARTITION BY
  RANGE (updated_at);

CREATE INDEX policy_audit_logs_entity_idx ON policy_audit_logs (entity, entity_key, updated_at DESC);

-- pg_partman v5 syntax (v4 also needs p_type => 'native')
SELECT
  partman.create_parent (
    p_parent_table := 'public.policy_audit_logs',
    p_control := 'updated_at',
    p_interval := '1 month',
    p_premake := 4
  );

-- ---------------------------------------------------------------------
-- usages: running totals per wallet and scope. The scope comes from the
-- type, selectors, window and unit of the matched policy, never from a policy id, so usage
-- outlives policy edits. Stateless policies (window NONE) have no row.
-- window_start: the day (DAILY), the first day of the month (MONTHLY) or
-- 1970-01-01 (LIFETIME), in UTC.
-- unit COUNT adds 1 to value, unit AMOUNT adds the money measured (the
-- transaction amount for LIMIT / WAIVER, the amount charged for AUXILIARY).
-- List partitions reject any window_type other than the three below.
-- ---------------------------------------------------------------------
CREATE TABLE usages (
  wallet_id text NOT NULL,
  policy_type_id bigint NOT NULL REFERENCES policy_types (id) ON DELETE RESTRICT,
  transaction_type text NOT NULL DEFAULT 'ALL',
  channel text NOT NULL DEFAULT 'ALL',
  direction text NOT NULL DEFAULT 'ALL',
  window_type text NOT NULL,
  unit_id bigint NOT NULL REFERENCES units (id) ON DELETE RESTRICT,
  window_start date NOT NULL,
  value numeric(24, 4) NOT NULL DEFAULT 0,
  PRIMARY KEY (
    wallet_id,
    policy_type_id,
    transaction_type,
    channel,
    direction,
    window_type,
    unit_id,
    window_start
  )
)
PARTITION BY
  LIST (window_type);

CREATE TABLE usage_daily PARTITION OF usages
  FOR VALUES IN ('DAILY') PARTITION BY RANGE (window_start);

CREATE TABLE usage_monthly PARTITION OF usages
  FOR VALUES IN ('MONTHLY') WITH (fillfactor = 75);

CREATE TABLE usage_lifetime PARTITION OF usages
  FOR VALUES IN ('LIFETIME') WITH (fillfactor = 75);

-- storage parameters cannot be set on a partitioned parent; the template
-- carries fillfactor to the daily partitions that pg_partman creates
CREATE TABLE usage_daily_template
  (LIKE usages INCLUDING DEFAULTS INCLUDING CONSTRAINTS INCLUDING INDEXES)
  WITH (fillfactor = 75);

SELECT
  partman.create_parent (
    p_parent_table := 'public.usage_daily',
    p_control := 'window_start',
    p_interval := '1 day',
    p_premake := 7,
    p_template_table := 'public.usage_daily_template'
  );

-- ---------------------------------------------------------------------
-- reservations: one row per request, partitioned monthly on created_at.
-- request_id is unique per partition (via the template table): Postgres
-- cannot enforce a unique index without the partition key on the parent.
-- status: PROCESSING | RESERVED | RELEASED | REJECTED | FAILED
-- ---------------------------------------------------------------------
CREATE TABLE reservations (
  id uuid NOT NULL, -- UUIDv7 from the application
  request_id text NOT NULL, -- the caller's requestId (uuid string or any string)
  status text NOT NULL,
  transaction_type text NOT NULL,
  channel text NOT NULL DEFAULT 'ALL',
  currency varchar(3) NOT NULL,
  amount numeric(24, 4) NOT NULL,
  sender_wallet_id text NOT NULL,
  sender_wallet_type text NOT NULL,
  sender_tier text NOT NULL,
  receiver_wallet_id text NOT NULL,
  receiver_wallet_type text NOT NULL,
  receiver_tier text NOT NULL,
  result jsonb, -- the response; null while PROCESSING
  created_at timestamptz NOT NULL DEFAULT now(), -- partition key, must be NOT NULL
  updated_at timestamptz,
  PRIMARY KEY (id, created_at)
)
PARTITION BY
  RANGE (created_at);

CREATE INDEX reservations_processing_idx ON reservations (created_at)
WHERE
  status = 'PROCESSING';

CREATE TABLE reservations_template
  (LIKE reservations INCLUDING DEFAULTS INCLUDING CONSTRAINTS);

ALTER TABLE reservations_template ADD UNIQUE (request_id);

SELECT
  partman.create_parent (
    p_parent_table := 'public.reservations',
    p_control := 'created_at',
    p_interval := '1 month',
    p_premake := 4,
    p_template_table := 'public.reservations_template'
  );

-- ---------------------------------------------------------------------
-- usage_ledger: immutable journal of every change to usages, one row per
-- reservation x entry_type x wallet x scope. RESERVE adds delta, RELEASE
-- records the reversal with a negative delta, so for every usages row
-- value = sum(delta) of its ledger rows. Each row copies the full key of
-- the usages row it touched plus a snapshot of the policy that applied;
-- snapshot columns have no foreign keys on purpose.
-- Partitioned monthly on reservation_created_at, so every entry of a
-- reservation lives in the same month as the reservation itself.
-- entry_type: RESERVE | RELEASE
-- ---------------------------------------------------------------------
CREATE TABLE usage_ledger (
  reservation_id uuid NOT NULL,
  reservation_created_at timestamptz NOT NULL,
  entry_type text NOT NULL,
  wallet_id text NOT NULL,
  policy_type_id bigint NOT NULL REFERENCES policy_types (id) ON DELETE RESTRICT,
  transaction_type text NOT NULL,
  channel text NOT NULL,
  direction text NOT NULL,
  window_type text NOT NULL,
  unit_id bigint NOT NULL REFERENCES units (id) ON DELETE RESTRICT,
  window_start date NOT NULL,
  delta numeric(24, 4) NOT NULL, -- RESERVE: 1 for COUNT, the amount for AMOUNT; RELEASE: the negation
  -- audit snapshot
  policy_id bigint, -- id in policies or custom_policies (shared policy_id_seq)
  policy_type_code text NOT NULL,
  operator text,
  limit_value numeric(24, 4),
  value_after numeric(24, 4) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (
    reservation_id,
    reservation_created_at,
    entry_type,
    wallet_id,
    policy_type_id,
    transaction_type,
    channel,
    direction,
    window_type,
    unit_id
  ),
  FOREIGN KEY (reservation_id, reservation_created_at)
    REFERENCES reservations (id, created_at) ON DELETE RESTRICT
)
PARTITION BY
  RANGE (reservation_created_at);

SELECT
  partman.create_parent (
    p_parent_table := 'public.usage_ledger',
    p_control := 'reservation_created_at',
    p_interval := '1 month',
    p_premake := 4
  );

CREATE TABLE outbox (
  id bigserial PRIMARY KEY,
  event_type text NOT NULL,
  event_key text NOT NULL,
  payload jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  published_at timestamptz
);

CREATE INDEX outbox_unpublished_idx ON outbox (id)
WHERE
  published_at IS NULL;

-- ---------------------------------------------------------------------
-- updated_at triggers: fire only when the row actually changed
-- ---------------------------------------------------------------------
CREATE TRIGGER units_set_updated_at
BEFORE UPDATE ON units FOR EACH ROW WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION set_updated_at ();

CREATE TRIGGER policy_types_set_updated_at
BEFORE UPDATE ON policy_types FOR EACH ROW WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION set_updated_at ();

CREATE TRIGGER policy_groups_set_updated_at
BEFORE UPDATE ON policy_groups FOR EACH ROW WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION set_updated_at ();

CREATE TRIGGER policies_set_updated_at
BEFORE UPDATE ON policies FOR EACH ROW WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION set_updated_at ();

CREATE TRIGGER custom_policies_set_updated_at
BEFORE UPDATE ON custom_policies FOR EACH ROW WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION set_updated_at ();

CREATE TRIGGER reservations_set_updated_at
BEFORE UPDATE ON reservations FOR EACH ROW WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION set_updated_at ();

