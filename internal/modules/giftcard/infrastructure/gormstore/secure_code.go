package gormstore

import (
	"errors"
	"fmt"
	"strings"

	giftcarddomain "github.com/dujiao-next/internal/modules/giftcard/domain"

	"gorm.io/gorm"
)

func (r *Store) prepareCardForStorage(card *giftcarddomain.GiftCard) error {
	if card == nil {
		return errors.New("invalid gift card")
	}
	normalizedCode := normalizeCode(card.Code)
	if normalizedCode == "" {
		return errors.New("gift card code is empty")
	}
	card.CodeMasked = maskCode(normalizedCode)
	if strings.EqualFold(strings.TrimSpace(card.RedeemType), giftcarddomain.GiftCardRedeemTypeProduct) {
		ciphertext, hash, masked, err := r.codec.sealProductCode(normalizedCode)
		if err != nil {
			return err
		}
		card.Code = ciphertext
		card.CodeHash = hash
		card.CodeMasked = masked
		return nil
	}
	card.Code = normalizedCode
	card.CodeHash = nil
	return nil
}

func (r *Store) hydrateCardMask(card *giftcarddomain.GiftCard) {
	if card == nil || strings.TrimSpace(card.CodeMasked) != "" {
		return
	}
	if card.CodeHash == nil || strings.TrimSpace(*card.CodeHash) == "" {
		card.CodeMasked = maskCode(card.Code)
		return
	}
	if r.codec == nil {
		return
	}
	plaintext, err := r.codec.revealProductCode(card.Code, card.CodeHash)
	if err == nil {
		card.CodeMasked = maskCode(plaintext)
	}
}

// RevealCode 只在确实需要明文的业务路径（例如管理员显式导出）中解密。
// 普通列表、查询、兑换匹配都不需要把明文恢复到实体。
func (r *Store) RevealCode(card *giftcarddomain.GiftCard) (string, error) {
	if r == nil || card == nil {
		return "", errors.New("invalid gift card")
	}
	if card.CodeHash == nil || strings.TrimSpace(*card.CodeHash) == "" {
		return normalizeCode(card.Code), nil
	}
	return r.codec.revealProductCode(card.Code, card.CodeHash)
}

// MigrateProductCodeEncryption 将历史产品兑换码从明文原地迁移为：
// AES-256-GCM 密文 + HMAC-SHA256 检索值 + 脱敏展示值。
// 迁移在一个数据库事务内完成，失败则整批回滚。
func (r *Store) MigrateProductCodeEncryption() (int64, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("gift card store is unavailable")
	}
	if r.codec == nil {
		return 0, errors.New("gift card product-code encryption key is unavailable")
	}

	var migrated int64
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var productCards []giftcarddomain.GiftCard
		if err := tx.Where(
			"redeem_type = ? AND (code_hash IS NULL OR code_hash = '')",
			giftcarddomain.GiftCardRedeemTypeProduct,
		).Order("id asc").Find(&productCards).Error; err != nil {
			return err
		}

		for idx := range productCards {
			card := &productCards[idx]
			plaintext := normalizeCode(card.Code)
			if plaintext == "" {
				return fmt.Errorf("gift card %d has empty product code", card.ID)
			}
			ciphertext, hash, masked, err := r.codec.sealProductCode(plaintext)
			if err != nil {
				return fmt.Errorf("encrypt gift card %d: %w", card.ID, err)
			}
			result := tx.Model(&giftcarddomain.GiftCard{}).
				Where("id = ? AND (code_hash IS NULL OR code_hash = '')", card.ID).
				Updates(map[string]interface{}{
					"code":        ciphertext,
					"code_hash":   hash,
					"code_masked": masked,
				})
			if result.Error != nil {
				return result.Error
			}
			migrated += result.RowsAffected
		}

		// V0.1 只加密产品兑换码；其他历史礼品卡仍保持原存储语义，
		// 但补齐脱敏展示值，避免后台列表直接返回完整码。
		var legacyCards []giftcarddomain.GiftCard
		if err := tx.Where(
			"(code_hash IS NULL OR code_hash = '') AND (code_masked IS NULL OR code_masked = '')",
		).Order("id asc").Find(&legacyCards).Error; err != nil {
			return err
		}
		for idx := range legacyCards {
			card := &legacyCards[idx]
			if err := tx.Model(&giftcarddomain.GiftCard{}).
				Where("id = ?", card.ID).
				Update("code_masked", maskCode(card.Code)).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return migrated, err
}
