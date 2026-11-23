package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zeann3th/policy-service/internal/core"
)

func handleError(c *gin.Context, err error) {
	_ = c.Error(err) // logged once by requestLogger
	for _, mapping := range []struct {
		target *core.AppError
		status int
	}{
		{core.ErrBadRequest, http.StatusBadRequest},
		{core.ErrNotFound, http.StatusNotFound},
		{core.ErrConflict, http.StatusConflict},
	} {
		if errors.Is(err, mapping.target) {
			// the AppError carries the specific message; its cause is not serialized
			var appErr *core.AppError
			if !errors.As(err, &appErr) {
				appErr = mapping.target
			}
			c.AbortWithStatusJSON(mapping.status, appErr)
			return
		}
	}
	c.AbortWithStatusJSON(http.StatusInternalServerError, core.ErrInternal)
}
