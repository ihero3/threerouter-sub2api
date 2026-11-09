package handler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// AuthHandler handles authentication-related requests
type AuthHandler struct {
	cfg                  *config.Config
	authService          *service.AuthService
	userService          *service.UserService
	settingSvc           *service.SettingService
	promoService         *service.PromoService
	redeemService        *service.RedeemService
	totpService          *service.TotpService
	userAttributeService *service.UserAttributeService
	clickCaptcha         *service.RegistrationClickCaptchaService

	dingTalkClientInstance *DingTalkClient
	dingTalkClientMu       sync.Mutex
}

// SetRegistrationClickCaptchaService 注入自建点击验证码服务，不改构造函数。
func (h *AuthHandler) SetRegistrationClickCaptchaService(svc *service.RegistrationClickCaptchaService) {
	h.clickCaptcha = svc
}

// NewAuthHandler creates a new AuthHandler
func NewAuthHandler(cfg *config.Config, authService *service.AuthService, userService *service.UserService, settingService *service.SettingService, promoService *service.PromoService, redeemService *service.RedeemService, totpService *service.TotpService, userAttributeService *service.UserAttributeService) *AuthHandler {
	return &AuthHandler{
		cfg:                  cfg,
		authService:          authService,
		userService:          userService,
		settingSvc:           settingService,
		promoService:         promoService,
		redeemService:        redeemService,
		totpService:          totpService,
		userAttributeService: userAttributeService,
	}
}

// RegisterRequest represents the registration request payload
type RegisterRequest struct {
	Email string `json:"email" binding:"required,email"`
	// 下限 8 位（NIST SP 800-63B 最低要求）；上限 72 字节是 bcrypt 硬限制，
	// 超过会导致 HashPassword 直接报错并让注册返回 500。
	Password              string `json:"password" binding:"required,min=8,max=72"`
	VerifyCode            string `json:"verify_code"`
	ClickCaptchaToken     string `json:"click_captcha_token"`
	TurnstileToken        string `json:"turnstile_token"`
	TencentCaptchaTicket  string `json:"tencent_captcha_ticket"`
	TencentCaptchaRandstr string `json:"tencent_captcha_randstr"`
	PromoCode             string `json:"promo_code"`      // 注册优惠码
	InvitationCode        string `json:"invitation_code"` // 邀请码
	AffCode               string `json:"aff_code"`        // 邀请返利码
	CompanyWebsite        string `json:"company_website"` // 蜜罐：非空即拦截
}

// honeypotFilled 蜜罐被自动填充：非空白即判定为脚本机器人。
// 字段名是贴近 SaaS 注册场景的常规字段（company_website），肉眼不可见，
// 正常用户不会触发；命中时 handler 返回假成功（不发邮件、不建号），
// 避免暴露风控信号给对抗方。
//
// 不设任何豁免值：字段对浏览器端完全不可见，正常用户只可能留空；
// 任何非空内容都来自脚本自动填充，放行 "1"/"test" 之类只会给对抗方留后门。
func honeypotFilled(value string) bool {
	return strings.TrimSpace(value) != ""
}

// SendVerifyCodeRequest 发送验证码请求
type SendVerifyCodeRequest struct {
	Email                 string `json:"email" binding:"required,email"`
	ClickCaptchaToken     string `json:"click_captcha_token"`
	TurnstileToken        string `json:"turnstile_token"`
	TencentCaptchaTicket  string `json:"tencent_captcha_ticket"`
	TencentCaptchaRandstr string `json:"tencent_captcha_randstr"`
	CompanyWebsite        string `json:"company_website"` // 蜜罐：非空即拦截
}

// SendVerifyCodeResponse 发送验证码响应
type SendVerifyCodeResponse struct {
	Message   string `json:"message"`
	Countdown int    `json:"countdown"` // 倒计时秒数
}

// LoginRequest represents the login request payload
type LoginRequest struct {
	Email                 string `json:"email" binding:"required,email"`
	Password              string `json:"password" binding:"required"`
	TurnstileToken        string `json:"turnstile_token"`
	TencentCaptchaTicket  string `json:"tencent_captcha_ticket"`
	TencentCaptchaRandstr string `json:"tencent_captcha_randstr"`
}

