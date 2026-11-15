-- +goose Up
ALTER TABLE limit_types RENAME TO policy_types;
ALTER TABLE policy_types RENAME CONSTRAINT limit_types_pkey TO policy_types_pkey;
ALTER TABLE policy_types RENAME CONSTRAINT limit_types_code_key TO policy_types_code_key;
ALTER TABLE policy_types RENAME CONSTRAINT limit_types_code_check TO policy_types_code_check;
ALTER TABLE policy_types RENAME CONSTRAINT limit_types_unit_id_fkey TO policy_types_unit_id_fkey;
ALTER TRIGGER limit_types_set_updated_at ON policy_types RENAME TO policy_types_set_updated_at;
ALTER SEQUENCE limit_types_id_seq RENAME TO policy_types_id_seq;

ALTER TABLE limit_groups RENAME TO policy_groups;
ALTER TABLE policy_groups RENAME CONSTRAINT limit_groups_pkey TO policy_groups_pkey;
ALTER TABLE policy_groups RENAME CONSTRAINT limit_groups_wallet_type_currency_tier_key TO policy_groups_wallet_type_currency_tier_key;
ALTER TRIGGER limit_groups_set_updated_at ON policy_groups RENAME TO policy_groups_set_updated_at;
ALTER SEQUENCE limit_groups_id_seq RENAME TO policy_groups_id_seq;

ALTER TABLE limit_values RENAME TO policies;
ALTER TABLE policies RENAME COLUMN limit_group_id TO policy_group_id;
ALTER TABLE policies RENAME COLUMN limit_type_id TO policy_type_id;
ALTER TABLE policies RENAME CONSTRAINT limit_values_pkey TO policies_pkey;
ALTER TABLE policies RENAME CONSTRAINT limit_values_limit_group_id_fkey TO policies_policy_group_id_fkey;
ALTER TABLE policies RENAME CONSTRAINT limit_values_limit_type_id_fkey TO policies_policy_type_id_fkey;
ALTER TABLE policies RENAME CONSTRAINT limit_values_operator_check TO policies_operator_check;
ALTER INDEX limit_values_identity_idx RENAME TO policies_identity_idx;
ALTER INDEX limit_values_group_page_idx RENAME TO policies_group_page_idx;
ALTER TRIGGER limit_values_set_updated_at ON policies RENAME TO policies_set_updated_at;
ALTER SEQUENCE limit_values_id_seq RENAME TO policies_id_seq;

ALTER TABLE user_limits RENAME TO custom_policies;
ALTER TABLE custom_policies RENAME COLUMN limit_type_id TO policy_type_id;
ALTER TABLE custom_policies RENAME CONSTRAINT user_limits_pkey TO custom_policies_pkey;
ALTER TABLE custom_policies RENAME CONSTRAINT user_limits_limit_type_id_fkey TO custom_policies_policy_type_id_fkey;
ALTER TABLE custom_policies RENAME CONSTRAINT user_limits_operator_check TO custom_policies_operator_check;
ALTER TABLE custom_policies RENAME CONSTRAINT user_limits_check TO custom_policies_effective_window_check;
ALTER INDEX user_limits_identity_idx RENAME TO custom_policies_identity_idx;
ALTER INDEX user_limits_wallet_page_idx RENAME TO custom_policies_wallet_page_idx;
ALTER INDEX user_limits_expiry_idx RENAME TO custom_policies_expiry_idx;
ALTER TRIGGER user_limits_set_updated_at ON custom_policies RENAME TO custom_policies_set_updated_at;
ALTER SEQUENCE user_limits_id_seq RENAME TO custom_policies_id_seq;

ALTER TABLE limit_audit_logs RENAME TO policy_audit_logs;
ALTER TABLE policy_audit_logs RENAME CONSTRAINT limit_audit_logs_pkey TO policy_audit_logs_pkey;
ALTER TABLE policy_audit_logs RENAME CONSTRAINT limit_audit_logs_action_check TO policy_audit_logs_action_check;
ALTER INDEX limit_audit_logs_entity_idx RENAME TO policy_audit_logs_entity_idx;
ALTER SEQUENCE limit_audit_logs_id_seq RENAME TO policy_audit_logs_id_seq;

