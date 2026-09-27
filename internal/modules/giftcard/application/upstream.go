package application

import (
	"strings"
	"time"

	giftcardcontract "github.com/dujiao-next/internal/modules/giftcard/contract"
)

const wanghahaPlusUpstreamBatchNo = "UPSTREAM-WANGHAHA-PLUS"

type upstreamProductCodeImporter interface {
	ImportProductCodesToBatch(batchNo, name string, productID, skuID uint, codes []string, now time.Time) (int, error)
}

// ImportWanghahaPlusCodes 把王哈哈 Plus CDK 导入内部加密池。
// 明文只在本次请求内存中存在；响应只返回数量。
func (s *Service) ImportWanghahaPlusCodes(codes []string) (int, error) {
	if s == nil || s.repo == nil || s.products == nil || s.skus == nil {
		return 0, giftcardcontract.ErrCreateFailed
	}
	clean := make([]string, 0, len(codes))
	for _, raw := range codes {
		if value := strings.TrimSpace(raw); value != "" {
			clean = append(clean, value)
		}
	}
	if len(clean) == 0 || len(clean) > 1000 {
		return 0, giftcardcontract.ErrInvalid
	}

	product, err := s.products.GetBySlug("chatgpt-plus", true)
	if err != nil {
		return 0, giftcardcontract.ErrFetchFailed
	}
	if product == nil {
		return 0, giftcardcontract.ErrInvalid
	}
	sku, err := s.skus.GetByProductAndCode(product.ID, "PLUS")
	if err != nil {
		return 0, giftcardcontract.ErrFetchFailed
	}
	if sku == nil || !sku.IsActive {
		return 0, giftcardcontract.ErrInvalid
	}

	importer, ok := s.repo.(upstreamProductCodeImporter)
	if !ok {
		return 0, giftcardcontract.ErrCreateFailed
	}
	imported, err := importer.ImportProductCodesToBatch(
		wanghahaPlusUpstreamBatchNo,
		"Wanghaha Plus Upstream CDK",
		product.ID,
		sku.ID,
		clean,
		time.Now(),
	)
	if err != nil {
		return 0, giftcardcontract.ErrCreateFailed
	}
	return imported, nil
}
