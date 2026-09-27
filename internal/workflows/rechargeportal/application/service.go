package rechargeportal

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/cache"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	giftcardapp "github.com/dujiao-next/internal/modules/giftcard/application"
	giftcarddomain "github.com/dujiao-next/internal/modules/giftcard/domain"
	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/sensitiveform"
)

var (
	ErrInvalid        = errors.New("recharge portal invalid")
	ErrNotFound       = errors.New("recharge portal state not found")
	ErrUnavailable    = errors.New("recharge portal unavailable")
	ErrWrongProduct   = errors.New("recharge portal wrong product")
	ErrInvalidSession = errors.New("recharge portal invalid session")
)

const (
	previewTTL   = 15 * time.Minute
	preflightTTL = 15 * time.Minute
	resultTTL    = 7 * 24 * time.Hour
)

type CardService interface {
	ResolveGiftCard(code string) (*giftcardapp.ResolveResult, error)
	RedeemPublicProductCode(input giftcardapp.RedeemInput) (*giftcardapp.RedeemResult, error)
	GenerateMirrorSimulationCode() (string, error)
}

type Service struct {
	cards    CardService
	products productcontract.Repository
	skus     productcontract.SKURepository
	orders   ordercontract.Store
	codec    *sensitiveform.Codec
}

type PreviewResult struct {
	RedemptionToken string      `json:"redemption_token"`
	ProductTitle    jsonmap.JSON `json:"product_title"`
	SKUSnapshot     jsonmap.JSON `json:"sku_snapshot"`
}

type PreflightResult struct {
	PreflightToken string `json:"preflight_token"`
	Email          string `json:"email,omitempty"`
	AccountID      string `json:"account_id,omitempty"`
}

type RedeemResult struct {
	ResultToken string `json:"result_token"`
	OrderNo     string `json:"order_no"`
	Status      string `json:"status"`
}

