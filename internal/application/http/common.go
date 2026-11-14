package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type handler struct {
}

func (h *handler) checkHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func RegisterUtilityRoutes(rg *gin.RouterGroup) {
	handler := &handler{}

	rg.GET("/healthz", handler.checkHealth)
}

