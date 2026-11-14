package queries

import (
	"os"
	"strings"
	"testing"
)

func TestActiveUserLimitsQueryUsesEffectiveWindow(t *testing.T) {
	query, err := os.ReadFile("limit.sql")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	sql := strings.ReplaceAll(string(query), "\r\n", "\n")

	for _, want := range []string{
		"-- name: GetActiveUserLimits :many",
		"JOIN limit_types lt ON lt.id = ul.limit_type_id",
		"ul.wallet_id = $1",
		"ul.effective_from <= now()",
		"ul.effective_to IS NULL",
		"ul.effective_to > now()",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("query missing %q", want)
		}
	}
}