func captchaProof(turnstileToken, tencentTicket, tencentRandstr string) service.CaptchaProof {
	return service.CaptchaProof{
		TurnstileToken: turnstileToken,
		TencentTicket:  tencentTicket,
		TencentRandstr: tencentRandstr,
	}
}

// AuthResponse 认证响应格式（匹配前端期望）
type AuthResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"` // 新增：Refresh Token
	ExpiresIn    int       `json:"expires_in,omitempty"`    // 新增：Access Token有效期（秒）
	TokenType    string    `json:"token_type"`
	User         *dto.User `json:"user"`
}

func ensureLoginUserActive(user *service.User) error {
	if user == nil {
		return infraerrors.Unauthorized("INVALID_USER", "user not found")
	}
	if !user.IsActive() {
		return service.ErrUserNotActive
	}
	return nil
}

// respondWithTokenPair 生成 Token 对并返回认证响应
// 如果 Token 对生成失败，回退到只返回 Access Token（向后兼容）
func (h *AuthHandler) respondWithTokenPair(c *gin.Context, user *service.User) {
	respondWithTokenPair(c, h.authService, user)
}

func respondWithTokenPair(c *gin.Context, authService *service.AuthService, user *service.User) {
	if err := ensureLoginUserActive(user); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	tokenPair, err := authService.GenerateTokenPair(c.Request.Context(), user, "")
	if err != nil {
		slog.Error("failed to generate token pair", "error", err, "user_id", user.ID)
		// 回退到只返回Access Token
		token, tokenErr := authService.GenerateToken(c.Request.Context(), user)
		if tokenErr != nil {
			response.InternalError(c, "Failed to generate token")
			return
		}
		response.Success(c, AuthResponse{
			AccessToken: token,
			TokenType:   "Bearer",
			User:        dto.UserFromService(user),
		})
		return
	}
	response.Success(c, AuthResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
		TokenType:    "Bearer",
		User:         dto.UserFromService(user),
	})
}

func (h *AuthHandler) ensureBackendModeAllowsUser(ctx context.Context, user *service.User) error {
	if user == nil {
		return infraerrors.Unauthorized("INVALID_USER", "user not found")
	}
	if h == nil || !h.isBackendModeEnabled(ctx) || user.IsAdmin() {
		return nil
	}
	return infraerrors.Forbidden("BACKEND_MODE_ADMIN_ONLY", "Backend mode is active. Only admin login is allowed.")
}

func (h *AuthHandler) ensureBackendModeAllowsNewUserLogin(ctx context.Context) error {
	if h == nil || !h.isBackendModeEnabled(ctx) {
		return nil
	}
	return infraerrors.Forbidden("BACKEND_MODE_ADMIN_ONLY", "Backend mode is active. Only admin login is allowed.")
}

func (h *AuthHandler) isBackendModeEnabled(ctx context.Context) bool {
	if h == nil || h.settingSvc == nil {
		return false
	}
	settings, err := h.settingSvc.GetPublicSettings(ctx)
	if err == nil && settings != nil {
		return settings.BackendModeEnabled
	}
	return h.settingSvc.IsBackendModeEnabled(ctx)
}

// enforceClickCaptchaForRegistration 邮箱验证开启且功能启用时返回 true。
func (h *AuthHandler) enforceClickCaptchaForRegistration(c *gin.Context) bool {
	if h == nil || h.clickCaptcha == nil || h.settingSvc == nil {
		return false
	}
	if !h.settingSvc.IsEmailVerifyEnabled(c.Request.Context()) {
		return false
	}
	return h.settingSvc.IsRegistrationClickCaptchaEnabled(c.Request.Context())
}

// validateClickCaptchaForEmailSending 对所有会触发发信的公开接口统一做自建点击验证码校验
// （非消费式校验，token 仍由后续提交动作消费）。开启自建验证码即强制生效，
// 防止机器人绕过前端直接调用接口刷邮件、打爆 SMTP 日配额。
func (h *AuthHandler) validateClickCaptchaForEmailSending(c *gin.Context, token string) error {
	if h == nil || h.clickCaptcha == nil || h.settingSvc == nil {
		return nil
	}
	if !h.settingSvc.IsRegistrationClickCaptchaEnabled(c.Request.Context()) {
		return nil
	}
	clientIP := middleware2.SecurityClientIP(c)
	ipHash := service.HashFingerprint(clientIP, c.Request.UserAgent())
	return h.clickCaptcha.ValidateToken(c.Request.Context(), token, ipHash, ipHash)
}

