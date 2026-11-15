package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/zeann3th/policy-service/internal/core"
	db "github.com/zeann3th/policy-service/internal/infrastructure/postgres/codegen"
)

func TestMapDBErrorUsesHTTPDomainSemantics(t *testing.T) {
	for _, test := range []struct {
		input error
		want  error
	}{
		{pgx.ErrNoRows, core.ErrNotFound},
		{&pgconn.PgError{Code: "23503"}, core.ErrNotFound},
		{&pgconn.PgError{Code: "23505"}, core.ErrConflict},
		{&pgconn.PgError{Code: "23514"}, core.ErrBadRequest},
	} {
		if got := mapDBError(test.input); !errors.Is(got, test.want) {
			t.Errorf("mapDBError(%v) = %v, want %v", test.input, got, test.want)
		}
	}
}

func TestAuditFailureRollsBackMutation(t *testing.T) {
	_ = godotenv.Load("../../../.env")
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	store := &limitStoreImpl{db: pool}
	code := fmt.Sprintf("ROLLBACK_%d", time.Now().UnixNano())

	err = store.transact(ctx, func(q *db.Queries) error {
		unit, err := q.CreateUnit(ctx, db.CreateUnitParams{Code: code, Active: true})
		if err != nil {
			return err
		}
		_, err = q.CreateLimitAuditLog(ctx, db.CreateLimitAuditLogParams{Entity: entityUnit, EntityKey: fmt.Sprint(unit.ID), Action: "INVALID", NewValue: []byte(`{"code":"rollback"}`), UpdatedBy: "test"})
		return err
	})
	if err == nil {
		t.Fatal("transaction succeeded, want audit constraint failure")
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM units WHERE code = $1", code).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("unit count = %d, mutation was not rolled back", count)
	}
}