type Result struct {
	OrderNo string `json:"order_no"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type previewState struct {
	CodeCipher string `json:"code_cipher"`
	ProductID  uint   `json:"product_id"`
	SKUID      uint   `json:"sku_id"`
}

type preflightState struct {
	CodeCipher    string `json:"code_cipher"`
	SessionCipher string `json:"session_cipher"`
	ProductID     uint   `json:"product_id"`
	SKUID         uint   `json:"sku_id"`
}

type resultState struct {
	OrderID uint `json:"order_id"`
}

func New(cards CardService, products productcontract.Repository, skus productcontract.SKURepository, orders ordercontract.Store, appSecret string) *Service {
	return &Service{
		cards: cards,
		products: products,
		skus: skus,
		orders: orders,
		codec: sensitiveform.New(appSecret),
	}
}

type SimulationFixture struct {
	Enabled     bool   `json:"enabled"`
	TestCode    string `json:"test_code,omitempty"`
	TestSession string `json:"test_session_json,omitempty"`
}

func (s *Service) SimulationFixture(ctx context.Context, enabled bool) (*SimulationFixture, error) {
	if !enabled {
		return &SimulationFixture{Enabled: false}, nil
	}
	if s == nil || s.cards == nil {
		return nil, ErrUnavailable
	}
	code, err := s.cards.GenerateMirrorSimulationCode()
	if err != nil {
		return nil, ErrUnavailable
	}
	return &SimulationFixture{
		Enabled:  true,
		TestCode: code,
		TestSession: "{"user":{"email":"simulation@example.invalid"},"sessionToken":"simulation-session-token","accessToken":"simulation-access-token","account_id":"11111111-1111-1111-1111-111111111111"}",
	}, nil
}

func (s *Service) Preview(ctx context.Context, code string) (*PreviewResult, error) {
	if s == nil || s.cards == nil || s.products == nil || s.skus == nil || s.codec == nil || !cache.Enabled() {
		return nil, ErrUnavailable
	}
	code = strings.TrimSpace(strings.ToUpper(code))
	if code == "" {
		return nil, ErrInvalid
	}
	resolved, err := s.cards.ResolveGiftCard(code)
	if err != nil {
		return nil, err
	}
	if resolved == nil || resolved.RedeemType != giftcarddomain.GiftCardRedeemTypeProduct {
		return nil, ErrWrongProduct
	}
	product, err := s.products.GetByID(fmt.Sprintf("%d", resolved.ProductID))
	if err != nil || product == nil {
		return nil, ErrInvalid
	}
	if !strings.EqualFold(strings.TrimSpace(product.Slug), "chatgpt-pro-20x") {
		return nil, ErrWrongProduct
	}
	sku, err := s.skus.GetByID(resolved.SKUID)
	if err != nil || sku == nil || !strings.EqualFold(strings.TrimSpace(sku.SKUCode), "PRO20X") {
		return nil, ErrWrongProduct
	}
	codeCipher, err := s.codec.Seal(code)
	if err != nil {
		return nil, ErrUnavailable
	}
	token, err := newToken()
	if err != nil {
		return nil, ErrUnavailable
	}
	if err := cache.SetJSONRequired(ctx, previewKey(token), previewState{
		CodeCipher: codeCipher,
		ProductID: resolved.ProductID,
		SKUID: resolved.SKUID,
	}, previewTTL); err != nil {
		return nil, ErrUnavailable
	}
	return &PreviewResult{
		RedemptionToken: token,
		ProductTitle: resolved.ProductTitle,
		SKUSnapshot: resolved.SKUSnapshot,
	}, nil
}

func (s *Service) Preflight(ctx context.Context, redemptionToken, sessionJSON string) (*PreflightResult, error) {
	if s == nil || s.codec == nil || !cache.Enabled() {
		return nil, ErrUnavailable
	}
	redemptionToken = strings.TrimSpace(redemptionToken)
	sessionJSON = strings.TrimSpace(sessionJSON)
	if redemptionToken == "" || sessionJSON == "" {
		return nil, ErrInvalid
	}
	var preview previewState
	ok, err := cache.GetJSON(ctx, previewKey(redemptionToken), &preview)
	if err != nil {
		return nil, ErrUnavailable
	}
	if !ok {
		return nil, ErrNotFound
	}
	if _, err := s.codec.Open(preview.CodeCipher); err != nil {
		return nil, ErrUnavailable
	}
	email, accountID, err := inspectSession(sessionJSON)
	if err != nil {
		return nil, ErrInvalidSession
	}
	sessionCipher, err := s.codec.Seal(sessionJSON)
	if err != nil {
		return nil, ErrUnavailable
	}
	preflightToken, err := newToken()
	if err != nil {
		return nil, ErrUnavailable
	}
	if err := cache.SetJSONRequired(ctx, preflightKey(preflightToken), preflightState{
		CodeCipher: preview.CodeCipher,
		SessionCipher: sessionCipher,
		ProductID: preview.ProductID,
		SKUID: preview.SKUID,
	}, preflightTTL); err != nil {
		return nil, ErrUnavailable
	}
	return &PreflightResult{
		PreflightToken: preflightToken,
		Email: email,
		AccountID: accountID,
	}, nil
}

func (s *Service) Redeem(ctx context.Context, preflightToken string) (*RedeemResult, error) {
	if s == nil || s.cards == nil || s.codec == nil || !cache.Enabled() {
		return nil, ErrUnavailable
	}
	preflightToken = strings.TrimSpace(preflightToken)
	if preflightToken == "" {
		return nil, ErrInvalid
	}
	var state preflightState
	ok, err := cache.GetDelJSONRequired(ctx, preflightKey(preflightToken), &state)
	if err != nil {
		return nil, ErrUnavailable
	}
	if !ok {
		return nil, ErrNotFound
	}
	code, err := s.codec.Open(state.CodeCipher)
	if err != nil {
		return nil, ErrUnavailable
	}
	sessionJSON, err := s.codec.Open(state.SessionCipher)
	if err != nil {
		return nil, ErrUnavailable
	}

	redeemed, err := s.cards.RedeemPublicProductCode(giftcardapp.RedeemInput{
		UserID: 0,
		Code: code,
		ManualFormData: jsonmap.JSON{
			"session_json": sessionJSON,
		},
	})
	if err != nil {
		return nil, err
	}
	if redeemed == nil || redeemed.Order == nil || redeemed.Order.ID == 0 {
		return nil, ErrUnavailable
	}
	resultToken, err := newToken()
	if err != nil {
		return nil, ErrUnavailable
	}
	if err := cache.SetJSONRequired(ctx, resultKey(resultToken), resultState{OrderID: redeemed.Order.ID}, resultTTL); err != nil {
		return nil, ErrUnavailable
	}
	return &RedeemResult{
		ResultToken: resultToken,
		OrderNo: redeemed.Order.OrderNo,
		Status: publicStatus(redeemed.Order.Status),
	}, nil
}

func (s *Service) Result(ctx context.Context, token string) (*Result, error) {
	if s == nil || s.orders == nil || !cache.Enabled() {
		return nil, ErrUnavailable
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrInvalid
	}
	var state resultState
	ok, err := cache.GetJSON(ctx, resultKey(token), &state)
	if err != nil {
		return nil, ErrUnavailable
	}
	if !ok || state.OrderID == 0 {
		return nil, ErrNotFound
	}
	order, err := s.orders.GetByID(state.OrderID)
	if err != nil {
		return nil, ErrUnavailable
	}
	if order == nil {
		return nil, ErrNotFound
	}
	message := ""
	if order.Fulfillment != nil {
		message = strings.TrimSpace(order.Fulfillment.Payload)
	}
	return &Result{
		OrderNo: order.OrderNo,
		Status: publicStatus(order.Status),
		Message: message,
	}, nil
}

func publicStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "delivered":
		return "completed"
	case "canceled", "cancelled", "refunded", "failed":
		return "failed"
	case "fulfilling", "paid":
		return "processing"
	default:
		return "pending"
	}
}

func inspectSession(raw string) (string, string, error) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return "", "", err
	}
	if findString(data, "sessionToken", "session_token") == "" {
		if token, ok := data["token"].(map[string]interface{}); !ok || findString(token, "sessionToken", "session_token") == "" {
			return "", "", ErrInvalidSession
		}
	}
	email := ""
	if user, ok := data["user"].(map[string]interface{}); ok {
		email = findString(user, "email")
	}
	if email == "" {
		if account, ok := data["account"].(map[string]interface{}); ok {
			email = findString(account, "email")
		}
	}
	if email == "" {
		email = findString(data, "email")
	}
	accountID := findString(data, "account_id", "accountId")
	if accountID == "" {
		if account, ok := data["account"].(map[string]interface{}); ok {
			accountID = findString(account, "account_id", "id")
		}
	}
	if accountID == "" {
		if accounts, ok := data["accounts"].(map[string]interface{}); ok {
			var candidate interface{}
			if v, exists := accounts["default"]; exists {
				candidate = v
			} else {
				for _, v := range accounts {
					candidate = v
					break
				}
			}
			if row, ok := candidate.(map[string]interface{}); ok {
				accountID = findString(row, "account_id", "id")
				if accountID == "" {
					if account, ok := row["account"].(map[string]interface{}); ok {
						accountID = findString(account, "account_id", "id")
					}
				}
			}
		}
	}
	return strings.TrimSpace(email), strings.TrimSpace(accountID), nil
}

func findString(data map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := data[key].(string); ok {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func newToken() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func previewKey(token string) string   { return "recharge:preview:" + token }
func preflightKey(token string) string { return "recharge:preflight:" + token }
func resultKey(token string) string    { return "recharge:result:" + token }

var _ = productdomain.Product{}
var _ = orderdomain.Order{}
