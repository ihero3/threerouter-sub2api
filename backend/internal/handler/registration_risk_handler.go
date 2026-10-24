package handler

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// RegistrationClickCaptchaHandler 自建顺序点击验证码接口。
type RegistrationClickCaptchaHandler struct {
	clickCaptcha *service.RegistrationClickCaptchaService
}

// NewRegistrationClickCaptchaHandler 创建 handler。
func NewRegistrationClickCaptchaHandler(clickCaptcha *service.RegistrationClickCaptchaService) *RegistrationClickCaptchaHandler {
	return &RegistrationClickCaptchaHandler{clickCaptcha: clickCaptcha}
}

// captchaVerifyRequestBody 请求体：按题目模式二选一携带点选坐标或滑块落点+轨迹。
type captchaVerifyRequestBody struct {
	ChallengeID string                    `json:"challenge_id"`
	Clicks      []service.ClickPoint      `json:"clicks"`     // image_click
	DropX       float64                   `json:"drop_x"`     // slider
	Trajectory  []service.TrajectoryPoint `json:"trajectory"` // slider
}

// Challenge POST /api/v1/auth/captcha/challenge
func (h *RegistrationClickCaptchaHandler) Challenge(c *gin.Context) {
	if h == nil || h.clickCaptcha == nil {
		response.InternalError(c, "Click captcha service not configured")
		return
	}
	ipHash := service.HashFingerprint(ip.GetClientIP(c), c.Request.UserAgent())
	challenge, err := h.clickCaptcha.CreateChallenge(c.Request.Context(), ipHash, ipHash)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, challenge)
}

// Verify POST /api/v1/auth/captcha/verify
func (h *RegistrationClickCaptchaHandler) Verify(c *gin.Context) {
	if h == nil || h.clickCaptcha == nil {
		response.InternalError(c, "Click captcha service not configured")
		return
	}
	var req captchaVerifyRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if strings.TrimSpace(req.ChallengeID) == "" {
		response.BadRequest(c, "Invalid request: challenge_id is required")
		return
	}
	ipHash := service.HashFingerprint(ip.GetClientIP(c), c.Request.UserAgent())
	proof := service.CaptchaVerifyProof{
		Clicks:     req.Clicks,
		DropX:      req.DropX,
		Trajectory: req.Trajectory,
	}
	token, ttl, err := h.clickCaptcha.VerifyChallenge(c.Request.Context(), req.ChallengeID, proof, ipHash, ipHash)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{
		"captcha_token": token,
		"expires_in":    ttl,
	})
}
