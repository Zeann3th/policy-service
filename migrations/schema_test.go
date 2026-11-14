package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestLimitSchemaUsesIDsForInternalRelationships(t *testing.T) {
	schema, err := os.ReadFile("00002_limit_setup.sql")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	sql := strings.ReplaceAll(string(schema), "\r\n", "\n")

	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS limit_types (\n  id bigserial PRIMARY KEY,\n  code text NOT NULL UNIQUE",
		"limit_type_id bigint NOT NULL REFERENCES limit_types (id)",
		"wallet_type text NOT NULL",
		"currency varchar(3) NOT NULL",
		"tier text NOT NULL",
		"transaction_type text NOT NULL DEFAULT 'ALL'",
		"channel text NOT NULL DEFAULT 'ALL'",
		"('COIN_MIN'",
		"('COIN_MAX'",
		"('TXN_MIN_AMT'",
		"('TXN_MAX_AMT'",
		"('DAILY_OUT_MAX'",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("migration missing %q", want)
		}
	}

	for _, unwanted := range []string{
		"wallet_type_code",
		"currency_code",
		"tier_code",
		"limit_type_code",
		"transaction_type_code",
		"channel_code",
	} {
		if strings.Contains(sql, unwanted) {
			t.Fatalf("migration still contains %q", unwanted)
		}
	}
}
