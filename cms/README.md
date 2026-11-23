# Policy Service CMS

Admin console for the policy service: policy types, policy groups, system
and custom policies, effective config, cache reload, and a dry run that
evaluates a transaction against live usage without saving anything.

Next.js (App Router) + TypeScript, no UI kit. Every page is a client
component that calls the service's HTTP API straight from the browser.

## Run

```bash
cd cms
cp .env.example .env.local      # point NEXT_PUBLIC_POLICY_API at the service
npm install
npm run dev                     # http://localhost:3000
```

The service must allow this origin. In the service's `.env`:

```
CORS_ALLOWED_ORIGINS=http://localhost:3000
```

(`*` allows any origin.) The e2e stack (`e2e/docker-compose.yml`) already
allows `http://localhost:3000`; to use it, set
`NEXT_PUBLIC_POLICY_API=http://localhost:18080/policy-service/api`.

## What it does

| Page | API |
|---|---|
| Dry run | `POST /v1/reservations/dry-run`: resolves both sides' policies, reads live usage (no lock, no write), evaluates exactly like a reservation. Usage cells are editable and sent back as `usageOverrides` by policy id. |
| Policy groups | `GET/POST /v1/policies/groups`, `GET/DELETE /v1/policies/groups/{id}`, and the group's policies under `/policies` |
| Custom policies | `/v1/policies/custom/{walletId}` |
| Policy types | `/v1/policies/types` |
| Effective config | `GET /v1/policies?walletType&currency&tier&walletId` |
| Cache | `POST /v1/cache/reload` (namespaces or one relative key) |

- Every write sends `updatedBy: "admin"` (`lib/api.ts`, `ACTOR`) until a
  gateway supplies the real user.
- Every call is logged to the browser console and to the API log panel
  (rail, bottom left) with its request and response body.
- The policy form only offers the fields a kind takes (`lib/policy.ts`,
  `SHAPES`, mirroring `policy.shapes` in the service, which stays the
  authority and rejects anything else).
- Money and policy values stay decimal strings end to end; totals are added
  with BigInt, never floats.

## Scripts

`npm run dev`, `npm run build`, `npm run typecheck`.
