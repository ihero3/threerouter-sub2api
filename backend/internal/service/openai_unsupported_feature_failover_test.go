package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// 线上真实报文：聚合型上游网关在 responses 阶段拒绝 web_search 工具。
const unsupportedFeatureRealBody = `{"error":{"code":"responses_feature_not_supported","message":"tool type 'web_search' is not supported by this gateway phase.","param":"","type":"unsupported_feature"}}`

// 无结构化 code、仅 type=unsupported_feature 的变体。
const unsupportedFeatureTypeOnlyBody = `{"error":{"message":"web_search is unavailable on this endpoint","type":"unsupported_feature"}}`

// 无结构化 code/type、仅 message 含 gateway-phase 措辞的兜底分支。
const unsupportedFeatureMsgOnlyBody = `{"error":{"message":"tool type 'web_search' is not supported by this gateway","type":"invalid_request_error"}}`

// 普通 400（参数错误），绝不能误判为可换号。
const unsupportedFeatureGenericBody = `{"error":{"code":"invalid_request_error","message":"Invalid 'tools' format","type":"invalid_request_error"}}`

func TestOpenAIUnsupportedToolFeatureError_Detector(t *testing.T) {
	require.True(t, isOpenAIUnsupportedToolFeatureError(http.StatusBadRequest, "", []byte(unsupportedFeatureRealBody)),
		"code=responses_feature_not_supported 必须识别")
	require.True(t, isOpenAIUnsupportedToolFeatureError(http.StatusBadRequest, "", []byte(unsupportedFeatureTypeOnlyBody)),
		"type=unsupported_feature 必须识别")
	require.True(t, isOpenAIUnsupportedToolFeatureError(http.StatusBadRequest,
		"tool type 'web_search' is not supported by this gateway phase.", nil),
		"仅有 upstreamMsg 的 gateway-phase 措辞必须识别")
	require.True(t, isOpenAIUnsupportedToolFeatureError(http.StatusBadRequest, "", []byte(unsupportedFeatureMsgOnlyBody)),
		"message 含 gateway-phase 措辞必须识别")

	require.False(t, isOpenAIUnsupportedToolFeatureError(http.StatusBadRequest, "", []byte(unsupportedFeatureGenericBody)),
		"普通 400 参数错误不能误判为可换号")
	require.False(t, isOpenAIUnsupportedToolFeatureError(http.StatusTooManyRequests, "", []byte(unsupportedFeatureRealBody)),
		"非 400 状态码不识别（429 走既有路径）")
	require.False(t, isOpenAIUnsupportedToolFeatureError(http.StatusBadRequest, "", []byte(contextWindowBody)),
		"上下文超长必须交给 isOpenAIContextWindowError，本检测器不应命中")
	require.False(t, isOpenAIUnsupportedToolFeatureError(http.StatusBadRequest, "", nil),
		"空报文不识别")
}

func TestOpenAIUnsupportedToolFeatureError_FailoverWorthy(t *testing.T) {
	require.True(t, (&OpenAIGatewayService{}).shouldFailoverOpenAIUpstreamResponse(http.StatusBadRequest, "", []byte(unsupportedFeatureRealBody)),
		"shouldFailoverOpenAIUpstreamResponse 必须对该 400 返回 true，网关才能自动换号重试")
	require.False(t, (&OpenAIGatewayService{}).shouldFailoverOpenAIUpstreamResponse(http.StatusBadRequest, "", []byte(unsupportedFeatureGenericBody)),
		"普通 400 不应被视为可换号")
}

func TestOpenAIUnsupportedToolFeatureError_FailoverErrorContracts(t *testing.T) {
	err := newOpenAIUpstreamFailoverError(http.StatusBadRequest, http.Header{}, []byte(unsupportedFeatureRealBody), "", false)

	require.Equal(t, NextAccountRetry, err.NextAccountAction, "必须尝试下一个上游账号")
	require.True(t, err.ShouldRetryNextAccount(), "ShouldRetryNextAccount 必须为真")
	require.Equal(t, GatewayFailureScopeAccount, err.Scope, "能力不匹配是账号级失败")
	require.Equal(t, OpenAIUnsupportedFeatureReason, err.Reason)
	require.False(t, err.RetryableOnSameAccount, "同账号重试无意义：本上游就是不支持该特性")
	require.False(t, err.RequestScopedTransient, "不是请求级瞬时故障")

	// 客户端文案必须统一，绝不透传上游内部措辞。
	require.NotEmpty(t, err.ClientMessage)
	require.NotContains(t, err.ClientMessage, "gateway phase", "不得透传上游网关内部措辞")
	require.Equal(t, http.StatusBadRequest, err.ClientStatusCode)
}
