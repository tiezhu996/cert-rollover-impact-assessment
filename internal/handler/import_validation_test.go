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

	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
)

func newImportTestApp(t *testing.T) (*gin.Engine, uint) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.TrustAnchor{}, &model.CertificateChain{}, &model.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	pki, err := x509util.GenerateDemoPKI(now)
	if err != nil {
		t.Fatal(err)
	}
	oldCert, err := x509util.ParseCertificatePEM(pki.OldRootPEM)
	if err != nil {
		t.Fatal(err)
	}
	anchor := model.TrustAnchor{AnchorCode: "SEED-ROOT", SubjectDN: x509util.SubjectDN(oldCert), SerialNumber: oldCert.SerialNumber.String(), FingerprintSHA256: x509util.Fingerprint(oldCert), KeyAlgorithm: x509util.KeyAlgorithm(oldCert), CertificateState: "valid", PemRedacted: pki.OldRootPEM, NotBefore: oldCert.NotBefore, NotAfter: oldCert.NotAfter, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&anchor).Error; err != nil {
		t.Fatal(err)
	}
	transactions := repository.NewTransactionManager(db)
	anchorSvc := service.NewTrustAnchorService(repository.NewTrustAnchorRepository(db), repository.NewAuditRepository(db), transactions)
	chainSvc := service.NewCertificateChainService(repository.NewCertificateChainRepository(db), repository.NewTrustAnchorRepository(db), repository.NewAuditRepository(db), transactions)
	gin.SetMode(gin.TestMode)
	api := gin.New()
	api.Use(func(c *gin.Context) {
		c.Set("actor", util.Actor{UserID: 2, Username: "operator", DisplayName: "operator", Team: "PKI Platform", Role: "pki_operator"})
		c.Next()
	})
	v1 := api.Group("/api/v1")
	limiter := middleware.NewRateLimiter(1000)
	router.RegisterTrustAnchorRoutes(v1, handler.NewTrustAnchorHandler(anchorSvc), limiter)
	router.RegisterCertificateChainRoutes(v1, handler.NewCertificateChainHandler(chainSvc), limiter)
	return api, anchor.ID
}

func postJSON(t *testing.T, api *gin.Engine, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)
	return w
}

func TestImportInvalidAnchorReturnsValidationError(t *testing.T) {
	api, _ := newImportTestApp(t)
	w := postJSON(t, api, "/api/v1/trust-anchors", map[string]any{"anchor_code": "BAD-ANCHOR", "certificate_pem": "this is definitely not a pem certificate block but long enough to pass the minimum length validation gate 1234567890"})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestImportInvalidChainBundleReturnsValidationError(t *testing.T) {
	api, anchorID := newImportTestApp(t)
	w := postJSON(t, api, "/api/v1/certificate-chains", map[string]any{"chain_code": "BAD-CHAIN", "trust_anchor_id": anchorID, "certificates_pem": []string{"not a pem at all"}})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestImportChainWithBadSignatureRejected(t *testing.T) {
	api, anchorID := newImportTestApp(t)
	now := time.Now().UTC()
	pki, err := x509util.GenerateDemoPKI(now)
	if err != nil {
		t.Fatal(err)
	}
	w := postJSON(t, api, "/api/v1/certificate-chains", map[string]any{
		"chain_code":       "SIG-BAD",
		"trust_anchor_id":  anchorID,
		"certificates_pem": []string{pki.NewLeafPEM},
	})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for chain not signed by the trust anchor, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestImportNonSelfSignedAnchorRejected(t *testing.T) {
	api, _ := newImportTestApp(t)
	intermediatePEM := makeIntermediateCA(t)
	w := postJSON(t, api, "/api/v1/trust-anchors", map[string]any{"anchor_code": "NON-SELF-SIGNED", "certificate_pem": intermediatePEM})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for non-self-signed anchor, got %d body=%s", w.Code, w.Body.String())
	}
}


func makeIntermediateCA(t *testing.T) string {
	t.Helper()
	now := time.Now().UTC()
	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	rootTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1001),
		Subject:               pkix.Name{Organization: []string{"Test"}, CommonName: "Test Root"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(2, 0, 0),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		SignatureAlgorithm:    x509.ECDSAWithSHA256,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTemplate, rootTemplate, &rootKey.PublicKey, rootKey)
	if err != nil {
		t.Fatal(err)
	}
	rootCert, err := x509.ParseCertificate(rootDER)
	if err != nil {
		t.Fatal(err)
	}
	interKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	interTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(4242),
		Subject:               pkix.Name{Organization: []string{"Test"}, CommonName: "Test Intermediate CA"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(1, 0, 0),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		SignatureAlgorithm:    x509.ECDSAWithSHA256,
	}
	interDER, err := x509.CreateCertificate(rand.Reader, interTemplate, rootCert, &interKey.PublicKey, rootKey)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: interDER}))
}