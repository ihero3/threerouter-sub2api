package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// 本文件锁住「跨厂商 failover 状态码集合必须同口径」这一条。
//
// 缺陷原型：OpenAI/Anthropic/Bedrock 走 OpenAIGatewayService.shouldFailoverUpstreamError，
// 含 402/404/405；而 Gemini 与 Antigravity 各自维护一份只含 401/403/429/529 的副本。
// 结果是同一个 402（上游配额耗尽）在 OpenAI 上会换号重试、在 Gemini 上直接回错给客户端，
// 属于典型的「多厂商 adapter 横向不一致」。

func TestFailoverStatusSetsAreConsistentAcrossVendors(t *testing.T) {
	t.Parallel()

	openAI := &OpenAIGatewayService{}
	gemini := &GeminiMessagesCompatService{}
	antigravity := &AntigravityGatewayService{}

	statuses := []int{
		http.StatusUnauthorized,        // 401
		http.StatusPaymentRequired,     // 402
		http.StatusForbidden,           // 403
		http.StatusNotFound,            // 404
		http.StatusMethodNotAllowed,    // 405
		http.StatusTooManyRequests,     // 429
		529,                            // overloaded
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusBadRequest,          // 400 —— 客户端错误，不该换号
		422,                            // 语义错误，不该换号
	}
	for _, status := range statuses {
		want := openAI.shouldFailoverUpstreamError(status)
		require.Equal(t, want, gemini.shouldFailoverGeminiUpstreamError(status),
			"Gemini 与 OpenAI 口径不一致: status=%d", status)
		require.Equal(t, want, antigravity.shouldFailoverUpstreamError(status),
			"Antigravity 与 OpenAI 口径不一致: status=%d", status)
	}
}

func TestFailoverStatusSetsFailoverOnAccountScopedStatuses(t *testing.T) {
	t.Parallel()

	// 逐条钉死语义，防止有人把整份表改成全 true 来"通过"上一个测试。
	gemini := &GeminiMessagesCompatService{}
	antigravity := &AntigravityGatewayService{}

	for _, status := range []int{
		http.StatusUnauthorized,
		http.StatusPaymentRequired,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusMethodNotAllowed,
		http.StatusTooManyRequests,
		529,
		http.StatusInternalServerError,
		http.StatusServiceUnavailable,
	} {
		require.True(t, gemini.shouldFailoverGeminiUpstreamError(status), "Gemini status=%d", status)
		require.True(t, antigravity.shouldFailoverUpstreamError(status), "Antigravity status=%d", status)
	}

	// 反向护栏：400/422 是客户端请求问题，换号重试没有意义。
	for _, status := range []int{http.StatusBadRequest, 422} {
		require.False(t, gemini.shouldFailoverGeminiUpstreamError(status), "Gemini status=%d", status)
		require.False(t, antigravity.shouldFailoverUpstreamError(status), "Antigravity status=%d", status)
	}
}
