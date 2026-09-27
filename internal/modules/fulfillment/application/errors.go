package application

import (
	"errors"

	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	orderapp "github.com/dujiao-next/internal/modules/order/application"
)

var (
	ErrFulfillmentInvalid      = productcontract.ErrFulfillmentInvalid
	ErrFulfillmentExists       = errors.New("fulfillment exists")
	ErrFulfillmentCreateFailed = errors.New("fulfillment create failed")
	ErrFulfillmentNotAuto      = errors.New("fulfillment not auto")
	ErrOrderNotFound           = orderapp.ErrOrderNotFound
	ErrOrderFetchFailed        = orderapp.ErrOrderFetchFailed
	ErrOrderStatusInvalid      = orderapp.ErrOrderStatusInvalid
	ErrOrderUpdateFailed       = orderapp.ErrOrderUpdateFailed
	ErrCardSecretInsufficient  = orderapp.ErrCardSecretInsufficient
	ErrPlusExecutorUnavailable = errors.New("plus fulfillment executor unavailable")
	ErrPlusExecutionFailed     = errors.New("plus fulfillment execution failed")
	ErrFulfillmentNotPlus               = errors.New("fulfillment not plus")
	ErrKeleaiPro20xExecutorUnavailable  = errors.New("keleai pro20x executor unavailable")
	ErrKeleaiPro20xExecutionFailed      = errors.New("keleai pro20x execution failed")
	ErrFulfillmentNotKeleaiPro20x       = errors.New("fulfillment not keleai pro20x")
)
