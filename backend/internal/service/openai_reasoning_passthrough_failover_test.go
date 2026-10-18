package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// 本工单真实报文：DeepSeek /v1/responses 报 reasoning_text 未回传（chat→responses 桥接路径）。
const reasoningTextPassbackBody = `{"error":{"code":"invalid_request_error","message":"The ` + "`reasoning_text`" + ` in the thinking mode must be passed back to the API. (request_id: 420bfef4-f9e0-4406-8f67-bcac17ecf76c)","type":"invalid_request_error"}}`

// CC→Responses 桥接变体：报 reasoning_content 未回传。
const reasoningContentPassbackBody = `{"error":{"message":"The ` + "`reasoning_content`" + ` in the thinking mode must be passed back to the API.","type":"invalid_request_error"}}`

// 普通 400，不应被误判为可换号。
const genericBadRequestBodyForPassback = `{"error":{"message":"Invalid 'tools' format","type":"invalid_request_error"}}`

func TestOpenAIReasoningPassthrough_Detector(t *testing.T) {
	require.True(t, isOpenAIReasoningPassthroughError(http.StatusBadRequest, "", []byte(reasoningTextPassbackBody)),
		"reasoning_text + passed back 必须识别")
	require.True(t, isOpenAIReasoningPassthroughError(http.StatusBadRequest, "", []byte(reasoningContentPassbackBody)),
		"reasoning_content + passed back 必须识别")

	require.False(t, isOpenAIReasoningPassthroughError(http.StatusBadRequest, "", []byte(genericBadRequestBodyForPassback)),
		"普通 400 不能误判为可换号")
	require.False(t, isOpenAIReasoningPassthroughError(http.StatusTooManyRequests, "", []byte(reasoningTextPassbackBody)),
		"非 400 状态码不识别（429 走既有路径）")
}

func TestOpenAIReasoningPassthrough_FailoverWorthy(t *testing.T) {
	require.True(t, (&OpenAIGatewayService{}).shouldFailoverOpenAIUpstreamResponse(http.StatusBadRequest, "", []byte(reasoningTextPassbackBody)),
		"shouldFailoverOpenAIUpstreamResponse 必须对该 400 返回 true（native 与 CC 路径共用）")
	require.False(t, (&OpenAIGatewayService{}).shouldFailoverOpenAIUpstreamResponse(http.StatusBadRequest, "", []byte(genericBadRequestBodyForPassback)),
		"普通 400 不应被视为可换号")
}

func TestOpenAIReasoningPassthrough_FailoverErrorContracts(t *testing.T) {
	err := newOpenAIUpstreamFailoverError(http.StatusBadRequest, http.Header{}, []byte(reasoningTextPassbackBody), "", false)

	require.Equal(t, NextAccountRetry, err.NextAccountAction, "必须尝试下一个上游账号（=换源）")
	require.True(t, err.ShouldRetryNextAccount(), "ShouldRetryNextAccount 必须为真")
	require.Equal(t, GatewayFailureScopeAccount, err.Scope)
	require.Equal(t, OpenAIReasoningPassthroughReason, err.Reason)
	require.False(t, err.RetryableOnSameAccount, "同账号重试无意义：同一请求体在任何同模型账号都必失败")

	// 客户端文案必须统一，绝不透传上游原文。
	require.NotEmpty(t, err.ClientMessage)
	require.Equal(t, http.StatusBadRequest, err.ClientStatusCode)
	require.NotContains(t, err.ClientMessage, "passed back", "不得透传上游原文")
	require.NotContains(t, err.ClientMessage, "reasoning_text", "不得透传上游内部字段名")
	require.NotContains(t, err.ClientMessage, "420bfef4", "不得透传上游 request_id")
}
