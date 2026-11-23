# 01 Smoke: functional correctness

**Script:** `k6/smoke.js` (1 VU, 1 iteration). **Run:** `RESET=1 e2e/scripts/run.sh smoke`

Needs a fresh database (`RESET=1`), because the waiver counts transfers from
the start of the month.

## Cases

| Group | Wallets | Expectation |
|---|---|---|
| Waiver | 502 (PERSONAL T1, no custom) -> 10 (merchant) | Transfers 1-5: fee 0. Transfer 6: fee 1,100, tax 110 |
| Payment fees | 508 -> 10, 200,000 | Sender fee 1,000 (0.5%), tax 100. Merchant receiving fee 2,000 (1%) |
| T3 discount | 9 (PERSONAL T3) -> 11, PAYMENT 200,000 | Fee 1,000, discount 100, tax 90 |
| PROMO custom | 3 (T3, profile 3), 6th transfer | Fee 2,000 (custom overrides 1,100), discount 1,000 (custom 50% overrides 10%), tax 100 |
| Cashback | 7 (T1, profile 2) -> 8, PAYMENT 200,000 | Fee 1,000. Discount 2,000 (1% of the amount, base AMOUNT). Tax 100 (10% of NET_FEE, so cashback is not subtracted) |
| TIGHT custom | 15 (profile 0) | 2.5M is REJECTED with `isViolated`. 2.0M then fits, so the rejection consumed nothing |
| Expired custom | 50 | Expired `TXN_MAX <= 1` is ignored, so the request is RESERVED |
| Release | 601 -> 602 | RESERVED, then DELETE gives RELEASED, a second DELETE returns the identical response (idempotent), an unknown id gives 404 |
| Validation | 701 -> 702 | Amount 0, more than 4 decimals, same wallet, no policy: 400. Duplicate requestId: 409 |

**Pass criteria:** `checks rate == 1`, and every `verify.sh` invariant is PASS.

The expected values were checked against the running service with curl
(T3 payment: 1000 / 100 / 90, merchant fee 2000) before this suite was
written.

## Results

| Date | Commit | Checks | Invariants | Notes |
|---|---|---|---|---|
| | | | | |