// enforceRegistrationIPHourlyRisk 同一 IP 1 小时内的注册数上限（默认 8，可在后台配置），
// 超出后直接拦截后续注册与发码请求。统计失败时放行，避免误伤正常用户。
func (h *AuthHandler) enforceRegistrationIPHourlyRisk(c *gin.Context, clientIP string) bool {
	if h == nil || h.authService == nil {
		return true
	}
	count, err := h.authService.CountRecentRegistrationsByIP(c.Request.Context(), clientIP)
	if err != nil {
		slog.Warn("[Register] count recent registrations by IP failed", "ip", clientIP, "error", err)
		return true
	}
	if count >= h.authService.GetRegistrationIPHourlyLimit(c.Request.Context()) {
		response.ErrorFrom(c, service.ErrRegistrationIPHourlyLimit)
		return false
	}
	return true
}

// CheckRegistrationEmail 注册邮箱风险预检：只做规则校验，不发送验证码。
// 前端在点击“继续”后调用，命中高危规则时直接提示并停留当前页。
func (h *AuthHandler) CheckRegistrationEmail(c *gin.Context) {
	var req struct {
		Email          string `json:"email" binding:"required"`
		CompanyWebsite string `json:"company_website"` // 蜜罐：非空即拦截
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 蜜罐命中：假装邮箱校验通过，不暴露风控信号。
	if honeypotFilled(req.CompanyWebsite) {
		response.Success(c, gin.H{"valid": true})
		return
	}

	if err := h.authService.ValidateRegistrationEmailForSignup(c.Request.Context(), req.Email); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"valid": true})
}

// Register handles user registration
// POST /api/v1/auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// 蜜罐命中：假装注册成功发了验证码，避免暴露风控信号给对抗方。
	// 实际上什么都没做（不建号、不消耗验证码、不占配额）。
	if honeypotFilled(req.CompanyWebsite) {
		response.Success(c, SendVerifyCodeResponse{
			Message:   "Verification code sent successfully",
			Countdown: 60,
		})
		return
	}

	// 注册 IP 取安全客户端 IP（与审计日志、使用记录同源），
	// 由 SessionBindingContext 中间件统一解析，避免与其他链路取值不一致。
	clientIP := middleware2.SecurityClientIP(c)
	// 验证当前启用的验证码（邮箱验证码注册场景避免重复校验一次性票据）
	proof := captchaProof(req.TurnstileToken, req.TencentCaptchaTicket, req.TencentCaptchaRandstr)
	if err := h.authService.VerifyCaptchaForRegister(c.Request.Context(), proof, clientIP, req.VerifyCode); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if !h.enforceRegistrationIPHourlyRisk(c, clientIP) {
		return
	}

	// 注册前原子消费点击验证 token：并发请求中只有一个能取到载荷，
	// 杜绝同一个 token 被并发用于注册多个账号。
	var consumedCaptcha *service.ClickCaptchaTokenPayloadRef
	if h.enforceClickCaptchaForRegistration(c) {
		ipHash := service.HashFingerprint(clientIP, c.Request.UserAgent())
		payload, err := h.clickCaptcha.ConsumeToken(c.Request.Context(), req.ClickCaptchaToken, ipHash, ipHash)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		consumedCaptcha = payload
	}

	_, user, err := h.authService.RegisterWithVerification(
		c.Request.Context(),
		req.Email,
		req.Password,
		req.VerifyCode,
		req.PromoCode,
		req.InvitationCode,
		req.AffCode,
		clientIP,
	)
	if err != nil {
		// 精确记录注册失败原因，便于定位 500 根因（ApplicationError 会映射为 4xx/5xx，裸 error 映射为 500）
		slog.Error("[Register] RegisterWithVerification failed", "error_type", fmt.Sprintf("%T", err), "error", err)
		// 建号失败归还已消费的 token，避免用户重新做一次人机验证。
		// 归还必须脱离请求上下文：客户端超时/取消会连带取消 c.Request.Context()，
		// 用被取消的上下文写 Redis 会让归还静默失败，token 被永久消费掉。
		if consumedCaptcha != nil {
			restoreCtx, restoreCancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 3*time.Second)
			_ = h.clickCaptcha.RestoreToken(restoreCtx, req.ClickCaptchaToken, consumedCaptcha)
			restoreCancel()
		}
		response.ErrorFrom(c, err)
		return
	}

	h.respondWithTokenPair(c, user)
}

