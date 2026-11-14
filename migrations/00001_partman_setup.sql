-- +goose Up
CREATE SCHEMA IF NOT EXISTS partman;

CREATE EXTENSION IF NOT EXISTS pg_partman SCHEMA partman;

CREATE EXTENSION IF NOT EXISTS pg_cron;

SELECT
  cron.schedule (
    'partman-maintenance',
    '0 * * * *',
    $$CALL partman.run_maintenance_proc()$$
  );

-- +goose Down
SELECT
  cron.unschedule ('partman-maintenance');

DROP EXTENSION IF EXISTS pg_cron;

DROP EXTENSION IF EXISTS pg_partman;

DROP SCHEMA IF EXISTS partman CASCADE;
