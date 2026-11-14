package postgres

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/zeann3th/haphaestus/internal/core/limit"
	sqlcdb "github.com/zeann3th/haphaestus/internal/infrastructure/postgres/sqlcdb"
)

func TestToLimitValuesMapsGeneratedRows(t *testing.T) {
	rows := []sqlcdb.LimitValue{
		{
			LimitGroupID:        42,
			LimitTypeCode:       "DAILY_AMOUNT",
			TransactionTypeCode: "TRANSFER",
			ChannelCode:         "MOBILE",
			Operator:            "LTE",
			Value:               decimal.RequireFromString("1000.2500"),
		},
	}

	values := toLimitValues(rows)

	if len(values) != 1 {
		t.Fatalf("toLimitValues() length = %d, want 1", len(values))
	}
	value := values[0]
	if value.LimitGroupID != rows[0].LimitGroupID ||
		value.LimitTypeCode != rows[0].LimitTypeCode ||
		value.TransactionTypeCode != rows[0].TransactionTypeCode ||
		value.ChannelCode != rows[0].ChannelCode ||
		value.Operator != limit.OpLTE ||
		!value.Value.Equal(rows[0].Value) {
		t.Fatalf("toLimitValues() value = %#v", value)
	}
}
