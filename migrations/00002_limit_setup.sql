-- +goose Up
-- ---------------------------------------------------------------------
-- shared trigger: updated_at stays NULL until the first real UPDATE
-- ---------------------------------------------------------------------
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_updated_at () RETURNS trigger AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END
$$ LANGUAGE plpgsql;

-- +goose StatementEnd
-- ---------------------------------------------------------------------
-- reference codes (wallet type, currency, tier, txn type, channel ...)
-- no FKs to this table: validate against an in-memory copy
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS reference_codes (
  kind text NOT NULL,
  code text NOT NULL,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz,
  PRIMARY KEY (kind, code),
  CHECK (code = upper(code))
);

CREATE TABLE IF NOT EXISTS limit_types (
  id bigserial PRIMARY KEY,
  code text NOT NULL UNIQUE CHECK (code = upper(code)),
  description text,
  unit text,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz
);

INSERT INTO limit_types (code, description, unit) VALUES
  ('COIN_MIN', 'Minimum balance a wallet must keep after a transaction', 'BALANCE'),
  ('COIN_MAX', 'Maximum balance a wallet may hold after a transaction', 'BALANCE'),
  ('TXN_MIN_AMT', 'Minimum amount of a single transaction', 'AMOUNT'),
  ('TXN_MAX_AMT', 'Maximum amount of a single transaction', 'AMOUNT'),
  ('DAILY_OUT_MAX', 'Maximum total sent out per day (counter lives in policy)', 'AMOUNT')
ON CONFLICT (code) DO NOTHING;

CREATE TABLE IF NOT EXISTS limit_groups (
  id bigserial PRIMARY KEY,
  wallet_type text NOT NULL,
  currency varchar(3) NOT NULL, -- ISO 4217
  tier text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz,
  UNIQUE (wallet_type, currency, tier)
);

CREATE TABLE IF NOT EXISTS limit_values (
  limit_group_id bigint NOT NULL REFERENCES limit_groups (id) ON DELETE CASCADE,
  limit_type_id bigint NOT NULL REFERENCES limit_types (id),
  transaction_type text NOT NULL DEFAULT 'ALL',
  channel text NOT NULL DEFAULT 'ALL',
  operator text NOT NULL CHECK (
    operator IN ('EQ', 'NEQ', 'GT', 'GTE', 'LT', 'LTE')
  ),
  value numeric(24, 4) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz,
  PRIMARY KEY (
    limit_group_id,
    limit_type_id,
    transaction_type,
    channel
  )
);

CREATE TABLE IF NOT EXISTS user_limits (
  wallet_id text NOT NULL,
  limit_type_id bigint NOT NULL REFERENCES limit_types (id),
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
  PRIMARY KEY (
    wallet_id,
    limit_type_id,
    transaction_type,
    channel
  ),
  CHECK (
    effective_to IS NULL
    OR effective_from < effective_to
  )
);

CREATE INDEX IF NOT EXISTS user_limits_expiry_idx ON user_limits (effective_to)
WHERE
  effective_to IS NOT NULL;

-- ---------------------------------------------------------------------
-- updated_at triggers
-- ---------------------------------------------------------------------
CREATE TRIGGER reference_codes_set_updated_at
BEFORE UPDATE ON reference_codes FOR EACH ROW WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION set_updated_at ();

CREATE TRIGGER limit_types_set_updated_at
BEFORE UPDATE ON limit_types FOR EACH ROW WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION set_updated_at ();

CREATE TRIGGER limit_groups_set_updated_at
BEFORE UPDATE ON limit_groups FOR EACH ROW WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION set_updated_at ();

CREATE TRIGGER limit_values_set_updated_at
BEFORE UPDATE ON limit_values FOR EACH ROW WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION set_updated_at ();

CREATE TRIGGER user_limits_set_updated_at
BEFORE UPDATE ON user_limits FOR EACH ROW WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION set_updated_at ();

-- ---------------------------------------------------------------------
-- audit logs: native range partitions by month via pg_partman
-- partition key (updated_at) must be part of the primary key
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS limit_audit_logs (
  id bigserial,
  entity text NOT NULL, -- limit_type | limit_group | limit_value | user_limit
  entity_key text NOT NULL,
  action text NOT NULL CHECK (action IN ('INSERT', 'UPDATE', 'DELETE')),
  old_value jsonb,
  new_value jsonb,
  updated_by text NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now(), -- partition key, must be NOT NULL
  PRIMARY KEY (id, updated_at)
)
PARTITION BY
  RANGE (updated_at);

CREATE INDEX IF NOT EXISTS limit_audit_logs_entity_idx ON limit_audit_logs (entity, entity_key, updated_at DESC);

-- pg_partman v5 syntax (v4 also needs p_type => 'native')
SELECT
  partman.create_parent (
    p_parent_table := 'public.limit_audit_logs',
    p_control := 'updated_at',
    p_interval := '1 month',
    p_premake := 4
  );

-- Optional retention (uncomment to drop partitions older than 24 months):
-- UPDATE partman.part_config
-- SET retention = '24 months', retention_keep_table = false
-- WHERE parent_table = 'public.limit_audit_logs';
-- +goose Down
DELETE FROM partman.part_config
WHERE
  parent_table = 'public.limit_audit_logs';

DROP TABLE IF EXISTS limit_audit_logs CASCADE;

DROP TABLE IF EXISTS user_limits;

DROP TABLE IF EXISTS limit_values;

DROP TABLE IF EXISTS limit_groups;

DROP TABLE IF EXISTS limit_types;

DROP TABLE IF EXISTS reference_codes;

DROP FUNCTION IF EXISTS set_updated_at ();
