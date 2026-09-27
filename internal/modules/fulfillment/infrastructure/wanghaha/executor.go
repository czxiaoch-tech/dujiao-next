package wanghaha

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	fulfillmentapp "github.com/dujiao-next/internal/modules/fulfillment/application"
	giftcardgormstore "github.com/dujiao-next/internal/modules/giftcard/infrastructure/gormstore"
)

const (
	defaultBaseURL = "https://sub.whh985.com"
	upstreamBatch  = "UPSTREAM-WANGHAHA-PLUS"
)

var accountIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

var (
	errInvalidAccountID = errors.New("invalid chatgpt account id")
	errNoUpstreamCode   = errors.New("upstream code unavailable")
	errUpstreamFailed   = errors.New("upstream fulfillment failed")
	errUpstreamTimeout  = errors.New("upstream fulfillment timeout")
)

type Executor struct {
	codes      *giftcardgormstore.Store
	client     *http.Client
	baseURL    string
	pollEvery  time.Duration
	maxWait    time.Duration
}

func New(codes *giftcardgormstore.Store) *Executor {
	return &Executor{
		codes:     codes,
		client:    &http.Client{Timeout: 20 * time.Second},
		baseURL:   defaultBaseURL,
		pollEvery: 3 * time.Second,
		maxWait:   11 * time.Minute,
	}
}

func (e *Executor) Execute(ctx context.Context, input fulfillmentapp.PlusExecutionInput) (fulfillmentapp.PlusExecutionResult, error) {
	if e == nil || e.codes == nil || input.OrderID == 0 {
		return fulfillmentapp.PlusExecutionResult{}, errUpstreamFailed
	}
	accountID := readAccountID(input.ManualFormSubmission)
	if !accountIDPattern.MatchString(accountID) {
		return fulfillmentapp.PlusExecutionResult{}, errInvalidAccountID
	}

	now := time.Now()
	card, err := e.codes.ReserveActiveProductCodeByBatchNo(upstreamBatch, input.OrderID, now)
	if err != nil {
		return fulfillmentapp.PlusExecutionResult{}, errUpstreamFailed
	}
	if card == nil {
		return fulfillmentapp.PlusExecutionResult{}, errNoUpstreamCode
	}
	cdk, err := e.codes.RevealCode(card)
	if err != nil || strings.TrimSpace(cdk) == "" {
		return fulfillmentapp.PlusExecutionResult{}, errUpstreamFailed
	}

	taskID, done, err := e.start(ctx, cdk, accountID)
	if err != nil {
		return fulfillmentapp.PlusExecutionResult{}, errUpstreamFailed
	}
	if !done {
		if strings.TrimSpace(taskID) == "" {
			return fulfillmentapp.PlusExecutionResult{}, errUpstreamFailed
		}
		done, err = e.wait(ctx, taskID)
		if err != nil {
			return fulfillmentapp.PlusExecutionResult{}, err
		}
	}
	if !done {
		return fulfillmentapp.PlusExecutionResult{}, errUpstreamFailed
	}

	if err := e.codes.CompleteReservedProductCode(card.ID, input.OrderID, time.Now()); err != nil {
		return fulfillmentapp.PlusExecutionResult{}, errUpstreamFailed
	}
	return fulfillmentapp.PlusExecutionResult{
		PublicMessage: "ChatGPT Plus 自动充值已完成",
	}, nil
}

