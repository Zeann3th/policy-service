package http

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zeann3th/haphaestus/internal/core/limit"
	"github.com/zeann3th/haphaestus/internal/infrastructure/config"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

type Server struct {
	httpServer *http.Server
	listener   net.Listener
	logger     *slog.Logger
}

func New(
	cfg *config.Config,
	logger *slog.Logger,
	db *pgxpool.Pool,
	limitService limit.LimitService,
) (*Server, error) {
	gin.SetMode(cfg.HttpConfig.GinMode)

	router := gin.Default()

	router.Use(otelgin.Middleware(cfg.ServiceName))

	prefixGroup := router.Group(fmt.Sprintf("/%s/api", cfg.ServiceName))

	RegisterUtilityRoutes(prefixGroup)
	RegisterLimitRoutes(prefixGroup, limitService)

	addr := fmt.Sprintf(":%s", cfg.HttpConfig.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	return &Server{
		httpServer: &http.Server{
			Addr:    addr,
			Handler: router,
		},
		listener: listener,
		logger:   logger,
	}, nil
}

func (s *Server) Start() error {
	s.logger.Info("[HTTP] Starting HTTP server", "addr", s.listener.Addr().String())
	if err := s.httpServer.Serve(s.listener); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("[HTTP] Shutting down HTTP server")
	return s.httpServer.Shutdown(ctx)
}
