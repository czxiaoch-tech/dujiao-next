package giftcardhttp

import (
	"testing"
	"time"

	giftcardapp "github.com/dujiao-next/internal/modules/giftcard/application"
	giftcarddomain "github.com/dujiao-next/internal/modules/giftcard/domain"
)

func TestGenerateRequestMapsToGenerateInput(t *testing.T) {
	name := "春节礼品卡"
	expires := time.Date(2026, 12, 31, 15, 4, 5, 0, time.UTC)
	req := generateRequest{
		Name:      name,
		Quantity:  3,
		Amount:    "10.50",
		ExpiresAt: expires.Format(time.RFC3339),
	}
	if req.Name != name || req.Quantity != 3 || req.Amount != "10.50" {
		t.Fatalf("generate request fields mismatch: %#v", req)
	}
	if _, err := time.Parse(time.RFC3339, req.ExpiresAt); err != nil {
		t.Fatalf("expires_at parse: %v", err)
	}
	_ = giftcardapp.GenerateInput{}
}

func TestUpdateRequestClearExpiresAtConvention(t *testing.T) {
	empty := ""
	req := updateRequest{ExpiresAt: &empty}
	if req.ExpiresAt == nil || *req.ExpiresAt != "" {
		t.Fatalf("expected empty expires_at to signal clear")
	}
}

func TestMaskAdminGiftCardMasksOnlyProductRedemptionCodes(t *testing.T) {
	product := giftcarddomain.GiftCard{
		Code:       "GC2609251234560000ABCDEF1234",
		RedeemType: giftcarddomain.GiftCardRedeemTypeProduct,
	}
	masked := maskAdminGiftCard(product)
	if masked.Code == product.Code {
		t.Fatal("product redemption code must be masked in admin response")
	}
	if masked.Code[:6] != product.Code[:6] || masked.Code[len(masked.Code)-4:] != product.Code[len(product.Code)-4:] {
		t.Fatalf("unexpected product code mask: %q", masked.Code)
	}

	wallet := giftcarddomain.GiftCard{
		Code:       "GC-WALLET-001",
		RedeemType: giftcarddomain.GiftCardRedeemTypeWallet,
	}
	if got := maskAdminGiftCard(wallet); got.Code != wallet.Code {
		t.Fatalf("wallet gift card changed outside V0.1 scope: %q", got.Code)
	}
}
