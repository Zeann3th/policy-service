package proto

import (
	"os"
	"strings"
	"testing"
)

func TestLimitProtoGeneratesIntoGRPCCodegenPackage(t *testing.T) {
	data, err := os.ReadFile("limit.proto")
	if err != nil {
		t.Fatal(err)
	}
	want := `option go_package = "github.com/zeann3th/policy-service/internal/application/grpc/codegen;grpc";`
	if !strings.Contains(string(data), want) {
		t.Fatalf("limit.proto missing %q", want)
	}
}
