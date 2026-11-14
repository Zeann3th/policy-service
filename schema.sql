CREATE TABLE reference_codes (
  kind text NOT NULL,
  code text NOT NULL,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz,
  PRIMARY KEY (kind, code),
  CHECK (code = upper(code))
);

CREATE TABLE limit_types (
  code text PRIMARY KEY CHECK (code = upper(code)),
  description text,
  unit text,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz
);

CREATE TABLE limit_groups (
  id bigserial PRIMARY KEY,
  wallet_type_code text NOT NULL,
  currency_code varchar(3) NOT NULL,
  tier_code text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz,
  UNIQUE (wallet_type_code, currency_code, tier_code)
);

CREATE TABLE limit_values (
  limit_group_id bigint NOT NULL REFERENCES limit_groups (id) ON DELETE CASCADE,
  limit_type_code text NOT NULL REFERENCES limit_types (code),
  transaction_type_code text NOT NULL DEFAULT 'ALL',
  channel_code text NOT NULL DEFAULT 'ALL',
  operator text NOT NULL CHECK (
    operator IN ('EQ', 'NEQ', 'GT', 'GTE', 'LT', 'LTE')
  ),
  value numeric(24, 4) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz,
  PRIMARY KEY (
    limit_group_id,
    limit_type_code,
    transaction_type_code,
    channel_code
  )
);

CREATE TABLE user_limits (
  wallet_id text NOT NULL,
  limit_type_code text NOT NULL REFERENCES limit_types (code),
  transaction_type_code text NOT NULL DEFAULT 'ALL',
  channel_code text NOT NULL DEFAULT 'ALL',
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
  PRIMARY KEY (
    wallet_id,
    limit_type_code,
    transaction_type_code,
    channel_code
  ),
  CHECK (
    effective_to IS NULL
    OR effective_from < effective_to
  )
);

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
