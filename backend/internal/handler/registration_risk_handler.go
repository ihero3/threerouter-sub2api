package handler

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
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

// captchaVerifyRequestBody 请求体：按题目模式三选一携带点选坐标 / 滑块落点 / 旋转角度 + 轨迹。
type captchaVerifyRequestBody struct {
	ChallengeID string                    `json:"challenge_id"`
	Clicks      []service.ClickPoint      `json:"clicks"`     // image_click
	DropX       float64                   `json:"drop_x"`     // slider
	Angle       float64                   `json:"angle"`      // rotation
	Trajectory  []service.TrajectoryPoint `json:"trajectory"` // slider / rotation
}

// Challenge POST /api/v1/auth/captcha/challenge
func (h *RegistrationClickCaptchaHandler) Challenge(c *gin.Context) {
	if h == nil || h.clickCaptcha == nil {
		response.InternalError(c, "Click captcha service not configured")
		return
	}
	clientIP := middleware2.SecurityClientIP(c)
	ipHash := service.HashFingerprint(clientIP, c.Request.UserAgent())
	// 把可信链解析出的真实客户端 IP 通过可选 context value 传入，供 ASN 机房判定使用；
	// 不破坏 CreateChallenge 既有签名。
	ctx := service.WithClientIPForChallenge(c.Request.Context(), clientIP)
	challenge, err := h.clickCaptcha.CreateChallenge(ctx, ipHash, ipHash)
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
	ipHash := service.HashFingerprint(middleware2.SecurityClientIP(c), c.Request.UserAgent())
	proof := service.CaptchaVerifyProof{
		Clicks:     req.Clicks,
		DropX:      req.DropX,
		Angle:      req.Angle,
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
