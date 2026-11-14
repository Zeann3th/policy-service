package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/limit"
	sqlcdb "github.com/zeann3th/policy-service/internal/infrastructure/postgres/sqlcdb"
)

type fakeLimitQueries struct {
	groupErr  error
	userRows  []sqlcdb.GetActiveUserLimitsRow
	userError error
}

func (q *fakeLimitQueries) GetActiveUserLimits(context.Context, string) ([]sqlcdb.GetActiveUserLimitsRow, error) {
	return q.userRows, q.userError
}

func (q *fakeLimitQueries) GetLimitGroupID(context.Context, sqlcdb.GetLimitGroupIDParams) (int64, error) {
	return 0, q.groupErr
}

func (q *fakeLimitQueries) GetLimitValuesByGroupID(context.Context, int64) ([]sqlcdb.GetLimitValuesByGroupIDRow, error) {
	return nil, nil
}

func TestGetLimitGroupIDReturnsNotFound(t *testing.T) {
	store := &limitStoreImpl{queries: &fakeLimitQueries{groupErr: pgx.ErrNoRows}}

	_, err := store.GetLimitGroupID(context.Background(), "PERSONAL", "USD", "TIER_1")
	if !errors.Is(err, core.ErrNotFound) || !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("GetLimitGroupID() error = %v", err)
	}
}

func TestGetActiveUserLimitsMapsGeneratedRows(t *testing.T) {
	effectiveTo := time.Now().Add(time.Hour)
	rows := []sqlcdb.GetActiveUserLimitsRow{{
		WalletID:        "wallet-123",
		LimitTypeID:     7,
		LimitType:       "TXN_MAX_AMT",
		TransactionType: "TRANSFER_OUT",
		Channel:         "USSD",
		Operator:        "LTE",
		Value:           decimal.RequireFromString("100000"),
		EffectiveFrom:   time.Now(),
		EffectiveTo:     &effectiveTo,
	}}
	store := &limitStoreImpl{queries: &fakeLimitQueries{userRows: rows}}

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
		value.TransactionType != rows[0].TransactionType ||
		value.Channel != rows[0].Channel ||
		value.Operator != limit.OpLTE ||
		!value.Value.Equal(rows[0].Value) {
		t.Fatalf("GetActiveUserLimits() value = %#v", value)
	}
}

func TestToLimitValuesMapsGeneratedRows(t *testing.T) {
	rows := []sqlcdb.GetLimitValuesByGroupIDRow{
		{
			LimitGroupID:    42,
			LimitTypeID:     7,
			LimitType:       "TXN_MAX_AMT",
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
		value.TransactionType != rows[0].TransactionType ||
		value.Channel != rows[0].Channel ||
		value.Operator != limit.OpLTE ||
		!value.Value.Equal(rows[0].Value) {
		t.Fatalf("toLimitValues() value = %#v", value)
	}
}
