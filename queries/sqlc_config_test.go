package queries

import (
	"os"
	"strings"
	"testing"
)

func TestSQLCGeneratesIntoPostgresCodegenPackage(t *testing.T) {
	data, err := os.ReadFile("../sqlc.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `out: "internal/infrastructure/postgres/codegen"`) {
		t.Fatal("sqlc output must use internal/infrastructure/postgres/codegen")
	}
}
