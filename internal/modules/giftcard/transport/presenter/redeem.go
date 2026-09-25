package presenter

import (
	"time"

	giftcardapp "github.com/dujiao-next/internal/modules/giftcard/application"
	giftcarddomain "github.com/dujiao-next/internal/modules/giftcard/domain"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	walletpresenter "github.com/dujiao-next/internal/modules/wallet/transport/presenter"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
)

// GiftCardRedeemResp 保留余额礼品卡的原兼容响应。
type GiftCardRedeemResp struct {
	GiftCard    GiftCardResp                          `json:"gift_card"`
	Wallet      walletpresenter.WalletAccountResp     `json:"wallet"`
	Transaction walletpresenter.WalletTransactionResp `json:"transaction"`
	WalletDelta money.Amount                          `json:"wallet_delta"`
}

type ProductTargetResp struct {
	ProductID        uint         `json:"product_id"`
	SKUID            uint         `json:"sku_id"`
	Title            jsonmap.JSON `json:"title"`
	SKUSnapshot      jsonmap.JSON `json:"sku_snapshot"`
	ManualFormSchema jsonmap.JSON `json:"manual_form_schema"`
}

type GiftCardResolveResp struct {
	RedeemType string             `json:"redeem_type"`
	Name       string             `json:"name"`
	Amount     money.Amount       `json:"amount"`
	Currency   string             `json:"currency"`
	Product    *ProductTargetResp `json:"product,omitempty"`
}

type ProductOrderResp struct {
	ID      uint   `json:"id"`
	OrderNo string `json:"order_no"`
	Status  string `json:"status"`
}

type UserGiftCardRedeemResp struct {
	GiftCard    GiftCardResp                           `json:"gift_card"`
	Wallet      *walletpresenter.WalletAccountResp     `json:"wallet,omitempty"`
	Transaction *walletpresenter.WalletTransactionResp `json:"transaction,omitempty"`
	WalletDelta *money.Amount                          `json:"wallet_delta,omitempty"`
	Order       *ProductOrderResp                      `json:"order,omitempty"`
}

// GiftCardResp 礼品卡响应（兑换后）。
type GiftCardResp struct {
	ID              uint         `json:"id"`
	Name            string       `json:"name"`
	Code            string       `json:"code"`
	Amount          money.Amount `json:"amount"`
	Currency        string       `json:"currency"`
	RedeemType      string       `json:"redeem_type"`
	ProductID       *uint        `json:"product_id,omitempty"`
	SKUID           *uint        `json:"sku_id,omitempty"`
	RedeemedOrderID *uint        `json:"redeemed_order_id,omitempty"`
	Status          string       `json:"status"`
	RedeemedAt      *time.Time   `json:"redeemed_at"`
}

func NewGiftCardResp(c *giftcarddomain.GiftCard) GiftCardResp {
	if c == nil {
		return GiftCardResp{}
	}
	redeemType := c.RedeemType
	if redeemType == "" {
		redeemType = giftcarddomain.GiftCardRedeemTypeWallet
	}
	return GiftCardResp{
		ID:              c.ID,
		Name:            c.Name,
		Code:            c.CodeMasked,
		Amount:          c.Amount,
		Currency:        c.Currency,
		RedeemType:      redeemType,
		ProductID:       c.ProductID,
		SKUID:           c.SKUID,
		RedeemedOrderID: c.RedeemedOrderID,
		Status:          c.Status,
		RedeemedAt:      c.RedeemedAt,
	}
}

func NewGiftCardResolveResp(result *giftcardapp.ResolveResult) GiftCardResolveResp {
	if result == nil {
		return GiftCardResolveResp{}
	}
	resp := GiftCardResolveResp{
		RedeemType: result.RedeemType,
		Name:       result.Name,
		Amount:     result.Amount,
		Currency:   result.Currency,
	}
	if result.RedeemType == giftcarddomain.GiftCardRedeemTypeProduct {
		resp.Product = &ProductTargetResp{
			ProductID:        result.ProductID,
			SKUID:            result.SKUID,
			Title:            result.ProductTitle,
			SKUSnapshot:      result.SKUSnapshot,
			ManualFormSchema: result.ManualFormSchema,
		}
	}
	return resp
}

// NewGiftCardRedeemResp 构造余额礼品卡兼容响应。
func NewGiftCardRedeemResp(card *giftcarddomain.GiftCard, account *walletdomain.Account, txn *walletdomain.Transaction) GiftCardRedeemResp {
	return GiftCardRedeemResp{
		GiftCard:    NewGiftCardResp(card),
		Wallet:      walletpresenter.NewWalletAccountResp(account),
		Transaction: walletpresenter.NewWalletTransactionResp(txn),
		WalletDelta: card.Amount,
	}
}

func NewUserGiftCardRedeemResp(result *giftcardapp.RedeemResult) UserGiftCardRedeemResp {
	if result == nil {
		return UserGiftCardRedeemResp{}
	}
	resp := UserGiftCardRedeemResp{GiftCard: NewGiftCardResp(result.Card)}
	if result.Wallet != nil && result.Transaction != nil && result.Card != nil {
		wallet := walletpresenter.NewWalletAccountResp(result.Wallet)
		transaction := walletpresenter.NewWalletTransactionResp(result.Transaction)
		delta := result.Card.Amount
		resp.Wallet = &wallet
		resp.Transaction = &transaction
		resp.WalletDelta = &delta
	}
	if result.Order != nil {
		resp.Order = &ProductOrderResp{
			ID:      result.Order.ID,
			OrderNo: result.Order.OrderNo,
			Status:  result.Order.Status,
		}
	}
	return resp
}
