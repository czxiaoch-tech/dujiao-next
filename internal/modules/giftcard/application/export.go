package application

import (
	"encoding/csv"
	"strconv"
	"strings"
	"time"

	giftcardcontract "github.com/dujiao-next/internal/modules/giftcard/contract"
	giftcarddomain "github.com/dujiao-next/internal/modules/giftcard/domain"

	"github.com/dujiao-next/internal/constants"
)

type giftCardCodeRevealer interface {
	RevealCode(card *giftcarddomain.GiftCard) (string, error)
}

// Export 导出礼品卡。
func (s *Service) Export(ids []uint, format string) ([]byte, string, error) {
	if s == nil || s.repo == nil {
		return nil, "", giftcardcontract.ErrFetchFailed
	}
	normalizedIDs := normalizeIDs(ids)
	if len(normalizedIDs) == 0 {
		return nil, "", giftcardcontract.ErrInvalid
	}
	normalizedFormat := strings.TrimSpace(strings.ToLower(format))
	if normalizedFormat != constants.ExportFormatCSV && normalizedFormat != constants.ExportFormatTXT {
		return nil, "", giftcardcontract.ErrInvalid
	}

	cards, err := s.repo.ListByIDs(normalizedIDs)
	if err != nil {
		return nil, "", giftcardcontract.ErrFetchFailed
	}
	if len(cards) == 0 {
		return nil, "", giftcardcontract.ErrNotFound
	}

	revealedCodes := make(map[uint]string, len(cards))
	for idx := range cards {
		code, err := revealGiftCardCode(s.repo, &cards[idx])
		if err != nil {
			return nil, "", giftcardcontract.ErrFetchFailed
		}
		revealedCodes[cards[idx].ID] = code
	}

	if normalizedFormat == constants.ExportFormatTXT {
		lines := make([]string, 0, len(cards))
		for _, card := range cards {
			lines = append(lines, revealedCodes[card.ID])
		}
		return []byte(strings.Join(lines, "\n")), "text/plain; charset=utf-8", nil
	}

	builder := &strings.Builder{}
	writer := csv.NewWriter(builder)
	if err := writer.Write([]string{
		"id",
		"batch_no",
		"name",
		"code",
		"amount",
		"currency",
		"redeem_type",
		"product_id",
		"sku_id",
		"status",
		"redeemed_user_id",
		"redeemed_order_id",
		"redeemed_at",
		"expires_at",
		"created_at",
	}); err != nil {
		return nil, "", giftcardcontract.ErrFetchFailed
	}
	for _, card := range cards {
		batchNo := ""
		if card.Batch != nil {
			batchNo = card.Batch.BatchNo
		}
		redeemedUserID := ""
		if card.RedeemedUserID != nil {
			redeemedUserID = strconv.FormatUint(uint64(*card.RedeemedUserID), 10)
		}
		productID := ""
		if card.ProductID != nil {
			productID = strconv.FormatUint(uint64(*card.ProductID), 10)
		}
		skuID := ""
		if card.SKUID != nil {
			skuID = strconv.FormatUint(uint64(*card.SKUID), 10)
		}
		redeemedOrderID := ""
		if card.RedeemedOrderID != nil {
			redeemedOrderID = strconv.FormatUint(uint64(*card.RedeemedOrderID), 10)
		}
		redeemType := strings.TrimSpace(card.RedeemType)
		if redeemType == "" {
			redeemType = "wallet"
		}
		record := []string{
			strconv.FormatUint(uint64(card.ID), 10),
			batchNo,
			card.Name,
			revealedCodes[card.ID],
			card.Amount.String(),
			card.Currency,
			redeemType,
			productID,
			skuID,
			card.Status,
			redeemedUserID,
			redeemedOrderID,
			formatNullableTime(card.RedeemedAt),
			formatNullableTime(card.ExpiresAt),
			card.CreatedAt.Format(time.RFC3339),
		}
		if err := writer.Write(record); err != nil {
			return nil, "", giftcardcontract.ErrFetchFailed
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, "", giftcardcontract.ErrFetchFailed
	}
	return []byte(builder.String()), "text/csv; charset=utf-8", nil
}

func revealGiftCardCode(repo giftcardcontract.Repository, card *giftcarddomain.GiftCard) (string, error) {
	if card == nil {
		return "", giftcardcontract.ErrFetchFailed
	}
	if revealer, ok := repo.(giftCardCodeRevealer); ok {
		return revealer.RevealCode(card)
	}
	if card.CodeHash != nil {
		return "", giftcardcontract.ErrFetchFailed
	}
	return strings.TrimSpace(card.Code), nil
}