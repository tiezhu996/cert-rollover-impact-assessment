package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"pki-certificate-rollover-impact/backend/internal/util"
)

func TestRequestIDNeverEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"request_id": util.RequestID(c)})
	})
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if got := w.Header().Get("X-Request-ID"); got == "" {
		t.Fatal("request id must never be empty")
	}
}
