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

	grpcserver "github.com/zeann3th/haphaestus/internal/application/grpc"
	httpserver "github.com/zeann3th/haphaestus/internal/application/http"
	"github.com/zeann3th/haphaestus/internal/core/limit"
	"github.com/zeann3th/haphaestus/internal/infrastructure/config"
	"github.com/zeann3th/haphaestus/internal/infrastructure/postgres"
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

	limitStore := postgres.NewLimitStore(db)
	limitService := limit.NewService(logger, limitStore)

	httpServer, err := httpserver.New(cfg, logger, db, limitService)
	if err != nil {
		return fmt.Errorf("create HTTP server: %w", err)
	}

	grpcServer, err := grpcserver.New(cfg, logger, db, limitService)
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
