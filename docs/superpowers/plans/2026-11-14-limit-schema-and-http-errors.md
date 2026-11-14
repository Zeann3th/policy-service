# Limit Schema and HTTP Errors Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace text-based internal limit-type relationships with IDs and expose direct success responses plus typed HTTP errors.

**Architecture:** PostgreSQL stores internal relationships by numeric ID and SQL joins origin tables for external values. Core services return request/response DTOs plus typed errors; the HTTP adapter alone selects status codes and JSON error bodies.

**Tech Stack:** Go 1.26, PostgreSQL, Goose, SQLC, Gin, shopspring/decimal.

**Spec:** `docs/superpowers/specs/2026-11-14-limit-schema-and-http-errors-design.md`

## Global Constraints

- Rewrite `migrations/00002_limit_setup.sql`; do not create a compatibility migration.
- Preserve `code` only on `reference_codes` and `limit_types` origin tables.
- Do not add `role_type`.
- Do not commit, push, tag, release, or publish.
- Do not run network calls inside a transaction.

## Review Focus

- Missing limit group returns HTTP 404 without exposing PostgreSQL errors.
- Invalid request binding returns HTTP 400 with `{code,message}`.
- Unexpected store failure returns HTTP 500 with a generic message.
- SQL queries return external limit type codes while relationships remain ID-based.
- Cache hits and misses produce the same external response shape.

---

### Task 1: Rewrite schema and generated persistence types

**Files:**
- Modify: `migrations/00002_limit_setup.sql`
- Modify: `schema.sql`
- Modify: `queries/limit.sql`
- Regenerate: `internal/infrastructure/postgres/sqlcdb/*.go`

**Interfaces:**
- Produces: `limit_types(id, code)` and ID-based limit relationships; `GetLimitValuesByGroupID` returns joined external names.

- [ ] Add schema assertions for IDs, foreign keys, renamed columns, uniqueness, and seed codes.
- [ ] Rewrite both schema sources and the SQL query projection.
- [ ] Run `sqlc generate` and inspect generated diffs.
- [ ] Run Goose down/up against the configured local database and verify status.

### Task 2: Align domain and store mappings

**Files:**
- Modify: `internal/core/limit/entity.go`
- Modify: `internal/core/limit/store.go` if signatures require clearer names
- Modify: `internal/infrastructure/postgres/limit_store.go`
- Modify: `internal/infrastructure/postgres/limit_store_test.go`
- Modify: `internal/core/limit/service.go`
- Modify: `internal/core/limit/service_test.go`
- Modify: `internal/core/limit/command.go`

**Interfaces:**
- Consumes: regenerated SQLC rows with `limit_type_id` plus joined `limit_type`.
- Produces: DTO fields `walletType`, `currency`, `tier`, `limitType`, `transactionType`, and `channel`.

- [ ] Update tests first to require the renamed fields and ID/code mapping.
- [ ] Run focused tests and confirm expected compile/assertion failures.
- [ ] Implement the smallest model, mapper, cache-key, and DTO changes.
- [ ] Run focused package tests.

### Task 3: Add typed errors and HTTP semantics

**Files:**
- Create: `internal/core/error.go`
- Create: `internal/core/error_test.go`
- Create: `internal/application/http/error.go`
- Create: `internal/application/http/error_test.go`
- Modify: `internal/application/http/limit.go`
- Add/modify: `internal/application/http/limit_test.go`
- Modify: `internal/infrastructure/postgres/limit_store.go`

**Interfaces:**
- Produces: `core.AppError`, `core.ErrBadRequest`, `core.ErrNotFound`, `core.ErrInternal`, and HTTP mapping to 400/404/500.

- [ ] Write failing tests for `errors.Is`/unwrapping and safe JSON fields.
- [ ] Write failing handler tests for direct 200 success and 400/404/500 errors.
- [ ] Implement the minimal typed error and handler mapping.
- [ ] Translate `pgx.ErrNoRows` at the persistence boundary without leaking its message.
- [ ] Run focused HTTP and core tests.

### Task 4: Verify and review

**Files:**
- Review all files above and generated SQLC output.

- [ ] Run `go test ./... -count=1`.
- [ ] Run `go build ./...` and `go vet ./...`.
- [ ] Run `git diff --check` and inspect the complete diff for unrelated changes.
- [ ] Check Sonar configuration; run it if configured, otherwise report it unavailable.
- [ ] Review standards and spec compliance, including transaction boundaries and external model exposure.
- [ ] Report changes without committing or pushing.
