# 05 Soak: leaks and drift

**Script:** `k6/soak.js`. **Run:** `RESET=1 e2e/scripts/run.sh soak -e DURATION=1h`
(default `RATE=600`, `DURATION=30m`)

A steady rate below the target for a long time. Leaks and drift only show
over time.

## What to look at in `results/soak-*-monitor.csv`

| Signal | Healthy | Leak / problem |
|---|---|---|
| `policy_e2e_app` mem_usage | climbs, then plateaus within minutes (GC steady state) | grows linearly for the whole run |
| `policy_e2e_app` pids | flat | grows (OS threads stuck in syscalls) |
| `db_connections` | at most `pool_max_conns` + a few, flat | grows, or does not drop after the run |
| `lock_waits` | spiky, mostly 0 | permanently > 0 |
| p95 over time (k6 output) | flat | creeping up (bloat, missing index, partition growth) |

Also compare `verify.sh` before and after:

* dead tuples on `usages` (hot rows, upserted per request): autovacuum should
  keep `n_dead_tup` bounded;
* `temp_files` should stay 0;
* the invariants must all PASS.

Goroutines are not exported: the service has no pprof or metrics endpoint.
Memory and PIDs from docker stats are the proxy. If a leak is suspected,
expose `net/http/pprof` on a separate port in a debug build.

## Results

| Date | Commit | RATE | Duration | App mem start -> end | DB conns start -> end | p95 first -> last 5 min | Dead tuples (usages) | Invariants | Notes |
|---|---|---|---|---|---|---|---|---|---|
| | | | | | | | | | |