func readAccountID(submission map[string]interface{}) string {
	if submission == nil {
		return ""
	}
	for _, key := range []string{"account_id", "accountId", "recharge_account"} {
		if value, ok := submission[key].(string); ok {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func (e *Executor) start(ctx context.Context, cdk, accountID string) (string, bool, error) {
	body := map[string]interface{}{
		"cdk":            cdk,
		"cdkey":          cdk,
		"account_id":     accountID,
		"force":          true,
		"force_recharge": true,
		"forceRecharge":  true,
		"overwrite":      true,
	}
	var response map[string]interface{}
	if err := e.requestJSON(ctx, http.MethodPost, "/site-api/external/redeem/appstore/start-stable", body, &response); err != nil {
		return "", false, err
	}
	taskID := responseTaskID(response)
	if taskID != "" {
		return taskID, false, nil
	}
	if responseWaiting(response) {
		return "", false, nil
	}
	if responseSucceeded(response) {
		return "", true, nil
	}
	return "", false, errUpstreamFailed
}

func (e *Executor) wait(ctx context.Context, taskID string) (bool, error) {
	waitCtx, cancel := context.WithTimeout(ctx, e.maxWait)
	defer cancel()

	ticker := time.NewTicker(e.pollEvery)
	defer ticker.Stop()

	for {
		select {
		case <-waitCtx.Done():
			return false, errUpstreamTimeout
		case <-ticker.C:
			var response map[string]interface{}
			path := "/site-api/external/redeem/appstore/status2?task_id=" + url.QueryEscape(taskID)
			if err := e.requestJSON(waitCtx, http.MethodGet, path, nil, &response); err != nil {
				continue
			}
			if responseSucceeded(response) {
				return true, nil
			}
			if responseFailed(response) {
				return false, errUpstreamFailed
			}
		}
	}
}

func (e *Executor) requestJSON(ctx context.Context, method, path string, body interface{}, out *map[string]interface{}) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return errUpstreamFailed
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(e.baseURL, "/")+path, reader)
	if err != nil {
		return errUpstreamFailed
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return errUpstreamFailed
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return errUpstreamFailed
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := decoder.Decode(out); err != nil {
		return errUpstreamFailed
	}
	return nil
}

func responseTaskID(value map[string]interface{}) string {
	if value == nil {
		return ""
	}
	for _, key := range []string{"taskId", "task_id", "id"} {
		if v := scalarString(value[key]); v != "" {
			return v
		}
	}
	if data, ok := value["data"].(map[string]interface{}); ok {
		for _, key := range []string{"taskId", "task_id", "id"} {
			if v := scalarString(data[key]); v != "" {
				return v
			}
		}
	}
	return ""
}

func responseSucceeded(value map[string]interface{}) bool {
	status := responseStatus(value)
	if success, ok := value["success"].(bool); ok && success {
		return true
	}
	if code := scalarString(value["code"]); code == "1" && !responseWaiting(value) {
		return true
	}
	switch status {
	case "success", "completed", "complete", "done", "2":
		return true
	default:
		return false
	}
}

func responseWaiting(value map[string]interface{}) bool {
	if value == nil {
		return false
	}
	if code := scalarString(value["code"]); code == "2" {
		return true
	}
	if data, ok := value["data"].(map[string]interface{}); ok {
		if waiting, ok := data["waiting"].(bool); ok && waiting {
			return true
		}
	}
	switch responseStatus(value) {
	case "pending", "processing", "waiting", "queued", "created", "running", "0":
		return true
	default:
		return false
	}
}

func responseFailed(value map[string]interface{}) bool {
	if value == nil {
		return true
	}
	if success, ok := value["success"].(bool); ok && !success && !responseWaiting(value) {
		return true
	}
	if code := scalarString(value["code"]); code == "0" && !responseWaiting(value) {
		return true
	}
	switch responseStatus(value) {
	case "failed", "failure", "error", "cancelled", "canceled", "timeout", "internal_error", "3":
		return true
	default:
		return false
	}
}

func responseStatus(value map[string]interface{}) string {
	if value == nil {
		return ""
	}
	if data, ok := value["data"].(map[string]interface{}); ok {
		for _, key := range []string{"status", "orderStatus"} {
			if v := scalarString(data[key]); v != "" {
				return strings.ToLower(v)
			}
		}
	}
	for _, key := range []string{"status", "orderStatus"} {
		if v := scalarString(value[key]); v != "" {
			return strings.ToLower(v)
		}
	}
	return ""
}

func scalarString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return fmt.Sprintf("%.0f", v)
	case json.Number:
		return v.String()
	default:
		return ""
	}
}

var _ fulfillmentapp.PlusExecutor = (*Executor)(nil)
