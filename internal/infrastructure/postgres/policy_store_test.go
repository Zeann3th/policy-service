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
		"cp.unit",
		"cp.base",
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
	if strings.Count(policySQL, "pt.active = true") != 2 {
		t.Fatal("resolver queries must exclude inactive policy types")
	}

	queries := map[string][]string{
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

func readSQL(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

func TestPolicySchemaForeignKeysAndIDs(t *testing.T) {
	sql := readSQL(t, "../../../schema.sql")
	for _, want := range []string{
		"CREATE TABLE policy_types",
		"CREATE TABLE policy_groups",
		"CREATE TABLE policies",
		"CREATE TABLE custom_policies",
		"CREATE TABLE policy_audit_logs",
		"CREATE TABLE usages",
		"CREATE TABLE reservations",
		"CREATE TABLE usage_ledger",
		"CREATE TABLE outbox",
		"REFERENCES policy_groups (id) ON DELETE CASCADE",
		"REFERENCES policy_types (id) ON DELETE CASCADE",
		"REFERENCES policy_types (id) ON DELETE RESTRICT",
		"REFERENCES reservations (id, created_at) ON DELETE RESTRICT",
		"id uuid NOT NULL",
		"p_parent_table := 'public.reservations'",
		"p_parent_table := 'public.usage_ledger'",
		"ALTER TABLE reservations_template ADD UNIQUE (request_id)",
		"entry_type text NOT NULL",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("schema.sql missing %q", want)
		}
	}
	for _, unwanted := range []string{"CHECK (", "counters"} {
		if strings.Contains(sql, unwanted) {
			t.Errorf("schema.sql must not contain %q", unwanted)
		}
	}
	if strings.Count(sql, "id bigint PRIMARY KEY DEFAULT nextval('policy_id_seq')") != 2 {
		t.Error("policies and custom_policies must share policy_id_seq")
	}
	policyTypes := sql[strings.Index(sql, "CREATE TABLE policy_types ("):strings.Index(sql, "INSERT INTO policy_types")]
	for _, field := range []string{"window", "unit", "base", "operator", "cap"} {
		if strings.Contains(policyTypes, field) {
			t.Errorf("policy_types must not define %s: it belongs to the policy", field)
		}
	}
	for _, table := range []string{"policies", "custom_policies"} {
		block := sql[strings.Index(sql, "CREATE TABLE "+table+" ("):strings.Index(sql, "CREATE UNIQUE INDEX "+table+"_identity_idx")]
		for _, want := range []string{"window_type text NOT NULL DEFAULT 'NONE'", "unit_id bigint REFERENCES units (id) ON DELETE RESTRICT", "base text,", "operator text,", "cap numeric(24, 4)"} {
			if !strings.Contains(block, want) {
				t.Errorf("%s must define %q", table, want)
			}
		}
		identity := sql[strings.Index(sql, "CREATE UNIQUE INDEX "+table+"_identity_idx"):]
		identity = identity[:strings.Index(identity, ";")]
		if !strings.Contains(identity, "unit_id\n) NULLS NOT DISTINCT") {
			t.Errorf("%s identity must include unit_id with NULLS NOT DISTINCT", table)
		}
	}
}

func TestBaselineMigrationMatchesSchema(t *testing.T) {
	migration := readSQL(t, "../../../migrations/00001_baseline.sql")
	up := migration[strings.Index(migration, "-- updated_at stays NULL"):strings.Index(migration, "-- +goose Down")]
	up = strings.NewReplacer("-- +goose StatementBegin\n", "", "-- +goose StatementEnd\n", "").Replace(up)
	if up != readSQL(t, "../../../schema.sql") {
		t.Fatal("schema.sql must match the Up section of migrations/00001_baseline.sql")
	}
}

type fakePolicyQueries struct {
	groupErr  error
	userRows  []codegen.GetActiveCustomPoliciesRow
	userError error
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

func (q *fakePolicyQueries) GetNextPolicyStart(context.Context, int64) (time.Time, error) {
	return time.Time{}, pgx.ErrNoRows
}

func TestGetNextPolicyStartWithoutScheduleIsNil(t *testing.T) {
	next, err := (&PolicyStore{resolverQueries: &fakePolicyQueries{}}).GetNextPolicyStart(context.Background(), 1)
	if err != nil || next != nil {
		t.Fatalf("GetNextPolicyStart() = %v, %v", next, err)
	}
}

func TestGetPolicyGroupIDReturnsNotFound(t *testing.T) {
	store := &PolicyStore{resolverQueries: &fakePolicyQueries{groupErr: pgx.ErrNoRows}}

	_, err := store.GetGroupID(context.Background(), "PERSONAL", "USD", "TIER_1")
	if !errors.Is(err, core.ErrNotFound) || !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("GetPolicyGroupID() error = %v", err)
	}
}

func TestGetActiveCustomPoliciesMapsGeneratedRows(t *testing.T) {
	effectiveTo := time.Now().Add(time.Hour)
	amountUnit := int64(1)
	rows := []codegen.GetActiveCustomPoliciesRow{{
		WalletID:        "wallet-123",
		PolicyTypeID:    7,
		PolicyType:      "TXN_MAX",
		TransactionType: "TRANSFER_OUT",
		Channel:         "USSD",
		Direction:       "OUT",
		WindowType:      "DAILY",
		UnitID:          &amountUnit,
		Unit:            text(policy.UnitAmount),
		Operator:        text(policy.OpLTE),
		Value:           decimal.RequireFromString("100000"),
		EffectiveFrom:   time.Now(),
		EffectiveTo:     &effectiveTo,
	}}
	store := &PolicyStore{resolverQueries: &fakePolicyQueries{userRows: rows}}

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
		value.Unit != policy.UnitAmount ||
		value.UnitID == nil || *value.UnitID != amountUnit ||
		value.Base != "" ||
		value.TransactionType != rows[0].TransactionType ||
		value.Channel != rows[0].Channel ||
		value.Direction != policy.DirectionOut ||
		value.WindowType != policy.WindowDaily ||
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
			PolicyType:      "TXN_FEE",
			TransactionType: "TRANSFER",
			Channel:         "MOBILE",
			Direction:       "IN",
			WindowType:      "NONE",
			Base:            text(policy.BaseNetFee),
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
		value.UnitID != nil ||
		value.Base != policy.BaseNetFee ||
		value.Operator != "" ||
		value.TransactionType != rows[0].TransactionType ||
		value.Channel != rows[0].Channel ||
		value.Direction != policy.DirectionIn ||
		value.WindowType != policy.WindowNone ||
		!value.Value.Equal(rows[0].Value) {
		t.Fatalf("toPolicies() value = %#v", value)
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
	code := fmt.Sprintf("ROLLBACK_%d", time.Now().UnixNano())

	err = transact(ctx, pool, func(q *codegen.Queries) error {
		policyType, err := q.CreatePolicyType(ctx, codegen.CreatePolicyTypeParams{Code: code, Active: true})
		if err != nil {
			return err
		}
		_, err = q.CreatePolicyAuditLog(ctx, codegen.CreatePolicyAuditLogParams{Entity: entityPolicyType, EntityKey: fmt.Sprint(policyType.ID), Action: string(policy.AuditInsert), NewValue: []byte(`not json`), UpdatedBy: "test"})
		return err
	})
	if err == nil {
		t.Fatal("transaction succeeded, want audit insert failure")
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM policy_types WHERE code = $1", code).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("policy type count = %d, mutation was not rolled back", count)
	}
}
