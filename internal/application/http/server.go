package http

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	docs "github.com/zeann3th/policy-service/docs"
	cachecore "github.com/zeann3th/policy-service/internal/core/cache"
	"github.com/zeann3th/policy-service/internal/core/policy"
	"github.com/zeann3th/policy-service/internal/core/reservation"
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
	reservationService reservation.Service,
) (*Server, error) {
	gin.SetMode(cfg.HTTP.GinMode)

	router := gin.New()

	router.Use(otelgin.Middleware(cfg.ServiceName), requestLogger(logger), gin.CustomRecoveryWithWriter(io.Discard, recoverPanic))
	if len(cfg.HTTP.CORSOrigins) > 0 {
		router.Use(corsMiddleware(cfg.HTTP.CORSOrigins))
	}

	prefixGroup := router.Group(fmt.Sprintf("/%s/api", cfg.ServiceName))

	RegisterUtilityRoutes(prefixGroup)
	RegisterUnitRoutes(prefixGroup, unitService)
	RegisterPolicyGroupRoutes(prefixGroup, groupService)
	RegisterPolicyTypeRoutes(prefixGroup, typeService)
	RegisterPolicyRoutes(prefixGroup, policyService)
	RegisterCacheRoutes(prefixGroup, cacheService)
	RegisterReservationRoutes(prefixGroup, reservationService)
	if cfg.HTTP.SwaggerEnabled {
		docs.SwaggerInfo.BasePath = fmt.Sprintf("/%s/api", cfg.ServiceName)
		prefixGroup.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
	}

	addr := fmt.Sprintf(":%s", cfg.HTTP.Port)
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

const allOrigins = "*"

// corsMiddleware lets the listed browser origins (or any, with "*") call the
// API. No credentials are allowed, so "*" stays a valid response header.
func corsMiddleware(origins []string) gin.HandlerFunc {
	config := cors.Config{
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
		MaxAge:       12 * time.Hour,
	}
	if slices.Contains(origins, allOrigins) {
		config.AllowAllOrigins = true
	} else {
		config.AllowOrigins = origins
	}
	return cors.New(config)
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