// SendVerifyCode 发送邮箱验证码
// POST /api/v1/auth/send-verify-code
func (h *AuthHandler) SendVerifyCode(c *gin.Context) {
	var req SendVerifyCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// 蜜罐命中：假装流程正常，不做任何实际动作（不发邮件、不查配额）。
	if honeypotFilled(req.CompanyWebsite) {
		response.Success(c, SendVerifyCodeResponse{
			Message:   "Verification code sent successfully",
			Countdown: 60,
		})
		return
	}

	clientIP := middleware2.SecurityClientIP(c)

	proof := captchaProof(req.TurnstileToken, req.TencentCaptchaTicket, req.TencentCaptchaRandstr)
	if err := h.authService.VerifyCaptcha(c.Request.Context(), proof, clientIP); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 邮箱验证开启且启用点击验证时，发送验证码前必须持有有效的真人验证 token（非消费式校验），
	// 杜绝机器人绕过前端直接调用本接口刷邮件；token 仍由后续注册接口做一次性消费。
	if h.enforceClickCaptchaForRegistration(c) {
		ipHash := service.HashFingerprint(clientIP, c.Request.UserAgent())
		if err := h.clickCaptcha.ValidateToken(c.Request.Context(), req.ClickCaptchaToken, ipHash, ipHash); err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}

	// 同一 IP 1 小时内注册超过上限时不再下发验证码。
	if !h.enforceRegistrationIPHourlyRisk(c, clientIP) {
		return
	}

	result, err := h.authService.SendVerifyCodeAsync(c.Request.Context(), req.Email, c.GetHeader("Accept-Language"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, SendVerifyCodeResponse{
		Message:   "Verification code sent successfully",
		Countdown: result.Countdown,
	})
}

// Login handles user login
// POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	proof := captchaProof(req.TurnstileToken, req.TencentCaptchaTicket, req.TencentCaptchaRandstr)
	if err := h.authService.VerifyCaptcha(c.Request.Context(), proof, ip.GetClientIP(c)); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	token, user, err := h.authService.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	_ = token // token 由 authService.Login 返回但此处由 respondWithTokenPair 重新生成

	if err := h.ensureBackendModeAllowsUser(c.Request.Context(), user); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Check if TOTP 2FA is enabled for this user
	if h.totpService != nil && h.settingSvc.IsTotpEnabled(c.Request.Context()) && user.TotpEnabled {
		// Create a temporary login session for 2FA
		tempToken, err := h.totpService.CreateLoginSession(c.Request.Context(), user.ID, user.Email)
		if err != nil {
			response.InternalError(c, "Failed to create 2FA session")
			return
		}

		response.Success(c, TotpLoginResponse{
			Requires2FA:     true,
			TempToken:       tempToken,
			UserEmailMasked: service.MaskEmail(user.Email),
		})
		return
	}

	h.authService.RecordSuccessfulLogin(c.Request.Context(), user.ID)

	h.respondWithTokenPair(c, user)
}

// TotpLoginResponse represents the response when 2FA is required
type TotpLoginResponse struct {
	Requires2FA     bool   `json:"requires_2fa"`
	TempToken       string `json:"temp_token,omitempty"`
	UserEmailMasked string `json:"user_email_masked,omitempty"`
}

// Login2FARequest represents the 2FA login request
type Login2FARequest struct {
	TempToken string `json:"temp_token" binding:"required"`
	TotpCode  string `json:"totp_code" binding:"required,len=6"`
}

