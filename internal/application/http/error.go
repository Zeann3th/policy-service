package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zeann3th/policy-service/internal/core"
)

func handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, core.ErrBadRequest):
		c.AbortWithStatusJSON(http.StatusBadRequest, core.ErrBadRequest)
	case errors.Is(err, core.ErrNotFound):
		c.AbortWithStatusJSON(http.StatusNotFound, core.ErrNotFound)
	default:
		c.AbortWithStatusJSON(http.StatusInternalServerError, core.ErrInternal)
	}
}
