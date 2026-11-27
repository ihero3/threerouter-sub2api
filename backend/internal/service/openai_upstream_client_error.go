package service

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// openAIUpstreamClientErrorFallbackType 是上游没给 error.type 时的兜底值。
// 与 handleCompatErrorResponse 对 400 的取值保持一致。
const openAIUpstreamClientErrorFallbackType = "invalid_request_error"

// openAIUpstreamClientErrorFallbackMessage 是上游连 message 都没给时的兜底文案。
// 仍然比 "Upstream request failed" 明确：它说明拒绝来自请求本身，而不是链路故障。
const openAIUpstreamClientErrorFallbackMessage = "Upstream rejected the request"

// OpenAIClientParameterErrorClientMessage 是上游 400 明确属于「请求参数非法」时
// 回给客户端的统一文案。不点名内部上游、不回显上游原文（客户端文案红线），
// 但必须让用户知道是参数/模型兼容性问题并可自行修正——旧文案
// "Upstream rejected the request" 只说明被拒绝，用户无从下手。
const OpenAIClientParameterErrorClientMessage = "Invalid request parameters: the selected model rejected the request body. Check the request parameters or use a model that supports them"

// isOpenAIDeterministicClientError 判断上游状态码是否表示「请求本身非法」。
//
// 只认 400：同一份请求体换任何账号、重试多少次都会得到同样的结果。
//   - 401/402/403 是网关运营方的凭据/账单问题，继续包成 502，不向客户端暴露上游账号状态。
//   - 404/405 既可能是模型不存在，也可能是上游 base_url 配错，同属运营方问题，保持现状。
//   - 429 已有独立分支映射成 429。
//   - 413 在更上面就按 request-body-too-large 走 failover 了。
func isOpenAIDeterministicClientError(statusCode int) bool {
	return statusCode == http.StatusBadRequest
}

// openAICompatibleClientParameterPhrases 是上游把「请求参数非法」表达成 400 的
// 常见措辞（含 neutoken 的 "Invalid request parameters."、OpenAI 的
// "unknown parameter"/"invalid schema for"、vLLM/SGLang 的 "extra fields not
// permitted" 等）。只在 400 且正文没有模型级/上下文级语义时使用。
var openAICompatibleClientParameterPhrases = []string{
	"invalid request parameters",
	"invalid request parameter",
	"invalid parameters",
	"invalid parameter",
	"invalid value for",
	"missing required parameter",
	"missing required field",
	"unknown parameter",
	"unrecognized request argument",
	"unexpected parameter",
	"unsupported parameter",
	"extra fields not permitted",
	"invalid schema for",
	"invalid request body",
	"request body is invalid",
	"request body is not valid",
	"malformed request",
	"invalid field",
	"unknown field",
}

// openAIUpstreamAccount400Keywords 是 400 里明确属于账号/凭据/账单问题的措辞。
// 这些即使带 invalid_request_error 类型也不能被当成客户端参数错误，否则会绕过
// 既有的停用/冷却逻辑（organization disabled / KYC / 余额不足）。
var openAIUpstreamAccount400Keywords = []string{
	"organization has been disabled",
	"organization is disabled",
	"identity verification is required",
	"credit balance",
	"insufficient balance",
	"insufficient quota",
	"account has been disabled",
	"account is disabled",
	"account has been deactivated",
	"billing issue",
	"payment required",
	"invalid api key",
	"api key is invalid",
	"authentication failed",
	"permission denied",
}