// Login2FA completes the login with 2FA verification
// POST /api/v1/auth/login/2fa
func (h *AuthHandler) Login2FA(c *gin.Context) {
	var req Login2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	slog.Debug("login_2fa_request",
		"temp_token_len", len(req.TempToken),
		"totp_code_len", len(req.TotpCode))

	// Get the login session
	session, err := h.totpService.GetLoginSession(c.Request.Context(), req.TempToken)
	if err != nil || session == nil {
		tokenPrefix := ""
		if len(req.TempToken) >= 8 {
			tokenPrefix = req.TempToken[:8]
		}
		slog.Debug("login_2fa_session_invalid",
			"temp_token_prefix", tokenPrefix,
			"error", err)
		response.BadRequest(c, "Invalid or expired 2FA session")
		return
	}

	slog.Debug("login_2fa_session_found",
		"user_id", session.UserID,
		"email", session.Email)

	// Verify the TOTP code
	if err := h.totpService.VerifyCode(c.Request.Context(), session.UserID, req.TotpCode); err != nil {
		slog.Debug("login_2fa_verify_failed",
			"user_id", session.UserID,
			"error", err)
		response.ErrorFrom(c, err)
		return
	}

	// Get the user (before session deletion so we can check backend mode)
	user, err := h.userService.GetByID(c.Request.Context(), session.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := ensureLoginUserActive(user); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if err := h.ensureBackendModeAllowsUser(c.Request.Context(), user); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if session.PendingOAuthBind != nil {
		pendingSvc, err := h.pendingIdentityService()
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}

		pendingSession, err := pendingSvc.GetBrowserSession(
			c.Request.Context(),
			session.PendingOAuthBind.PendingSessionToken,
			session.PendingOAuthBind.BrowserSessionKey,
		)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}

		decision, err := h.ensurePendingOAuthAdoptionDecision(c, pendingSession.ID, oauthAdoptionDecisionRequest{})
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		if err := applyPendingOAuthBinding(
			c.Request.Context(),
			h.entClient(),
			h.authService,
			h.userService,
			pendingSession,
			decision,
			&user.ID,
			true,
			true,
		); err != nil {
			response.ErrorFrom(c, infraerrors.InternalServer("PENDING_AUTH_BIND_APPLY_FAILED", "failed to bind pending oauth identity").WithCause(err))
			return
		}
		if _, err := pendingSvc.ConsumeBrowserSession(
			c.Request.Context(),
			pendingSession.SessionToken,
			pendingSession.BrowserSessionKey,
		); err != nil {
			response.ErrorFrom(c, err)
			return
		}

		secureCookie := isRequestHTTPS(c)
		clearOAuthPendingSessionCookie(c, secureCookie)
		clearOAuthPendingBrowserCookie(c, secureCookie)
		h.authService.RecordSuccessfulLogin(c.Request.Context(), user.ID)

		user, err = h.userService.GetByID(c.Request.Context(), session.UserID)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}

	// Delete the login session (only after all checks pass)
	_ = h.totpService.DeleteLoginSession(c.Request.Context(), req.TempToken)

	if session.PendingOAuthBind == nil {
		h.authService.RecordSuccessfulLogin(c.Request.Context(), user.ID)
	}

	h.respondWithTokenPair(c, user)
}

// GetCurrentUser handles getting current authenticated user
// GET /api/v1/auth/me
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	user, err := h.userService.GetByID(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	identities, err := h.userService.GetProfileIdentitySummaries(c.Request.Context(), subject.UserID, user)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	type UserResponse struct {
		userProfileResponse
		RunMode string `json:"run_mode"`
	}

	runMode := config.RunModeStandard
	if h.cfg != nil {
		runMode = h.cfg.RunMode
	}

	response.Success(c, UserResponse{
		userProfileResponse: userProfileResponseFromService(user, identities),
		RunMode:             runMode,
	})
}

// ValidatePromoCodeRequest 验证优惠码请求
type ValidatePromoCodeRequest struct {
	Code string `json:"code" binding:"required"`
}

// ValidatePromoCodeResponse 验证优惠码响应
type ValidatePromoCodeResponse struct {
	Valid       bool    `json:"valid"`
	BonusAmount float64 `json:"bonus_amount,omitempty"`
	ErrorCode   string  `json:"error_code,omitempty"`
	Message     string  `json:"message,omitempty"`
}

