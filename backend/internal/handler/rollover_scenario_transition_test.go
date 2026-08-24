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
	"pki-certificate-rollover-impact/backend/internal/constants"
	"pki-certificate-rollover-impact/backend/internal/handler"
	"pki-certificate-rollover-impact/backend/internal/middleware"
	"pki-certificate-rollover-impact/backend/internal/model"
	"pki-certificate-rollover-impact/backend/internal/repository"
	"pki-certificate-rollover-impact/backend/internal/router"
	"pki-certificate-rollover-impact/backend/internal/service"
	"pki-certificate-rollover-impact/backend/internal/util"
)

func newRolloverTransitionApp(t *testing.T) (*gin.Engine, uint) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.TrustAnchor{}, &model.RolloverScenario{}, &model.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	oldAnchor := model.TrustAnchor{AnchorCode: "T-OLD", SubjectDN: "CN=old", SerialNumber: "1", FingerprintSHA256: strings.Repeat("a", 64), KeyAlgorithm: "ECDSA", CertificateState: "valid", PemRedacted: "cert", NotBefore: now.Add(-time.Hour), NotAfter: now.AddDate(1, 0, 0), CreatedAt: now, UpdatedAt: now}
	newAnchor := model.TrustAnchor{AnchorCode: "T-NEW", SubjectDN: "CN=new", SerialNumber: "2", FingerprintSHA256: strings.Repeat("b", 64), KeyAlgorithm: "ECDSA", CertificateState: "valid", PemRedacted: "cert", NotBefore: now.Add(-time.Hour), NotAfter: now.AddDate(1, 0, 0), CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&oldAnchor).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&newAnchor).Error; err != nil {
		t.Fatal(err)
	}
	scenario := model.RolloverScenario{Name: "executing-scenario", OldAnchorID: oldAnchor.ID, NewAnchorID: newAnchor.ID,
		OverlapStart: now.Add(time.Hour), OverlapEnd: now.Add(2 * time.Hour), CandidateChainIDs: "[]",
		AlgorithmVersion: "v1", InputHash: "h", InputSnapshot: "{}", SimulationTime: now.Add(90 * time.Minute),
		AffectedServicesJSON: "[]", BrokenPathsJSON: "[]", PathEvidenceJSON: "[]",
		ScenarioState: string(constants.ScenarioExecuting), Explanation: "test", CreatedBy: 2, CreatedByName: "operator",
		CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&scenario).Error; err != nil {
		t.Fatal(err)
	}
	svc := service.NewRolloverScenarioService(repository.NewRolloverScenarioRepository(db), repository.NewTrustAnchorRepository(db), repository.NewCertificateChainRepository(db), nil, repository.NewAuditRepository(db), repository.NewTransactionManager(db))
	gin.SetMode(gin.TestMode)
	api := gin.New()
	api.Use(func(c *gin.Context) {
		c.Set("actor", util.Actor{UserID: 2, Username: "operator", DisplayName: "operator", Team: "PKI Platform", Role: "pki_operator"})
		c.Next()
	})
	v1 := api.Group("/api/v1")
	router.RegisterRolloverScenarioRoutes(v1, handler.NewRolloverScenarioHandler(svc), middleware.NewRateLimiter(1000))
	return api, scenario.ID
}

func TestScenarioRollbackTransitionAccepted(t *testing.T) {
	api, id := newRolloverTransitionApp(t)
	body, _ := json.Marshal(map[string]any{"to_state": "rollback", "comment": "roll it back"})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/rollover-scenarios/%d/transition", id), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for rollback transition, got %d body=%s", w.Code, w.Body.String())
	}
	var envelope struct {
		Data struct {
			ScenarioState string `json:"scenario_state"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.ScenarioState != "rollback" {
		t.Fatalf("expected scenario_state rollback, got %s", envelope.Data.ScenarioState)
	}
}

func TestCreatorSelfVerifyReturnsConflict(t *testing.T) {
	api, id := newRolloverTransitionApp(t)
	body, _ := json.Marshal(map[string]any{"to_state": "verified"})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/rollover-scenarios/%d/transition", id), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 REVIEWER_SEPARATION for creator self verify, got %d body=%s", w.Code, w.Body.String())
	}
}

func openScenarioDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.TrustAnchor{}, &model.RolloverScenario{}, &model.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func timeNow() time.Time { return time.Now().UTC() }
func timeHour() time.Duration { return time.Hour }
func timeMinute() time.Duration { return time.Minute }
