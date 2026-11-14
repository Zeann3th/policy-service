package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type commonHandler struct {
}

func (h *commonHandler) checkHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func RegisterUtilityRoutes(rg *gin.RouterGroup) {
	handler := &commonHandler{}

	rg.GET("/healthz", handler.checkHealth)
}

