package simulation

import (
	"context"
	"fmt"
	"time"

	fulfillmentapp "github.com/dujiao-next/internal/modules/fulfillment/application"
)

type Pro20xExecutor struct {
	delay time.Duration
}

func NewPro20xExecutor() *Pro20xExecutor {
	return &Pro20xExecutor{delay: 2 * time.Second}
}

func (e *Pro20xExecutor) Execute(ctx context.Context, input fulfillmentapp.KeleaiPro20xExecutionInput) (fulfillmentapp.KeleaiPro20xExecutionResult, error) {
	select {
	case <-ctx.Done():
		return fulfillmentapp.KeleaiPro20xExecutionResult{}, ctx.Err()
	case <-time.After(e.delay):
	}

	return fulfillmentapp.KeleaiPro20xExecutionResult{
		PublicMessage:     "ChatGPT Pro 20X 模拟充值已完成",
		ProviderReference: fmt.Sprintf("simulation-pro20x-%d", input.OrderID),
	}, nil
}

var _ fulfillmentapp.KeleaiPro20xExecutor = (*Pro20xExecutor)(nil)
