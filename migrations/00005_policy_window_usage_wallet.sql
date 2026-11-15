-- +goose Up
ALTER TABLE policies
  ADD COLUMN window_type text NOT NULL DEFAULT 'NONE'
    CHECK (window_type IN ('NONE', 'DAILY', 'MONTHLY', 'LIFETIME'));

ALTER TABLE usage RENAME COLUMN subject_id TO wallet_id;
ALTER TABLE usage_daily_template RENAME COLUMN subject_id TO wallet_id;

-- +goose Down
ALTER TABLE usage_daily_template RENAME COLUMN wallet_id TO subject_id;
ALTER TABLE usage RENAME COLUMN wallet_id TO subject_id;

ALTER TABLE policies DROP COLUMN window_type;
