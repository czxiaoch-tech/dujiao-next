package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	orderapp "github.com/dujiao-next/internal/modules/order/application"
	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

type KeleaiPro20xExecutionInput struct {
	OrderID              uint
	OrderNo              string
	ProductID            uint
	SKUID                uint
	ManualFormSubmission jsonmap.JSON
}

type KeleaiPro20xExecutionResult struct {
	PublicMessage     string
	ProviderReference string
	Scenario          string
}

type KeleaiPro20xExecutor interface {
	Execute(ctx context.Context, input KeleaiPro20xExecutionInput) (KeleaiPro20xExecutionResult, error)
}

func (s *Service) ExecuteKeleaiPro20xFulfillment(ctx context.Context, orderID uint) (*fulfillmentdomain.Fulfillment, error) {
	if s == nil || orderID == 0 {
		return nil, ErrFulfillmentInvalid
	}
	if s.keleaiPro20xExecutor == nil {
		return nil, ErrKeleaiPro20xExecutorUnavailable
	}
	order, err := s.orderStore.GetByID(orderID)
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}
	if order.ParentID == nil && len(order.Children) > 0 {
		return nil, ErrFulfillmentInvalid
	}
	if strings.TrimSpace(order.Status) != constants.OrderStatusFulfilling || len(order.Items) != 1 {
		return nil, ErrOrderStatusInvalid
	}
	item := order.Items[0]
	if item.ProductID == 0 || item.SKUID == 0 || s.products == nil {
		return nil, ErrFulfillmentNotKeleaiPro20x
	}
	product, err := s.products.GetByID(fmt.Sprintf("%d", item.ProductID))
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	if product == nil || !strings.EqualFold(strings.TrimSpace(product.Slug), "chatgpt-pro-20x") {
		return nil, ErrFulfillmentNotKeleaiPro20x
	}
	if existing, err := s.fulfillmentRepo.GetByOrderID(orderID); err != nil {
		return nil, ErrOrderFetchFailed
	} else if existing != nil {
		return nil, ErrFulfillmentExists
	}

	result, execErr := s.keleaiPro20xExecutor.Execute(ctx, KeleaiPro20xExecutionInput{
		OrderID: order.ID, OrderNo: order.OrderNo, ProductID: item.ProductID, SKUID: item.SKUID,
		ManualFormSubmission: item.ManualFormSubmissionJSON,
	})
	if execErr != nil {
		return nil, ErrKeleaiPro20xExecutionFailed
	}

	if scenario := strings.ToLower(strings.TrimSpace(result.Scenario)); scenario != "" && scenario != "success" {
		status := map[string]string{"failure": "failed", "timeout": "timeout", "manual": "manual"}[scenario]
		if scenario == "pending" {
			return nil, nil
		}
		if status == "" {
			return nil, ErrKeleaiPro20xExecutionFailed
		}
		if err := s.orderStore.WithinTransaction(func(tx ordercontract.Transaction) error {
			return tx.Orders().UpdateFields(orderID, map[string]interface{}{"status": status, "updated_at": time.Now()})
		}); err != nil {
			return nil, ErrOrderUpdateFailed
		}
		return nil, nil
	}

	publicMessage := strings.TrimSpace(result.PublicMessage)
	if publicMessage == "" {
		publicMessage = "ChatGPT Pro 20X 自动交付已完成"
	}
	deliveryData := jsonmap.JSON{}
	if ref := strings.TrimSpace(result.ProviderReference); ref != "" {
		deliveryData["provider_reference"] = ref
	}

	now := time.Now()
	var created *fulfillmentdomain.Fulfillment
	err = s.orderStore.WithinTransaction(func(tx ordercontract.Transaction) error {
		if _, found, err := tx.Fulfillments().FindByOrderIDForUpdate(orderID); err != nil {
			return err
		} else if found {
			return ErrFulfillmentExists
		}
		fulfillment := &fulfillmentdomain.Fulfillment{
			OrderID: orderID, Type: constants.FulfillmentTypeUpstream,
			Status: constants.FulfillmentStatusDelivered, Payload: publicMessage,
			LogisticsJSON: deliveryData, DeliveredAt: &now, CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Fulfillments().Create(fulfillment); err != nil {
			return ErrFulfillmentCreateFailed
		}
		if err := tx.Orders().UpdateFields(orderID, map[string]interface{}{
			"status": constants.OrderStatusCompleted, "updated_at": now,
		}); err != nil {
			return ErrOrderUpdateFailed
		}
		created = fulfillment
		return nil
	})
	if err != nil {
		return nil, err
	}

	if s.orderQueue != nil {
		_, _ = orderapp.EnqueueStatusEmailTaskIfEligible(
			s.orderStore, s.orderQueue, s.settingService, s.defaultEmailConfig,
			orderID, constants.OrderStatusCompleted,
		)
	}
	go s.NotifyBotOrderFulfilled(order.UserID, orderID)
	if s.downstreamCallbackSvc != nil {
		s.downstreamCallbackSvc.EnqueueCallback(orderID)
	}
	return created, nil
}
