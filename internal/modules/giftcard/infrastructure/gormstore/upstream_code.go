package gormstore

import (
	"errors"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	giftcarddomain "github.com/dujiao-next/internal/modules/giftcard/domain"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)


// ImportProductCodesToBatch 把外部上游 CDK 加密导入指定内部批次。
// 只返回导入数量，不返回任何明文卡密。
func (r *Store) ImportProductCodesToBatch(batchNo, name string, productID, skuID uint, codes []string, now time.Time) (int, error) {
	if r == nil || r.db == nil || r.codec == nil || productID == 0 || skuID == 0 {
		return 0, errors.New("invalid upstream product-code import")
	}
	batchNo = strings.TrimSpace(strings.ToUpper(batchNo))
	name = strings.TrimSpace(name)
	if batchNo == "" || name == "" {
		return 0, errors.New("invalid upstream product-code batch")
	}
	if now.IsZero() {
		now = time.Now()
	}

	seen := make(map[string]struct{}, len(codes))
	normalized := make([]string, 0, len(codes))
	for _, raw := range codes {
		code := normalizeCode(raw)
		if code == "" {
			continue
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		normalized = append(normalized, code)
	}
	if len(normalized) == 0 || len(normalized) > 1000 {
		return 0, errors.New("invalid upstream product-code count")
	}

	imported := 0
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var batch giftcarddomain.GiftCardBatch
		err := tx.Where("batch_no = ? AND deleted_at IS NULL", batchNo).First(&batch).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			batch = giftcarddomain.GiftCardBatch{
				BatchNo:   batchNo,
				Name:      name,
				Amount:    money.FromDecimal(decimal.Zero),
				Currency:  constants.SiteCurrencyDefault,
				Quantity:  0,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := tx.Create(&batch).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		store := r.WithTx(tx)
		cards := make([]giftcarddomain.GiftCard, 0, len(normalized))
		for _, code := range normalized {
			pid := productID
			sid := skuID
			card := giftcarddomain.GiftCard{
				BatchID:    &batch.ID,
				Name:       name,
				Code:       code,
				Amount:     money.FromDecimal(decimal.Zero),
				Currency:   constants.SiteCurrencyDefault,
				RedeemType: giftcarddomain.GiftCardRedeemTypeProduct,
				ProductID:  &pid,
				SKUID:      &sid,
				Status:     giftcarddomain.GiftCardStatusActive,
				CreatedAt:  now,
				UpdatedAt:  now,
			}
			if err := store.prepareCardForStorage(&card); err != nil {
				return err
			}
			cards = append(cards, card)
		}

		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&cards)
		if result.Error != nil {
			return result.Error
		}
		imported = int(result.RowsAffected)
		if imported > 0 {
			if err := tx.Model(&giftcarddomain.GiftCardBatch{}).
				Where("id = ?", batch.ID).
				Updates(map[string]interface{}{
					"quantity":   gorm.Expr("quantity + ?", imported),
					"updated_at": now,
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return imported, nil
}

// ReserveActiveProductCodeByBatchNo 原子预留指定后台批次中的一张产品码。
// V0.1 用于隐藏上游 CDK；只接受精确 batch_no，不使用模糊匹配。
func (r *Store) ReserveActiveProductCodeByBatchNo(batchNo string, orderID uint, now time.Time) (*giftcarddomain.GiftCard, error) {
	if r == nil || r.db == nil || orderID == 0 {
		return nil, errors.New("invalid upstream product-code reservation")
	}
	batchNo = strings.TrimSpace(strings.ToUpper(batchNo))
	if batchNo == "" {
		return nil, errors.New("upstream product-code batch is empty")
	}
	if now.IsZero() {
		now = time.Now()
	}

	var reserved *giftcarddomain.GiftCard
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var card giftcarddomain.GiftCard
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Joins("JOIN gift_card_batches ON gift_card_batches.id = gift_cards.batch_id").
			Where("gift_cards.deleted_at IS NULL").
			Where("gift_card_batches.deleted_at IS NULL").
			Where("gift_card_batches.batch_no = ?", batchNo).
			Where("gift_cards.redeem_type = ?", giftcarddomain.GiftCardRedeemTypeProduct).
			Where("gift_cards.status = ?", giftcarddomain.GiftCardStatusActive).
			Where("(gift_cards.expires_at IS NULL OR gift_cards.expires_at >= ?)", now).
			Order("gift_cards.id ASC").
			First(&card).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}

		result := tx.Model(&giftcarddomain.GiftCard{}).
			Where("id = ? AND status = ?", card.ID, giftcarddomain.GiftCardStatusActive).
			Updates(map[string]interface{}{
				"status":              giftcarddomain.GiftCardStatusReserved,
				"redeemed_order_id":   orderID,
				"updated_at":          now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("upstream product-code reservation lost")
		}

		card.Status = giftcarddomain.GiftCardStatusReserved
		card.RedeemedOrderID = &orderID
		card.UpdatedAt = now
		reserved = &card
		return nil
	})
	if err != nil {
		return nil, err
	}
	return reserved, nil
}

// CompleteReservedProductCode 在上游明确成功后把预留码标记为已使用。
// 模糊失败/超时不调用本方法，卡保持 reserved，避免重复消费。
func (r *Store) CompleteReservedProductCode(cardID, orderID uint, now time.Time) error {
	if r == nil || r.db == nil || cardID == 0 || orderID == 0 {
		return errors.New("invalid upstream product-code completion")
	}
	if now.IsZero() {
		now = time.Now()
	}
	result := r.db.Model(&giftcarddomain.GiftCard{}).
		Where("id = ? AND status = ? AND redeemed_order_id = ?",
			cardID, giftcarddomain.GiftCardStatusReserved, orderID).
		Updates(map[string]interface{}{
			"status":      giftcarddomain.GiftCardStatusRedeemed,
			"redeemed_at": now,
			"updated_at":  now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("upstream product-code completion lost")
	}
	return nil
}
