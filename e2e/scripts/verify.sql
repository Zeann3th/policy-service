-- e2e invariants and health, run after (or during) a k6 suite.
-- Every row of the first table must say PASS.
\pset footer off
\echo '== invariants (all must PASS) =='
WITH ledger AS (
  SELECT wallet_id, policy_type_id, transaction_type, channel, direction, window_type, unit, window_start,
         sum(delta) AS total
  FROM usage_ledger
  GROUP BY 1, 2, 3, 4, 5, 6, 7, 8
), checks (name, failures) AS (
  -- usage is exactly the sum of its ledger deltas
  SELECT 'usage = sum(ledger delta)', count(*)
  FROM usages u
  FULL JOIN ledger l USING (wallet_id, policy_type_id, transaction_type, channel, direction, window_type, unit, window_start)
  WHERE coalesce(u.value, 0) <> coalesce(l.total, 0)
  UNION ALL
  SELECT 'no negative usage', count(*) FROM usages WHERE value < 0
  UNION ALL
  -- nothing stays PROCESSING (a crash or lost update would leave it)
  SELECT 'no PROCESSING older than 1 minute', count(*)
  FROM reservations WHERE status = 'PROCESSING' AND created_at < now() - interval '1 minute'
  UNION ALL
  SELECT 'REJECTED / FAILED have no ledger', count(*)
  FROM reservations r JOIN usage_ledger l ON l.reservation_id = r.id AND l.reservation_created_at = r.created_at
  WHERE r.status IN ('REJECTED', 'FAILED')
  UNION ALL
  SELECT 'RESERVED have no RELEASE entries', count(*)
  FROM reservations r JOIN usage_ledger l ON l.reservation_id = r.id AND l.reservation_created_at = r.created_at
  WHERE r.status = 'RESERVED' AND l.entry_type = 'RELEASE'
  UNION ALL
  -- every RESERVE entry of a RELEASED reservation has its exact reversal
  SELECT 'RELEASED fully reversed', count(*)
  FROM (
    SELECT l.reservation_id
    FROM reservations r JOIN usage_ledger l ON l.reservation_id = r.id AND l.reservation_created_at = r.created_at
    WHERE r.status = 'RELEASED'
    GROUP BY l.reservation_id, l.wallet_id, l.policy_type_id, l.transaction_type, l.channel, l.direction, l.window_type, l.unit
    HAVING sum(l.delta) <> 0 OR count(*) <> 2
  ) unbalanced
  UNION ALL
  SELECT 'deadlocks detected by Postgres', deadlocks
  FROM pg_stat_database WHERE datname = current_database()
)
SELECT CASE WHEN failures = 0 THEN 'PASS' ELSE 'FAIL' END AS result, name, failures FROM checks;

\echo '== reservations by status =='
SELECT status, count(*) FROM reservations GROUP BY status ORDER BY status;

\echo '== ledger entries by type =='
SELECT entry_type, count(*), sum(delta) FROM usage_ledger GROUP BY entry_type ORDER BY entry_type;

\echo '== database counters =='
SELECT xact_commit, xact_rollback, deadlocks, conflicts, temp_files,
       pg_size_pretty(temp_bytes) AS temp, blks_hit * 100 / nullif(blks_hit + blks_read, 0) AS cache_hit_pct
FROM pg_stat_database WHERE datname = current_database();

\echo '== connections (leak check: should drop back after the run) =='
SELECT coalesce(application_name, '') AS app, state, wait_event_type, count(*)
FROM pg_stat_activity WHERE datname = current_database()
GROUP BY 1, 2, 3 ORDER BY 4 DESC;

\echo '== lock waits right now =='
SELECT count(*) AS waiting FROM pg_stat_activity WHERE wait_event_type = 'Lock';

\echo '== largest tables / dead tuples =='
SELECT relname, n_live_tup, n_dead_tup, pg_size_pretty(pg_total_relation_size(relid)) AS size
FROM pg_stat_user_tables ORDER BY pg_total_relation_size(relid) DESC LIMIT 12;

\echo '== top statements by total time =='
SELECT calls, round(total_exec_time) AS total_ms, round(mean_exec_time::numeric, 3) AS mean_ms,
       round(max_exec_time::numeric, 1) AS max_ms, left(regexp_replace(query, '\s+', ' ', 'g'), 90) AS query
FROM pg_stat_statements WHERE dbid = (SELECT oid FROM pg_database WHERE datname = current_database())
ORDER BY total_exec_time DESC LIMIT 12;
