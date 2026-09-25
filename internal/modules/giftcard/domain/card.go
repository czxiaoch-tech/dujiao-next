package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/money"
)

const (
	GiftCardStatusActive   = "active"
	GiftCardStatusRedeemed = "redeemed"
	GiftCardStatusDisabled = "disabled"
)

const (
	GiftCardRedeemTypeWallet  = "wallet"
	GiftCardRedeemTypeProduct = "product"
)

// GiftCard 礼品卡
type GiftCard struct {
	ID              uint           `gorm:"primarykey" json:"id"`                                                // 主键
	BatchID         *uint          `gorm:"index" json:"batch_id,omitempty"`                                     // 批次ID
	Name            string         `gorm:"type:varchar(120);not null" json:"name"`                              // 礼品卡名称
	Code            string         `gorm:"type:text;uniqueIndex;not null" json:"-"`                             // 底层卡密存储；产品兑换码落库为密文，不直接输出
	CodeHash        *string        `gorm:"type:varchar(64);uniqueIndex" json:"-"`                               // 产品兑换码 HMAC-SHA256 精确检索值
	CodeMasked      string         `gorm:"type:varchar(80)" json:"code"`                                        // 默认对外仅返回脱敏卡密
	Amount          money.Amount   `gorm:"type:decimal(20,2);not null" json:"amount"`                           // 面额（余额礼品卡有效）
	Currency        string         `gorm:"type:varchar(16);not null;default:'CNY'" json:"currency"`             // 币种
	RedeemType      string         `gorm:"type:varchar(24);index;not null;default:'wallet'" json:"redeem_type"` // 兑换类型（wallet/product）
	ProductID       *uint          `gorm:"index" json:"product_id,omitempty"`                                   // 产品兑换码绑定商品
	SKUID           *uint          `gorm:"column:sku_id;index" json:"sku_id,omitempty"`                         // 产品兑换码绑定 SKU
	RedeemedOrderID *uint          `gorm:"index" json:"redeemed_order_id,omitempty"`                            // 产品兑换后生成订单
	Status          string         `gorm:"type:varchar(24);index;not null;default:'active'" json:"status"`      // 状态
	ExpiresAt       *time.Time     `gorm:"index" json:"expires_at"`                                             // 过期时间
	RedeemedAt      *time.Time     `gorm:"index" json:"redeemed_at"`                                            // 兑换时间
	RedeemedUserID  *uint          `gorm:"index" json:"redeemed_user_id,omitempty"`                             // 兑换用户ID
	WalletTxnID     *uint          `gorm:"index" json:"wallet_txn_id,omitempty"`                                // 钱包流水ID
	CreatedAt       time.Time      `gorm:"index" json:"created_at"`                                             // 创建时间
	UpdatedAt       time.Time      `gorm:"index" json:"updated_at"`                                             // 更新时间
	DeletedAt       *time.Time     `gorm:"index" json:"-"`                                                      // 软删除时间
	Batch           *GiftCardBatch `gorm:"foreignKey:BatchID" json:"batch,omitempty"`                           // 批次信息
}

// TableName 指定表名
func (GiftCard) TableName() string {
	return "gift_cards"
}