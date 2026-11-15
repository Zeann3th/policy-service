package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/limit"
	codegen "github.com/zeann3th/policy-service/internal/infrastructure/postgres/codegen"
)

func TestSQLAndCodegenContracts(t *testing.T) {
	queryDir := filepath.Clean("../../../queries")
	limitQuery, err := os.ReadFile(filepath.Join(queryDir, "limit.sql"))
	if err != nil {
		t.Fatal(err)
	}
	limitSQL := strings.ReplaceAll(string(limitQuery), "\r\n", "\n")
	for _, want := range []string{
		"-- name: GetUnits :many",
		"lt.unit_id",
		"-- name: GetActiveUserLimits :many",
		"JOIN limit_types lt ON lt.id = ul.limit_type_id",
		"ul.wallet_id = $1",
		"ul.effective_from <= now()",
		"ul.effective_to IS NULL",
		"ul.effective_to > now()",
	} {
		if !strings.Contains(limitSQL, want) {
			t.Fatalf("limit.sql missing %q", want)
		}
	}
	if strings.Count(limitSQL, "lt.active = true") != 2 || strings.Count(limitSQL, "u.active = true") != 2 {
		t.Fatal("resolver queries must exclude inactive limit types and units")
	}

	queries := map[string][]string{
		"unit.sql":        {"CreateUnit", "GetUnitByID", "ListUnits", "CountUnits", "UpdateUnit", "DeleteUnit", "ORDER BY id"},
		"limit_type.sql":  {"CreateLimitType", "GetLimitTypeByID", "ListLimitTypes", "CountLimitTypes", "UpdateLimitType", "DeleteLimitType", "ORDER BY id"},
		"limit_group.sql": {"CreateLimitGroup", "GetLimitGroupByID", "DeleteLimitGroup"},
		"limit_value.sql": {"CreateLimitValue", "GetLimitValueByID", "ListLimitValues", "CountLimitValues", "UpdateLimitValue", "DeleteLimitValue", "limit_group_id", "ORDER BY id"},
		"user_limit.sql":  {"CreateUserLimit", "GetUserLimitByID", "ListUserLimits", "CountUserLimits", "UpdateUserLimit", "DeleteUserLimit", "wallet_id", "ORDER BY id"},
		"audit.sql":       {"CreateLimitAuditLog", "old_value", "new_value", "updated_by"},
	}
	for name, fragments := range queries {
		data, err := os.ReadFile(filepath.Join(queryDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, fragment := range fragments {
			if !strings.Contains(string(data), fragment) {
				t.Errorf("%s missing %q", name, fragment)
			}
		}
	}

	config, err := os.ReadFile("../../../sqlc.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), `out: "internal/infrastructure/postgres/codegen"`) {
		t.Fatal("sqlc output must use internal/infrastructure/postgres/codegen")
	}
}

type fakeLimitQueries struct {
	groupErr  error
	unitRows  []codegen.Unit
	unitError error
	userRows  []codegen.GetActiveUserLimitsRow
	userError error
}

func (q *fakeLimitQueries) GetUnits(context.Context) ([]codegen.Unit, error) {
	return q.unitRows, q.unitError
}

func (q *fakeLimitQueries) GetActiveUserLimits(context.Context, string) ([]codegen.GetActiveUserLimitsRow, error) {
	return q.userRows, q.userError
}

func (q *fakeLimitQueries) GetLimitGroupID(context.Context, codegen.GetLimitGroupIDParams) (int64, error) {
	return 0, q.groupErr
}

func (q *fakeLimitQueries) GetLimitValuesByGroupID(context.Context, int64) ([]codegen.GetLimitValuesByGroupIDRow, error) {
	return nil, nil
}

func TestGetLimitGroupIDReturnsNotFound(t *testing.T) {
	store := &limitStoreImpl{resolverQueries: &fakeLimitQueries{groupErr: pgx.ErrNoRows}}

	_, err := store.GetLimitGroupID(context.Background(), "PERSONAL", "USD", "TIER_1")
	if !errors.Is(err, core.ErrNotFound) || !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("GetLimitGroupID() error = %v", err)
	}
}

func TestGetActiveUserLimitsMapsGeneratedRows(t *testing.T) {
	effectiveTo := time.Now().Add(time.Hour)
	rows := []codegen.GetActiveUserLimitsRow{{
		WalletID:        "wallet-123",
		LimitTypeID:     7,
		LimitType:       "TXN_MAX_AMT",
		UnitID:          3,
		TransactionType: "TRANSFER_OUT",
		Channel:         "USSD",
		Operator:        "LTE",
		Value:           decimal.RequireFromString("100000"),
		EffectiveFrom:   time.Now(),
		EffectiveTo:     &effectiveTo,
	}}
	store := &limitStoreImpl{resolverQueries: &fakeLimitQueries{userRows: rows}}

	values, err := store.GetActiveUserLimits(context.Background(), "wallet-123")
	if err != nil {
		t.Fatalf("GetActiveUserLimits() error = %v", err)
	}
	if len(values) != 1 {
		t.Fatalf("GetActiveUserLimits() length = %d", len(values))
	}
	value := values[0]
	if value.LimitTypeID != rows[0].LimitTypeID ||
		value.LimitType != rows[0].LimitType ||
		value.UnitID != rows[0].UnitID ||
		value.TransactionType != rows[0].TransactionType ||
		value.Channel != rows[0].Channel ||
		value.Operator != limit.OpLTE ||
		!value.Value.Equal(rows[0].Value) {
		t.Fatalf("GetActiveUserLimits() value = %#v", value)
	}
}

func TestToLimitValuesMapsGeneratedRows(t *testing.T) {
	rows := []codegen.GetLimitValuesByGroupIDRow{
		{
			LimitGroupID:    42,
			LimitTypeID:     7,
			LimitType:       "TXN_MAX_AMT",
			UnitID:          3,
			TransactionType: "TRANSFER",
			Channel:         "MOBILE",
			Operator:        "LTE",
			Value:           decimal.RequireFromString("1000.2500"),
		},
	}

	values := toLimitValues(rows)

	if len(values) != 1 {
		t.Fatalf("toLimitValues() length = %d, want 1", len(values))
	}
	value := values[0]
	if value.LimitGroupID != rows[0].LimitGroupID ||
		value.LimitTypeID != rows[0].LimitTypeID ||
		value.LimitType != rows[0].LimitType ||
		value.UnitID != rows[0].UnitID ||
		value.TransactionType != rows[0].TransactionType ||
		value.Channel != rows[0].Channel ||
		value.Operator != limit.OpLTE ||
		!value.Value.Equal(rows[0].Value) {
		t.Fatalf("toLimitValues() value = %#v", value)
	}
}

func TestGetUnitsMapsGeneratedRows(t *testing.T) {
	rows := []codegen.Unit{{ID: 3, Code: "AMOUNT", Active: true}}
	store := &limitStoreImpl{resolverQueries: &fakeLimitQueries{unitRows: rows}}

	units, err := store.GetUnits(context.Background())
	if err != nil {
		t.Fatalf("GetUnits() error = %v", err)
	}
	if len(units) != 1 || units[0].ID != 3 || units[0].Code != "AMOUNT" || !units[0].Active {
		t.Fatalf("GetUnits() = %#v", units)
	}
}

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

	err = store.transact(ctx, func(q *codegen.Queries) error {
		unit, err := q.CreateUnit(ctx, codegen.CreateUnitParams{Code: code, Active: true})
		if err != nil {
			return err
		}
		_, err = q.CreateLimitAuditLog(ctx, codegen.CreateLimitAuditLogParams{Entity: entityUnit, EntityKey: fmt.Sprint(unit.ID), Action: "INVALID", NewValue: []byte(`{"code":"rollback"}`), UpdatedBy: "test"})
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
