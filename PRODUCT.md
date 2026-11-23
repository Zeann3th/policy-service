# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Stack

Backend: Go policy service (gin HTTP + gRPC, Postgres, Redis). Admin CMS: Next.js (App Router, TypeScript) in `cms/`, chosen by the user. The CMS calls the service through a Next.js rewrite, so the browser never needs CORS.

## Users

Both ops/product admins and developers/QA. Admins configure fees, limits, waivers, discounts and tax per wallet type and tier, and grant per-wallet overrides. Engineers check whether a configuration change takes effect before real traffic hits it. The UI reads in business terms by default; ids and raw JSON are one click away.

## Product Purpose

The policy service decides, per transaction, which limits apply and what fee, discount and tax each side pays, and reserves usage against daily/monthly/lifetime windows. The CMS is where people configure those policies and check them with a dry run (a reservation that is evaluated against live usage but never saved). Success: a change is configured, previewed and confirmed without touching production data.

## Positioning

Internal tool. Its distinct value is the dry run: evaluate any transaction against the real resolved policies and the wallet's live usage, optionally overriding usage, and see exactly which policy produced which number.

## Operating Context

- Entities: policy types (code + kind), policy groups (wallet type x currency x tier), system policies per group, custom policies per wallet (override system policies with the same identity), effective config per wallet, cache reload, reservations and release.
- Kinds: LIMIT, WAIVER, FEE, DISCOUNT, TAX. A policy row carries unit (LIMIT/WAIVER), base (charges; empty = flat), operator (LIMIT), window, value, and an effective range.
- There is no authentication yet; the CMS sends `updatedBy: "admin"` on every write until a gateway exists.

## Capabilities and Constraints

- CRUD for types, groups, system and custom policies; effective config lookup; cache reload; dry run.
- Dry run must not write anything (no reservation, usage or ledger rows).
- Money values are decimal strings with up to 4 places; never floats.
- Open: authentication/roles, audit-log viewer, bulk import.

## Evidence on Hand

Seeded e2e data (6 groups, 65 system policies, 500 wallets with custom policies) in `e2e/seed/seed.sql`. No brand assets, logo or customers exist; do not invent them.

## Product Principles

1. Show the effect, not just the config: every edit should be one step from a dry run.
2. Business-readable first, exact underneath: plain labels, with ids and raw values available.
3. Make the shape of a policy obvious: fields that do not apply to a kind are not offered.
4. Destructive actions are deliberate and say what they remove.
