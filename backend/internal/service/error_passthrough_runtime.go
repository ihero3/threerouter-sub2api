package service

import (
	"strings"

	"github.com/gin-gonic/gin"
)

const errorPassthroughServiceContextKey = "error_passthrough_service"

// BindErrorPassthroughService 将错误透传服务绑定到请求上下文，供 service 层在非 failover 场景下复用规则。
func BindErrorPassthroughService(c *gin.Context, svc *ErrorPassthroughService) {
	if c == nil || svc == nil {
		return
	}
	c.Set(errorPassthroughServiceContextKey, svc)
}

func getBoundErrorPassthroughService(c *gin.Context) *ErrorPassthroughService {
	if c == nil {
		return nil
	}
	v, ok := c.Get(errorPassthroughServiceContextKey)
	if !ok {
		return nil
	}
	svc, ok := v.(*ErrorPassthroughService)
	if !ok {
		return nil
	}
	return svc
}

// applyErrorPassthroughRule 按规则决定是否对客户端响应使用透传状态码。
// 状态码：PassthroughCode=true 透传上游状态码，否则使用规则配置的 ResponseCode。
// 消息（仅管理员显式配置的两种模式才改写默认平台文案）：
//   - PassthroughBody=true：透传上游响应体中的错误消息（管理员主动选择放行原文）；
//   - 否则 CustomMessage 非空：使用管理员配置的自定义文案；
//   - 两者都没有时保持调用方传入的平台统一文案，绝不隐式回显上游原文。
//
// 上游完整错误始终通过 OpsUpstreamErrorEvent 记录给管理员。
func applyErrorPassthroughRule(
	c *gin.Context,
	platform string,
	upstreamStatus int,
	responseBody []byte,
	defaultStatus int,
	defaultErrType string,
	defaultErrMsg string,
) (status int, errType string, errMsg string, matched bool) {
	status = defaultStatus
	errType = defaultErrType
	errMsg = defaultErrMsg

	svc := getBoundErrorPassthroughService(c)
	if svc == nil {
		return status, errType, errMsg, false
	}

	rule := svc.MatchRule(platform, upstreamStatus, responseBody)
	if rule == nil {
		return status, errType, errMsg, false
	}

	status = upstreamStatus
	if !rule.PassthroughCode && rule.ResponseCode != nil {
		status = *rule.ResponseCode
	}

	// 命中 skip_monitoring 时在 context 中标记，供 ops_error_logger 跳过记录。
	if rule.SkipMonitoring {
		c.Set(OpsSkipPassthroughKey, true)
	}

	errType = "upstream_error"

	switch {
	case rule.PassthroughBody:
		// 管理员显式放行上游原始错误消息。
		if upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(responseBody)); upstreamMsg != "" {
			errMsg = upstreamMsg
		}
	case rule.CustomMessage != nil:
		if custom := strings.TrimSpace(*rule.CustomMessage); custom != "" {
			errMsg = custom
		}
	}

	return status, errType, errMsg, true
}
