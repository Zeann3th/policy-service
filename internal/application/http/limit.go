package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/limit"
)

type limitHandler struct {
	service limit.LimitService
}

func (h *limitHandler) getLimitConfig(c *gin.Context) {
	var request limit.GetLimitConfigRequest
	if err := c.ShouldBindQuery(&request); err != nil {
		handleError(c, core.ErrBadRequest.WithCause(err))
		return
	}

	response, err := h.service.GetLimitConfig(c.Request.Context(), request)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

func RegisterLimitRoutes(rg *gin.RouterGroup, limitService limit.LimitService) {
	handler := &limitHandler{
		service: limitService,
	}

	rg.GET("/v1/limits", handler.getLimitConfig)
}
