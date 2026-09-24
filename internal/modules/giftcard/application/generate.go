package application

import (
	"errors"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	giftcardcontract "github.com/dujiao-next/internal/modules/giftcard/contract"
	giftcarddomain "github.com/dujiao-next/internal/modules/giftcard/domain"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// Generate 生成余额礼品卡或产品兑换码批次。
func (s *Service) Generate(input GenerateInput) (*giftcarddomain.GiftCardBatch, int, error) {
	if s == nil || s.repo == nil {
		return nil, 0, giftcardcontract.ErrCreateFailed
	}

	name := strings.TrimSpace(input.Name)
	if name == "" || input.Quantity <= 0 || input.Quantity > 10000 {
		return nil, 0, giftcardcontract.ErrInvalid
	}

	redeemType := normalizeRedeemType(input.RedeemType)
	amount := input.Amount.Decimal.Round(2)
	var productID *uint
	var skuID *uint
	switch redeemType {
	case giftcarddomain.GiftCardRedeemTypeWallet:
		if amount.LessThanOrEqual(decimal.Zero) {
			return nil, 0, giftcardcontract.ErrInvalid
		}
	case giftcarddomain.GiftCardRedeemTypeProduct:
		product, sku, err := s.resolveProductTarget(input.ProductID, input.SKUID)
		if err != nil || product == nil || sku == nil {
			return nil, 0, giftcardcontract.ErrInvalid
		}
		pid := product.ID
		sid := sku.ID
		productID = &pid
		skuID = &sid
		amount = decimal.Zero
	default:
		return nil, 0, giftcardcontract.ErrInvalid
	}

	currency := constants.SiteCurrencyDefault
	if s.currency != nil {
		if value := strings.TrimSpace(s.currency.SiteCurrency()); value != "" {
			currency = value
		}
	}

	now := time.Now()
	batch := &giftcarddomain.GiftCardBatch{
		BatchNo:   generateBatchNo(now),
		Name:      name,
		Amount:    money.FromDecimal(amount),
		Currency:  currency,
		Quantity:  input.Quantity,
		ExpiresAt: normalizeExpireAt(input.ExpiresAt),
		CreatedBy: input.CreatedBy,
		CreatedAt: now,
		UpdatedAt: now,
	}

	cards := make([]giftcarddomain.GiftCard, 0, input.Quantity)
	for i := 0; i < input.Quantity; i++ {
		cards = append(cards, giftcarddomain.GiftCard{
			Name:       name,
			Code:       generateCode(now, i),
			Amount:     money.FromDecimal(amount),
			Currency:   currency,
			RedeemType: redeemType,
			ProductID:  productID,
			SKUID:      skuID,
			Status:     giftcarddomain.GiftCardStatusActive,
			ExpiresAt:  normalizeExpireAt(input.ExpiresAt),
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}

	if err := s.repo.WithinTransaction(func(repo giftcardcontract.Repository) error {
		if err := repo.CreateBatch(batch, cards); err != nil {
			return giftcardcontract.ErrBatchCreateFailed
		}
		return nil
	}); err != nil {
		if errors.Is(err, giftcardcontract.ErrBatchCreateFailed) {
			return nil, 0, giftcardcontract.ErrBatchCreateFailed
		}
		return nil, 0, giftcardcontract.ErrCreateFailed
	}
	return batch, input.Quantity, nil
}
