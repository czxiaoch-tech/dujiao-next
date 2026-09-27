package application

import (
	"strings"

	giftcardcontract "github.com/dujiao-next/internal/modules/giftcard/contract"
	giftcarddomain "github.com/dujiao-next/internal/modules/giftcard/domain"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

// GenerateMirrorSimulationCode 为镜像实验生成一张新的 Pro20X 本地产品码。
// 仅由显式 simulation 模式调用；返回码只用于当前实验页面。
func (s *Service) GenerateMirrorSimulationCode() (string, error) {
	if s == nil || s.repo == nil || s.products == nil || s.skus == nil {
		return "", giftcardcontract.ErrCreateFailed
	}
	product, err := s.products.GetBySlug("chatgpt-pro-20x", true)
	if err != nil || product == nil {
		return "", giftcardcontract.ErrFetchFailed
	}
	sku, err := s.skus.GetByProductAndCode(product.ID, "PRO20X")
	if err != nil || sku == nil || !sku.IsActive {
		return "", giftcardcontract.ErrFetchFailed
	}

	batch, _, err := s.Generate(GenerateInput{
		Name:       "Mirror Simulation Pro20X",
		Quantity:   1,
		Amount:     money.FromDecimal(decimal.Zero),
		RedeemType: giftcarddomain.GiftCardRedeemTypeProduct,
		ProductID:  product.ID,
		SKUID:      sku.ID,
	})
	if err != nil || batch == nil {
		return "", giftcardcontract.ErrCreateFailed
	}

	cards, _, err := s.repo.List(giftcardcontract.ListFilter{
		BatchNo:  strings.TrimSpace(batch.BatchNo),
		Page:     1,
		PageSize: 1,
	})
	if err != nil || len(cards) == 0 {
		return "", giftcardcontract.ErrFetchFailed
	}
	code, err := s.repo.RevealCode(&cards[0])
	if err != nil || strings.TrimSpace(code) == "" {
		return "", giftcardcontract.ErrFetchFailed
	}
	return strings.TrimSpace(code), nil
}
