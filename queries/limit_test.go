package queries

import (
	"os"
	"path/filepath"
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
		"-- name: GetUnits :many",
		"lt.unit_id",
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

func TestAdminQueriesExposeScopedCRUDAndPagination(t *testing.T) {
	wants := map[string][]string{
		"unit.sql":        {"CreateUnit", "GetUnitByID", "ListUnits", "CountUnits", "UpdateUnit", "DeleteUnit", "ORDER BY id"},
		"limit_type.sql":  {"CreateLimitType", "GetLimitTypeByID", "ListLimitTypes", "CountLimitTypes", "UpdateLimitType", "DeleteLimitType", "ORDER BY id"},
		"limit_group.sql": {"CreateLimitGroup", "GetLimitGroupByID", "DeleteLimitGroup"},
		"limit_value.sql": {"CreateLimitValue", "GetLimitValueByID", "ListLimitValues", "CountLimitValues", "UpdateLimitValue", "DeleteLimitValue", "limit_group_id", "ORDER BY id"},
		"user_limit.sql":  {"CreateUserLimit", "GetUserLimitByID", "ListUserLimits", "CountUserLimits", "UpdateUserLimit", "DeleteUserLimit", "wallet_id", "ORDER BY id"},
		"audit.sql":       {"CreateLimitAuditLog", "old_value", "new_value", "updated_by"},
	}
	for name, fragments := range wants {
		data, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, fragment := range fragments {
			if !strings.Contains(string(data), fragment) {
				t.Errorf("%s missing %q", name, fragment)
			}
		}
	}

	data, err := os.ReadFile("limit.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(data)
	if strings.Count(sql, "lt.active = true") != 2 || strings.Count(sql, "u.active = true") != 2 {
		t.Fatal("resolver queries must exclude inactive limit types and units")
	}
}
