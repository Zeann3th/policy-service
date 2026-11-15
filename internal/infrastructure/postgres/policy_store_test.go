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
	"github.com/zeann3th/policy-service/internal/core/policy"
	codegen "github.com/zeann3th/policy-service/internal/infrastructure/postgres/codegen"
)

func TestSQLAndCodegenContracts(t *testing.T) {
	queryDir := filepath.Clean("../../../queries")
	policyQuery, err := os.ReadFile(filepath.Join(queryDir, "policy_config.sql"))
	if err != nil {
		t.Fatal(err)
	}
	policySQL := strings.ReplaceAll(string(policyQuery), "\r\n", "\n")
	for _, want := range []string{
		"-- name: GetUnits :many",
		"pt.unit_id",
		"-- name: GetActiveCustomPolicies :many",
		"JOIN policy_types pt ON pt.id = cp.policy_type_id",
		"cp.wallet_id = $1",
		"cp.effective_from <= now()",
		"cp.effective_to IS NULL",
		"cp.effective_to > now()",
	} {
		if !strings.Contains(policySQL, want) {
			t.Fatalf("policy_config.sql missing %q", want)
		}
	}
	if strings.Count(policySQL, "pt.active = true") != 2 || strings.Count(policySQL, "u.active = true") != 2 {
		t.Fatal("resolver queries must exclude inactive policy types and units")
	}

	queries := map[string][]string{
		"unit.sql":          {"CreateUnit", "GetUnitByID", "ListUnits", "CountUnits", "UpdateUnit", "DeleteUnit", "ORDER BY id"},
		"policy_type.sql":   {"CreatePolicyType", "GetPolicyTypeByID", "ListPolicyTypes", "CountPolicyTypes", "UpdatePolicyType", "DeletePolicyType", "ORDER BY id"},
		"policy_group.sql":  {"CreatePolicyGroup", "GetPolicyGroupByID", "DeletePolicyGroup"},
		"policy.sql":        {"CreatePolicy", "GetPolicyByID", "ListPolicies", "CountPolicies", "UpdatePolicy", "DeletePolicy", "policy_group_id", "ORDER BY id"},
		"custom_policy.sql": {"CreateCustomPolicy", "GetCustomPolicyByID", "ListCustomPolicies", "CountCustomPolicies", "UpdateCustomPolicy", "DeleteCustomPolicy", "wallet_id", "ORDER BY id"},
		"audit.sql":         {"CreatePolicyAuditLog", "old_value", "new_value", "updated_by"},
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

func TestPolicySchemaUsesPolicyNamesAndCascadeForeignKeys(t *testing.T) {
	schema, err := os.ReadFile("../../../schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(schema)
	for _, want := range []string{
		"CREATE TABLE policy_types",
		"CREATE TABLE policy_groups",
		"CREATE TABLE policies",
		"CREATE TABLE custom_policies",
		"CREATE TABLE policy_audit_logs",
		"REFERENCES units (id) ON DELETE CASCADE",
		"REFERENCES policy_groups (id) ON DELETE CASCADE",
		"REFERENCES policy_types (id) ON DELETE CASCADE",
		"REFERENCES policy_types (id, window_type) ON DELETE RESTRICT",
		"REFERENCES counters (id, window_type) ON DELETE RESTRICT",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("schema.sql missing %q", want)
		}
	}
}

func TestPolicyRenameMigrationProtectsHistory(t *testing.T) {
	migration, err := os.ReadFile("../../../migrations/00004_policy_names.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(migration)
	for _, want := range []string{
		"ALTER TABLE limit_types RENAME TO policy_types",
		"ALTER TABLE limit_groups RENAME TO policy_groups",
		"ALTER TABLE limit_values RENAME TO policies",
		"ALTER TABLE user_limits RENAME TO custom_policies",
		"REFERENCES policy_types (id, window_type) ON DELETE RESTRICT",
		"REFERENCES counters (id, window_type) ON DELETE RESTRICT",
		"DROP TABLE counters",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("migration missing %q", want)
		}
	}
}

type fakePolicyQueries struct {
	groupErr  error
	unitRows  []codegen.Unit
	unitError error
	userRows  []codegen.GetActiveCustomPoliciesRow
	userError error
}

func (q *fakePolicyQueries) GetUnits(context.Context) ([]codegen.Unit, error) {
	return q.unitRows, q.unitError
}

func (q *fakePolicyQueries) GetActiveCustomPolicies(context.Context, string) ([]codegen.GetActiveCustomPoliciesRow, error) {
	return q.userRows, q.userError
}

func (q *fakePolicyQueries) GetPolicyGroupID(context.Context, codegen.GetPolicyGroupIDParams) (int64, error) {
	return 0, q.groupErr
}

func (q *fakePolicyQueries) GetPoliciesByGroupID(context.Context, int64) ([]codegen.GetPoliciesByGroupIDRow, error) {
	return nil, nil
}

func TestGetPolicyGroupIDReturnsNotFound(t *testing.T) {
	store := &policyStoreImpl{resolverQueries: &fakePolicyQueries{groupErr: pgx.ErrNoRows}}

	_, err := store.GetGroupID(context.Background(), "PERSONAL", "USD", "TIER_1")
	if !errors.Is(err, core.ErrNotFound) || !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("GetPolicyGroupID() error = %v", err)
	}
}

func TestGetActiveCustomPoliciesMapsGeneratedRows(t *testing.T) {
	effectiveTo := time.Now().Add(time.Hour)
	rows := []codegen.GetActiveCustomPoliciesRow{{
		WalletID:        "wallet-123",
		PolicyTypeID:    7,
		PolicyType:      "TXN_MAX_AMT",
		UnitID:          3,
		TransactionType: "TRANSFER_OUT",
		Channel:         "USSD",
		Direction:       "OUT",
		Operator:        "LTE",
		Value:           decimal.RequireFromString("100000"),
		EffectiveFrom:   time.Now(),
		EffectiveTo:     &effectiveTo,
	}}
	store := &policyStoreImpl{resolverQueries: &fakePolicyQueries{userRows: rows}}

	values, err := store.GetActiveCustomPolicies(context.Background(), "wallet-123")
	if err != nil {
		t.Fatalf("GetActiveCustomPolicies() error = %v", err)
	}
	if len(values) != 1 {
		t.Fatalf("GetActiveCustomPolicies() length = %d", len(values))
	}
	value := values[0]
	if value.PolicyTypeID != rows[0].PolicyTypeID ||
		value.PolicyType != rows[0].PolicyType ||
		value.UnitID != rows[0].UnitID ||
		value.TransactionType != rows[0].TransactionType ||
		value.Channel != rows[0].Channel ||
		value.Direction != policy.DirectionOut ||
		value.Operator != policy.OpLTE ||
		!value.Value.Equal(rows[0].Value) {
		t.Fatalf("GetActiveCustomPolicies() value = %#v", value)
	}
}

func TestToPoliciesMapsGeneratedRows(t *testing.T) {
	rows := []codegen.GetPoliciesByGroupIDRow{
		{
			PolicyGroupID:   42,
			PolicyTypeID:    7,
			PolicyType:      "TXN_MAX_AMT",
			UnitID:          3,
			TransactionType: "TRANSFER",
			Channel:         "MOBILE",
			Direction:       "IN",
			Operator:        "LTE",
			Value:           decimal.RequireFromString("1000.2500"),
		},
	}

	values := toPolicies(rows)

	if len(values) != 1 {
		t.Fatalf("toPolicies() length = %d, want 1", len(values))
	}
	value := values[0]
	if value.PolicyGroupID != rows[0].PolicyGroupID ||
		value.PolicyTypeID != rows[0].PolicyTypeID ||
		value.PolicyType != rows[0].PolicyType ||
		value.UnitID != rows[0].UnitID ||
		value.TransactionType != rows[0].TransactionType ||
		value.Channel != rows[0].Channel ||
		value.Direction != policy.DirectionIn ||
		value.Operator != policy.OpLTE ||
		!value.Value.Equal(rows[0].Value) {
		t.Fatalf("toPolicies() value = %#v", value)
	}
}

func TestGetActiveUnitsMapsGeneratedRows(t *testing.T) {
	rows := []codegen.Unit{{ID: 3, Code: "AMOUNT", Active: true}}
	store := &policyStoreImpl{resolverQueries: &fakePolicyQueries{unitRows: rows}}

	units, err := store.GetActive(context.Background())
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
		{&pgconn.PgError{Code: "23503"}, core.ErrConflict},
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
	store := &policyStoreImpl{db: pool}
	code := fmt.Sprintf("ROLLBACK_%d", time.Now().UnixNano())

	err = store.transact(ctx, func(q *codegen.Queries) error {
		unit, err := q.CreateUnit(ctx, codegen.CreateUnitParams{Code: code, Active: true})
		if err != nil {
			return err
		}
		_, err = q.CreatePolicyAuditLog(ctx, codegen.CreatePolicyAuditLogParams{Entity: entityUnit, EntityKey: fmt.Sprint(unit.ID), Action: "INVALID", NewValue: []byte(`{"code":"rollback"}`), UpdatedBy: "test"})
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