UPDATE policy_audit_logs
SET entity = CASE entity
  WHEN 'limit_type' THEN 'policy_type'
  WHEN 'limit_group' THEN 'policy_group'
  WHEN 'limit_value' THEN 'policy'
  WHEN 'user_limit' THEN 'custom_policy'
  ELSE entity
END;

UPDATE partman.part_config
SET parent_table = 'public.policy_audit_logs'
WHERE parent_table = 'public.limit_audit_logs';

INSERT INTO units (code) VALUES ('COUNT'), ('PERCENT')
ON CONFLICT (code) DO NOTHING;

ALTER TABLE policy_types
  ADD COLUMN kind text NOT NULL DEFAULT 'LIMIT'
    CHECK (kind IN ('LIMIT', 'WAIVER', 'FEE', 'TAX', 'DISCOUNT')),
  ADD COLUMN window_type text NOT NULL DEFAULT 'NONE'
    CHECK (window_type IN ('NONE', 'DAILY', 'MONTHLY', 'LIFETIME')),
  ADD CONSTRAINT policy_types_id_window_key UNIQUE (id, window_type);

ALTER TABLE policies
  ADD COLUMN direction text NOT NULL DEFAULT 'ALL'
    CHECK (direction IN ('ALL', 'OUT', 'IN'));

DROP INDEX policies_identity_idx;
CREATE UNIQUE INDEX policies_identity_idx ON policies (
  policy_group_id,
  policy_type_id,
  transaction_type,
  channel,
  direction
);

ALTER TABLE custom_policies
  ADD COLUMN direction text NOT NULL DEFAULT 'ALL'
    CHECK (direction IN ('ALL', 'OUT', 'IN'));

DROP INDEX custom_policies_identity_idx;
CREATE UNIQUE INDEX custom_policies_identity_idx ON custom_policies (
  wallet_id,
  policy_type_id,
  transaction_type,
  channel,
  direction
);

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
  subject_id text NOT NULL,
  counter_id bigint NOT NULL,
  window_type text NOT NULL,
  window_start date NOT NULL,
  value numeric(24, 4) NOT NULL DEFAULT 0,
  PRIMARY KEY (subject_id, counter_id, window_type, window_start),
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

-- +goose Down
DELETE FROM partman.part_config
WHERE parent_table = 'public.usage_daily';

DROP TABLE usage CASCADE;
DROP TABLE usage_daily_template;
DROP TABLE counters;

DROP INDEX custom_policies_identity_idx;
ALTER TABLE custom_policies DROP COLUMN direction;
CREATE UNIQUE INDEX custom_policies_identity_idx ON custom_policies (
  wallet_id,
  policy_type_id,
  transaction_type,
  channel
);

DROP INDEX policies_identity_idx;
ALTER TABLE policies DROP COLUMN direction;
CREATE UNIQUE INDEX policies_identity_idx ON policies (
  policy_group_id,
  policy_type_id,
  transaction_type,
  channel
);

ALTER TABLE policy_types
  DROP CONSTRAINT policy_types_id_window_key,
  DROP COLUMN window_type,
  DROP COLUMN kind;

DELETE FROM units WHERE code IN ('COUNT', 'PERCENT');

UPDATE partman.part_config
SET parent_table = 'public.limit_audit_logs'
WHERE parent_table = 'public.policy_audit_logs';

UPDATE policy_audit_logs
SET entity = CASE entity
  WHEN 'policy_type' THEN 'limit_type'
  WHEN 'policy_group' THEN 'limit_group'
  WHEN 'policy' THEN 'limit_value'
  WHEN 'custom_policy' THEN 'user_limit'
  ELSE entity
END;

