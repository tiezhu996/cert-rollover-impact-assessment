package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"pki-certificate-rollover-impact/backend/internal/algorithm"
	"pki-certificate-rollover-impact/backend/internal/constants"
	"pki-certificate-rollover-impact/backend/internal/dto"
	"pki-certificate-rollover-impact/backend/internal/model"
	"pki-certificate-rollover-impact/backend/internal/repository"
	"pki-certificate-rollover-impact/backend/internal/util"

	"gorm.io/gorm"
)

func replaySnapshot(t *testing.T, db *gormDB) string {
	t.Helper()
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	snapshot := algorithm.NewSnapshot(algorithm.ScenarioConfig{Name: "replay", OldAnchorID: 1, NewAnchorID: 2, OverlapStart: start, OverlapEnd: start.Add(24 * time.Hour), CandidateChainIDs: []uint{11}, SimulationTime: start.Add(12 * time.Hour)},
		[]algorithm.AnchorSnapshot{{ID: 1, Code: "old", State: "valid", NotBefore: start.AddDate(-2, 0, 0), NotAfter: start.AddDate(0, 3, 0)}, {ID: 2, Code: "new", State: "valid", NotBefore: start.Add(-time.Hour), NotAfter: start.AddDate(2, 0, 0)}},
		[]algorithm.ChainSnapshot{{ID: 10, Code: "old-chain", AnchorID: 1, LeafSubject: "CN=api", ValidFrom: start.AddDate(-1, 0, 0), ValidTo: start.AddDate(0, 2, 0), State: "validated", ValidationValid: true}, {ID: 11, Code: "new-chain", AnchorID: 2, LeafSubject: "CN=api", ValidFrom: start.Add(-time.Hour), ValidTo: start.AddDate(1, 0, 0), State: "validated", ValidationValid: true}},
		[]algorithm.ServiceSnapshot{{ID: 101, Code: "gateway", ChainID: 10, TrustAnchorIDs: []uint{1, 2}, Criticality: "critical", State: "active"}})
	raw, err := snapshot.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func newReplayScenario(t *testing.T, db *gormDB, state, inputSnapshot, affectedJSON, pathsJSON string) model.RolloverScenario {
	return newReplayScenarioHash(t, db, state, inputSnapshot, affectedJSON, pathsJSON, "replay-hash")
}

func newReplayScenarioHash(t *testing.T, db *gormDB, state, inputSnapshot, affectedJSON, pathsJSON, hash string) model.RolloverScenario {
	return newReplayScenarioHashKey(t, db, state, inputSnapshot, affectedJSON, pathsJSON, hash, "")
}

func newReplayScenarioHashKey(t *testing.T, db *gormDB, state, inputSnapshot, affectedJSON, pathsJSON, hash, key string) model.RolloverScenario {
	t.Helper()
	scenario := minimalScenario(t, db, "replay-scenario", hash, key, state, 7, 0)
	scenario.InputSnapshot = inputSnapshot
	scenario.AffectedServicesJSON = affectedJSON
	scenario.BrokenPathsJSON = pathsJSON
	if err := db.Save(&scenario).Error; err != nil {
		t.Fatal(err)
	}
	return scenario
}

func replayService(db *gormDB) *RolloverScenarioService {
	return NewRolloverScenarioService(repository.NewRolloverScenarioRepository(db), nil, nil, nil, repository.NewAuditRepository(db), repository.NewTransactionManager(db))
}

func runReplay(t *testing.T, db *gormDB, id uint) (dto.RolloverScenarioResponse, error) {
	t.Helper()
	return replayService(db).Replay(context.Background(), id, util.Actor{UserID: 7, Username: "operator", Role: string(constants.RolePKIOperator)}, "replay-request")
}

func TestReplayMismatchReturnsConflict(t *testing.T) {
	db := newScenarioTestDB(t)
	scenario := newReplayScenario(t, db, "simulated", replaySnapshot(t, db), "[]", "[]")
	_, err := runReplay(t, db, scenario.ID)
	var apiErr *util.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusConflict {
		t.Fatalf("expected 409 replay mismatch, got %v", err)
	}
}

func TestReplayDetectsPartialTampering(t *testing.T) {
	db := newScenarioTestDB(t)
	snapshot := replaySnapshot(t, db)
	scenario := newReplayScenario(t, db, "simulated", snapshot, "[]", "[]")
	// recompute the real affected list so only broken paths differ
	res, err := algorithm.Simulate(mustDecodeSnapshot(t, snapshot))
	if err != nil {
		t.Fatal(err)
	}
	affectedJSON, _ := json.Marshal(res.AffectedServices)
	scenario.AffectedServicesJSON = string(affectedJSON)
	scenario.BrokenPathsJSON = "[]"
	if err := db.Save(&scenario).Error; err != nil {
		t.Fatal(err)
	}
	_, err = runReplay(t, db, scenario.ID)
	var apiErr *util.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusConflict {
		t.Fatalf("expected 409 when broken paths were tampered, got %v", err)
	}
}

func TestReplayDraftReturnsConflict(t *testing.T) {
	db := newScenarioTestDB(t)
	scenario := newReplayScenario(t, db, "draft", replaySnapshot(t, db), "[]", "[]")
	_, err := runReplay(t, db, scenario.ID)
	var apiErr *util.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusConflict || apiErr.Code != util.CodeStateTransition {
		t.Fatalf("expected 409 state transition for draft replay, got %v", err)
	}
}

func TestReplayVerifiedOnlyAffectsTargetScenario(t *testing.T) {
	db := newScenarioTestDB(t)
	snapshot := replaySnapshot(t, db)
	res, err := algorithm.Simulate(mustDecodeSnapshot(t, snapshot))
	if err != nil {
		t.Fatal(err)
	}
	affectedJSON, _ := json.Marshal(res.AffectedServices)
	pathsJSON, _ := json.Marshal(res.BrokenPaths)
	evidenceJSON, _ := json.Marshal(res.Evidence)
	a := newReplayScenarioHashKey(t, db, "simulated", snapshot, string(affectedJSON), string(pathsJSON), "replay-hash-a", "idem-a")
	a.PathEvidenceJSON = string(evidenceJSON)
	a.Explanation = res.Explanation
	if err := db.Save(&a).Error; err != nil {
		t.Fatal(err)
	}
	b := newReplayScenarioHashKey(t, db, "simulated", snapshot, "[]", "[]", "replay-hash-b", "idem-b")
	if _, err := runReplay(t, db, a.ID); err != nil {
		t.Fatalf("replay of consistent scenario should pass, got %v", err)
	}
	var first model.RolloverScenario
	if err := db.First(&first, a.ID).Error; err != nil {
		t.Fatal(err)
	}
	var second model.RolloverScenario
	if err := db.First(&second, b.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !first.ReplayVerified {
		t.Fatal("target scenario replay_verified should be set")
	}
	if second.ReplayVerified {
		t.Fatal("replay of scenario A must not mark scenario B as replay verified")
	}
}


type gormDB = gorm.DB

func mustDecodeSnapshot(t *testing.T, raw string) algorithm.Snapshot {
	t.Helper()
	snapshot, err := algorithm.DecodeSnapshot(raw)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}