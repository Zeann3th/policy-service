# 04 Contention: hot wallets, deadlocks, racing releases

**Script:** `k6/contention.js`. **Run:** `RESET=1 e2e/scripts/run.sh contention`
(`-e HOT=4 -e RATE=400` for harder contention)

The worst case for the wallet locks.

* **opposing** (RATE/s): every reserve is between two of HOT wallets (T3, no
  custom policies), in random direction. Requests constantly need the same
  two advisory locks in opposite orders.
* **racingRelease** (RATE/10 per second): reserve, then fire 4 concurrent
  DELETEs for the same requestId. All 4 must get 200 with the identical
  body.

## Why it should not deadlock

`ReservationStore.Transact` maps each wallet id to a 64-bit lock key
(FNV-1a), sorts and dedupes the keys, and takes `pg_advisory_xact_lock`
in ascending key order. Every transaction acquires locks in one global
order, so a wait cycle cannot form. That holds even when two wallet ids
hash to the same key, which is why it sorts keys, not ids. Release takes the
same locks, then moves the status `RESERVED -> RELEASED` guarded by
`WHERE status = 'RESERVED'`, so only one of the racing DELETEs reverses the
usage. The others find it RELEASED and return the same response: the
response time is the stored release time (`updated_at`), so the bodies are
identical. `verify.sh` confirms nothing was reversed twice.

## Pass criteria

| Check | Target |
|---|---|
| `http_req_failed{name:reserve}` | 0 (a deadlock would be a 500) |
| `racing releases answer alike` | 100% (4 x 200, identical bodies) |
| `reserve_latency` p99 | < 2 s (requests queue on the lock by design) |
| verify: deadlocks detected by Postgres | 0 |
| verify: RELEASED fully reversed, no negative usage | PASS |

Expect higher latency than in load: requests on a hot wallet are serialized
on purpose. Throughput per wallet pair is bounded by one transaction at a
time.

## Results

| Date | Commit | HOT | RATE | Reserve 5xx | Release race | p99 (ms) | PG deadlocks | Invariants | Notes |
|---|---|---|---|---|---|---|---|---|---|
| | | | | | | | | | |
