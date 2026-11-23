#!/usr/bin/env bash
# Prints the invariants (must all PASS) and DB health; also saves them.
# Exits non-zero when an invariant fails.
source "$(dirname "$0")/common.sh"
out="$RESULTS/verify-${1:-manual}-$(stamp).txt"
"${PSQL[@]}" < e2e/scripts/verify.sql | tee "$out"
echo "saved $out"
! grep -q "^ FAIL" "$out"
