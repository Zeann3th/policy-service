package grpc

import (
	"context"
	"math"
	"os"
	"strings"
	"testing"

	pb "github.com/zeann3th/policy-service/internal/application/grpc/codegen"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/limit"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestLimitProtoGeneratesIntoGRPCCodegenPackage(t *testing.T) {
	data, err := os.ReadFile("../../../proto/limit.proto")
	if err != nil {
		t.Fatal(err)
	}
	want := `option go_package = "github.com/zeann3th/policy-service/internal/application/grpc/codegen;grpc";`
	if !strings.Contains(string(data), want) {
		t.Fatalf("limit.proto missing %q", want)
	}
}

type grpcServiceStub struct {
	limit.LimitService
	page limit.PageRequest
}

func TestUpdateUnitRequiresActivePresence(t *testing.T) {
	handler := &limitGRPCHandler{service: &grpcServiceStub{}}
	_, err := handler.UpdateUnit(context.Background(), &pb.UpdateUnitRequest{Id: 1, UpdatedBy: "admin"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %v", status.Code(err))
	}
}

func TestUnsignedIDConversionRejectsOverflow(t *testing.T) {
	if _, err := intID(math.MaxUint64); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("error = %v", err)
	}
}

func (s *grpcServiceStub) ListUnits(_ context.Context, page limit.PageRequest) (limit.Page[limit.UnitResponse], error) {
	s.page = page
	return limit.Page[limit.UnitResponse]{Page: 1, Limit: 20}, nil
}

func TestListUnitsPreservesZeroDefaultsForDomain(t *testing.T) {
	service := &grpcServiceStub{}
	handler := &limitGRPCHandler{service: service}
	response, err := handler.ListUnits(context.Background(), &pb.PageRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if service.page.Page != 0 || service.page.Limit != 0 || response.Page != 1 || response.Limit != 20 {
		t.Fatalf("domain page = %#v, response = %#v", service.page, response)
	}
}

func TestGRPCErrorUsesCanonicalCodes(t *testing.T) {
	for _, test := range []struct {
		err  error
		code codes.Code
	}{{core.ErrBadRequest, codes.InvalidArgument}, {core.ErrNotFound, codes.NotFound}, {core.ErrConflict, codes.AlreadyExists}, {context.Canceled, codes.Internal}} {
		if got := status.Code(grpcError(test.err)); got != test.code {
			t.Fatalf("grpcError(%v) = %v, want %v", test.err, got, test.code)
		}
	}
}
