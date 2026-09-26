package gormstore_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	giftcardapp "github.com/dujiao-next/internal/modules/giftcard/application"
	giftcarddomain "github.com/dujiao-next/internal/modules/giftcard/domain"
	"github.com/dujiao-next/internal/modules/giftcard/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const secureCodeTestSecret = "test-only-0.2500-gift-card-secret-key"

func newSecureGiftCardDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:giftcard_secure_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&giftcarddomain.GiftCardBatch{}, &giftcarddomain.GiftCard{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestProductCodeEncryptedAtRestHashLookupMaskedAndRevealOnExport(t *testing.T) {
	db := newSecureGiftCardDB(t)
	store := gormstore.New(db, secureCodeTestSecret)
	plaintext := "GC-SECURE-PRODUCT-001"
	batch := &giftcarddomain.GiftCardBatch{
		BatchNo:  "GCB-SECURE-001",
		Name:     "安全产品兑换码",
		Amount:   money.FromDecimal(decimal.Zero),
		Currency: "CNY",
		Quantity: 1,
	}
	cards := []giftcarddomain.GiftCard{{
		Name:       "安全产品兑换码",
		Code:       plaintext,
		Amount:     money.FromDecimal(decimal.Zero),
		Currency:   "CNY",
		RedeemType: giftcarddomain.GiftCardRedeemTypeProduct,
		Status:     giftcarddomain.GiftCardStatusActive,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}}
	if err := store.CreateBatch(batch, cards); err != nil {
		t.Fatalf("create encrypted product code: %v", err)
	}

	var raw giftcarddomain.GiftCard
	if err := db.Where("batch_id = ?", batch.ID).First(&raw).Error; err != nil {
		t.Fatalf("load raw card: %v", err)
	}
	if raw.Code == plaintext || !strings.HasPrefix(raw.Code, "enc:v1:") {
		t.Fatalf("product code must be encrypted at rest, got %q", raw.Code)
	}
	if raw.CodeHash == nil || strings.TrimSpace(*raw.CodeHash) == "" {
		t.Fatal("product code hash must be stored")
	}
	if raw.CodeMasked == "" || raw.CodeMasked == plaintext {
		t.Fatalf("masked code must not equal plaintext, got %q", raw.CodeMasked)
	}

	found, err := store.GetByCode(plaintext)
	if err != nil || found == nil {
		t.Fatalf("hash lookup failed: card=%+v err=%v", found, err)
	}
	if byCiphertext, err := store.GetByCode(raw.Code); err != nil || byCiphertext != nil {
		t.Fatalf("ciphertext must not be accepted as a redemption code: card=%+v err=%v", byCiphertext, err)
	}

	revealed, err := store.RevealCode(found)
	if err != nil {
		t.Fatalf("reveal code: %v", err)
	}
	if revealed != plaintext {
		t.Fatalf("revealed code=%q want=%q", revealed, plaintext)
	}

	payload, err := json.Marshal(found)
	if err != nil {
		t.Fatalf("marshal card: %v", err)
	}
	if strings.Contains(string(payload), plaintext) || strings.Contains(string(payload), raw.Code) {
		t.Fatalf("serialized card leaked sensitive code: %s", payload)
	}
	if !strings.Contains(string(payload), raw.CodeMasked) {
		t.Fatalf("serialized card must contain masked code: %s", payload)
	}

	service := giftcardapp.NewService(giftcardapp.Options{Repo: store})
	exported, contentType, err := service.Export([]uint{raw.ID}, "txt")
	if err != nil {
		t.Fatalf("export encrypted product code: %v", err)
	}
	if contentType != "text/plain; charset=utf-8" || string(exported) != plaintext {
		t.Fatalf("unexpected export: type=%q content=%q", contentType, exported)
	}
}

func TestMigrateProductCodeEncryptionEncryptsLegacyProductOnly(t *testing.T) {
	db := newSecureGiftCardDB(t)
	productPlaintext := "GC-LEGACY-PRODUCT-001"
	walletPlaintext := "GC-LEGACY-WALLET-001"

	product := giftcarddomain.GiftCard{
		Name:       "历史产品码",
		Code:       productPlaintext,
		Amount:     money.FromDecimal(decimal.Zero),
		Currency:   "CNY",
		RedeemType: giftcarddomain.GiftCardRedeemTypeProduct,
		Status:     giftcarddomain.GiftCardStatusActive,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	wallet := giftcarddomain.GiftCard{
		Name:       "历史余额卡",
		Code:       walletPlaintext,
		Amount:     money.FromDecimal(decimal.NewFromInt(10)),
		Currency:   "CNY",
		RedeemType: giftcarddomain.GiftCardRedeemTypeWallet,
		Status:     giftcarddomain.GiftCardStatusActive,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("seed product: %v", err)
	}
	if err := db.Create(&wallet).Error; err != nil {
		t.Fatalf("seed wallet: %v", err)
	}

	store := gormstore.New(db, secureCodeTestSecret)
	migrated, err := store.MigrateProductCodeEncryption()
	if err != nil {
		t.Fatalf("migrate product codes: %v", err)
	}
	if migrated != 1 {
		t.Fatalf("migrated=%d want=1", migrated)
	}

	var migratedProduct giftcarddomain.GiftCard
	if err := db.First(&migratedProduct, product.ID).Error; err != nil {
		t.Fatalf("reload product: %v", err)
	}
	if migratedProduct.Code == productPlaintext || migratedProduct.CodeHash == nil {
		t.Fatalf("legacy product code was not encrypted: %+v", migratedProduct)
	}
	if found, err := store.GetByCode(productPlaintext); err != nil || found == nil {
		t.Fatalf("migrated product code lookup failed: card=%+v err=%v", found, err)
	}

	var untouchedWallet giftcarddomain.GiftCard
	if err := db.First(&untouchedWallet, wallet.ID).Error; err != nil {
		t.Fatalf("reload wallet: %v", err)
	}
	if untouchedWallet.Code != walletPlaintext || untouchedWallet.CodeHash != nil {
		t.Fatalf("V0.1 must not change wallet-card storage semantics: %+v", untouchedWallet)
	}
	if untouchedWallet.CodeMasked == "" || untouchedWallet.CodeMasked == walletPlaintext {
		t.Fatalf("legacy wallet code must still receive masked presentation: %+v", untouchedWallet)
	}
}
