-- +goose Up
ALTER TABLE custom_policies
  ADD COLUMN window_type text NOT NULL DEFAULT 'NONE'
    CHECK (window_type IN ('NONE', 'DAILY', 'MONTHLY', 'LIFETIME'));

DROP INDEX policies_identity_idx;
CREATE UNIQUE INDEX policies_identity_idx ON policies (
  policy_group_id,
  policy_type_id,
  transaction_type,
  channel,
  direction,
  window_type
);

DROP INDEX custom_policies_identity_idx;
CREATE UNIQUE INDEX custom_policies_identity_idx ON custom_policies (
  wallet_id,
  policy_type_id,
  transaction_type,
  channel,
  direction,
  window_type
);

-- +goose Down
DROP INDEX custom_policies_identity_idx;
ALTER TABLE custom_policies DROP COLUMN window_type;
CREATE UNIQUE INDEX custom_policies_identity_idx ON custom_policies (
  wallet_id,
  policy_type_id,
  transaction_type,
  channel,
  direction
);

DROP INDEX policies_identity_idx;
CREATE UNIQUE INDEX policies_identity_idx ON policies (
  policy_group_id,
  policy_type_id,
  transaction_type,
  channel,
  direction
);
