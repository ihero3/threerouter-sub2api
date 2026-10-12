package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// 线上真实报文：火山方舟 glm-5-3-flash 收到 max_completion_tokens=250000（模型上限 131072）。
// 注意 message 里用反引号包裹了参数名。
const maxCompletionTokensRealBody = `{"error":{"code":"InvalidParameter","message":"The parameter ` + "`max_completion_tokens`" + ` specified in the request are not valid: integer above maximum value, expected a value <= 131072, but got 250000 instead. Request id: 021791682039806cf23db4d38572816d86b5d934e5fbaa16c2d31","param":"max_completion_tokens","type":"BadRequest"}}`

// param 为空、但 message 含 max_completion_tokens + 超上限措辞（兜底分支）。
const maxCompletionTokensEmptyParamBody = `{"error":{"message":"max_completion_tokens is not valid: expected a value <= 8192, but got 32000 instead","type":"invalid_request_error"}}`

// 普通 400，不应被误判为可换号。
const genericBadRequestBody = `{"error":{"message":"Invalid 'tools' format","type":"invalid_request_error"}}`

// 上下文超长：走 isOpenAIContextWindowError，与本检测器正交。
const contextWindowBody = `{"error":{"message":"This model's maximum context length is 128000 tokens. However, you requested 200000 tokens.","type":"invalid_request_error"}}`

func TestOpenAIMaxCompletionTokensExceeded_Detector(t *testing.T) {
	require.True(t, isOpenAIMaxCompletionTokensExceeded(http.StatusBadRequest, "", []byte(maxCompletionTokensRealBody)),
		"上游 error.param==max_completion_tokens 必须识别")
	require.True(t, isOpenAIMaxCompletionTokensExceeded(http.StatusBadRequest, "", []byte(maxCompletionTokensEmptyParamBody)),
		"param 为空但 message 含 max_completion_tokens + 超上限措辞也必须识别")

	require.False(t, isOpenAIMaxCompletionTokensExceeded(http.StatusBadRequest, "", []byte(genericBadRequestBody)),
		"普通 400 不能误判为可换号")
	require.False(t, isOpenAIMaxCompletionTokensExceeded(http.StatusBadRequest, "", []byte(contextWindowBody)),
		"上下文超长必须交给 isOpenAIContextWindowError，本检测器不应命中")
	require.False(t, isOpenAIMaxCompletionTokensExceeded(http.StatusTooManyRequests, "", []byte(maxCompletionTokensRealBody)),
		"非 400 状态码不识别（429 走既有路径）")
}

func TestOpenAIMaxCompletionTokensExceeded_FailoverWorthy(t *testing.T) {
	require.True(t, (&OpenAIGatewayService{}).shouldFailoverOpenAIUpstreamResponse(http.StatusBadRequest, "", []byte(maxCompletionTokensRealBody)),
		"shouldFailoverOpenAIUpstreamResponse 必须对该 400 返回 true（native 与 CC 路径共用）")
	require.False(t, (&OpenAIGatewayService{}).shouldFailoverOpenAIUpstreamResponse(http.StatusBadRequest, "", []byte(genericBadRequestBody)),
		"普通 400 不应被视为可换号")
}

func TestOpenAIMaxCompletionTokensExceeded_FailoverErrorContracts(t *testing.T) {
	err := newOpenAIUpstreamFailoverError(http.StatusBadRequest, http.Header{}, []byte(maxCompletionTokensRealBody), "", false)

	require.Equal(t, NextAccountRetry, err.NextAccountAction, "必须尝试下一个上游账号")
	require.True(t, err.ShouldRetryNextAccount(), "ShouldRetryNextAccount 必须为真")
	require.Equal(t, GatewayFailureScopeAccount, err.Scope)
	require.Equal(t, openAIMaxCompletionTokensReason, err.Reason)
	require.False(t, err.RetryableOnSameAccount, "同账号重试无意义：本账号模型就是上限低")

	// 客户端文案必须统一，绝不透传上游内部数字。
	require.NotEmpty(t, err.ClientMessage)
	require.NotContains(t, err.ClientMessage, "250000", "不得透传客户端请求值")
	require.NotContains(t, err.ClientMessage, "131072", "不得透传上游模型上限")
	require.NotContains(t, err.ClientMessage, "max_completion_tokens", "不得回显上游参数名原文")
	require.Equal(t, http.StatusBadRequest, err.ClientStatusCode)
}
