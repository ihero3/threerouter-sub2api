package handler

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestFailoverPassthroughNeverLeaksUpstreamMessage 锁死 failover 耗尽命中透传规则时
// 客户端文案的来源。
//
// error_passthrough_runtime.go 已明确「PassthroughBody 不再影响客户端文案，上游原文
// 一律不下发」，gateway_handler.go:handleFailoverExhausted 也是这么实现的；但
// OpenAI 原生网关与 Gemini v1beta 网关各自维护了一份兄弟出口，仍把
// ExtractUpstreamErrorMessage 的结果直接写进 error.message，泄漏上游账号状态、
// 配额策略、内网地址等实现细节。这里静态锁死两处口径一致。
func TestFailoverPassthroughNeverLeaksUpstreamMessage(t *testing.T) {
	for _, file := range []string{"openai_gateway_handler.go", "gemini_v1beta_handler.go"} {
		t.Run(file, func(t *testing.T) {
			src, err := os.ReadFile(file)
			require.NoError(t, err)
			body := string(src)
			require.NotContains(t, body, "msg := service.ExtractUpstreamErrorMessage(responseBody)",
				"上游原文不得作为客户端可见 message")
			require.Contains(t, body, "service.MapUpstreamErrorToClient(statusCode)",
				"命中透传规则时必须使用平台统一文案")
			require.Contains(t, body, "service.SetOpsUpstreamError(c, statusCode, upstreamMsg, \"\")",
				"上游原文只允许进 OpsUpstreamErrorEvent 留档")
		})
	}
}
