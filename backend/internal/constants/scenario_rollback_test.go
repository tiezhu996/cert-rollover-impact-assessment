package constants

import "testing"

func TestScenarioStateValuesIncludeRollback(t *testing.T) {
	values := ScenarioStateValues()
	for _, v := range values {
		if v == ScenarioRollback {
			return
		}
	}
	t.Fatalf("scenario state values must include rollback: %v", values)
}

func TestCanTransitionScenarioAllowsRollback(t *testing.T) {
	if !CanTransitionScenario(ScenarioExecuting, ScenarioRollback) {
		t.Fatal("executing -> rollback must be a legal transition")
	}
}
