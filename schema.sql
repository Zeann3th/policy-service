CREATE TABLE units (
  id bigserial PRIMARY KEY,
  code text NOT NULL UNIQUE,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz,
  CHECK (code = upper(code))
);

INSERT INTO units (code) VALUES ('BALANCE'), ('AMOUNT'), ('COUNT'), ('PERCENT')
ON CONFLICT (code) DO NOTHING;

CREATE TABLE policy_types (
  id bigserial PRIMARY KEY,
  code text NOT NULL UNIQUE CHECK (code = upper(code)),
  description text,
  unit_id bigint NOT NULL REFERENCES units (id) ON DELETE CASCADE,
  kind text NOT NULL DEFAULT 'LIMIT' CHECK (
    kind IN ('LIMIT', 'WAIVER', 'FEE', 'TAX', 'DISCOUNT')
  ),
  window_type text NOT NULL DEFAULT 'NONE' CHECK (
    window_type IN ('NONE', 'DAILY', 'MONTHLY', 'LIFETIME')
  ),
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz,
  UNIQUE (id, window_type)
);

INSERT INTO policy_types (code, description, unit_id)
SELECT v.code, v.description, u.id
FROM (VALUES
  ('COIN_MIN', 'Minimum balance a wallet must keep after a transaction', 'BALANCE'),
  ('COIN_MAX', 'Maximum balance a wallet may hold after a transaction', 'BALANCE'),
  ('TXN_MIN_AMT', 'Minimum amount of a single transaction', 'AMOUNT'),
  ('TXN_MAX_AMT', 'Maximum amount of a single transaction', 'AMOUNT'),
  ('DAILY_OUT_MAX', 'Maximum total sent out per day (counter lives in policy)', 'AMOUNT')
) AS v (code, description, unit)
JOIN units u ON u.code = v.unit
ON CONFLICT (code) DO NOTHING;

CREATE TABLE policy_groups (
  id bigserial PRIMARY KEY,
  wallet_type text NOT NULL,
  currency varchar(3) NOT NULL,
  tier text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz,
  UNIQUE (wallet_type, currency, tier)
);

CREATE TABLE policies (
  id bigserial PRIMARY KEY,
  policy_group_id bigint NOT NULL REFERENCES policy_groups (id) ON DELETE CASCADE,
  policy_type_id bigint NOT NULL REFERENCES policy_types (id) ON DELETE CASCADE,
  transaction_type text NOT NULL DEFAULT 'ALL',
  channel text NOT NULL DEFAULT 'ALL',
  direction text NOT NULL DEFAULT 'ALL' CHECK (
    direction IN ('ALL', 'OUT', 'IN')
  ),
  window_type text NOT NULL DEFAULT 'NONE' CHECK (
    window_type IN ('NONE', 'DAILY', 'MONTHLY', 'LIFETIME')
  ),
  operator text NOT NULL CHECK (
    operator IN ('EQ', 'NEQ', 'GT', 'GTE', 'LT', 'LTE')
  ),
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
  id bigserial PRIMARY KEY,
  wallet_id text NOT NULL,
  policy_type_id bigint NOT NULL REFERENCES policy_types (id) ON DELETE CASCADE,
  transaction_type text NOT NULL DEFAULT 'ALL',
  channel text NOT NULL DEFAULT 'ALL',
  direction text NOT NULL DEFAULT 'ALL' CHECK (
    direction IN ('ALL', 'OUT', 'IN')
  ),
  window_type text NOT NULL DEFAULT 'NONE' CHECK (
    window_type IN ('NONE', 'DAILY', 'MONTHLY', 'LIFETIME')
  ),
  operator text NOT NULL CHECK (
    operator IN ('EQ', 'NEQ', 'GT', 'GTE', 'LT', 'LTE')
  ),
  value numeric(24, 4) NOT NULL,
  effective_from timestamptz NOT NULL DEFAULT now(),
  effective_to timestamptz,
  reason text,
  created_by text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz,
  CHECK (
    effective_to IS NULL
    OR effective_from < effective_to
  )
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

CREATE TABLE policy_audit_logs (
  id bigserial,
  entity text NOT NULL,
  entity_key text NOT NULL,
  action text NOT NULL CHECK (action IN ('INSERT', 'UPDATE', 'DELETE')),
  old_value jsonb,
  new_value jsonb,
  updated_by text NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (id, updated_at)
)
PARTITION BY
  RANGE (updated_at);

CREATE INDEX policy_audit_logs_entity_idx ON policy_audit_logs (entity, entity_key, updated_at DESC);

CREATE TABLE counters (
  id bigserial PRIMARY KEY,
  policy_type_id bigint NOT NULL,
  window_type text NOT NULL CHECK (window_type <> 'NONE'),
  transaction_type text NOT NULL DEFAULT 'ALL',
  channel text NOT NULL DEFAULT 'ALL',
  direction text NOT NULL DEFAULT 'ALL',
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (policy_type_id, transaction_type, channel, direction),
  UNIQUE (id, window_type),
  FOREIGN KEY (policy_type_id, window_type)
    REFERENCES policy_types (id, window_type) ON DELETE RESTRICT
);

CREATE TABLE usage (
  wallet_id text NOT NULL,
  counter_id bigint NOT NULL,
  window_type text NOT NULL,
  window_start date NOT NULL,
  value numeric(24, 4) NOT NULL DEFAULT 0,
  PRIMARY KEY (wallet_id, counter_id, window_type, window_start),
  FOREIGN KEY (counter_id, window_type)
    REFERENCES counters (id, window_type) ON DELETE RESTRICT
) PARTITION BY LIST (window_type);

CREATE TABLE usage_daily PARTITION OF usage
  FOR VALUES IN ('DAILY') PARTITION BY RANGE (window_start);

CREATE TABLE usage_monthly PARTITION OF usage
  FOR VALUES IN ('MONTHLY') WITH (fillfactor = 75);

CREATE TABLE usage_lifetime PARTITION OF usage
  FOR VALUES IN ('LIFETIME') WITH (fillfactor = 75);

CREATE TABLE usage_daily_template
  (LIKE usage INCLUDING DEFAULTS INCLUDING CONSTRAINTS INCLUDING INDEXES)
  WITH (fillfactor = 75);

SELECT partman.create_parent(
  p_parent_table := 'public.usage_daily',
  p_control := 'window_start',
  p_interval := '1 day',
  p_premake := 7,
  p_template_table := 'public.usage_daily_template'
);
