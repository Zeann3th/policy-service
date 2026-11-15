-- name: CreatePolicyAuditLog :one
INSERT INTO policy_audit_logs
  (entity, entity_key, action, old_value, new_value, updated_by)
VALUES
  (sqlc.arg(entity), sqlc.arg(entity_key), sqlc.arg(action),
   sqlc.narg(old_value), sqlc.narg(new_value), sqlc.arg(updated_by))
RETURNING *;
