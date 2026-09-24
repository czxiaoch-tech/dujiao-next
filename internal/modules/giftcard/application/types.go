package application

import (
	"time"

	giftcarddomain "github.com/dujiao-next/internal/modules/giftcard/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
)

// GenerateInput 生成礼品卡输入。
type GenerateInput struct {
	Name       string
	Quantity   int
	Amount     money.Amount
	RedeemType string
	ProductID  uint
	SKUID      uint
	ExpiresAt  *time.Time
	CreatedBy  *uint
}

// ListInput 礼品卡列表输入。
type ListInput struct {
	Code           string
	Status         string
	BatchNo        string
	RedeemedUserID uint
	CreatedFrom    *time.Time
	CreatedTo      *time.Time
	RedeemedFrom   *time.Time
	RedeemedTo     *time.Time
	ExpiresFrom    *time.Time
	ExpiresTo      *time.Time
	Page           int
	PageSize       int
}

// UpdateInput 礼品卡更新输入。
type UpdateInput struct {
	Name           *string
	Status         *string
	ExpiresAt      *time.Time
	ClearExpiresAt bool
}

// RedeemInput 礼品卡兑换输入。
type RedeemInput struct {
	UserID         uint
	Code           string
	ManualFormData jsonmap.JSON
}

type ResolveResult struct {
	RedeemType       string
	Name             string
	Amount           money.Amount
	Currency         string
	ProductID        uint
	SKUID            uint
	ProductTitle     jsonmap.JSON
	SKUSnapshot      jsonmap.JSON
	ManualFormSchema jsonmap.JSON
}

type RedeemResult struct {
	Card        *giftcarddomain.GiftCard
	Wallet      *walletdomain.Account
	Transaction *walletdomain.Transaction
	Order       *orderdomain.Order
}
