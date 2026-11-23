#!/usr/bin/env bash
# Builds and starts the e2e stack (migrate + seed run once, then the app).
# The cicd stack must be up first: its network is joined as a bridge.
source "$(dirname "$0")/common.sh"
docker network inspect cicd_default >/dev/null 2>&1 || {
  echo "cicd_default network missing: docker compose -f cicd/docker-compose.yml up -d" >&2
  exit 1
}
"${COMPOSE[@]}" up -d --build --wait
curl -fsS http://localhost:18080/policy-service/api/healthz && echo
