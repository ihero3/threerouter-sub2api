package handler

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 400 是确定性请求参数错误，不是上游链路故障：不能再被映射成 502
// upstream_error（客户端会按可重试故障反复重放），也不能只回
// "Upstream request failed" 让人无从下手。
func TestMapUpstreamError_400IsClientParameterError(t *testing.T) {
	status, errType, message := (&OpenAIGatewayHandler{}).mapUpstreamError(http.StatusBadRequest)

	require.Equal(t, http.StatusBadRequest, status)
	require.Equal(t, "invalid_request_error", errType)
	require.Equal(t, service.OpenAIClientParameterErrorClientMessage, message)
}
