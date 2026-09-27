package keleai

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	fulfillmentapp "github.com/dujiao-next/internal/modules/fulfillment/application"
	giftcardgormstore "github.com/dujiao-next/internal/modules/giftcard/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/sensitiveform"
)

const (
	defaultBaseURL = "https://keleai.pro"
	upstreamBatch  = "UPSTREAM-KELEAI-PRO20X"
)

var (
	errInvalidSession   = errors.New("invalid chatgpt session")
	errNoUpstreamCode   = errors.New("upstream code unavailable")
	errUpstreamFailed   = errors.New("upstream fulfillment failed")
	errUpstreamTimeout  = errors.New("upstream fulfillment timeout")
)

type Executor struct {
	codes     *giftcardgormstore.Store
	session   *sensitiveform.Codec
	client    *http.Client
	baseURL   string
	pollEvery time.Duration
	maxWait   time.Duration
}

func New(codes *giftcardgormstore.Store, appSecret string) *Executor {
	return &Executor{
		codes: codes,
		session: sensitiveform.New(appSecret),
		client: &http.Client{Timeout: 20 * time.Second},
		baseURL: defaultBaseURL,
		pollEvery: 3 * time.Second,
		maxWait: 30 * time.Minute,
	}
}

func (e *Executor) Execute(ctx context.Context, input fulfillmentapp.KeleaiPro20xExecutionInput) (fulfillmentapp.KeleaiPro20xExecutionResult, error) {
	if e == nil || e.codes == nil || e.session == nil || input.OrderID == 0 {
		return fulfillmentapp.KeleaiPro20xExecutionResult{}, errUpstreamFailed
	}

	sessionJSON, err := e.readSession(input.ManualFormSubmission)
	if err != nil {
		return fulfillmentapp.KeleaiPro20xExecutionResult{}, errInvalidSession
	}

	card, err := e.codes.ReserveActiveProductCodeByBatchNo(upstreamBatch, input.OrderID, time.Now())
	if err != nil {
		return fulfillmentapp.KeleaiPro20xExecutionResult{}, errUpstreamFailed
	}
	if card == nil {
		return fulfillmentapp.KeleaiPro20xExecutionResult{}, errNoUpstreamCode
	}
	cdk, err := e.codes.RevealCode(card)
	if err != nil || strings.TrimSpace(cdk) == "" {
		return fulfillmentapp.KeleaiPro20xExecutionResult{}, errUpstreamFailed
	}

	redemptionToken, err := e.preview(ctx, cdk, input.OrderID)
	if err != nil {
		return fulfillmentapp.KeleaiPro20xExecutionResult{}, errUpstreamFailed
	}
	preflightToken, err := e.preflight(ctx, cdk, redemptionToken, sessionJSON, input.OrderID)
	if err != nil {
		return fulfillmentapp.KeleaiPro20xExecutionResult{}, errUpstreamFailed
	}

	clientRequestID, err := makeClientRequestID(redemptionToken, preflightToken)
	if err != nil {
		return fulfillmentapp.KeleaiPro20xExecutionResult{}, errUpstreamFailed
	}
	status, err := e.redeem(ctx, redemptionToken, preflightToken, clientRequestID, input.OrderID)
	if err != nil {
		return fulfillmentapp.KeleaiPro20xExecutionResult{}, errUpstreamFailed
	}
	if !isSuccess(status) {
		if isFailure(status) {
			return fulfillmentapp.KeleaiPro20xExecutionResult{}, errUpstreamFailed
		}
		status, err = e.wait(ctx, redemptionToken, input.OrderID)
		if err != nil {
			return fulfillmentapp.KeleaiPro20xExecutionResult{}, err
		}
	}
	if !isSuccess(status) {
		return fulfillmentapp.KeleaiPro20xExecutionResult{}, errUpstreamFailed
	}

	if err := e.codes.CompleteReservedProductCode(card.ID, input.OrderID, time.Now()); err != nil {
		return fulfillmentapp.KeleaiPro20xExecutionResult{}, errUpstreamFailed
	}
	return fulfillmentapp.KeleaiPro20xExecutionResult{
		PublicMessage: "ChatGPT Pro 20X 自动充值已完成",
	}, nil
}

func (e *Executor) readSession(submission map[string]interface{}) (string, error) {
	if submission == nil {
		return "", errInvalidSession
	}
	stored, _ := submission["session_json"].(string)
	plaintext, err := e.session.Open(stored)
	if err != nil {
		return "", errInvalidSession
	}
	plaintext = html.UnescapeString(strings.TrimSpace(plaintext))
	var payload map[string]interface{}
	if json.Unmarshal([]byte(plaintext), &payload) != nil {
		return "", errInvalidSession
	}
	if sessionToken(payload) == "" {
		return "", errInvalidSession
	}
	return plaintext, nil
}

