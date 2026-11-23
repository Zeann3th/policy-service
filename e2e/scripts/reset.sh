#!/usr/bin/env bash
# Drops the e2e database volume and starts again from a fresh migrate + seed.
# Run before each suite: usage (daily/monthly limits, waiver counts) and
# pg_stat counters accumulate across runs. Only touches policy-e2e volumes.
source "$(dirname "$0")/common.sh"
"${COMPOSE[@]}" down -v
"$(dirname "$0")/up.sh"
"${PSQL[@]}" -c "SELECT pg_stat_statements_reset()" >/dev/null
