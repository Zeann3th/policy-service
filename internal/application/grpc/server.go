package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/jackc/pgx/v5/pgxpool"
	pb "github.com/zeann3th/policy-service/internal/application/grpc/codegen"
	"github.com/zeann3th/policy-service/internal/core/limit"
	"github.com/zeann3th/policy-service/internal/infrastructure/config"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

type Server struct {
	grpcServer *grpc.Server
	listener   net.Listener
	logger     *slog.Logger
}

func New(
	cfg *config.Config,
	logger *slog.Logger,
	db *pgxpool.Pool,
	limitService limit.LimitService,
) (*Server, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GrpcConfig.Port))
	if err != nil {
		return nil, err
	}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()))
	pb.RegisterLimitServiceServer(grpcServer, &limitGRPCHandler{service: limitService})

	return &Server{
		grpcServer: grpcServer,
		listener:   listener,
		logger:     logger,
	}, nil
}

func (s *Server) Start() error {
	s.logger.Info("[GRPC] Starting GRPC server", "addr", s.listener.Addr().String())
	return s.grpcServer.Serve(s.listener)
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("[GRPC] Shutting down GRPC server")

	stopped := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		return nil
	case <-ctx.Done():
		s.grpcServer.Stop()
		<-stopped
		return ctx.Err()
	}
}
