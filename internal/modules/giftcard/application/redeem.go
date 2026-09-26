package application

import (
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/catalog/product/manualform"
	giftcardcontract "github.com/dujiao-next/internal/modules/giftcard/contract"
	giftcarddomain "github.com/dujiao-next/internal/modules/giftcard/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/shopspring/decimal"
)

// ResolveGiftCard 读取兑换码类型与产品表单，不消耗卡密。
func (s *Service) ResolveGiftCard(code string) (*ResolveResult, error) {
	if s == nil || s.repo == nil {
		return nil, giftcardcontract.ErrFetchFailed
	}
	normalizedCode := strings.TrimSpace(strings.ToUpper(code))
	if normalizedCode == "" {
		return nil, giftcardcontract.ErrInvalid
	}
	card, err := s.repo.GetByCode(normalizedCode)
	if err != nil {
		return nil, giftcardcontract.ErrFetchFailed
	}
	if err := validateCardUsable(card, time.Now()); err != nil {
		return nil, err
	}

	redeemType := normalizeRedeemType(card.RedeemType)
	result := &ResolveResult{
		RedeemType: redeemType,
		Name:       card.Name,
		Amount:     card.Amount,
		Currency:   card.Currency,
	}
	switch redeemType {
	case giftcarddomain.GiftCardRedeemTypeWallet:
		if card.Amount.Decimal.Round(2).LessThanOrEqual(decimal.Zero) {
			return nil, giftcardcontract.ErrInvalid
		}
		return result, nil
	case giftcarddomain.GiftCardRedeemTypeProduct:
		if card.ProductID == nil || *card.ProductID == 0 || card.SKUID == nil || *card.SKUID == 0 {
			return nil, giftcardcontract.ErrInvalid
		}
		product, sku, err := s.resolveProductTarget(*card.ProductID, *card.SKUID)
		if err != nil {
			return nil, err
		}
		schema, err := manualform.NormalizeSchema(product.ManualFormSchemaJSON)
		if err != nil {
			return nil, giftcardcontract.ErrInvalid
		}
		result.ProductID = product.ID
		result.SKUID = sku.ID
		result.ProductTitle = product.TitleJSON
		result.SKUSnapshot = jsonmap.JSON{
			"sku_id":      sku.ID,
			"sku_code":    sku.SKUCode,
			"spec_values": sku.SpecValuesJSON,
		}
		result.ManualFormSchema = schema
		return result, nil
	default:
		return nil, giftcardcontract.ErrInvalid
	}
}

// RedeemCode 为用户中心提供统一兑换：余额卡入钱包，产品码直接生成待人工履约订单。
func (s *Service) RedeemCode(input RedeemInput) (*RedeemResult, error) {
	resolved, err := s.ResolveGiftCard(input.Code)
	if err != nil {
		return nil, err
	}
	switch resolved.RedeemType {
	case giftcarddomain.GiftCardRedeemTypeWallet:
		card, account, txn, err := s.RedeemGiftCard(input)
		if err != nil {
			return nil, err
		}
		return &RedeemResult{Card: card, Wallet: account, Transaction: txn}, nil
	case giftcarddomain.GiftCardRedeemTypeProduct:
		card, order, err := s.redeemProductGiftCard(input)
		if err != nil {
			return nil, err
		}
		return &RedeemResult{Card: card, Order: order}, nil
	default:
		return nil, giftcardcontract.ErrInvalid
	}
}

