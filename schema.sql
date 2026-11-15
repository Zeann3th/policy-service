CREATE TABLE units (
  id bigserial PRIMARY KEY,
  code text NOT NULL UNIQUE,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz,
  CHECK (code = upper(code))
);

INSERT INTO units (code) VALUES ('BALANCE'), ('AMOUNT')
ON CONFLICT (code) DO NOTHING;

CREATE TABLE limit_types (
  id bigserial PRIMARY KEY,
  code text NOT NULL UNIQUE CHECK (code = upper(code)),
  description text,
  unit_id bigint NOT NULL REFERENCES units (id) ON DELETE CASCADE,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz
);

INSERT INTO limit_types (code, description, unit_id)
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

CREATE TABLE limit_groups (
  id bigserial PRIMARY KEY,
  wallet_type text NOT NULL,
  currency varchar(3) NOT NULL,
  tier text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz,
  UNIQUE (wallet_type, currency, tier)
);

CREATE TABLE limit_values (
  id bigserial PRIMARY KEY,
  limit_group_id bigint NOT NULL REFERENCES limit_groups (id) ON DELETE CASCADE,
  limit_type_id bigint NOT NULL REFERENCES limit_types (id) ON DELETE CASCADE,
  transaction_type text NOT NULL DEFAULT 'ALL',
  channel text NOT NULL DEFAULT 'ALL',
  operator text NOT NULL CHECK (
    operator IN ('EQ', 'NEQ', 'GT', 'GTE', 'LT', 'LTE')
  ),
  value numeric(24, 4) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz
);

CREATE UNIQUE INDEX limit_values_identity_idx ON limit_values (
  limit_group_id,
  limit_type_id,
  transaction_type,
  channel
);

CREATE INDEX limit_values_group_page_idx ON limit_values (limit_group_id, id);

CREATE TABLE user_limits (
  id bigserial PRIMARY KEY,
  wallet_id text NOT NULL,
  limit_type_id bigint NOT NULL REFERENCES limit_types (id) ON DELETE CASCADE,
  transaction_type text NOT NULL DEFAULT 'ALL',
  channel text NOT NULL DEFAULT 'ALL',
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

CREATE UNIQUE INDEX user_limits_identity_idx ON user_limits (
  wallet_id,
  limit_type_id,
  transaction_type,
  channel
);

CREATE INDEX user_limits_wallet_page_idx ON user_limits (wallet_id, id);

CREATE INDEX user_limits_expiry_idx ON user_limits (effective_to)
WHERE
  effective_to IS NOT NULL;

CREATE TABLE limit_audit_logs (
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

CREATE INDEX limit_audit_logs_entity_idx ON limit_audit_logs (entity, entity_key, updated_at DESC);
