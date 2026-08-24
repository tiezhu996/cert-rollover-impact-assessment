package dto

import (
	"testing"
	"time"

	"pki-certificate-rollover-impact/backend/internal/model"
)

func TestTrustAnchorShortFingerprintNoPanic(t *testing.T) {
	anchor := model.TrustAnchor{AnchorCode: "SHORT-FP", FingerprintSHA256: "abc", NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(1, 0, 0)}
	response := NewTrustAnchorResponse(anchor, 0, time.Now().UTC())
	if len(response.PemRedacted) == 0 {
		t.Fatal("expected a redacted public certificate summary")
	}
}
