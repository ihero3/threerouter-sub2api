package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// 线上真实报文：OpenAI 兼容上游把模型不存在表达成 400，
// 并把 HTTP 状态码回填到 error.code（而不是语义化的 model_not_found）。
const openAIModelNotSupportedRealBody = `{"error":{"code":"400","message":"Unsupported model deepseek-v4-pro"}}`

func TestOpenAIModelNotSupported_IsFailoverWorthy(t *testing.T) {
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	svc := &OpenAIGatewayService{accountRepo: &openAIStream403AccountRepo{}}

	require.True(t,
		svc.shouldFailoverOpenAIUpstreamResponse(account, http.StatusBadRequest, "Unsupported model deepseek-v4-pro", []byte(openAIModelNotSupportedRealBody)),
		"上游明确说不支持该模型时必须换号，而不是把通用 400 直接回给客户端",
	)

	// 没有账号选择器可消费 failover 哨兵时仍保持确定性 400（单账号/直接调用）。
	require.False(t,
		(&OpenAIGatewayService{}).shouldFailoverOpenAIUpstreamResponse(account, http.StatusBadRequest, "Unsupported model deepseek-v4-pro", []byte(openAIModelNotSupportedRealBody)),
	)

	// 透传模式绕过账号映射，模型可用性同样按账号判定，可换号。
	require.True(t,
		shouldFailoverOpenAIPassthroughResponse(account, http.StatusBadRequest, []byte(openAIModelNotSupportedRealBody)),
	)
	require.False(t,
		shouldFailoverOpenAIPassthroughResponse(nil, http.StatusBadRequest, []byte(openAIModelNotSupportedRealBody)),
	)
}

func TestOpenAIModelNotSupported_FailoverErrorContract(t *testing.T) {
	err := newOpenAIUpstreamFailoverError(
		http.StatusBadRequest,
		http.Header{},
		[]byte(openAIModelNotSupportedRealBody),
		"Unsupported model deepseek-v4-pro",
		false,
	)

	require.Equal(t, OpenAIModelNotSupportedReason, err.Reason)
	require.Equal(t, NextAccountRetry, err.NextAccountAction)
	require.True(t, err.ShouldRetryNextAccount(), "其他账号可能映射/接入该模型，必须继续换号")
	require.Equal(t, GatewayFailureScopeAccount, err.Scope, "模型能力属于账号级失败")
	require.False(t, err.RetryableOnSameAccount, "同账号重试必然再次失败")
	require.False(t, err.RequestScopedTransient)
	require.Equal(t, http.StatusNotFound, err.ClientStatusCode)
	require.Equal(t, OpenAIModelNotSupportedClientMessage, err.ClientMessage)
	require.NotContains(t, err.ClientMessage, "deepseek", "不得把账号映射后的内部上游模型名下发给客户端")
}

func TestOpenAIModelNotSupported_Generic400KeepsOldContract(t *testing.T) {
	body := []byte(`{"error":{"code":"invalid_request_error","message":"Invalid value for temperature"}}`)
	err := newOpenAIUpstreamFailoverError(http.StatusBadRequest, http.Header{}, body, "Invalid value for temperature", false)

	require.NotEqual(t, OpenAIModelNotSupportedReason, err.Reason)
	require.Zero(t, err.ClientStatusCode)
	require.Empty(t, err.ClientMessage)
}
