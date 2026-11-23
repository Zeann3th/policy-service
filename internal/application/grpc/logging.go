package grpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/zeann3th/policy-service/internal/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// statusError is a gRPC status that keeps the error it was mapped from, so
// the logging interceptor can log the cause the client never sees.
type statusError struct {
	status *status.Status
	cause  error
}

func (e *statusError) Error() string              { return e.status.Err().Error() }
func (e *statusError) GRPCStatus() *status.Status { return e.status }
func (e *statusError) Unwrap() error              { return e.cause }

// requestLogger writes exactly one log line per unary call with its request
// and response. It is the only place gRPC errors and panics are logged.
func requestLogger(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (response any, err error) {
		start := time.Now()
		defer func() {
			if recovered := recover(); recovered != nil {
				err = &statusError{status: status.New(codes.Internal, core.ErrInternal.Message), cause: fmt.Errorf("panic: %v\n%s", recovered, debug.Stack())}
				response = nil
			}
			code := status.Code(err)
			attrs := []any{
				"code", code.String(),
				"latencyMs", time.Since(start).Milliseconds(),
				"request", marshal(request),
				"response", marshal(response),
			}
			if err != nil {
				cause := err
				if unwrapped := errors.Unwrap(err); unwrapped != nil {
					cause = unwrapped
				}
				attrs = append(attrs, "error", cause)
			}
			message := fmt.Sprintf("[GRPC] %s --> %s", info.FullMethod, code)
			switch code {
			case codes.OK:
				logger.InfoContext(ctx, message, attrs...)
			case codes.Internal, codes.Unknown, codes.DataLoss, codes.Unavailable:
				logger.ErrorContext(ctx, message+" unexpected error", attrs...)
			default:
				logger.WarnContext(ctx, message, attrs...)
			}
		}()
		return handler(ctx, request)
	}
}

func marshal(value any) string {
	message, ok := value.(proto.Message)
	if !ok || message == nil {
		return ""
	}
	data, err := protojson.Marshal(message)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(data)
}
