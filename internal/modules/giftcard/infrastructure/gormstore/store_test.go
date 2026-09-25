package gormstore_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	giftcardcontract "github.com/dujiao-next/internal/modules/giftcard/contract"
	giftcarddomain "github.com/dujiao-next/internal/modules/giftcard/domain"
	"github.com/dujiao-next/internal/modules/giftcard/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func newGiftCardStoreTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:giftcard_store_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&giftcarddomain.GiftCardBatch{}, &giftcarddomain.GiftCard{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestGiftCardStoreSoftDeleteHidesCardFromEveryReadAndWritePath(t *testing.T) {
	db := newGiftCardStoreTestDB(t)
	store := gormstore.New(db)
	batch := &giftcarddomain.GiftCardBatch{
		BatchNo:  "GCB-DELETE",
		Name:     "待删除批次",
		Amount:   money.FromDecimal(decimal.NewFromInt(10)),
		Currency: "CNY",
		Quantity: 1,
	}
	cards := []giftcarddomain.GiftCard{{
		Name:     "待删除礼品卡",
		Code:     "GC-DELETE-001",
		Amount:   money.FromDecimal(decimal.NewFromInt(10)),
		Currency: "CNY",
		Status:   giftcarddomain.GiftCardStatusActive,
	}}
	if err := store.CreateBatch(batch, cards); err != nil {
		t.Fatalf("create batch: %v", err)
	}
	var card giftcarddomain.GiftCard
	if err := db.Where("batch_id = ?", batch.ID).First(&card).Error; err != nil {
		t.Fatalf("load created card: %v", err)
	}

	if err := store.Delete(card.ID); err != nil {
		t.Fatalf("delete card: %v", err)
	}
	if got, err := store.GetByID(card.ID); err != nil || got != nil {
		t.Fatalf("GetByID after delete = (%v, %v), want (nil, nil)", got, err)
	}
	if got, err := store.GetByCodeForUpdate(card.Code); err != nil || got != nil {
		t.Fatalf("GetByCodeForUpdate after delete = (%v, %v), want (nil, nil)", got, err)
	}
	if got, err := store.ListByIDs([]uint{card.ID}); err != nil || len(got) != 0 {
		t.Fatalf("ListByIDs after delete = (%v, %v), want empty", got, err)
	}
	if got, total, err := store.List(giftcardcontract.ListFilter{}); err != nil || total != 0 || len(got) != 0 {
		t.Fatalf("List after delete = (%v, %d, %v), want empty", got, total, err)
	}
	if affected, err := store.BatchUpdateStatus([]uint{card.ID}, giftcarddomain.GiftCardStatusDisabled, time.Now()); err != nil || affected != 0 {
		t.Fatalf("BatchUpdateStatus after delete = (%d, %v), want zero", affected, err)
	}

	var raw giftcarddomain.GiftCard
	if err := db.First(&raw, card.ID).Error; err != nil {
		t.Fatalf("load raw deleted card: %v", err)
	}
	if raw.DeletedAt == nil {
		t.Fatal("deleted card must retain a non-nil deleted_at marker")
	}
	if raw.Status != giftcarddomain.GiftCardStatusActive {
		t.Fatalf("deleted card status changed to %s", raw.Status)
	}
}

func TestGiftCardStoreEncryptsProductCodesAtRest(t *testing.T) {
	db := newGiftCardStoreTestDB(t)
	store := gormstore.New(db, "product-code-test-secret-32-bytes")
	batch := &giftcarddomain.GiftCardBatch{
		BatchNo:  "GCB-SECURE",
		Name:     "产品兑换码安全批次",
		Amount:   money.FromDecimal(decimal.Zero),
		Currency: "CNY",
		Quantity: 1,
	}
	plaintext := "GC-SECURE-PRODUCT-001"
	cards := []giftcarddomain.GiftCard{{
		Name:       "产品兑换码",
		Code:       plaintext,
		Amount:     money.FromDecimal(decimal.Zero),
		Currency:   "CNY",
		RedeemType: giftcarddomain.GiftCardRedeemTypeProduct,
		Status:     giftcarddomain.GiftCardStatusActive,
	}}
	if err := store.CreateBatch(batch, cards); err != nil {
		t.Fatalf("create secure batch: %v", err)
	}

	var raw giftcarddomain.GiftCard
	if err := db.Where("batch_id = ?", batch.ID).First(&raw).Error; err != nil {
		t.Fatalf("load raw product code: %v", err)
	}
	if raw.Code == plaintext || !strings.HasPrefix(raw.Code, "gcv1:") {
		t.Fatalf("product code was not encrypted at rest: %q", raw.Code)
	}
	if raw.CodeHash == nil || strings.TrimSpace(*raw.CodeHash) == "" {
		t.Fatal("product code lookup hash was not persisted")
	}

	got, err := store.GetByCode(strings.ToLower(plaintext))
	if err != nil {
		t.Fatalf("lookup secure product code: %v", err)
	}
	if got == nil || got.Code != plaintext {
		t.Fatalf("automatic decrypt failed: %+v", got)
	}
}