ALTER SEQUENCE policy_audit_logs_id_seq RENAME TO limit_audit_logs_id_seq;
ALTER INDEX policy_audit_logs_entity_idx RENAME TO limit_audit_logs_entity_idx;
ALTER TABLE policy_audit_logs RENAME CONSTRAINT policy_audit_logs_action_check TO limit_audit_logs_action_check;
ALTER TABLE policy_audit_logs RENAME CONSTRAINT policy_audit_logs_pkey TO limit_audit_logs_pkey;
ALTER TABLE policy_audit_logs RENAME TO limit_audit_logs;

ALTER SEQUENCE custom_policies_id_seq RENAME TO user_limits_id_seq;
ALTER TRIGGER custom_policies_set_updated_at ON custom_policies RENAME TO user_limits_set_updated_at;
ALTER INDEX custom_policies_expiry_idx RENAME TO user_limits_expiry_idx;
ALTER INDEX custom_policies_wallet_page_idx RENAME TO user_limits_wallet_page_idx;
ALTER INDEX custom_policies_identity_idx RENAME TO user_limits_identity_idx;
ALTER TABLE custom_policies RENAME CONSTRAINT custom_policies_effective_window_check TO user_limits_check;
ALTER TABLE custom_policies RENAME CONSTRAINT custom_policies_operator_check TO user_limits_operator_check;
ALTER TABLE custom_policies RENAME CONSTRAINT custom_policies_policy_type_id_fkey TO user_limits_limit_type_id_fkey;
ALTER TABLE custom_policies RENAME CONSTRAINT custom_policies_pkey TO user_limits_pkey;
ALTER TABLE custom_policies RENAME COLUMN policy_type_id TO limit_type_id;
ALTER TABLE custom_policies RENAME TO user_limits;

ALTER SEQUENCE policies_id_seq RENAME TO limit_values_id_seq;
ALTER TRIGGER policies_set_updated_at ON policies RENAME TO limit_values_set_updated_at;
ALTER INDEX policies_group_page_idx RENAME TO limit_values_group_page_idx;
ALTER INDEX policies_identity_idx RENAME TO limit_values_identity_idx;
ALTER TABLE policies RENAME CONSTRAINT policies_operator_check TO limit_values_operator_check;
ALTER TABLE policies RENAME CONSTRAINT policies_policy_type_id_fkey TO limit_values_limit_type_id_fkey;
ALTER TABLE policies RENAME CONSTRAINT policies_policy_group_id_fkey TO limit_values_limit_group_id_fkey;
ALTER TABLE policies RENAME CONSTRAINT policies_pkey TO limit_values_pkey;
ALTER TABLE policies RENAME COLUMN policy_type_id TO limit_type_id;
ALTER TABLE policies RENAME COLUMN policy_group_id TO limit_group_id;
ALTER TABLE policies RENAME TO limit_values;

ALTER SEQUENCE policy_groups_id_seq RENAME TO limit_groups_id_seq;
ALTER TRIGGER policy_groups_set_updated_at ON policy_groups RENAME TO limit_groups_set_updated_at;
ALTER TABLE policy_groups RENAME CONSTRAINT policy_groups_wallet_type_currency_tier_key TO limit_groups_wallet_type_currency_tier_key;
ALTER TABLE policy_groups RENAME CONSTRAINT policy_groups_pkey TO limit_groups_pkey;
ALTER TABLE policy_groups RENAME TO limit_groups;

ALTER SEQUENCE policy_types_id_seq RENAME TO limit_types_id_seq;
ALTER TRIGGER policy_types_set_updated_at ON policy_types RENAME TO limit_types_set_updated_at;
ALTER TABLE policy_types RENAME CONSTRAINT policy_types_unit_id_fkey TO limit_types_unit_id_fkey;
ALTER TABLE policy_types RENAME CONSTRAINT policy_types_code_check TO limit_types_code_check;
ALTER TABLE policy_types RENAME CONSTRAINT policy_types_code_key TO limit_types_code_key;
ALTER TABLE policy_types RENAME CONSTRAINT policy_types_pkey TO limit_types_pkey;
ALTER TABLE policy_types RENAME TO limit_types;
