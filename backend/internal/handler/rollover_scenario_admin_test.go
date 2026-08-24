package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"pki-certificate-rollover-impact/backend/internal/constants"
	"pki-certificate-rollover-impact/backend/internal/handler"
	"pki-certificate-rollover-impact/backend/internal/middleware"
	"pki-certificate-rollover-impact/backend/internal/model"
	"pki-certificate-rollover-impact/backend/internal/repository"
	"pki-certificate-rollover-impact/backend/internal/router"
	"pki-certificate-rollover-impact/backend/internal/service"
	"pki-certificate-rollover-impact/backend/internal/util"
)

func TestAdminCannotVerifyOwnScenario(t *testing.T) {
	api, id := newAdminTransitionApp(t)
	body, _ := json.Marshal(map[string]any{"to_state": "verified"})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/rollover-scenarios/%d/transition", id), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for admin verifying own scenario, got %d body=%s", w.Code, w.Body.String())
	}
}


func newAdminTransitionApp(t *testing.T) (*gin.Engine, uint) {
	t.Helper()
	db := openScenarioDB(t)
	now := timeNow()
	scenario := model.RolloverScenario{Name: "admin-scenario", OldAnchorID: 1, NewAnchorID: 2,
		OverlapStart: now.Add(timeHour()), OverlapEnd: now.Add(2 * timeHour()), CandidateChainIDs: "[]",
		AlgorithmVersion: "v1", InputHash: "h-admin", InputSnapshot: "{}", SimulationTime: now.Add(90 * timeMinute()),
		AffectedServicesJSON: "[]", BrokenPathsJSON: "[]", PathEvidenceJSON: "[]",
		ScenarioState: string(constants.ScenarioExecuting), Explanation: "test", CreatedBy: 1, CreatedByName: "admin",
		CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&scenario).Error; err != nil {
		t.Fatal(err)
	}
	svc := service.NewRolloverScenarioService(repository.NewRolloverScenarioRepository(db), repository.NewTrustAnchorRepository(db), repository.NewCertificateChainRepository(db), nil, repository.NewAuditRepository(db), repository.NewTransactionManager(db))
	gin.SetMode(gin.TestMode)
	api := gin.New()
	api.Use(func(c *gin.Context) {
		c.Set("actor", util.Actor{UserID: 1, Username: "admin", DisplayName: "admin", Team: "PKI Platform", Role: "admin"})
		c.Next()
	})
	v1 := api.Group("/api/v1")
	router.RegisterRolloverScenarioRoutes(v1, handler.NewRolloverScenarioHandler(svc), middleware.NewRateLimiter(1000))
	return api, scenario.ID
}
