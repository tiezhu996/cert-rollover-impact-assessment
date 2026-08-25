package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestHealthzRespectsRequestContext(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:healthz-ctx?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/healthz", healthzHandler(db))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Fatalf("healthz must fail when the request context is canceled, got %d body=%s", w.Code, w.Body.String())
	}
}
