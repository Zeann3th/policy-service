package http

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestLoggerLogsOnceWithBodiesAndCause(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	router := gin.New()
	router.Use(requestLogger(slog.New(slog.NewTextHandler(&logs, nil))), gin.CustomRecoveryWithWriter(io.Discard, recoverPanic))
	router.POST("/fail", func(c *gin.Context) {
		body, _ := io.ReadAll(c.Request.Body)
		if string(body) != `{"a":1}` {
			t.Errorf("handler body = %q", body)
		}
		handleError(c, errors.New("db down"))
	})
	router.GET("/panic", func(*gin.Context) { panic("boom") })

	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodPost, "/fail", strings.NewReader(`{"a":1}`)),
		httptest.NewRequest(http.MethodGet, "/panic", nil),
	} {
		router.ServeHTTP(httptest.NewRecorder(), request)
	}

	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("want one line per request, got %d:\n%s", len(lines), logs.String())
	}
	for _, want := range []string{"level=ERROR", "POST /fail --> 500", `request="{\"a\":1}"`, "INTERNAL_ERROR", "error=\"db down\""} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("log %q missing %q", lines[0], want)
		}
	}
	if !strings.Contains(lines[1], "GET /panic --> 500") || !strings.Contains(lines[1], "panic: boom") {
		t.Errorf("panic log = %q", lines[1])
	}
}
