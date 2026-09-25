package gormstore

import (
	"errors"
	"strings"
	"time"

	giftcardcontract "github.com/dujiao-next/internal/modules/giftcard/contract"
	giftcarddomain "github.com/dujiao-next/internal/modules/giftcard/domain"
	giftcardsecurity "github.com/dujiao-next/internal/modules/giftcard/security"
	"github.com/dujiao-next/internal/persistence/gormutil"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const listStatusExpired = "expired"

// Store 是礼品卡仓储端口的 GORM 实现。
// codes 仅保护 redeem_type=product 的产品兑换码；余额礼品卡保持原行为，避免扩大 V0.1 范围。
type Store struct {
	db    *gorm.DB
	codes *giftcardsecurity.CodeProtector
}

// New 的 secret 为可选参数，保留既有测试/调用兼容；正式容器会注入 app.secret_key。
func New(db *gorm.DB, secrets ...string) *Store {
	var protector *giftcardsecurity.CodeProtector
	if len(secrets) > 0 && strings.TrimSpace(secrets[0]) != "" {
		protector = giftcardsecurity.NewCodeProtector(secrets[0])
	}
	return &Store{db: db, codes: protector}
}

func (r *Store) WithTx(tx *gorm.DB) *Store {
	if tx == nil {
		return r
	}
	return &Store{db: tx, codes: r.codes}
}

// WithinTransaction 为管理用例提供不暴露 GORM 的事务边界。
func (r *Store) WithinTransaction(fn func(repo giftcardcontract.Repository) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return fn(r.WithTx(tx))
	})
}

// Transaction 为兑换写路径提供可与钱包入账共享的事务。
func (r *Store) Transaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

// CreateBatch 创建礼品卡批次与卡片。
// 产品兑换码只在当前进程内短暂保留明文，写数据库前转换为密文 + HMAC 索引。
func (r *Store) CreateBatch(batch *giftcarddomain.GiftCardBatch, cards []giftcarddomain.GiftCard) error {
	if batch == nil {
		return errors.New("invalid gift card batch")
	}
	if err := r.db.Create(batch).Error; err != nil {
		return err
	}
	if len(cards) == 0 {
		return nil
	}

	storedCards := make([]giftcarddomain.GiftCard, len(cards))
	for idx := range cards {
		cards[idx].BatchID = &batch.ID
		stored, err := r.protectCard(&cards[idx])
		if err != nil {
			return err
		}
		storedCards[idx] = *stored
	}
	return r.db.Create(&storedCards).Error
}