// isOpenAICompatibleClientParameterError 判断上游 400 是否确定性地由客户端请求
// 参数引起（如 "Invalid request parameters."）。这类失败的特征是：同一份请求体
// 换任何账号都会被同样拒绝，账号本身仍然健康——因此不得冷却/停用上游账号，
// 也不应触发 failover（换号只会放大上游 400 调用）。
//
// 模型不存在/上下文超限/max_completion_tokens 超限/reasoning 回传缺失/工具特性
// 不支持等 400 有专属分类（可能换号成功），必须先排除，避免误吞它们的 failover。
func isOpenAICompatibleClientParameterError(statusCode int, body []byte) bool {
	if statusCode != http.StatusBadRequest || len(body) == 0 {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(body)))
	if msg == "" && !gjson.ValidBytes(body) {
		msg = strings.ToLower(strings.TrimSpace(string(body)))
	}
	if msg == "" {
		return false
	}
	// 可重试的 400（OpenAI 瞬时处理失败 / 容量抖动）也常带 invalid_request_error
	// 类型，必须先排除：它们需要 failover 或同账号重试，不是确定性参数错误。
	if isOpenAITransientProcessingError(statusCode, msg, body) ||
		isOpenAIRequestScopedCapacityShed(msg, body) {
		return false
	}
	for _, keyword := range openAIUpstreamAccount400Keywords {
		if strings.Contains(msg, keyword) {
			return false
		}
	}
	if isOpenAICompatibleModelNotFoundBody(body) ||
		isOpenAIContextWindowError("", body) ||
		isOpenAIMaxCompletionTokensExceeded(statusCode, "", body) ||
		isOpenAIReasoningPassthroughError(statusCode, "", body) ||
		isOpenAIUnsupportedToolFeatureError(statusCode, "", body) ||
		isOpenAICodexPlanGatedModelError(statusCode, body) ||
		isOpenAIHTTPUpstreamAccessStateError(statusCode, "", body) {
		return false
	}
	for _, phrase := range openAICompatibleClientParameterPhrases {
		if strings.Contains(msg, phrase) {
			return true
		}
	}
	// 结构化类型/错误码已明确是 invalid_request_error 时同样按客户端参数错误处理；
	// 但没有正文短语时只认这类语义化字段，避免把任意 400 都吞成参数错误。
	switch strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "error.type").String())) {
	case "invalid_request_error", "invalid_request":
		return true
	}
	switch strings.ToLower(strings.TrimSpace(extractUpstreamErrorCode(body))) {
	case "invalid_request_error", "invalid_request", "invalid_parameter", "invalid_argument", "bad_request":
		return true
	}
	return false
}

// openAIWrapperErrorArtifacts 是上游网关包装层回填的伪 type/code（把 HTTP 状态码
// 包成 bad_response_status_code 等），不具客户端可编程语义。回给客户端时归一成
// 标准 invalid_request_error，避免 SDK 按未知 type 走异常分支。
var openAIWrapperErrorArtifacts = map[string]struct{}{
	"bad_response_status_code": {},
}

func isOpenAIWrapperErrorArtifact(value string) bool {
	_, ok := openAIWrapperErrorArtifacts[strings.ToLower(strings.TrimSpace(value))]
	return ok
}

// writeOpenAIUpstreamClientError 以 OpenAI 错误体形状回写确定性客户端错误。
//
// 拍板（2026-09-24，本次落实）：上游原文一律不下发客户端。message 改用平台统一话术
// （openAIUpstreamClientMessage），不再回显上游文本。type/code/param 是结构化字段，
// 由 body 透传，帮助客户端定位非法字段（如 input[8].tools[...].parameters），不属于
// 上游内部错误日志，予以保留；包装层伪字段除外。
func writeOpenAIUpstreamClientError(c *gin.Context, statusCode int, body []byte) {
	// 参数错误归因给客户端：Ops 记为 request/client_request（不再算上游故障），
	// 与「不冷却/不停用上游账号」的口径一致。
	if isOpenAICompatibleClientParameterError(statusCode, body) {
		MarkOpsRequestScopedError(c, openAIUpstreamClientErrorFallbackType)
	}
	// 写出完整错误体后必须标记已提交：handler 侧
	// （ensureForwardErrorResponse / ensureOpenAIStreamReadErrorResponse）据此跳过
	// 兜底写入，否则同一条响应会被追加第二份错误。
	MarkResponseCommitted(c)
	errorPayload := gin.H{"type": openAIUpstreamClientErrorFallbackType}
	if errType := strings.TrimSpace(gjson.GetBytes(body, "error.type").String()); errType != "" && !isOpenAIWrapperErrorArtifact(errType) {
		errorPayload["type"] = errType
	}
	if code := strings.TrimSpace(extractUpstreamErrorCode(body)); code != "" && !isOpenAIWrapperErrorArtifact(code) {
		errorPayload["code"] = code
	}
	if param := strings.TrimSpace(gjson.GetBytes(body, "error.param").String()); param != "" {
		errorPayload["param"] = param
	}
	// 拍板：message 一律平台统一文案，绝不下发上游原文。
	errorPayload["message"] = openAIUpstreamClientMessage(statusCode)

	c.JSON(statusCode, gin.H{"error": errorPayload})
}

// WriteOpenAIUpstreamClientError preserves a structured deterministic upstream
// client error when the handler has exhausted all eligible accounts.
func WriteOpenAIUpstreamClientError(c *gin.Context, statusCode int, body []byte, upstreamMsg string) {
	// 本地策略：上游原文一律不下发，upstreamMsg 仅保留签名兼容，不参与响应。
	_ = upstreamMsg
	writeOpenAIUpstreamClientError(c, statusCode, body)
}
