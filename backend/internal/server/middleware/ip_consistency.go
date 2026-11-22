package middleware

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// enforceUserIPConsistency 在鉴权通过后校验用户 IP 一致性。
//
// 判定与处罚都在 service 层：窗口内出现 3 个及以上不同客户端 IP 时，统一处置为
// suspect 标记 + 扣减到 floor + 通知邮件（不再自动禁用）。返回 false 表示请求已被中断。
func enforceUserIPConsistency(
	c *gin.Context,
	ipConsistency *service.IPConsistencyService,
	userID int64,
	role string,
) bool {
	if ipConsistency == nil {
		return true
	}
	violation := ipConsistency.CheckUser(c.Request.Context(), userID, role, SecurityClientIP(c))
	if violation == nil {
		return true
	}
	MarkIngressRejected(c, IngressRejectUserInactive)
	AbortWithError(c, http.StatusForbidden, "USER_IP_MISMATCH",
		"Account restricted: multiple client IPs detected within the monitoring window")
	return false
}

// UserIPConsistencyGuard 鉴权之后按 context 中的 AuthSubject 校验 IP 一致性。
// 用于那些不走 api_key_auth / jwt_auth 的鉴权链路（如 Gemini 原生端点）。
func UserIPConsistencyGuard(ipConsistency *service.IPConsistencyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if ipConsistency == nil {
			c.Next()
			return
		}
		subject, ok := GetAuthSubjectFromContext(c)
		if !ok {
			c.Next()
			return
		}
		role := c.GetString(string(ContextKeyUserRole))
		if !enforceUserIPConsistency(c, ipConsistency, subject.UserID, role) {
			return
		}
		c.Next()
	}
}

// enforceNewUserFirstUseIP 校验新用户注册 IP 严格规则（注册后 3 分钟内任意一次调用）；
// 返回 false 表示请求已中断。只在 API Key 鉴权链路调用，JWT 控制台链路不参与。
func enforceNewUserFirstUseIP(
	c *gin.Context,
	ipConsistency *service.IPConsistencyService,
	user *service.User,
	googleStyle bool,
) bool {
	if ipConsistency == nil || user == nil {
		return true
	}
	violation := ipConsistency.CheckNewUserFirstUseIP(
		c.Request.Context(),
		user.ID,
		user.Role,
		SecurityClientIP(c),
		user.RegisterIP,
		user.CreatedAt,
	)
	if violation == nil {
		return true
	}
	MarkIngressRejected(c, IngressRejectUserInactive)
	message := "Account restricted: registration IP mismatch detected shortly after signup"
	if googleStyle {
		abortWithGoogleError(c, http.StatusForbidden, message)
	} else {
		AbortWithError(c, http.StatusForbidden, "USER_IP_MISMATCH", message)
	}
	return false
}

// NewUserFirstUseIPGuard 给「API Key 已从 context 取出」的鉴权链路补上新用户注册 IP 校验。
// 用于 Google 风格鉴权（/v1beta、/antigravity/v1beta），避免在多个鉴权构造函数间反复传参。
func NewUserFirstUseIPGuard(ipConsistency *service.IPConsistencyService, googleStyle bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if ipConsistency == nil {
			c.Next()
			return
		}
		apiKey, ok := GetAPIKeyFromContext(c)
		if !ok || apiKey == nil || apiKey.User == nil {
			c.Next()
			return
		}
		if !enforceNewUserFirstUseIP(c, ipConsistency, apiKey.User, googleStyle) {
			return
		}
		c.Next()
	}
}
