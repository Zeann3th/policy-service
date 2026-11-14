package http

import (
	"github.com/gin-gonic/gin"
	"github.com/zeann3th/haphaestus/internal/core/limit"
)

type limitHandler struct {
	service limit.LimitService
}

func (h *limitHandler) getLimitConfig(c *gin.Context) {

}

func RegisterLimitRoutes(rg *gin.RouterGroup, limitService limit.LimitService) {
	handler := &limitHandler{
		service: limitService,
	}

	rg.GET("/v1/limits", handler.getLimitConfig)
}