// RedeemGiftCard 保留原余额礼品卡语义，渠道接口继续只允许余额卡。
func (s *Service) RedeemGiftCard(input RedeemInput) (*giftcarddomain.GiftCard, *walletdomain.Account, *walletdomain.Transaction, error) {
	if s == nil || s.redeemer == nil {
		return nil, nil, nil, giftcardcontract.ErrFetchFailed
	}
	code := strings.TrimSpace(strings.ToUpper(input.Code))
	if input.UserID == 0 || code == "" {
		return nil, nil, nil, giftcardcontract.ErrInvalid
	}

	var (
		resultCard *giftcarddomain.GiftCard
		resultAcc  *walletdomain.Account
		resultTxn  *walletdomain.Transaction
	)
	err := s.redeemer.WithinRedeemTransaction(func(tx giftcardcontract.RedeemTransaction) error {
		card, err := tx.GetByCodeForUpdate(code)
		if err != nil {
			return giftcardcontract.ErrFetchFailed
		}
		if err := validateCardUsable(card, time.Now()); err != nil {
			return err
		}
		if normalizeRedeemType(card.RedeemType) != giftcarddomain.GiftCardRedeemTypeWallet {
			return giftcardcontract.ErrInvalid
		}
		if card.Amount.Decimal.Round(2).LessThanOrEqual(decimal.Zero) {
			return giftcardcontract.ErrInvalid
		}

		now := time.Now()
		account, txn, err := tx.CreditWallet(giftcardcontract.WalletCreditInput{
			UserID:    input.UserID,
			Amount:    card.Amount,
			Currency:  card.Currency,
			TxnType:   constants.WalletTxnTypeGiftCard,
			Reference: fmt.Sprintf("gift_card:%d", card.ID),
			Remark:    fmt.Sprintf("礼品卡兑换：%s", card.CodeMasked),
		})
		if err != nil {
			return err
		}

		card.Status = giftcarddomain.GiftCardStatusRedeemed
		card.RedeemedUserID = &input.UserID
		card.RedeemedAt = &now
		if txn != nil && txn.ID > 0 {
			card.WalletTxnID = &txn.ID
		}
		card.UpdatedAt = now
		if err := tx.UpdateCard(card); err != nil {
			return giftcardcontract.ErrUpdateFailed
		}
		resultCard = card
		resultAcc = account
		resultTxn = txn
		return nil
	})
	if err != nil {
		return nil, nil, nil, err
	}
	return resultCard, resultAcc, resultTxn, nil
}

func (s *Service) redeemProductGiftCard(input RedeemInput) (*giftcarddomain.GiftCard, *orderdomain.Order, error) {
	if s == nil || s.redeemer == nil {
		return nil, nil, giftcardcontract.ErrFetchFailed
	}
	code := strings.TrimSpace(strings.ToUpper(input.Code))
	if input.UserID == 0 || code == "" {
		return nil, nil, giftcardcontract.ErrInvalid
	}

	var (
		resultCard  *giftcarddomain.GiftCard
		resultOrder *orderdomain.Order
	)
	err := s.redeemer.WithinRedeemTransaction(func(tx giftcardcontract.RedeemTransaction) error {
		card, err := tx.GetByCodeForUpdate(code)
		if err != nil {
			return giftcardcontract.ErrFetchFailed
		}
		if err := validateCardUsable(card, time.Now()); err != nil {
			return err
		}
		if normalizeRedeemType(card.RedeemType) != giftcarddomain.GiftCardRedeemTypeProduct ||
			card.ProductID == nil || *card.ProductID == 0 ||
			card.SKUID == nil || *card.SKUID == 0 {
			return giftcardcontract.ErrInvalid
		}

		order, err := tx.CreateProductOrder(giftcardcontract.ProductOrderInput{
			UserID:         input.UserID,
			ProductID:      *card.ProductID,
			SKUID:          *card.SKUID,
			Currency:       card.Currency,
			ManualFormData: input.ManualFormData,
		})
		if err != nil {
			return err
		}

		now := time.Now()
		card.Status = giftcarddomain.GiftCardStatusRedeemed
		card.RedeemedUserID = &input.UserID
		card.RedeemedOrderID = &order.ID
		card.RedeemedAt = &now
		card.UpdatedAt = now
		if err := tx.UpdateCard(card); err != nil {
			return giftcardcontract.ErrUpdateFailed
		}
		resultCard = card
		resultOrder = order
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return resultCard, resultOrder, nil
}

func isGiftCardExpired(expiresAt *time.Time, now time.Time) bool {
	if expiresAt == nil || expiresAt.IsZero() {
		return false
	}
	return expiresAt.Before(now)
}
