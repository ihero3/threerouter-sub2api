package middleware

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// enforceUserIPConsistency 在鉴权通过后校验用户 IP 一致性。
//
// 判定与处罚都在 service 层：窗口内出现两个及以上不同客户端 IP 时，
// 账号会被自动置为 disabled。返回 false 表示请求已被中断。
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
		"Account disabled: client IP changed within the monitoring window")
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
