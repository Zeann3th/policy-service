# Limit Schema and HTTP Errors Design

## Intent

Use numeric identifiers for internal database relationships while keeping stable codes on origin/reference tables and at external boundaries. Return limit data directly on success and a safe typed error body with the correct HTTP status on failure.

## Decisions

- `limit_types` owns both `id` and unique `code`; `limit_values` and `user_limits` reference `limit_type_id`.
- Relationship columns drop the `_code` suffix: `wallet_type`, `currency`, `tier`, `transaction_type`, and `channel`.
- `reference_codes.kind`, `reference_codes.code`, and `limit_types.code` retain `code` because they define external identifiers.
- No `role_type` is stored because wallet type already determines role for current policy selection.
- SQL joins `limit_types` when an external limit type value is needed.
- Successful HTTP responses return `GetLimitConfigResponse` directly.
- Failed HTTP responses return `{code, message}` with semantic statuses: bad input 400, missing configuration 404, unexpected failure 500.
- Reusable `AppError` behavior belongs in `internal/core/error.go`; command DTOs remain data-only.
- The initial Goose migration may be rewritten because it has been rolled down and no data must be preserved.

## Non-goals

- No backward-compatible expand/contract migration.
- No role-based policy dimension.
- No accounting-service success envelope, error type field, parameter rendering, or HTTP-200 error responses.

## Risks

- Existing local databases must be down before applying the rewritten migration.
- Generated SQLC code and all mappings must change together with the schema.
