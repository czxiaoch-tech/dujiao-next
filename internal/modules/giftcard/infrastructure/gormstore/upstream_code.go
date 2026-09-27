package gormstore

import (
	"errors"
	"strings"
	"time"

	giftcarddomain "github.com/dujiao-next/internal/modules/giftcard/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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