func sessionToken(value map[string]interface{}) string {
	for _, key := range []string{"sessionToken", "session_token"} {
		if v, ok := value[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	if token, ok := value["token"].(map[string]interface{}); ok {
		for _, key := range []string{"sessionToken", "session_token"} {
			if v, ok := token[key].(string); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
}

func (e *Executor) preview(ctx context.Context, cdk string, orderID uint) (string, error) {
	var response map[string]interface{}
	if err := e.requestJSON(ctx, http.MethodPost, "/api/v1/public/cdk/preview", map[string]interface{}{
		"code": cdk,
	}, orderID, &response); err != nil {
		return "", err
	}
	token := nestedString(response, "redemption_token")
	if token == "" {
		return "", errUpstreamFailed
	}
	return token, nil
}

func (e *Executor) preflight(ctx context.Context, cdk, redemptionToken, sessionJSON string, orderID uint) (string, error) {
	var response map[string]interface{}
	if err := e.requestJSON(ctx, http.MethodPost, "/api/v1/public/cdk/preflight", map[string]interface{}{
		"code": cdk,
		"redemption_token": redemptionToken,
		"credential": map[string]interface{}{
			"mode": "session",
			"session": sessionJSON,
		},
	}, orderID, &response); err != nil {
		return "", err
	}
	if code, ok := response["code"].(float64); ok && code != 0 {
		return "", errUpstreamFailed
	}
	token := nestedString(response, "preflight_token")
	if token == "" {
		return "", errUpstreamFailed
	}
	return token, nil
}

func (e *Executor) redeem(ctx context.Context, redemptionToken, preflightToken, clientRequestID string, orderID uint) (string, error) {
	var response map[string]interface{}
	err := e.requestJSON(ctx, http.MethodPost, "/api/v1/public/cdk/redeem", map[string]interface{}{
		"redemption_token": redemptionToken,
		"preflight_token": preflightToken,
		"client_request_id": clientRequestID,
	}, orderID, &response)
	if err != nil {
		return "", err
	}
	return responseStatus(response), nil
}

func (e *Executor) wait(ctx context.Context, redemptionToken string, orderID uint) (string, error) {
	waitCtx, cancel := context.WithTimeout(ctx, e.maxWait)
	defer cancel()
	ticker := time.NewTicker(e.pollEvery)
	defer ticker.Stop()

	for {
		select {
		case <-waitCtx.Done():
			return "", errUpstreamTimeout
		case <-ticker.C:
			var response map[string]interface{}
			path := "/api/v1/public/cdk/result?token=" + url.QueryEscape(redemptionToken)
			if err := e.requestJSON(waitCtx, http.MethodGet, path, nil, orderID, &response); err != nil {
				continue
			}
			status := responseStatus(response)
			if isSuccess(status) || isFailure(status) {
				return status, nil
			}
		}
	}
}

func makeClientRequestID(redemptionToken, preflightToken string) (string, error) {
	raw, err := json.Marshal([]string{redemptionToken, preflightToken})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return "web-" + hex.EncodeToString(sum[:])[:40], nil
}

func (e *Executor) requestJSON(ctx context.Context, method, path string, body interface{}, orderID uint, out *map[string]interface{}) error {
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
	req.Header.Set("X-Redemption-Device", deviceID(orderID))
	resp, err := e.client.Do(req)
	if err != nil {
		return errUpstreamFailed
	}
	defer resp.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	var value map[string]interface{}
	if err := decoder.Decode(&value); err != nil {
		return errUpstreamFailed
	}
	if out != nil {
		*out = value
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errUpstreamFailed
	}
	return nil
}

func deviceID(orderID uint) string {
	return "aishopone-" + strings.TrimSpace(strings.ToLower(strings.ReplaceAll(time.Unix(int64(orderID), 0).UTC().Format("20060102T150405"), ":", "")))
}

func nestedString(value map[string]interface{}, key string) string {
	if value == nil {
		return ""
	}
	if v, ok := value[key].(string); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	if data, ok := value["data"].(map[string]interface{}); ok {
		if v, ok := data[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func responseStatus(value map[string]interface{}) string {
	if value == nil {
		return ""
	}
	order, _ := value["order"].(map[string]interface{})
	if order == nil {
		if data, ok := value["data"].(map[string]interface{}); ok {
			if nested, ok := data["order"].(map[string]interface{}); ok {
				order = nested
			} else {
				order = data
			}
		}
	}
	for _, source := range []map[string]interface{}{order, value} {
		if source == nil {
			continue
		}
		for _, key := range []string{"status", "stage"} {
			if v, ok := source[key].(string); ok && strings.TrimSpace(v) != "" {
				return strings.ToLower(strings.TrimSpace(v))
			}
		}
	}
	return ""
}

func isSuccess(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "complete", "success", "done":
		return true
	default:
		return false
	}
}

func isFailure(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "declined", "failed_precharge", "cancelled", "canceled", "failed", "failure", "error":
		return true
	default:
		return false
	}
}

var _ fulfillmentapp.KeleaiPro20xExecutor = (*Executor)(nil)
