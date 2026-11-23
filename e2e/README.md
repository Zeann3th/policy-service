# e2e and load tests

An isolated stack plus k6 suites for checking whether the reservation flow
holds about 1000 TPS without inconsistency, deadlocks or leaks.

## What runs where

| Piece | Where | Host port |
|---|---|---|
| Postgres 18, pg_partman, pg_cron, pg_stat_statements (`policy_e2e_postgres`) | docker | 55432 |
| Redis 8 (`policy_e2e_redis`) | docker | 56379 |
| policy-service, built from this repo (`policy_e2e_app`), 4 CPU / 1 GiB | docker | 18080 HTTP, 18081 gRPC |
| migrate (goose) and seed (psql), one-shot | docker | - |
| k6 | **your host** | - |

The stack joins the cicd network (`cicd_default`) as an external bridge, so
it sits on the same docker network as the dev stack. It never uses the dev
database: it has its own volume (`policy-e2e_e2e_pgdata`) and connects by
container name (`policy_e2e_*`). On `cicd_default` the names `postgres` and
`redis` resolve to the **dev** containers, so never use them in e2e config.

## Quick start (Git Bash)

```bash
docker compose -f cicd/docker-compose.yml up -d   # creates cicd_default
e2e/scripts/up.sh                                 # build, migrate, seed, start

RESET=1 e2e/scripts/run.sh smoke                  # functional check, fresh DB
RESET=1 e2e/scripts/run.sh load                   # 1000 TPS for 5 minutes
RESET=1 e2e/scripts/run.sh stress                 # ramp to 3000 TPS
RESET=1 e2e/scripts/run.sh contention             # hot wallets, racing releases
RESET=1 e2e/scripts/run.sh soak -e DURATION=1h    # leaks and drift

docker compose -f e2e/docker-compose.yml down -v  # remove everything
```

`run.sh` samples container and DB stats while k6 runs
(`results/<suite>-<time>-monitor.csv`). Afterwards it runs `verify.sh`
(`results/verify-<suite>-<time>.txt`) and exits non-zero if a k6 threshold or
an invariant fails. k6 writes its full summary JSON to `results/<suite>-*.json`.

Knobs: `E2E_APP_CPUS` (default 4) and `E2E_POOL_MAX_CONNS` (default 64) on
`up.sh`; `RATE`, `DURATION`, `VUS`, `MAX_VUS`, `RELEASE_RATIO`, `PEAK`,
`STEP`, `HOT` as `k6 -e` args. See each suite doc.

## Seed data (`seed/seed.sql`)

* 6 policy groups: PERSONAL / MERCHANT x T1 / T2 / T3, VND. Limits scale by
  tier (T1 1x, T2 5x, T3 20x).
* System policies per group. Units and policy types come from the
  baseline catalog. A policy's kind (from its type) picks the strategy:
  LIMIT, WAIVER or AUXILIARY (charges: TXN_FEE, TXN_DISCOUNT, TXN_TAX). The
  row says how its value is read: `unit` (what a LIMIT / WAIVER measures, or
  what an AUXILIARY cap counts), `base` (what a charge percentage is of;
  empty = flat), `operator` (LIMIT only), `cap` (most a charge takes per
  window). Responses list each side's `charges` by type, discounts negative.

  | Type | Selector | Window | Unit / base | Rule |
  |---|---|---|---|---|
  | COIN_MIN | OUT | - | unit BALANCE | >= 0; config for accounting (type is_enforced false: not evaluated here) |
  | TXN_MIN | OUT | - | unit AMOUNT | >= 1,000 |
  | TXN_MAX | OUT | - | unit AMOUNT | <= 20M x tier |
  | TOTAL_MAX | OUT | DAILY | unit AMOUNT | <= 100M x tier |
  | TOTAL_MAX | OUT | MONTHLY | unit AMOUNT | <= 1B x tier |
  | TOTAL_MAX | OUT | DAILY | unit COUNT | <= 2,000 x tier transactions |
  | NO_FEE_TRANSACTION | TRANSFER OUT | MONTHLY | unit COUNT | first 5 free |
  | TXN_FEE | TRANSFER OUT | - | flat | 1,100 |
  | TXN_FEE | PAYMENT OUT | - | base AMOUNT | 0.5% of the amount |
  | TXN_DISCOUNT | OUT, T3 only | - | base FEE | 10% off the fee (at most the fee) |
  | TXN_TAX | OUT | - | base NET_FEE | 10% of fee minus fee discounts |
  | COIN_MAX | IN | - | unit BALANCE | <= 100B; config for accounting |
  | TOTAL_MAX | IN | DAILY | unit AMOUNT | <= 500M x tier (x10 merchant) |
  | TXN_FEE | PAYMENT IN, merchants | - | base AMOUNT | 1% |

* 1000 wallets `e2e-w-0001..1000`: MERCHANT when i % 10 = 0, tier by i % 3
  (1 T1, 2 T2, 0 T3). `k6/lib.js` uses the same formula.
* Custom policies for wallets 1..500, by profile i % 5: 0 TIGHT (daily cap
  2M, which produces REJECTED), 1 VIP (10B daily, 500M per transaction),
  2 FEE_FREE (plus 1% cashback on PAYMENT: TXN_DISCOUNT on base AMOUNT,
  at most 50,000 a month: window MONTHLY, unit AMOUNT, cap 50,000),
  3 PROMO (flat fee 2,000, 50% fee discount), 4 APP_CAP (APP channel
  5M/day, 50 transactions/day). Every 50th wallet also has an expired
  override that must be ignored.
* A custom policy overrides the system one with the same identity: type,
  selectors, window and unit. The base and cap are not part of it, so a
  custom flat fee replaces a system percentage fee of the same scope.

## Suites

| # | Suite | Question it answers |
|---|---|---|
| 01 | [smoke](suites/01-smoke.md) | Are fees, waivers, discounts, tax, limits, release and validation right? |
| 02 | [load](suites/02-load.md) | Does it hold 1000 TPS within latency SLOs and stay consistent? |
| 03 | [stress](suites/03-stress.md) | Where is the knee, and does it degrade gracefully? |
| 04 | [contention](suites/04-contention.md) | Do the hot-wallet locks deadlock or double-release? |
| 05 | [soak](suites/05-soak.md) | Do memory, connections or latency drift over time? |

## Invariants checked by `verify.sh`

| Invariant | Meaning |
|---|---|
| usage = sum(ledger delta) | the running balance always equals its journal |
| no negative usage | releases never reverse more than was reserved |
| no PROCESSING older than 1 minute | no request was left half done |
| REJECTED / FAILED have no ledger | rejections and failures consume nothing |
| RESERVED have no RELEASE entries | no partial release |
| RELEASED fully reversed | each RESERVE row has exactly one opposite RELEASE row |
| deadlocks detected by Postgres = 0 | lock ordering works under load |

## Reading the numbers honestly

* Everything (k6, app, Postgres, Redis) shares one machine and Docker
  Desktop's VM, so k6 competes for CPU with the system under test. Treat the
  results as a lower bound, and note the machine in the results.
* The app logs every request and response body at INFO (by design, for
  debugging). At 1000 TPS that is a real cost; it is included in the numbers.
* `synchronous_commit` is on (durable commits). Every reservation is about
  15 DB round trips: one insert, one transaction holding two advisory locks,
  a usage read and upsert plus a ledger insert per tracked policy, and the
  status update. The pool (`pool_max_conns=64`) and the app's CPU limit are
  the first knobs to turn when the knee is low.
