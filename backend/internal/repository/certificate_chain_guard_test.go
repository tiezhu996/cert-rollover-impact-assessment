package repository

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"pki-certificate-rollover-impact/backend/internal/dto"
	"pki-certificate-rollover-impact/backend/internal/model"
)

func TestChainTransitionRejectsStaleFrom(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.CertificateChain{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	chain := model.CertificateChain{ChainCode: "GUARD-CHAIN", TrustAnchorID: 1, LeafSubject: "CN=leaf", CertificateRefsJSON: "[]", ChainFingerprint: "fp", ValidFrom: now.Add(-time.Hour), ValidTo: now.AddDate(1, 0, 0), ValidationResult: `{"valid":true}`, ChainState: "revoked", SourceChecksum: "sc", PublicChainPEM: "pem", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&chain).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewCertificateChainRepository(db)
	changed, err := repo.Transition(context.Background(), chain.ID, "validated", "deprecated")
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("transition from a stale source state must not succeed")
	}
}

func TestChainListFiltersByState(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.CertificateChain{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	chains := []model.CertificateChain{
		{ChainCode: "LIST-A", TrustAnchorID: 1, LeafSubject: "CN=a", CertificateRefsJSON: "[]", ChainFingerprint: "fp-a", ValidFrom: now.Add(-time.Hour), ValidTo: now.AddDate(1, 0, 0), ValidationResult: `{"valid":true}`, ChainState: "validated", SourceChecksum: "sc-a", PublicChainPEM: "pem", CreatedAt: now, UpdatedAt: now},
		{ChainCode: "LIST-B", TrustAnchorID: 1, LeafSubject: "CN=b", CertificateRefsJSON: "[]", ChainFingerprint: "fp-b", ValidFrom: now.Add(-time.Hour), ValidTo: now.AddDate(1, 0, 0), ValidationResult: `{"valid":true}`, ChainState: "deprecated", SourceChecksum: "sc-b", PublicChainPEM: "pem", CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&chains).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewCertificateChainRepository(db)
	items, _, err := repo.List(context.Background(), dto.CertificateChainQuery{State: "deprecated", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.ChainState != "deprecated" {
			t.Fatalf("state filter must return only matching chains, got %s", item.ChainState)
		}
	}
	if len(items) != 1 {
		t.Fatalf("expected exactly one deprecated chain, got %d", len(items))
	}
}
