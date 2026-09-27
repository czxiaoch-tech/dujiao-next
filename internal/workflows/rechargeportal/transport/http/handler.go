package rechargeportalhttp

import (
	"errors"
	"os"
	"strings"

	rechargeportal "github.com/dujiao-next/internal/workflows/rechargeportal/application"
	giftcardcontract "github.com/dujiao-next/internal/modules/giftcard/contract"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *rechargeportal.Service
}

func NewHandler(service *rechargeportal.Service) *Handler {
	if service == nil {
		panic("recharge portal handler: service is nil")
	}
	return &Handler{service: service}
}

type previewRequest struct {
	Code string `json:"code" binding:"required"`
}

type preflightRequest struct {
	RedemptionToken string `json:"redemption_token" binding:"required"`
	SessionJSON     string `json:"session_json" binding:"required"`
}

type redeemRequest struct {
	PreflightToken string `json:"preflight_token" binding:"required"`
}

func (h *Handler) Preview(c *gin.Context) {
	var req previewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	result, err := h.service.Preview(c.Request.Context(), strings.TrimSpace(req.Code))
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *Handler) Preflight(c *gin.Context) {
	var req preflightRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	result, err := h.service.Preflight(c.Request.Context(), req.RedemptionToken, req.SessionJSON)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *Handler) Redeem(c *gin.Context) {
	var req redeemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	result, err := h.service.Redeem(c.Request.Context(), req.PreflightToken)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *Handler) Result(c *gin.Context) {
	result, err := h.service.Result(c.Request.Context(), c.Query("token"))
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, result)
}

func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, rechargeportal.ErrInvalid),
		errors.Is(err, rechargeportal.ErrInvalidSession),
		errors.Is(err, rechargeportal.ErrWrongProduct),
		errors.Is(err, giftcardcontract.ErrInvalid),
		errors.Is(err, giftcardcontract.ErrExpired),
		errors.Is(err, giftcardcontract.ErrDisabled),
		errors.Is(err, giftcardcontract.ErrRedeemed):
		ginutil.RespondError(c, response.CodeBadRequest, "error.gift_card_invalid", nil)
	case errors.Is(err, rechargeportal.ErrNotFound),
		errors.Is(err, giftcardcontract.ErrNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.gift_card_not_found", nil)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.gift_card_redeem_failed", nil)
	}
}


func (h *Handler) Simulation(c *gin.Context) {
	enabled := strings.TrimSpace(os.Getenv("MIRROR_RECHARGE_SIMULATION")) == "1"
	data := gin.H{"enabled": enabled}
	if enabled {
		data["test_code"] = "ASO-SIM-PRO20X-001"
		data["test_session_json"] = "{"user":{"email":"simulation@example.invalid"},"sessionToken":"simulation-session-token","accessToken":"simulation-access-token","account_id":"11111111-1111-1111-1111-111111111111"}"
	}
	response.Success(c, data)
}
