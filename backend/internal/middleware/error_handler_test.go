package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"pki-certificate-rollover-impact/backend/internal/util"
)

func TestErrorHandlerLogsFailures(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ErrorHandler(logger))
	router.GET("/boom", func(c *gin.Context) {
		util.Fail(c, util.NewError(http.StatusNotFound, util.CodeNotFound, "nope"))
	})
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if !strings.Contains(buf.String(), "request failed") {
		t.Fatalf("error handler must log request failures, got: %s", buf.String())
	}
}
