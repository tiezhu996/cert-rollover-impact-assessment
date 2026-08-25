package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"pki-certificate-rollover-impact/backend/internal/model"
)

func newCtxTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.TrustAnchor{}, &model.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := db.Create(&model.TrustAnchor{AnchorCode: "CTX-ROOT", SubjectDN: "CN=ctx", SerialNumber: "1", FingerprintSHA256: strings.Repeat("b", 64), KeyAlgorithm: "ECDSA", CertificateState: "valid", PemRedacted: "cert", NotBefore: now.Add(-time.Hour), NotAfter: now.AddDate(1, 0, 0), CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.User{Username: "ctx-admin", DisplayName: "CTX Admin", Team: "PKI Platform", PasswordHash: "hash", Role: "admin", Active: true, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func canceledCtx() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func TestUserQueryRespectsCanceledContext(t *testing.T) {
	db := newCtxTestDB(t)
	repo := NewUserRepository(db)
	if _, err := repo.FindByUsername(canceledCtx(), "ctx-admin"); err == nil {
		t.Fatal("user query must fail when the request context is canceled")
	}
}

func TestAnchorQueryRespectsCanceledContext(t *testing.T) {
	db := newCtxTestDB(t)
	repo := NewTrustAnchorRepository(db)
	if _, err := repo.GetByID(canceledCtx(), 1); err == nil {
		t.Fatal("anchor query must fail when the request context is canceled")
	}
}

func TestTransactionRespectsCanceledContext(t *testing.T) {
	db := newCtxTestDB(t)
	manager := NewTransactionManager(db)
	err := manager.WithinTransaction(canceledCtx(), func(txCtx context.Context) error {
		tx, _ := txCtx.Value(transactionContextKey{}).(*gorm.DB)
		return tx.Exec("SELECT 1").Error
	})
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("transaction must fail with context canceled, got %v", err)
	}
}

func TestAuditListRespectsCanceledContext(t *testing.T) {
	db := newCtxTestDB(t)
	if err := db.Create(&model.AuditLog{RequestID: "r1", ActorID: 1, ActorName: "ctx-admin", ActorRole: "admin", EntityType: "trust_anchor", EntityID: 1, Action: "create", BeforeSnapshot: "{}", AfterSnapshot: "{}", CreatedAt: time.Now().UTC()}).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewAuditRepository(db)
	if _, _, err := repo.List(canceledCtx(), AuditQuery{Page: 1, PageSize: 10}); err == nil {
		t.Fatal("audit list must fail when the request context is canceled")
	}
}

func TestAnchorGetByIDsRespectsCanceledContext(t *testing.T) {
	db := newCtxTestDB(t)
	repo := NewTrustAnchorRepository(db)
	if _, err := repo.GetByIDs(canceledCtx(), []uint{1}); err == nil {
		t.Fatal("anchor GetByIDs must fail when the request context is canceled")
	}
}
