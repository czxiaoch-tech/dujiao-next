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

// PlusExecutionInput 是 Plus 上游执行器唯一可见的业务输入。
// ManualFormSubmission 仅在内存中传递给已授权执行器，禁止写入普通日志。
type PlusExecutionInput struct {
	OrderID              uint
	OrderNo              string
	ProductID            uint
	SKUID                 uint
	ManualFormSubmission jsonmap.JSON
}

// PlusExecutionResult 只允许返回可安全展示给用户的结果。
// ProviderReference 只能是非敏感业务引用，不得放 API Key、Token、账号密码或 Session。
type PlusExecutionResult struct {
	PublicMessage     string
	ProviderReference string
}

// PlusExecutor 是真实上游能力的最小插槽。V0.1 下一步只需实现这一接口。
type PlusExecutor interface {
	Execute(ctx context.Context, input PlusExecutionInput) (PlusExecutionResult, error)
}

// ExecutePlusAutoFulfillment 执行一次 Plus 自动交付。
// 成功：原子写 fulfillment + completed；失败：不写成功记录、不改状态，保留人工接管。
func (s *Service) ExecutePlusAutoFulfillment(ctx context.Context, orderID uint) (*fulfillmentdomain.Fulfillment, error) {
	if s == nil || orderID == 0 {
		return nil, ErrFulfillmentInvalid
	}
	if s.plusExecutor == nil {
		return nil, ErrPlusExecutorUnavailable
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
	if strings.TrimSpace(order.Status) != constants.OrderStatusFulfilling {
		return nil, ErrOrderStatusInvalid
	}
	if len(order.Items) != 1 {
		return nil, ErrFulfillmentInvalid
	}
	item := order.Items[0]
	if item.ProductID == 0 || item.SKUID == 0 {
		return nil, ErrFulfillmentInvalid
	}
	if s.products == nil {
		return nil, ErrFulfillmentNotPlus
	}
	product, err := s.products.GetByID(fmt.Sprintf("%d", item.ProductID))
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	if product == nil || !strings.EqualFold(strings.TrimSpace(product.Slug), "chatgpt-plus") {
		return nil, ErrFulfillmentNotPlus
	}
	if existing, err := s.fulfillmentRepo.GetByOrderID(orderID); err != nil {
		return nil, ErrOrderFetchFailed
	} else if existing != nil {
		return nil, ErrFulfillmentExists
	}

	result, execErr := s.plusExecutor.Execute(ctx, PlusExecutionInput{
		OrderID:              order.ID,
		OrderNo:              order.OrderNo,
		ProductID:            item.ProductID,
		SKUID:                 item.SKUID,
		ManualFormSubmission: item.ManualFormSubmissionJSON,
	})
	if execErr != nil {
		// 刻意丢弃上游错误正文，避免 Token / 账号资料等进入 worker 错误栈或普通日志。
		return nil, ErrPlusExecutionFailed
	}

	publicMessage := strings.TrimSpace(result.PublicMessage)
	if publicMessage == "" {
		publicMessage = "ChatGPT Plus 自动交付已完成"
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
			OrderID:       orderID,
			Type:          constants.FulfillmentTypeUpstream,
			Status:        constants.FulfillmentStatusDelivered,
			Payload:       publicMessage,
			LogisticsJSON: deliveryData,
			DeliveredAt:   &now,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := tx.Fulfillments().Create(fulfillment); err != nil {
			return ErrFulfillmentCreateFailed
		}
		if err := tx.Orders().UpdateFields(orderID, map[string]interface{}{
			"status":     constants.OrderStatusCompleted,
			"updated_at": now,
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
		if _, err := orderapp.EnqueueStatusEmailTaskIfEligible(
			s.orderStore,
			s.orderQueue,
			s.settingService,
			s.defaultEmailConfig,
			orderID,
			constants.OrderStatusCompleted,
		); err != nil {
			// 邮件失败不反向破坏已经完成的真实交付。
		}
	}
	go s.NotifyBotOrderFulfilled(order.UserID, orderID)
	if s.downstreamCallbackSvc != nil {
		s.downstreamCallbackSvc.EnqueueCallback(orderID)
	}
	return created, nil
}
