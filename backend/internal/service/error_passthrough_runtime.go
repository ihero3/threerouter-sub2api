package service

import (
	"strings"

	"github.com/gin-gonic/gin"
)

const errorPassthroughServiceContextKey = "error_passthrough_service"

// passthroughCustomMessageMaxLen 是管理员自定义文案回显给终端用户时的长度上限
// （字节）。防御性截断：文案由管理员填写，但不应对下游造成无界响应。
// 上游原文不在此列——它根本不会进入客户端响应。
const passthroughCustomMessageMaxLen = 2000

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
// 消息（仅管理员显式配置自定义文案时才改写默认平台文案）：
//   - CustomMessage 非空：使用管理员配置的自定义文案；
//   - 否则保持调用方传入的平台统一文案。
//
// 上游原文一律不下发给终端用户：
//   - 这里曾经在 PassthroughBody=true 时回显上游 message。那会把上游的内部实现
//     细节（账号状态、限流策略、配额、内网地址、provider 名称等）暴露给调用方，
//     与「上游错误下游用户不可见」的既定策略相悖，已移除。PassthroughBody 字段
//     保留仅为兼容既有规则数据，不再影响客户端文案。
//   - 响应体只在管理员侧留痕：上游完整错误始终通过 OpsUpstreamErrorEvent 记录。
//     （调用方各自提取的 upstreamMsg 也只进 Go error / 运维日志，不进响应体。）
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

	// 只接受管理员自己写的文案；上游响应体不参与客户端文案构造。
	if rule.CustomMessage != nil {
		if custom := strings.TrimSpace(*rule.CustomMessage); custom != "" {
			errMsg = truncateString(custom, passthroughCustomMessageMaxLen)
		}
	}

	return status, errType, errMsg, true
}
