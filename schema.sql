-- updated_at stays NULL until the first real UPDATE
CREATE OR REPLACE FUNCTION set_updated_at () RETURNS trigger AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END
$$ LANGUAGE plpgsql;

CREATE TABLE units (
  id bigserial PRIMARY KEY,
  code text NOT NULL UNIQUE,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz
);

INSERT INTO units (code) VALUES ('BALANCE'), ('AMOUNT'), ('COUNT'), ('PERCENT');

-- kind: LIMIT | WAIVER | FEE | TAX | DISCOUNT
-- The window lives on the policy rows, not on the type.
CREATE TABLE policy_types (
  id bigserial PRIMARY KEY,
  code text NOT NULL UNIQUE,
  description text,
  unit_id bigint NOT NULL REFERENCES units (id) ON DELETE RESTRICT,
  kind text NOT NULL DEFAULT 'LIMIT',
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz
);

INSERT INTO policy_types (code, description, unit_id, kind)
SELECT v.code, v.description, u.id, v.kind
FROM (VALUES
  ('COIN_MIN', 'Minimum balance a wallet must keep after a transaction', 'BALANCE', 'LIMIT'),
  ('COIN_MAX', 'Maximum balance a wallet may hold after a transaction', 'BALANCE', 'LIMIT'),
  ('TXN_MIN_AMT', 'Minimum amount of a single transaction', 'AMOUNT', 'LIMIT'),
  ('TXN_MAX_AMT', 'Maximum amount of a single transaction', 'AMOUNT', 'LIMIT'),
  ('TOTAL_MAX_AMT', 'Maximum total over a window', 'AMOUNT', 'LIMIT'),
  ('NO_FEE_TRANSACTION', 'First N transactions per window are free', 'COUNT', 'WAIVER'),
  ('FEE_AMT', 'Flat fee per transaction', 'AMOUNT', 'FEE')
) AS v (code, description, unit, kind)
JOIN units u ON u.code = v.unit;

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

-- transaction_type / channel / direction / window_type are selectors and
-- are immutable: changing one means a new row.
-- direction: ALL | OUT | IN; window_type: NONE | DAILY | MONTHLY | LIFETIME
-- operator: EQ | NEQ | GT | GTE | LT | LTE
CREATE TABLE policies (
  id bigint PRIMARY KEY DEFAULT nextval('policy_id_seq'),
  policy_group_id bigint NOT NULL REFERENCES policy_groups (id) ON DELETE CASCADE,
  policy_type_id bigint NOT NULL REFERENCES policy_types (id) ON DELETE CASCADE,
  transaction_type text NOT NULL DEFAULT 'ALL',
  channel text NOT NULL DEFAULT 'ALL',
  direction text NOT NULL DEFAULT 'ALL',
  window_type text NOT NULL DEFAULT 'NONE',
  operator text NOT NULL,
  value numeric(24, 4) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz
);

CREATE UNIQUE INDEX policies_identity_idx ON policies (
  policy_group_id,
  policy_type_id,
  transaction_type,
  channel,
  direction,
  window_type
);

CREATE INDEX policies_group_page_idx ON policies (policy_group_id, id);

CREATE TABLE custom_policies (
  id bigint PRIMARY KEY DEFAULT nextval('policy_id_seq'),
  wallet_id text NOT NULL,
  policy_type_id bigint NOT NULL REFERENCES policy_types (id) ON DELETE CASCADE,
  transaction_type text NOT NULL DEFAULT 'ALL',
  channel text NOT NULL DEFAULT 'ALL',
  direction text NOT NULL DEFAULT 'ALL',
  window_type text NOT NULL DEFAULT 'NONE',
  operator text NOT NULL,
  value numeric(24, 4) NOT NULL,
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
  window_type
);

CREATE INDEX custom_policies_wallet_page_idx ON custom_policies (wallet_id, id);

CREATE INDEX custom_policies_expiry_idx ON custom_policies (effective_to)
WHERE
  effective_to IS NOT NULL;

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
-- selectors of the matched policy, never from a policy id, so usage
-- outlives policy edits. Stateless policies (window NONE) have no row.
-- window_start: the day (DAILY), the first day of the month (MONTHLY) or
-- 1970-01-01 (LIFETIME), in UTC.
-- value: COUNT adds 1, AMOUNT adds the transaction amount.
-- List partitions reject any window_type other than the three below.
-- ---------------------------------------------------------------------
CREATE TABLE usages (
  wallet_id text NOT NULL,
  policy_type_id bigint NOT NULL REFERENCES policy_types (id) ON DELETE RESTRICT,
  transaction_type text NOT NULL DEFAULT 'ALL',
  channel text NOT NULL DEFAULT 'ALL',
  direction text NOT NULL DEFAULT 'ALL',
  window_type text NOT NULL,
  window_start date NOT NULL,
  value numeric(24, 4) NOT NULL DEFAULT 0,
  PRIMARY KEY (
    wallet_id,
    policy_type_id,
    transaction_type,
    channel,
    direction,
    window_type,
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
    window_type
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

CREATE TABLE processed_events (
  event_id text PRIMARY KEY,
  processed_at timestamptz NOT NULL DEFAULT now()
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

