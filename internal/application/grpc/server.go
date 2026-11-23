package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	pb "github.com/zeann3th/policy-service/internal/application/grpc/codegen"
	cachecore "github.com/zeann3th/policy-service/internal/core/cache"
	"github.com/zeann3th/policy-service/internal/core/policy"
	"github.com/zeann3th/policy-service/internal/core/reservation"
	"github.com/zeann3th/policy-service/internal/core/unit"
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
	unitService unit.Service,
	groupService policy.GroupService,
	typeService policy.TypeService,
	policyService policy.Service,
	cacheService cachecore.Service,
	reservationService reservation.Service,
) (*Server, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPC.Port))
	if err != nil {
		return nil, err
	}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(requestLogger(logger)))
	pb.RegisterUnitServiceServer(grpcServer, &unitHandler{service: unitService})
	pb.RegisterPolicyGroupServiceServer(grpcServer, &policyGroupHandler{service: groupService})
	pb.RegisterPolicyTypeServiceServer(grpcServer, &policyTypeHandler{service: typeService})
	pb.RegisterPolicyServiceServer(grpcServer, &policyHandler{service: policyService})
	pb.RegisterCacheServiceServer(grpcServer, &cacheHandler{service: cacheService})
	pb.RegisterReservationServiceServer(grpcServer, &reservationHandler{service: reservationService})

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
