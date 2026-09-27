package simulation

import (
	"context"
	"encoding/json"
	"testing"

	fulfillmentapp "github.com/dujiao-next/internal/modules/fulfillment/application"
	"github.com/dujiao-next/internal/shared/sensitiveform"
)

func TestExecuteReadsEncryptedScenario(t *testing.T) {
	const secret = "simulation-test-secret"
	codec := sensitiveform.New(secret)
	sessionJSON, err := json.Marshal(map[string]interface{}{
		"sessionToken":        "fake-session",
		"simulation_scenario": "failure",
	})
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := codec.Seal(string(sessionJSON))
	if err != nil {
		t.Fatal(err)
	}

	executor := NewPro20xExecutor(secret)
	result, err := executor.Execute(context.Background(), fulfillmentapp.KeleaiPro20xExecutionInput{
		OrderID:              42,
		ManualFormSubmission: map[string]interface{}{"session_json": sealed},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Scenario != "failed" {
		t.Fatalf("expected failed scenario from encrypted session, got %q", result.Scenario)
	}
	if result.PublicMessage != "模拟充值失败" {
		t.Fatalf("unexpected failure message: %q", result.PublicMessage)
	}
}

func TestExecutePendingScenario(t *testing.T) {
	executor := NewPro20xExecutor("simulation-test-secret")
	result, err := executor.Execute(context.Background(), fulfillmentapp.KeleaiPro20xExecutionInput{
		ManualFormSubmission: map[string]interface{}{"session_json": `{"simulation_scenario":"pending"}`},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Scenario != "pending" {
		t.Fatalf("expected pending scenario, got %q", result.Scenario)
	}
}
