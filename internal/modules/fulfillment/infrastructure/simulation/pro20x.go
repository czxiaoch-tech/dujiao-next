package simulation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	fulfillmentapp "github.com/dujiao-next/internal/modules/fulfillment/application"
	"github.com/dujiao-next/internal/shared/sensitiveform"
)

type Pro20xExecutor struct {
	delay   time.Duration
	session *sensitiveform.Codec
}

func NewPro20xExecutor(secret string) *Pro20xExecutor {
	return &Pro20xExecutor{delay: 2 * time.Second, session: sensitiveform.New(secret)}
}

func (e *Pro20xExecutor) Execute(ctx context.Context, input fulfillmentapp.KeleaiPro20xExecutionInput) (fulfillmentapp.KeleaiPro20xExecutionResult, error) {
	// The encrypted-by-persistence session field remains order-bound; scenario is
	// simulation-only metadata carried alongside it, never a process-global switch.
	scenario := "success"
	var form map[string]interface{}
	if raw, ok := input.ManualFormSubmission["session_json"].(string); ok {
		// Sensitive manual-form values are sealed before persistence. Open that
		// field so simulation metadata survives the order storage boundary.
		if strings.HasPrefix(raw, sensitiveform.Prefix) && e.session != nil {
			if plaintext, err := e.session.Open(raw); err == nil {
				raw = plaintext
			}
		}
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
	if scenario == "timeout" {
		delay = 5 * time.Second
	}
	select {
	case <-ctx.Done():
		return fulfillmentapp.KeleaiPro20xExecutionResult{}, ctx.Err()
	case <-time.After(delay):
	}
	if scenario == "failure" {
		scenario = "failed"
	}
	message := "ChatGPT Pro 20X 模拟充值已完成"
	if scenario == "failed" {
		message = "模拟充值失败"
	}
	if scenario == "timeout" {
		message = "模拟处理超时"
	}
	if scenario == "manual" {
		message = "已转人工处理"
	}
	return fulfillmentapp.KeleaiPro20xExecutionResult{PublicMessage: message, ProviderReference: fmt.Sprintf("simulation-pro20x-%d", input.OrderID), Scenario: scenario}, nil
}

var _ fulfillmentapp.KeleaiPro20xExecutor = (*Pro20xExecutor)(nil)
