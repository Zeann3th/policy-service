package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	cachecore "github.com/zeann3th/policy-service/internal/core/cache"
)

type cacheHandler struct {
	service cachecore.Service
}

func RegisterCacheRoutes(routes *gin.RouterGroup, service cachecore.Service) {
	handler := &cacheHandler{service: service}
	routes.POST("/v1/cache/reload", handler.reload)
}

// reload godoc
//
//	@Summary		Reload cached data
//	@Description	Evicts one relative key or invalidates one or more cache namespaces. Key and namespaces are mutually exclusive.
//	@Tags			Cache
//	@Accept			json
//	@Produce		json
//	@Param			request	body		cache.ReloadRequest	true	"Cache reload selector"
//	@Success		200		{object}	cache.ReloadResponse
//	@Failure		400,500	{object}	core.AppError
//	@Router			/v1/cache/reload [post]
func (h *cacheHandler) reload(c *gin.Context) {
	var request cachecore.ReloadRequest
	if !bindJSON(c, &request) {
		return
	}
	response, err := h.service.Reload(c.Request.Context(), request)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}
