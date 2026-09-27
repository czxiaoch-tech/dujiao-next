package simulation

import (
	"context"
	"encoding/json"
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
	// The encrypted-by-persistence session field remains order-bound; scenario is
	// simulation-only metadata carried alongside it, never a process-global switch.
	scenario := "success"
	var form map[string]interface{}
	if raw, ok := input.ManualFormSubmission["session_json"].(string); ok {
		if err := json.Unmarshal([]byte(raw), &form); err == nil {
			if value, ok := form["simulation_scenario"].(string); ok {
				scenario = value
			}
		}
	}
	if scenario != "success" && scenario != "failure" && scenario != "failed" && scenario != "pending" && scenario != "timeout" && scenario != "manual" {
		scenario = "success"
	}
	delay := e.delay
	if scenario == "pending" {
		return fulfillmentapp.KeleaiPro20xExecutionResult{Scenario: scenario}, nil
	}
	if scenario == "timeout" { delay = 5 * time.Second }
	select {
	case <-ctx.Done(): return fulfillmentapp.KeleaiPro20xExecutionResult{}, ctx.Err()
	case <-time.After(delay):
	}
	if scenario == "failure" { scenario = "failed" }
	message := "ChatGPT Pro 20X 模拟充值已完成"
	if scenario == "failed" { message = "模拟充值失败" }
	if scenario == "timeout" { message = "模拟处理超时" }
	if scenario == "manual" { message = "已转人工处理" }
	return fulfillmentapp.KeleaiPro20xExecutionResult{PublicMessage: message, ProviderReference: fmt.Sprintf("simulation-pro20x-%d", input.OrderID), Scenario: scenario}, nil
}

var _ fulfillmentapp.KeleaiPro20xExecutor = (*Pro20xExecutor)(nil)
