# 03 Stress: find the knee

**Script:** `k6/stress.js`. **Run:** `RESET=1 e2e/scripts/run.sh stress`
(`-e PEAK=4000 -e STEP=2m` to push further or hold longer)

A ramping arrival rate goes 100 -> 500 -> 1000 -> 1500 -> 2000 -> PEAK,
holding each step for STEP, then down to 0. The thresholds (failed < 1%,
p95 < 500 ms) are reported, not enforced mid-run, so the whole curve is
recorded.

## What to look at

* **The knee:** the first step where p95 climbs steeply or
  `dropped_iterations` starts growing. The sustainable TPS is the step before
  it.
* **Graceful degradation:** past the knee, latency should rise while errors
  stay near 0 (requests queue for pool connections). A burst of 500s means a
  timeout or pool exhaustion; check the app logs (`[HTTP] ... --> 500`
  lines carry the cause).
* **Recovery:** after ramp-down, `db_connections` and app memory in
  `monitor.csv` should return to idle levels.
* **Invariants** must still all PASS: overload may slow things down but must
  never corrupt usage.

## Results

| Date | Commit | Knee (it/s) | p95 at 1000 | p95 at knee | Errors past knee | Recovered | Invariants | Notes |
|---|---|---|---|---|---|---|---|---|
| | | | | | | | | |
