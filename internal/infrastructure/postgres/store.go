package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/policy"
	codegen "github.com/zeann3th/policy-service/internal/infrastructure/postgres/codegen"
)

// The stores share these helpers: error mapping, the transaction every
// audited write runs in, and nullable text columns.

// audit log entities
const (
	entityUnit         = "unit"
	entityPolicyType   = "policy_type"
	entityPolicyGroup  = "policy_group"
	entityPolicy       = "policy"
	entityCustomPolicy = "custom_policy"
)

// pgErrors maps the Postgres errors a request can cause (SQLSTATE) to
// domain errors; any other error stays an internal one.
var pgErrors = map[string]*core.AppError{
	"23505": core.ErrConflict,   // unique_violation: the row already exists
	"23503": core.ErrConflict,   // foreign_key_violation: still referenced, or refers to nothing
	"23514": core.ErrBadRequest, // check_violation
}

func mapDBError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return core.ErrNotFound.WithCause(err)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if mapped, ok := pgErrors[pgErr.Code]; ok {
			return mapped.WithCause(err)
		}
	}
	return err
}

// transact runs fn in a transaction; the error fn returns is mapped to a
// domain error and rolls the transaction back.
func transact(ctx context.Context, db *pgxpool.Pool, fn func(*codegen.Queries) error) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = fn(codegen.New(tx)); err != nil {
		return mapDBError(err)
	}
	return tx.Commit(ctx)
}

func audit(ctx context.Context, q *codegen.Queries, entity, key string, action policy.AuditAction, oldValue, newValue any, actor string) error {
	var oldJSON, newJSON []byte
	var err error
	if oldValue != nil {
		oldJSON, err = json.Marshal(oldValue)
		if err != nil {
			return err
		}
	}
	if newValue != nil {
		newJSON, err = json.Marshal(newValue)
		if err != nil {
			return err
		}
	}
	_, err = q.CreatePolicyAuditLog(ctx, codegen.CreatePolicyAuditLogParams{Entity: entity, EntityKey: key, Action: string(action), OldValue: oldJSON, NewValue: newJSON, UpdatedBy: actor})
	return err
}

// fromText reads a nullable text column; NULL is the zero value.
func fromText[T ~string](value *string) T {
	if value == nil {
		return ""
	}
	return T(*value)
}

// text writes a nullable text column; the zero value is NULL.
func text[T ~string](value T) *string {
	if value == "" {
		return nil
	}
	result := string(value)
	return &result
}
