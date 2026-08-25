package algorithm

import (
	"testing"
	"time"
)

func TestServiceChainOutsideCandidatesReachable(t *testing.T) {
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	anchors := []AnchorSnapshot{
		{ID: 1, Code: "old", State: "valid", NotBefore: start.AddDate(-2, 0, 0), NotAfter: start.AddDate(0, 3, 0)},
		{ID: 2, Code: "new", State: "valid", NotBefore: start.Add(-time.Hour), NotAfter: start.AddDate(2, 0, 0)},
	}
	chains := []ChainSnapshot{
		{ID: 10, Code: "own-chain", AnchorID: 1, LeafSubject: "CN=api", ValidFrom: start.AddDate(-1, 0, 0), ValidTo: start.AddDate(0, 2, 0), State: "validated", ValidationValid: true},
		{ID: 11, Code: "candidate-chain", AnchorID: 2, LeafSubject: "CN=api", ValidFrom: start.Add(-time.Hour), ValidTo: start.AddDate(1, 0, 0), State: "validated", ValidationValid: true},
	}
	services := []ServiceSnapshot{
		{ID: 101, Code: "gateway", ChainID: 10, TrustAnchorIDs: []uint{1}, Criticality: "critical", State: "active"},
	}
	config := ScenarioConfig{Name: "own chain not candidate", OldAnchorID: 1, NewAnchorID: 2,
		OverlapStart: start, OverlapEnd: start.Add(24 * time.Hour),
		CandidateChainIDs: []uint{11}, SimulationTime: start.Add(12 * time.Hour)}
	snapshot := NewSnapshot(config, anchors, chains, services)
	result, err := Simulate(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	overlapEnd := start.Add(24 * time.Hour)
	for _, ev := range result.Evidence {
		if !ev.At.Before(overlapEnd) {
			continue // after the overlap only the new anchor is active; trust gap is expected
		}
		for _, se := range ev.Services {
			if se.ServiceID == 101 && !se.Reachable {
				t.Fatalf("service 101 must stay reachable during overlap via its own chain even when it is not a candidate: %+v", se)
			}
		}
	}
}

func TestSnapshotHashStableAcrossInputOrder(t *testing.T) {
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	anchors := []AnchorSnapshot{
		{ID: 1, Code: "old", State: "valid", NotBefore: start.AddDate(-2, 0, 0), NotAfter: start.AddDate(0, 3, 0)},
		{ID: 2, Code: "new", State: "valid", NotBefore: start.Add(-time.Hour), NotAfter: start.AddDate(2, 0, 0)},
	}
	chains := []ChainSnapshot{
		{ID: 10, Code: "own-chain", AnchorID: 1, LeafSubject: "CN=api", ValidFrom: start.AddDate(-1, 0, 0), ValidTo: start.AddDate(0, 2, 0), State: "validated", ValidationValid: true},
	}
	config := ScenarioConfig{Name: "hash order", OldAnchorID: 1, NewAnchorID: 2,
		OverlapStart: start, OverlapEnd: start.Add(24 * time.Hour),
		CandidateChainIDs: []uint{10}, SimulationTime: start.Add(12 * time.Hour)}
	svcA := ServiceSnapshot{ID: 101, Code: "gateway", ChainID: 10, TrustAnchorIDs: []uint{2, 1}, Criticality: "critical", State: "active"}
	svcB := ServiceSnapshot{ID: 102, Code: "worker", ChainID: 10, TrustAnchorIDs: []uint{1, 2, 1}, DependencyIDs: []uint{101}, Criticality: "high", State: "active"}
	first := NewSnapshot(config, anchors, chains, []ServiceSnapshot{svcB, svcA})
	second := NewSnapshot(config, anchors, chains, []ServiceSnapshot{svcA, svcB})
	h1, err := first.Hash()
	if err != nil {
		t.Fatal(err)
	}
	h2, err := second.Hash()
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatalf("hash must be stable across input order: %s vs %s", h1, h2)
	}
}

func TestCriticalTimesUnique(t *testing.T) {
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	config := ScenarioConfig{Name: "dedup", OldAnchorID: 1, NewAnchorID: 2,
		OverlapStart: start, OverlapEnd: start.Add(24 * time.Hour),
		SimulationTime: start.Add(12 * time.Hour)}
	points := criticalTimes(config)
	seen := map[time.Time]bool{}
	for _, p := range points {
		if seen[p] {
			t.Fatalf("duplicate critical timepoint %v", p)
		}
		seen[p] = true
	}
	if len(points) != 5 {
		t.Fatalf("expected 5 unique timepoints, got %d: %v", len(points), points)
	}
}
