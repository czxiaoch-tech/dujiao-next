package gormuow

import (
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	"github.com/dujiao-next/internal/modules/catalog/product/manualform"
	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"
	giftcardcontract "github.com/dujiao-next/internal/modules/giftcard/contract"
	giftcarddomain "github.com/dujiao-next/internal/modules/giftcard/domain"
	"github.com/dujiao-next/internal/modules/giftcard/infrastructure/gormstore"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	walletapp "github.com/dujiao-next/internal/modules/wallet/application"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	walletgormstore "github.com/dujiao-next/internal/modules/wallet/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/dujiao-next/internal/shared/serial"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Runner 将礼品卡状态、钱包入账或产品订单创建绑定到同一个 GORM 事务。
type Runner struct {
	cards    *gormstore.Store
	wallet   *walletapp.Service
	products *productgormstore.ProductStore
	skus     *productgormstore.SKUStore
}

// New 保留原余额礼品卡构造方式，避免影响既有调用与渠道兑换。
func New(cards *gormstore.Store, wallet *walletapp.Service) *Runner {
	return &Runner{cards: cards, wallet: wallet}
}

// NewWithProducts 为产品兑换码额外注入商品与 SKU 仓储。
func NewWithProducts(
	cards *gormstore.Store,
	wallet *walletapp.Service,
	products *productgormstore.ProductStore,
	skus *productgormstore.SKUStore,
) *Runner {
	return &Runner{cards: cards, wallet: wallet, products: products, skus: skus}
}

func (r *Runner) WithinRedeemTransaction(fn func(tx giftcardcontract.RedeemTransaction) error) error {
	if r == nil || r.cards == nil || r.wallet == nil {
		return giftcardcontract.ErrFetchFailed
	}
	return r.cards.Transaction(func(tx *gorm.DB) error {
		return fn(&transaction{
			db:       tx,
			cards:    r.cards.WithTx(tx),
			wallet:   r.wallet,
			products: r.products.BindTx(tx),
			skus:     r.skus.BindTx(tx),
		})
	})
}

type transaction struct {
	db       *gorm.DB
	cards    *gormstore.Store
	wallet   *walletapp.Service
	products productcontract.Repository
	skus     productcontract.SKURepository
}

func (tx *transaction) GetByCodeForUpdate(code string) (*giftcarddomain.GiftCard, error) {
	return tx.cards.GetByCodeForUpdate(code)
}

func (tx *transaction) UpdateCard(card *giftcarddomain.GiftCard) error {
	return tx.cards.Update(card)
}

func (tx *transaction) CreditWallet(input giftcardcontract.WalletCreditInput) (*walletdomain.Account, *walletdomain.Transaction, error) {
	return tx.wallet.CreditInTransaction(walletgormstore.UseTransaction(tx.db), walletcontract.CreditInput{
		UserID:    input.UserID,
		Amount:    input.Amount,
		Currency:  input.Currency,
		Type:      input.TxnType,
		Reference: input.Reference,
		Remark:    input.Remark,
		OrderID:   input.OrderID,
	})
}

// CreateProductOrder 将已在外部渠道付款的产品兑换码直接转为 Dujiao 人工履约订单。
// 产品码本身就是预付凭证，因此这里不再创建 Dujiao 支付单，也不再次扣钱包。
func (tx *transaction) CreateProductOrder(input giftcardcontract.ProductOrderInput) (*orderdomain.Order, error) {
	if tx == nil || tx.db == nil || tx.products == nil || tx.skus == nil ||
		input.UserID == 0 || input.ProductID == 0 || input.SKUID == 0 {
		return nil, giftcardcontract.ErrInvalid
	}

	product, err := tx.products.GetByID(strconv.FormatUint(uint64(input.ProductID), 10))
	if err != nil {
		return nil, giftcardcontract.ErrFetchFailed
	}
	if product == nil || !product.IsActive {
		return nil, giftcardcontract.ErrInvalid
	}
	fulfillmentType := strings.ToLower(strings.TrimSpace(product.FulfillmentType))
	if fulfillmentType == "" {
		fulfillmentType = constants.FulfillmentTypeManual
	}
	if fulfillmentType != constants.FulfillmentTypeManual {
		return nil, giftcardcontract.ErrInvalid
	}

	sku, err := tx.skus.GetByID(input.SKUID)
	if err != nil {
		return nil, giftcardcontract.ErrFetchFailed
	}
	if sku == nil || sku.ProductID != product.ID || !sku.IsActive || sku.DeletedAt != nil {
		return nil, giftcardcontract.ErrInvalid
	}

	normalizedSchema, normalizedSubmission, err := manualform.ValidateAndNormalize(
		product.ManualFormSchemaJSON,
		input.ManualFormData,
	)
	if err != nil {
		return nil, giftcardcontract.ErrInvalid
	}

	price := sku.PriceAmount.Decimal.Round(2)
	if !price.IsPositive() {
		return nil, giftcardcontract.ErrInvalid
	}
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if currency == "" {
		currency = constants.SiteCurrencyDefault
	}

	now := time.Now()
	amount := money.FromDecimal(price)
	zero := money.FromDecimal(decimal.Zero)
	order := &orderdomain.Order{
		OrderNo:                 serial.Generate("DJ"),
		UserID:                  input.UserID,
		Status:                  constants.OrderStatusFulfilling,
		Currency:                currency,
		OriginalAmount:          amount,
		DiscountAmount:          zero,
		MemberDiscountAmount:    zero,
		PromotionDiscountAmount: zero,
		WholesaleDiscountAmount: zero,
		TotalAmount:             amount,
		WalletPaidAmount:        zero,
		OnlinePaidAmount:        amount,
		RefundedAmount:          zero,
		PaidAt:                  &now,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	if err := tx.db.Create(order).Error; err != nil {
		return nil, giftcardcontract.ErrCreateFailed
	}

	item := orderdomain.OrderItem{
		OrderID:    order.ID,
		ProductID:  product.ID,
		SKUID:      sku.ID,
		TitleJSON:  product.TitleJSON,
		SKUSnapshotJSON: jsonmap.JSON{
			"sku_id":      sku.ID,
			"sku_code":    sku.SKUCode,
			"spec_values": sku.SpecValuesJSON,
		},
		Tags:                         product.Tags,
		OriginalUnitPrice:            amount,
		UnitPrice:                    amount,
		CostPrice:                    sku.CostPriceAmount,
		Quantity:                     1,
		OriginalTotalPrice:           amount,
		TotalPrice:                   amount,
		CouponDiscount:               zero,
		MemberDiscount:               zero,
		PromotionDiscount:            zero,
		WholesaleDiscount:            zero,
		FulfillmentType:              constants.FulfillmentTypeManual,
		ManualFormSchemaSnapshotJSON: normalizedSchema,
		ManualFormSubmissionJSON:     normalizedSubmission,
		InstructionsJSON:             product.InstructionsJSON,
		CreatedAt:                    now,
		UpdatedAt:                    now,
	}
	if err := tx.db.Create(&item).Error; err != nil {
		return nil, giftcardcontract.ErrCreateFailed
	}
	order.Items = []orderdomain.OrderItem{item}
	return order, nil
}

var _ giftcardcontract.RedeemTransactionRunner = (*Runner)(nil)
var _ giftcardcontract.RedeemTransaction = (*transaction)(nil)
