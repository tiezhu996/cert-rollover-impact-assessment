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
	"pki-certificate-rollover-impact/backend/internal/model"
	"pki-certificate-rollover-impact/backend/internal/repository"
	"pki-certificate-rollover-impact/backend/internal/router"
	"pki-certificate-rollover-impact/backend/internal/service"
	"pki-certificate-rollover-impact/backend/internal/util"
)

func newDependentServiceTestApp(t *testing.T) (*gin.Engine, uint) {
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
	anchor := model.TrustAnchor{AnchorCode: "TEST-ROOT", SubjectDN: "CN=test-root", SerialNumber: "1", FingerprintSHA256: strings.Repeat("a", 64), KeyAlgorithm: "ECDSA", CertificateState: "valid", PemRedacted: "public certificate", NotBefore: now.Add(-time.Hour), NotAfter: now.AddDate(1, 0, 0), CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&anchor).Error; err != nil {
		t.Fatal(err)
	}
	chain := model.CertificateChain{ChainCode: "TEST-CHAIN", TrustAnchorID: anchor.ID, LeafSubject: "CN=test-leaf", CertificateRefsJSON: "[]", ChainFingerprint: "fp-001", ValidFrom: now.Add(-time.Hour), ValidTo: now.AddDate(1, 0, 0), ValidationResult: `{"valid":true}`, ChainState: "validated", SourceChecksum: "sc-001", PublicChainPEM: "pem", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&chain).Error; err != nil {
		t.Fatal(err)
	}
	svc := service.NewDependentServiceService(repository.NewDependentServiceRepository(db), repository.NewCertificateChainRepository(db), repository.NewTrustAnchorRepository(db), repository.NewAuditRepository(db), repository.NewTransactionManager(db))
	h := handler.NewDependentServiceHandler(svc)
	gin.SetMode(gin.TestMode)
	api := gin.New()
	api.Use(func(c *gin.Context) {
		c.Set("actor", util.Actor{UserID: 2, Username: "operator", DisplayName: "operator", Team: "PKI Platform", Role: "pki_operator"})
		c.Next()
	})
	v1 := api.Group("/api/v1")
	router.RegisterDependentServiceRoutes(v1, h)
	return api, anchor.ID
}

func doJSON(t *testing.T, api *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)
	return w
}

func serviceBody(code string, chainID, anchorID uint) string {
	payload, _ := json.Marshal(map[string]any{
		"service_code":           code,
		"name":                   "Dependent Service " + code,
		"owner_team":             "Payments Platform",
		"environment":            "production",
		"chain_id":               chainID,
		"client_trust_refs_json": []uint{anchorID},
		"protocol":               "mtls",
		"criticality":            "high",
		"dependency_edges_json":  []uint{},
	})
	return string(payload)
}

func TestMissingDependentServiceReturnsNotFound(t *testing.T) {
	api, _ := newDependentServiceTestApp(t)
	w := doJSON(t, api, http.MethodGet, "/api/v1/dependent-services/99999", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestMissingDependentServiceUpdateReturnsNotFound(t *testing.T) {
	api, anchorID := newDependentServiceTestApp(t)
	body := serviceBody("UPDATE-MISSING", 1, anchorID)
	w := doJSON(t, api, http.MethodPut, "/api/v1/dependent-services/99999", body)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestMissingDependentServiceDeactivateReturnsNotFound(t *testing.T) {
	api, _ := newDependentServiceTestApp(t)
	w := doJSON(t, api, http.MethodPost, "/api/v1/dependent-services/99999/deactivate", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestDuplicateServiceCodeReturnsConflict(t *testing.T) {
	api, anchorID := newDependentServiceTestApp(t)
	first := doJSON(t, api, http.MethodPost, "/api/v1/dependent-services", serviceBody("PAY-DUP", 1, anchorID))
	if first.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", first.Code, first.Body.String())
	}
	second := doJSON(t, api, http.MethodPost, "/api/v1/dependent-services", serviceBody("PAY-DUP", 1, anchorID))
	if second.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", second.Code, second.Body.String())
	}
}
