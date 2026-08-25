package util

import (
	"net/http"
	"testing"
)

func TestNotFoundReturns404(t *testing.T) {
	apiErr := NotFound("trust anchor")
	if apiErr.Status != http.StatusNotFound {
		t.Fatalf("NotFound must return 404, got %d", apiErr.Status)
	}
}