func TestGiftCardStoreLeavesWalletCodesUnchangedInV01(t *testing.T) {
	db := newGiftCardStoreTestDB(t)
	store := gormstore.New(db, "product-code-test-secret-32-bytes")
	batch := &giftcarddomain.GiftCardBatch{
		BatchNo:  "GCB-WALLET",
		Name:     "余额礼品卡批次",
		Amount:   money.FromDecimal(decimal.NewFromInt(10)),
		Currency: "CNY",
		Quantity: 1,
	}
	plaintext := "GC-WALLET-001"
	cards := []giftcarddomain.GiftCard{{
		Name:       "余额礼品卡",
		Code:       plaintext,
		Amount:     money.FromDecimal(decimal.NewFromInt(10)),
		Currency:   "CNY",
		RedeemType: giftcarddomain.GiftCardRedeemTypeWallet,
		Status:     giftcarddomain.GiftCardStatusActive,
	}}
	if err := store.CreateBatch(batch, cards); err != nil {
		t.Fatalf("create wallet batch: %v", err)
	}

	var raw giftcarddomain.GiftCard
	if err := db.Where("batch_id = ?", batch.ID).First(&raw).Error; err != nil {
		t.Fatalf("load raw wallet code: %v", err)
	}
	if raw.Code != plaintext {
		t.Fatalf("wallet code changed outside V0.1 scope: %q", raw.Code)
	}
	if raw.CodeHash != nil {
		t.Fatalf("wallet code should not receive product-code hash: %v", *raw.CodeHash)
	}
}

func TestGiftCardStoreBackfillsLegacyProductCodesAtomically(t *testing.T) {
	db := newGiftCardStoreTestDB(t)
	legacyCode := "GC-LEGACY-PRODUCT-001"
	legacy := giftcarddomain.GiftCard{
		Name:       "历史产品兑换码",
		Code:       legacyCode,
		Amount:     money.FromDecimal(decimal.Zero),
		Currency:   "CNY",
		RedeemType: giftcarddomain.GiftCardRedeemTypeProduct,
		Status:     giftcarddomain.GiftCardStatusActive,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatalf("seed legacy code: %v", err)
	}

	store := gormstore.New(db, "legacy-product-code-secret-32-bytes")
	changed, err := store.BackfillProductCodes()
	if err != nil {
		t.Fatalf("backfill product codes: %v", err)
	}
	if changed != 1 {
		t.Fatalf("expected one migrated code, got %d", changed)
	}

	var raw giftcarddomain.GiftCard
	if err := db.First(&raw, legacy.ID).Error; err != nil {
		t.Fatalf("load migrated raw row: %v", err)
	}
	if raw.Code == legacyCode || !strings.HasPrefix(raw.Code, "gcv1:") {
		t.Fatalf("legacy product code not encrypted: %q", raw.Code)
	}
	if raw.CodeHash == nil || strings.TrimSpace(*raw.CodeHash) == "" {
		t.Fatal("legacy product code hash missing")
	}

	got, err := store.GetByCode(legacyCode)
	if err != nil {
		t.Fatalf("lookup migrated code: %v", err)
	}
	if got == nil || got.Code != legacyCode {
		t.Fatalf("migrated code did not auto-decrypt: %+v", got)
	}

	changedAgain, err := store.BackfillProductCodes()
	if err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	if changedAgain != 0 {
		t.Fatalf("backfill must be idempotent, changed=%d", changedAgain)
	}
}
