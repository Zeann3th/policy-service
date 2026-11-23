package http

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zeann3th/policy-service/internal/core"
)

// maxLoggedBody caps how much of a request or response body is logged.
const maxLoggedBody = 64 << 10

type bodyRecorder struct {
	gin.ResponseWriter
	body bytes.Buffer
}

func (w *bodyRecorder) Write(data []byte) (int, error) {
	if room := maxLoggedBody - w.body.Len(); room > 0 {
		w.body.Write(data[:min(len(data), room)])
	}
	return w.ResponseWriter.Write(data)
}

// requestLogger writes exactly one log line per request with its request
// and response bodies. It is the only place HTTP errors are logged: handlers
// pass errors to handleError, which records them on the context.
func requestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.Contains(c.Request.URL.Path, "/swagger/") {
			c.Next()
			return
		}
		start := time.Now()
		var request []byte
		if c.Request.Body != nil {
			request, _ = io.ReadAll(io.LimitReader(c.Request.Body, maxLoggedBody))
			c.Request.Body = io.NopCloser(io.MultiReader(bytes.NewReader(request), c.Request.Body))
		}
		recorder := &bodyRecorder{ResponseWriter: c.Writer}
		c.Writer = recorder

		c.Next()

		status := c.Writer.Status()
		attrs := []any{
			"status", status,
			"latencyMs", time.Since(start).Milliseconds(),
			"request", string(request),
			"response", recorder.body.String(),
		}
		if err := c.Errors.Last(); err != nil {
			attrs = append(attrs, "error", err.Err)
		}
		message := fmt.Sprintf("[HTTP] %s %s --> %d", c.Request.Method, c.Request.URL.RequestURI(), status)
		switch {
		case status >= http.StatusInternalServerError:
			logger.ErrorContext(c.Request.Context(), message+" unexpected error", attrs...)
		case status >= http.StatusBadRequest:
			logger.WarnContext(c.Request.Context(), message, attrs...)
		default:
			logger.InfoContext(c.Request.Context(), message, attrs...)
		}
	}
}

// recoverPanic turns a panic into a 500 and leaves logging to requestLogger.
func recoverPanic(c *gin.Context, recovered any) {
	_ = c.Error(fmt.Errorf("panic: %v\n%s", recovered, debug.Stack()))
	c.AbortWithStatusJSON(http.StatusInternalServerError, core.ErrInternal)
}
