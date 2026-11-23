package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zeann3th/policy-service/internal/core"
)

func handleError(c *gin.Context, err error) {
	_ = c.Error(err) // logged once by requestLogger
	switch {
	case errors.Is(err, core.ErrBadRequest):
		var appErr *core.AppError
		if errors.As(err, &appErr) {
			c.AbortWithStatusJSON(http.StatusBadRequest, appErr)
			return
		}
		c.AbortWithStatusJSON(http.StatusBadRequest, core.ErrBadRequest)
	case errors.Is(err, core.ErrNotFound):
		c.AbortWithStatusJSON(http.StatusNotFound, core.ErrNotFound)
	case errors.Is(err, core.ErrConflict):
		c.AbortWithStatusJSON(http.StatusConflict, core.ErrConflict)
	default:
		c.AbortWithStatusJSON(http.StatusInternalServerError, core.ErrInternal)
	}
}
