package constants

import "testing"

func TestCanTransitionChainAllowsDeprecate(t *testing.T) {
	if !CanTransitionChain(ChainValidated, ChainDeprecated) {
		t.Fatal("validated -> deprecated must be a legal transition")
	}
}

func TestCanTransitionChainRejectsReverse(t *testing.T) {
	if CanTransitionChain(ChainDeprecated, ChainValidated) {
		t.Fatal("deprecated -> validated must be rejected")
	}
}