// GetByID 根据 ID 查询礼品卡。
func (r *Store) GetByID(id uint) (*giftcarddomain.GiftCard, error) {
	if id == 0 {
		return nil, nil
	}
	var card giftcarddomain.GiftCard
	if err := r.db.Where("gift_cards.deleted_at IS NULL").
		Preload("Batch", "deleted_at IS NULL").
		First(&card, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if err := r.unprotectCard(&card); err != nil {
		return nil, err
	}
	return &card, nil
}

// GetByCode 根据卡密查询礼品卡，不消耗卡密。
// 产品兑换码优先走 HMAC-SHA256 精确索引；无 hash 的历史行与余额卡走兼容精确匹配。
func (r *Store) GetByCode(code string) (*giftcarddomain.GiftCard, error) {
	return r.getByCode(r.db, code)
}

// GetByCodeForUpdate 根据卡密加锁查询礼品卡。
func (r *Store) GetByCodeForUpdate(code string) (*giftcarddomain.GiftCard, error) {
	return r.getByCode(r.db.Clauses(clause.Locking{Strength: "UPDATE"}), code)
}

func (r *Store) getByCode(query *gorm.DB, code string) (*giftcarddomain.GiftCard, error) {
	code = giftcardsecurity.NormalizeCode(code)
	if code == "" {
		return nil, nil
	}

	query = query.Where("gift_cards.deleted_at IS NULL")
	if r.codes != nil {
		query = query.Where(
			"(gift_cards.code_hash = ? OR ((gift_cards.code_hash IS NULL OR gift_cards.code_hash = '') AND gift_cards.code = ?))",
			r.codes.Hash(code),
			code,
		)
	} else {
		query = query.Where("gift_cards.code = ?", code)
	}

	var card giftcarddomain.GiftCard
	if err := query.First(&card).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if err := r.unprotectCard(&card); err != nil {
		return nil, err
	}
	return &card, nil
}

// List 查询礼品卡列表。
func (r *Store) List(filter giftcardcontract.ListFilter) ([]giftcarddomain.GiftCard, int64, error) {
	query := r.db.Model(&giftcarddomain.GiftCard{}).
		Where("gift_cards.deleted_at IS NULL").
		Preload("Batch", "deleted_at IS NULL")
	if code := giftcardsecurity.NormalizeCode(filter.Code); code != "" {
		if r.codes != nil {
			query = query.Where(
				"(gift_cards.code_hash = ? OR ((gift_cards.code_hash IS NULL OR gift_cards.code_hash = '') AND gift_cards.code = ?))",
				r.codes.Hash(code),
				code,
			)
		} else {
			query = query.Where("gift_cards.code LIKE ?", "%"+code+"%")
		}
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		now := time.Now()
		switch status {
		case listStatusExpired:
			query = query.Where("status = ? AND expires_at IS NOT NULL AND expires_at < ?", giftcarddomain.GiftCardStatusActive, now)
		case giftcarddomain.GiftCardStatusActive:
			query = query.Where("status = ? AND (expires_at IS NULL OR expires_at >= ?)", giftcarddomain.GiftCardStatusActive, now)
		default:
			query = query.Where("status = ?", status)
		}
	}
	if batchNo := strings.TrimSpace(strings.ToUpper(filter.BatchNo)); batchNo != "" {
		query = query.Joins("LEFT JOIN gift_card_batches ON gift_card_batches.id = gift_cards.batch_id").
			Where("gift_card_batches.deleted_at IS NULL AND gift_card_batches.batch_no LIKE ?", "%"+batchNo+"%")
	}
	if filter.RedeemedUserID > 0 {
		query = query.Where("redeemed_user_id = ?", filter.RedeemedUserID)
	}
	if filter.CreatedFrom != nil {
		query = query.Where("created_at >= ?", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		query = query.Where("created_at <= ?", *filter.CreatedTo)
	}
	if filter.RedeemedFrom != nil {
		query = query.Where("redeemed_at >= ?", *filter.RedeemedFrom)
	}
	if filter.RedeemedTo != nil {
		query = query.Where("redeemed_at <= ?", *filter.RedeemedTo)
	}
	if filter.ExpiresFrom != nil {
		query = query.Where("expires_at >= ?", *filter.ExpiresFrom)
	}
	if filter.ExpiresTo != nil {
		query = query.Where("expires_at <= ?", *filter.ExpiresTo)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query = gormutil.ApplyPagination(query, filter.Page, filter.PageSize)

	var cards []giftcarddomain.GiftCard
	if err := query.Order("id desc").Find(&cards).Error; err != nil {
		return nil, 0, err
	}
	if err := r.unprotectCards(cards); err != nil {
		return nil, 0, err
	}
	return cards, total, nil
}

// ListByIDs 按 ID 列表查询礼品卡。
// 显式导出会走这里，因此应用层仍可按权限自动取得明文，再交给导出流程。
func (r *Store) ListByIDs(ids []uint) ([]giftcarddomain.GiftCard, error) {
	if len(ids) == 0 {
		return []giftcarddomain.GiftCard{}, nil
	}
	var cards []giftcarddomain.GiftCard
	if err := r.db.Where("gift_cards.deleted_at IS NULL AND id IN ?", ids).
		Preload("Batch", "deleted_at IS NULL").
		Order("id asc").Find(&cards).Error; err != nil {
		return nil, err
	}
	if err := r.unprotectCards(cards); err != nil {
		return nil, err
	}
	return cards, nil
}

// Update 更新礼品卡。
// product 类型在 Save 前重新加密，避免已解密的运行时对象被明文写回数据库。
func (r *Store) Update(card *giftcarddomain.GiftCard) error {
	if card == nil {
		return errors.New("invalid gift card")
	}
	stored, err := r.protectCard(card)
	if err != nil {
		return err
	}
	return r.db.Save(stored).Error
}

// Delete 删除礼品卡。
func (r *Store) Delete(id uint) error {
	if id == 0 {
		return nil
	}
	return r.db.Model(&giftcarddomain.GiftCard{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", time.Now()).Error
}

// BatchUpdateStatus 批量更新礼品卡状态。
func (r *Store) BatchUpdateStatus(ids []uint, status string, updatedAt time.Time) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}
	result := r.db.Model(&giftcarddomain.GiftCard{}).
		Where("deleted_at IS NULL AND id IN ? AND status <> ?", ids, giftcarddomain.GiftCardStatusRedeemed).
		Updates(map[string]interface{}{
			"status":     strings.TrimSpace(status),
			"updated_at": updatedAt,
		})
	return result.RowsAffected, result.Error
}

// BackfillProductCodes 把 V0.1 上线前已存在的产品兑换码原地迁移为密文 + hash。
// 整批在同一数据库事务内完成；任何一条失败都会回滚，不留下半迁移状态。
func (r *Store) BackfillProductCodes() (int64, error) {
	if r == nil || r.db == nil || r.codes == nil {
		return 0, nil
	}

	var changed int64
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var rows []giftcarddomain.GiftCard
		if err := tx.Select("id", "redeem_type", "code", "code_hash").
			Where("redeem_type = ?", giftcarddomain.GiftCardRedeemTypeProduct).
			Find(&rows).Error; err != nil {
			return err
		}

		for idx := range rows {
			row := &rows[idx]
			plain, err := r.codes.Decrypt(row.Code)
			if err != nil {
				return err
			}
			expectedHash := r.codes.Hash(plain)
			currentHash := ""
			if row.CodeHash != nil {
				currentHash = strings.TrimSpace(*row.CodeHash)
			}
			needsCipher := !r.codes.IsEncrypted(row.Code)
			if !needsCipher && currentHash == expectedHash {
				continue
			}

			storedCode := row.Code
			if needsCipher {
				storedCode, err = r.codes.Encrypt(plain)
				if err != nil {
					return err
				}
			}
			if err := tx.Model(&giftcarddomain.GiftCard{}).
				Where("id = ?", row.ID).
				UpdateColumns(map[string]interface{}{
					"code":      storedCode,
					"code_hash": expectedHash,
				}).Error; err != nil {
				return err
			}
			changed++
		}

		return tx.Exec(
			"CREATE UNIQUE INDEX IF NOT EXISTS idx_gift_cards_product_code_hash_unique " +
				"ON gift_cards(code_hash) " +
				"WHERE redeem_type = 'product' AND code_hash IS NOT NULL AND code_hash <> ''",
		).Error
	})
	return changed, err
}

func (r *Store) protectCard(card *giftcarddomain.GiftCard) (*giftcarddomain.GiftCard, error) {
	if card == nil {
		return nil, errors.New("invalid gift card")
	}
	stored := *card
	if r.codes == nil ||
		!strings.EqualFold(strings.TrimSpace(card.RedeemType), giftcarddomain.GiftCardRedeemTypeProduct) {
		return &stored, nil
	}

	plain, err := r.codes.Decrypt(card.Code)
	if err != nil {
		return nil, err
	}
	ciphertext, err := r.codes.Encrypt(plain)
	if err != nil {
		return nil, err
	}
	hash := r.codes.Hash(plain)
	stored.Code = ciphertext
	stored.CodeHash = &hash
	return &stored, nil
}

func (r *Store) unprotectCard(card *giftcarddomain.GiftCard) error {
	if card == nil || r.codes == nil || !r.codes.IsEncrypted(card.Code) {
		return nil
	}
	plaintext, err := r.codes.Decrypt(card.Code)
	if err != nil {
		return err
	}
	card.Code = plaintext
	return nil
}

func (r *Store) unprotectCards(cards []giftcarddomain.GiftCard) error {
	for idx := range cards {
		if err := r.unprotectCard(&cards[idx]); err != nil {
			return err
		}
	}
	return nil
}

var _ giftcardcontract.Repository = (*Store)(nil)
