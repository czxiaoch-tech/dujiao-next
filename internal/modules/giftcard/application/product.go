package application

import (
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	giftcardcontract "github.com/dujiao-next/internal/modules/giftcard/contract"
	giftcarddomain "github.com/dujiao-next/internal/modules/giftcard/domain"
)

func normalizeRedeemType(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return giftcarddomain.GiftCardRedeemTypeWallet
	}
	return value
}

func validateCardUsable(card *giftcarddomain.GiftCard, now time.Time) error {
	if card == nil {
		return giftcardcontract.ErrNotFound
	}
	switch card.Status {
	case giftcarddomain.GiftCardStatusRedeemed:
		return giftcardcontract.ErrRedeemed
	case giftcarddomain.GiftCardStatusDisabled:
		return giftcardcontract.ErrDisabled
	case giftcarddomain.GiftCardStatusActive:
	default:
		return giftcardcontract.ErrInvalid
	}
	if isGiftCardExpired(card.ExpiresAt, now) {
		return giftcardcontract.ErrExpired
	}
	return nil
}

func (s *Service) resolveProductTarget(productID, skuID uint) (*productdomain.Product, *productdomain.ProductSKU, error) {
	if s == nil || s.products == nil || s.skus == nil || productID == 0 {
		return nil, nil, giftcardcontract.ErrInvalid
	}
	product, err := s.products.GetByID(strconv.FormatUint(uint64(productID), 10))
	if err != nil {
		return nil, nil, giftcardcontract.ErrFetchFailed
	}
	if product == nil || !product.IsActive {
		return nil, nil, giftcardcontract.ErrInvalid
	}
	fulfillmentType := strings.ToLower(strings.TrimSpace(product.FulfillmentType))
	if fulfillmentType == "" {
		fulfillmentType = constants.FulfillmentTypeManual
	}
	if fulfillmentType != constants.FulfillmentTypeManual {
		return nil, nil, giftcardcontract.ErrInvalid
	}

	if skuID == 0 {
		active := make([]productdomain.ProductSKU, 0, len(product.SKUs))
		for _, sku := range product.SKUs {
			if sku.IsActive && sku.DeletedAt == nil {
				active = append(active, sku)
			}
		}
		if len(active) != 1 {
			return nil, nil, giftcardcontract.ErrInvalid
		}
		skuID = active[0].ID
	}
	sku, err := s.skus.GetByID(skuID)
	if err != nil {
		return nil, nil, giftcardcontract.ErrFetchFailed
	}
	if sku == nil || sku.ProductID != product.ID || !sku.IsActive || sku.DeletedAt != nil {
		return nil, nil, giftcardcontract.ErrInvalid
	}
	if !sku.PriceAmount.Decimal.Round(2).IsPositive() {
		return nil, nil, giftcardcontract.ErrInvalid
	}
	return product, sku, nil
}

