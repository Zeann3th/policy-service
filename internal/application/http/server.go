package http

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	docs "github.com/zeann3th/policy-service/docs"
	cachecore "github.com/zeann3th/policy-service/internal/core/cache"
	"github.com/zeann3th/policy-service/internal/core/policy"
	"github.com/zeann3th/policy-service/internal/core/unit"
	"github.com/zeann3th/policy-service/internal/infrastructure/config"
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
	unitService unit.Service,
	groupService policy.GroupService,
	typeService policy.TypeService,
	policyService policy.Service,
	cacheService cachecore.Service,
) (*Server, error) {
	gin.SetMode(cfg.HttpConfig.GinMode)

	router := gin.Default()

	router.Use(otelgin.Middleware(cfg.ServiceName))

	prefixGroup := router.Group(fmt.Sprintf("/%s/api", cfg.ServiceName))
	docs.SwaggerInfo.BasePath = fmt.Sprintf("/%s/api", cfg.ServiceName)

	RegisterUtilityRoutes(prefixGroup)
	RegisterUnitRoutes(prefixGroup, unitService)
	RegisterPolicyGroupRoutes(prefixGroup, groupService)
	RegisterPolicyTypeRoutes(prefixGroup, typeService)
	RegisterPolicyRoutes(prefixGroup, policyService)
	RegisterCacheRoutes(prefixGroup, cacheService)
	prefixGroup.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))

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
