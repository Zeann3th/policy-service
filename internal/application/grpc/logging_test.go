package grpc

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	pb "github.com/zeann3th/policy-service/internal/application/grpc/codegen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRequestLoggerLogsCauseOnce(t *testing.T) {
	var logs bytes.Buffer
	interceptor := requestLogger(slog.New(slog.NewTextHandler(&logs, nil)))
	info := &grpc.UnaryServerInfo{FullMethod: "/policy.ui.grpc.ReservationService/Reserve"}

	_, err := interceptor(context.Background(), &pb.ReserveRequest{RequestId: "req-1"}, info, func(context.Context, any) (any, error) {
		return nil, grpcError(errors.New("db down"))
	})
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %s", status.Code(err))
	}
	_, err = interceptor(context.Background(), &pb.ReserveRequest{}, info, func(context.Context, any) (any, error) {
		panic("boom")
	})
	if status.Code(err) != codes.Internal {
		t.Fatalf("panic code = %s", status.Code(err))
	}

	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("want one line per call, got %d:\n%s", len(lines), logs.String())
	}
	for _, want := range []string{"level=ERROR", "Reserve --> Internal", `requestId`, `error="db down"`} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("log %q missing %q", lines[0], want)
		}
	}
	if !strings.Contains(lines[1], "panic: boom") {
		t.Errorf("panic log = %q", lines[1])
	}
}
