package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"pki-certificate-rollover-impact/backend/internal/handler"
	"pki-certificate-rollover-impact/backend/internal/middleware"
	"pki-certificate-rollover-impact/backend/internal/model"
	"pki-certificate-rollover-impact/backend/internal/repository"
	"pki-certificate-rollover-impact/backend/internal/router"
	"pki-certificate-rollover-impact/backend/internal/service"
	"pki-certificate-rollover-impact/backend/internal/util"
	"pki-certificate-rollover-impact/backend/internal/x509util"
)

func newErrorMappingApp(t *testing.T) *gin.Engine {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.TrustAnchor{}, &model.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	svc := service.NewTrustAnchorService(repository.NewTrustAnchorRepository(db), repository.NewAuditRepository(db), repository.NewTransactionManager(db))
	gin.SetMode(gin.TestMode)
	api := gin.New()
	api.Use(func(c *gin.Context) {
		c.Set("actor", util.Actor{UserID: 2, Username: "operator", DisplayName: "operator", Team: "PKI Platform", Role: "pki_operator"})
		c.Next()
	})
	v1 := api.Group("/api/v1")
	router.RegisterTrustAnchorRoutes(v1, handler.NewTrustAnchorHandler(svc), middleware.NewRateLimiter(1000))
	return api
}

func TestFailPreservesErrorStatus(t *testing.T) {
	api := newErrorMappingApp(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/trust-anchors/99999", nil)
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestSuccessPreservesStatus(t *testing.T) {
	api := newErrorMappingApp(t)
	now := time.Now().UTC()
	pki, err := x509util.GenerateDemoPKI(now)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"anchor_code": "VALID-ROOT", "certificate_pem": pki.OldRootPEM})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trust-anchors", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}
}
