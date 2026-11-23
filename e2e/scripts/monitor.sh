#!/usr/bin/env bash
# Samples CPU / memory / PIDs of the e2e containers and the DB connection
# count every INTERVAL seconds into a CSV, until stopped (Ctrl+C or kill).
# A leak shows as app memory or connections climbing at a steady rate.
source "$(dirname "$0")/common.sh"
INTERVAL="${INTERVAL:-5}"
out="${1:-$RESULTS/monitor-$(stamp).csv}"
echo "time,container,cpu_pct,mem_usage,mem_pct,pids,db_connections,lock_waits" > "$out"
echo "monitoring every ${INTERVAL}s -> $out"
while true; do
  now="$(date -u +%H:%M:%S)"
  db="$("${PSQL[@]}" -tAc "SELECT count(*) FILTER (WHERE backend_type = 'client backend'), count(*) FILTER (WHERE wait_event_type = 'Lock') FROM pg_stat_activity WHERE datname = current_database()" | tr '|' ',')"
  docker stats --no-stream --format '{{.Name}},{{.CPUPerc}},{{.MemUsage}},{{.MemPerc}},{{.PIDs}}' \
    policy_e2e_app policy_e2e_postgres policy_e2e_redis |
    while IFS= read -r line; do echo "$now,${line//%/},$db"; done >> "$out"
  sleep "$INTERVAL"
done
