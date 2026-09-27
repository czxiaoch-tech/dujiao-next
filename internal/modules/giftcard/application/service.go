package application

import (
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	giftcardcontract "github.com/dujiao-next/internal/modules/giftcard/contract"
)

// Service 礼品卡管理用例（不含兑换写路径）。
type PlusAutoFulfillmentQueue interface {
	EnqueuePlusAutoFulfill(orderID uint) error
}

type KeleaiPro20xFulfillmentQueue interface {
	EnqueueKeleaiPro20xFulfill(orderID uint) error
}

type Service struct {
	repo              giftcardcontract.Repository
	users             giftcardcontract.UserDirectory
	currency          giftcardcontract.CurrencyProvider
	redeemer          giftcardcontract.RedeemTransactionRunner
	products          productcontract.Repository
	skus              productcontract.SKURepository
	plusAutoFulfillQ        PlusAutoFulfillmentQueue
	keleaiPro20xFulfillQ   KeleaiPro20xFulfillmentQueue
}

// Options 组装管理用例依赖。
type Options struct {
	Repo             giftcardcontract.Repository
	Users            giftcardcontract.UserDirectory
	Currency         giftcardcontract.CurrencyProvider
	Redeemer         giftcardcontract.RedeemTransactionRunner
	Products         productcontract.Repository
	SKUs             productcontract.SKURepository
	PlusAutoFulfillQ      PlusAutoFulfillmentQueue
	KeleaiPro20xFulfillQ KeleaiPro20xFulfillmentQueue
}

func NewService(opts Options) *Service {
	if opts.Repo == nil {
		panic("giftcard service: repo is nil")
	}
	return &Service{
		repo:             opts.Repo,
		users:            opts.Users,
		currency:         opts.Currency,
		redeemer:         opts.Redeemer,
		products:         opts.Products,
		skus:             opts.SKUs,
		plusAutoFulfillQ:      opts.PlusAutoFulfillQ,
		keleaiPro20xFulfillQ: opts.KeleaiPro20xFulfillQ,
	}
}
