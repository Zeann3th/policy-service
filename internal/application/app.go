package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	grpcserver "github.com/zeann3th/policy-service/internal/application/grpc"
	httpserver "github.com/zeann3th/policy-service/internal/application/http"
	cachecore "github.com/zeann3th/policy-service/internal/core/cache"
	"github.com/zeann3th/policy-service/internal/core/policy"
	"github.com/zeann3th/policy-service/internal/core/reservation"
	"github.com/zeann3th/policy-service/internal/core/unit"
	rediscache "github.com/zeann3th/policy-service/internal/infrastructure/cache"
	"github.com/zeann3th/policy-service/internal/infrastructure/config"
	"github.com/zeann3th/policy-service/internal/infrastructure/postgres"
)

const shutdownTimeout = 10 * time.Second

type server interface {
	Start() error
	Shutdown(context.Context) error
}

func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := slog.Default()

	db, err := postgres.New(cfg, logger)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}

	redisClient := rediscache.NewRedisClient(cfg)
	defer redisClient.Close()

	applicationCache := rediscache.New(redisClient)
	responseCache := cachecore.NewVersioned(logger, applicationCache, cfg.ServiceName)
	rules := reservation.NewRules()
	unitService := unit.NewService(postgres.NewUnitStore(db), responseCache)
	groupService := policy.NewGroupService(postgres.NewGroupStore(db), responseCache)
	typeService := policy.NewTypeService(postgres.NewTypeStore(db), responseCache)
	policyService := policy.NewService(postgres.NewPolicyStore(db), rules, responseCache)
	reservationStore := postgres.NewReservationStore(db)
	reservationService := reservation.NewService(reservationStore, policyService)
	cacheService := cachecore.NewService(logger, applicationCache, cfg.ServiceName)

	httpServer, err := httpserver.New(cfg, logger, unitService, groupService, typeService, policyService, cacheService, reservationService)
	if err != nil {
		return fmt.Errorf("create HTTP server: %w", err)
	}

	grpcServer, err := grpcserver.New(cfg, logger, unitService, groupService, typeService, policyService, cacheService, reservationService)
	if err != nil {
		return fmt.Errorf("create gRPC server: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return run(ctx, httpServer, grpcServer)
}

func run(ctx context.Context, servers ...server) error {
	serverErrors := make(chan error, len(servers))
	for _, server := range servers {
		go func() {
			serverErrors <- server.Start()
		}()
	}

	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-serverErrors:
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	shutdownErrors := make(chan error, len(servers))
	for _, server := range servers {
		go func() {
			shutdownErrors <- server.Shutdown(shutdownCtx)
		}()
	}

	for range servers {
		runErr = errors.Join(runErr, <-shutdownErrors)
	}
	return runErr
}
