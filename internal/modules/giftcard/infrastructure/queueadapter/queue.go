package queueadapter

import (
	"github.com/dujiao-next/internal/queue"

	"github.com/hibiken/asynq"
)

// Queue 将产品兑换码后的 Plus 自动履约请求接入现有 Asynq。
type Queue struct {
	client *queue.Client
}

func New(client *queue.Client) *Queue {
	return &Queue{client: client}
}

// EnqueuePlusAutoFulfill V0.1 只执行一次，不配置自动重试；失败订单保留人工接管。
func (q *Queue) EnqueuePlusAutoFulfill(orderID uint) error {
	if q == nil || q.client == nil || orderID == 0 {
		return nil
	}
	return q.client.EnqueuePlusAutoFulfill(
		queue.PlusAutoFulfillPayload{OrderID: orderID},
		asynq.MaxRetry(0),
	)
}


// EnqueueKeleaiPro20xFulfill V0.1 只执行一次，不配置自动重试。
func (q *Queue) EnqueueKeleaiPro20xFulfill(orderID uint) error {
	if q == nil || q.client == nil || orderID == 0 {
		return nil
	}
	return q.client.EnqueueKeleaiPro20xFulfill(
		queue.KeleaiPro20xFulfillPayload{OrderID: orderID},
		asynq.MaxRetry(0),
	)
}
