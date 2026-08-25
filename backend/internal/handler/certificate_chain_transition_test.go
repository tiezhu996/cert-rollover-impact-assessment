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
)

func newChainTransitionApp(t *testing.T) (*gin.Engine, uint) {
	return newChainTransitionAppWithRole(t, "operator", "pki_operator")
}

func newChainTransitionAppWithRole(t *testing.T, username, role string) (*gin.Engine, uint) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.TrustAnchor{}, &model.CertificateChain{}, &model.DependentService{}, &model.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	anchor := model.TrustAnchor{AnchorCode: "T-ROOT", SubjectDN: "CN=root", SerialNumber: "1", FingerprintSHA256: strings.Repeat("a", 64), KeyAlgorithm: "ECDSA", CertificateState: "valid", PemRedacted: "cert", NotBefore: now.Add(-time.Hour), NotAfter: now.AddDate(1, 0, 0), CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&anchor).Error; err != nil {
		t.Fatal(err)
	}
	chain := model.CertificateChain{ChainCode: "CHAIN-VALID", TrustAnchorID: anchor.ID, LeafSubject: "CN=leaf", CertificateRefsJSON: "[]", ChainFingerprint: "fp", ValidFrom: now.Add(-time.Hour), ValidTo: now.AddDate(1, 0, 0), ValidationResult: `{"valid":true}`, ChainState: "validated", SourceChecksum: "sc", PublicChainPEM: "pem", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&chain).Error; err != nil {
		t.Fatal(err)
	}
	svc := service.NewCertificateChainService(repository.NewCertificateChainRepository(db), repository.NewTrustAnchorRepository(db), repository.NewAuditRepository(db), repository.NewTransactionManager(db))
	gin.SetMode(gin.TestMode)
	api := gin.New()
	api.Use(func(c *gin.Context) {
		c.Set("actor", util.Actor{UserID: 9, Username: username, DisplayName: username, Team: "PKI Platform", Role: role})
		c.Next()
	})
	v1 := api.Group("/api/v1")
	router.RegisterCertificateChainRoutes(v1, handler.NewCertificateChainHandler(svc), middleware.NewRateLimiter(1000))
	return api, chain.ID
}

func TestDeprecateValidatedChainAccepted(t *testing.T) {
	api, id := newChainTransitionApp(t)
	body, _ := json.Marshal(map[string]any{"to_state": "deprecated"})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/certificate-chains/%d/transition", id), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for deprecating a validated chain, got %d body=%s", w.Code, w.Body.String())
	}
	var envelope struct {
		Data struct {
			ChainState string `json:"chain_state"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.ChainState != "deprecated" {
		t.Fatalf("expected chain_state deprecated, got %s", envelope.Data.ChainState)
	}
}

func TestAuditorCannotTransitionChain(t *testing.T) {
	api, id := newChainTransitionAppWithRole(t, "auditor", "auditor")
	body, _ := json.Marshal(map[string]any{"to_state": "deprecated"})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/certificate-chains/%d/transition", id), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for auditor transition, got %d body=%s", w.Code, w.Body.String())
	}
}