// ValidatePromoCode 验证优惠码（公开接口，注册前调用）
// POST /api/v1/auth/validate-promo-code
func (h *AuthHandler) ValidatePromoCode(c *gin.Context) {
	// 检查优惠码功能是否启用
	if h.settingSvc != nil && !h.settingSvc.IsPromoCodeEnabled(c.Request.Context()) {
		response.Success(c, ValidatePromoCodeResponse{
			Valid:     false,
			ErrorCode: "PROMO_CODE_DISABLED",
		})
		return
	}

	var req ValidatePromoCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	promoCode, err := h.promoService.ValidatePromoCode(c.Request.Context(), req.Code)
	if err != nil {
		// 根据错误类型返回对应的错误码
		errorCode := "PROMO_CODE_INVALID"
		switch err {
		case service.ErrPromoCodeNotFound:
			errorCode = "PROMO_CODE_NOT_FOUND"
		case service.ErrPromoCodeExpired:
			errorCode = "PROMO_CODE_EXPIRED"
		case service.ErrPromoCodeDisabled:
			errorCode = "PROMO_CODE_DISABLED"
		case service.ErrPromoCodeMaxUsed:
			errorCode = "PROMO_CODE_MAX_USED"
		case service.ErrPromoCodeAlreadyUsed:
			errorCode = "PROMO_CODE_ALREADY_USED"
		}

		response.Success(c, ValidatePromoCodeResponse{
			Valid:     false,
			ErrorCode: errorCode,
		})
		return
	}

	if promoCode == nil {
		response.Success(c, ValidatePromoCodeResponse{
			Valid:     false,
			ErrorCode: "PROMO_CODE_INVALID",
		})
		return
	}

	response.Success(c, ValidatePromoCodeResponse{
		Valid:       true,
		BonusAmount: promoCode.BonusAmount,
	})
}

// ValidateInvitationCodeRequest 验证邀请码请求
type ValidateInvitationCodeRequest struct {
	Code string `json:"code" binding:"required"`
}

// ValidateInvitationCodeResponse 验证邀请码响应
type ValidateInvitationCodeResponse struct {
	Valid     bool   `json:"valid"`
	ErrorCode string `json:"error_code,omitempty"`
}

// ValidateInvitationCode 验证邀请码（公开接口，注册前调用）
// POST /api/v1/auth/validate-invitation-code
func (h *AuthHandler) ValidateInvitationCode(c *gin.Context) {
	// 检查邀请码功能是否启用
	if h.settingSvc == nil || !h.settingSvc.IsInvitationCodeEnabled(c.Request.Context()) {
		response.Success(c, ValidateInvitationCodeResponse{
			Valid:     false,
			ErrorCode: "INVITATION_CODE_DISABLED",
		})
		return
	}

	var req ValidateInvitationCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// 验证邀请码
	redeemCode, err := h.redeemService.GetByCode(c.Request.Context(), req.Code)
	if err != nil {
		response.Success(c, ValidateInvitationCodeResponse{
			Valid:     false,
			ErrorCode: "INVITATION_CODE_NOT_FOUND",
		})
		return
	}

	// 检查类型和状态
	if redeemCode.Type != service.RedeemTypeInvitation {
		response.Success(c, ValidateInvitationCodeResponse{
			Valid:     false,
			ErrorCode: "INVITATION_CODE_INVALID",
		})
		return
	}

	if redeemCode.Status != service.StatusUnused {
		response.Success(c, ValidateInvitationCodeResponse{
			Valid:     false,
			ErrorCode: "INVITATION_CODE_USED",
		})
		return
	}

	response.Success(c, ValidateInvitationCodeResponse{
		Valid: true,
	})
}

// ForgotPasswordRequest 忘记密码请求
type ForgotPasswordRequest struct {
	Email                 string `json:"email" binding:"required,email"`
	ClickCaptchaToken     string `json:"click_captcha_token"`
	TurnstileToken        string `json:"turnstile_token"`
	TencentCaptchaTicket  string `json:"tencent_captcha_ticket"`
	TencentCaptchaRandstr string `json:"tencent_captcha_randstr"`
}

