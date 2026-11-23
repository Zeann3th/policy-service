# Sourced by the e2e scripts. Paths are relative to the repo root.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"
COMPOSE=(docker compose -f e2e/docker-compose.yml)
PG_CONTAINER=policy_e2e_postgres
PSQL=(docker exec -i "$PG_CONTAINER" psql -U policy_service -d policy_service_e2e -q)
RESULTS="$ROOT/e2e/results"
mkdir -p "$RESULTS"
stamp() { date -u +%Y%m%dT%H%M%SZ; }
