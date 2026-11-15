-- +goose Up
ALTER TABLE limit_values ADD COLUMN id bigserial NOT NULL;

ALTER TABLE limit_values
  DROP CONSTRAINT limit_values_pkey,
  ADD PRIMARY KEY (id);

CREATE UNIQUE INDEX limit_values_identity_idx ON limit_values (
  limit_group_id,
  limit_type_id,
  transaction_type,
  channel
);

CREATE INDEX limit_values_group_page_idx ON limit_values (limit_group_id, id);

ALTER TABLE user_limits ADD COLUMN id bigserial NOT NULL;

ALTER TABLE user_limits
  DROP CONSTRAINT user_limits_pkey,
  ADD PRIMARY KEY (id);

CREATE UNIQUE INDEX user_limits_identity_idx ON user_limits (
  wallet_id,
  limit_type_id,
  transaction_type,
  channel
);

CREATE INDEX user_limits_wallet_page_idx ON user_limits (wallet_id, id);

-- +goose Down
DROP INDEX user_limits_wallet_page_idx;

DROP INDEX user_limits_identity_idx;

ALTER TABLE user_limits
  DROP CONSTRAINT user_limits_pkey,
  DROP COLUMN id,
  ADD PRIMARY KEY (
    wallet_id,
    limit_type_id,
    transaction_type,
    channel
  );

DROP INDEX limit_values_group_page_idx;

DROP INDEX limit_values_identity_idx;

ALTER TABLE limit_values
  DROP CONSTRAINT limit_values_pkey,
  DROP COLUMN id,
  ADD PRIMARY KEY (
    limit_group_id,
    limit_type_id,
    transaction_type,
    channel
  );
