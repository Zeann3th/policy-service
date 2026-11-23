# 02 Load: 1000 TPS sustained

**Script:** `k6/load.js`. **Run:** `RESET=1 e2e/scripts/run.sh load`
(defaults: `RATE=1000 DURATION=5m RELEASE_RATIO=0.1`)

A constant-arrival-rate executor runs RATE iterations per second whatever
the latency. A slow system shows up as `dropped_iterations`, not as a lower
request rate. Each iteration is one request:

* 90%: reserve between two random wallets out of the 1000. Mix: TRANSFER 50%,
  PAYMENT 33%, WITHDRAW 17%; channels APP / WEB / ALL; amounts 10k-500k.
* 10%: release of a reservation this VU made earlier.

## Pass criteria (k6 thresholds)

| Metric | Target |
|---|---|
| `dropped_iterations` | < 1% of the target over a minute (RATE x 0.6) |
| `http_req_failed` | < 0.1% |
| `unexpected_responses` | < 0.1% (any non-200 reserve, non-200 release) |
| `reserve_latency` | p95 < 200 ms, p99 < 500 ms |
| `release_latency` | p95 < 200 ms, p99 < 500 ms |

The `verify.sh` invariants must all PASS. The REJECTED share comes from
TIGHT / APP_CAP wallets and daily caps; it is business behaviour, not an
error.

## What to look at

* `iterations/s` should be about RATE, with `dropped_iterations` near 0. That
  is the TPS claim.
* `reserve p50/p95/p99`.
* `monitor.csv`: app CPU near its limit (4 CPUs) points to the app as the
  bottleneck; Postgres CPU or `lock_waits` point to the DB.
  `db_connections` near `pool_max_conns` means the pool is the bottleneck.
* `verify` top statements: the costliest queries per call.

## Results

| Date | Commit | RATE | Achieved it/s | Dropped | p95 / p99 (ms) | Failed | Invariants | App CPU / Mem peak | Notes |
|---|---|---|---|---|---|---|---|---|---|
| | | | | | | | | | |