// ForgotPasswordResponse 忘记密码响应
type ForgotPasswordResponse struct {
	Message string `json:"message"`
}

// ForgotPassword 请求密码重置
// POST /api/v1/auth/forgot-password
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	proof := captchaProof(req.TurnstileToken, req.TencentCaptchaTicket, req.TencentCaptchaRandstr)
	if err := h.authService.VerifyCaptcha(c.Request.Context(), proof, ip.GetClientIP(c)); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := h.validateClickCaptchaForEmailSending(c, req.ClickCaptchaToken); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	frontendBaseURL := strings.TrimSpace(h.settingSvc.GetFrontendURL(c.Request.Context()))
	if frontendBaseURL == "" {
		slog.Error("frontend_url not configured in settings or config; cannot build password reset link")
		response.InternalError(c, "Password reset is not configured")
		return
	}

	// Request password reset (async)
	// Note: This returns success even if email doesn't exist (to prevent enumeration)
	if err := h.authService.RequestPasswordResetAsync(c.Request.Context(), req.Email, frontendBaseURL, c.GetHeader("Accept-Language")); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, ForgotPasswordResponse{
		Message: "If your email is registered, you will receive a password reset link shortly.",
	})
}

// ResetPasswordRequest 重置密码请求
type ResetPasswordRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=72"`
}

// ResetPasswordResponse 重置密码响应
type ResetPasswordResponse struct {
	Message string `json:"message"`
}

// ResetPassword 重置密码
// POST /api/v1/auth/reset-password
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Reset password
	if err := h.authService.ResetPassword(c.Request.Context(), req.Email, req.Token, req.NewPassword); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, ResetPasswordResponse{
		Message: "Your password has been reset successfully. You can now log in with your new password.",
	})
}

// ==================== Token Refresh Endpoints ====================

// RefreshTokenRequest 刷新Token请求
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// RefreshTokenResponse 刷新Token响应
type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"` // Access Token有效期（秒）
	TokenType    string `json:"token_type"`
}

// RefreshToken 刷新Token
// POST /api/v1/auth/refresh
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	result, err := h.authService.RefreshTokenPair(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Backend mode: block non-admin token refresh
	if h.settingSvc.IsBackendModeEnabled(c.Request.Context()) && result.UserRole != "admin" {
		response.Forbidden(c, "Backend mode is active. Only admin login is allowed.")
		return
	}

	response.Success(c, RefreshTokenResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    result.ExpiresIn,
		TokenType:    "Bearer",
	})
}

// LogoutRequest 登出请求
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token,omitempty"` // 可选：撤销指定的Refresh Token
}

// LogoutResponse 登出响应
type LogoutResponse struct {
	Message string `json:"message"`
}

// Logout 用户登出
// POST /api/v1/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	var req LogoutRequest
	// 允许空请求体（向后兼容）
	_ = c.ShouldBindJSON(&req)

	// 如果提供了Refresh Token，撤销它
	if req.RefreshToken != "" {
		if err := h.authService.RevokeRefreshToken(c.Request.Context(), req.RefreshToken); err != nil {
			slog.Debug("failed to revoke refresh token", "error", err)
			// 不影响登出流程
		}
	}
	h.consumePendingOAuthSessionOnLogout(c)
	clearOAuthLogoutCookies(c)

	response.Success(c, LogoutResponse{
		Message: "Logged out successfully",
	})
}

// RevokeAllSessionsResponse 撤销所有会话响应
type RevokeAllSessionsResponse struct {
	Message string `json:"message"`
}

// RevokeAllSessions 撤销当前用户的所有会话
// POST /api/v1/auth/revoke-all-sessions
func (h *AuthHandler) RevokeAllSessions(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	if err := h.authService.RevokeAllUserTokens(c.Request.Context(), subject.UserID); err != nil {
		slog.Error("failed to revoke all sessions", "user_id", subject.UserID, "error", err)
		response.InternalError(c, "Failed to revoke sessions")
		return
	}

	response.Success(c, RevokeAllSessionsResponse{
		Message: "All sessions have been revoked. Please log in again.",
	})
}
