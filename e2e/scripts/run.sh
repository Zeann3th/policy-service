#!/usr/bin/env bash
# Runs one k6 suite with monitoring and verification around it:
#   e2e/scripts/run.sh load                 # RATE=1000 DURATION=5m
#   e2e/scripts/run.sh stress -e PEAK=4000  # extra args go to k6
#   RESET=1 e2e/scripts/run.sh load         # fresh database first
# Suites: smoke load stress contention soak. Needs k6 on the host.
source "$(dirname "$0")/common.sh"
suite="${1:?usage: run.sh <smoke|load|stress|contention|soak> [k6 args]}"
shift
[[ -f "e2e/k6/$suite.js" ]] || { echo "unknown suite $suite" >&2; exit 1; }
command -v k6 >/dev/null || { echo "k6 not found on PATH" >&2; exit 1; }
[[ "${RESET:-0}" == 1 ]] && e2e/scripts/reset.sh

run="$suite-$(stamp)"
e2e/scripts/monitor.sh "$RESULTS/$run-monitor.csv" & monitor=$!
trap 'kill $monitor 2>/dev/null || true' EXIT

status=0
RESULTS_DIR="$RESULTS" k6 run "$@" "e2e/k6/$suite.js" || status=$?
kill "$monitor" 2>/dev/null || true

# let in-flight transactions finish, then check
sleep 2
e2e/scripts/verify.sh "$run" || status=1
echo "results: $RESULTS/$run-*  (k6 summary JSON: $RESULTS/$suite-*.json)"
exit $status
