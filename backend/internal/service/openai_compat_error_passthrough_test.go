package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 本文件锁住「兼容路径（ChatCompletions / Anthropic）错误兜底不得下发上游原文」这一拍板。
//
// 缺陷原型：handleCompatErrorResponse 的通用兜底（openai_gateway_upstream_errors.go:873）
// 直接把 upstreamMsg 回显给客户端，而原生 Responses 路径（handleErrorResponse）对
// 5xx / 401 / 402 / 403 / 429 早已改用平台统一文案。两者不一致，且违反
// 「上游模型服务商的错误日志不得下发客户端」。

func newCompatErrorTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	return c, rec
}

func newCompatErrorAccount() *Account {
	return &Account{ID: 1, Platform: PlatformOpenAI, Name: "a"}
}

// 自定义错误码已开启、且列表不含目标状态码的账号：ShouldHandleErrorCode 返回 false，
// 命中 handleCompatErrorResponse 的「未处理状态码」早退分支（Upstream gateway error）。
func newCompatErrorAccountCustomCodesExcluding(excluded int) *Account {
	codes := make([]any, 0, 1)
	if excluded != 400 {
		codes = append(codes, float64(400))
	}
	return &Account{
		ID:       1,
		Type:     AccountTypeAPIKey,
		Platform: PlatformOpenAI,
		Name:     "a",
		Credentials: map[string]any{
			"custom_error_codes_enabled": true,
			"custom_error_codes":         codes,
		},
	}
}

func TestHandleCompatErrorResponse_GenericErrorDoesNotLeakUpstream(t *testing.T) {
	t.Parallel()

	// 通用 500（非确定性、非 context-window）：客户端文案只能是平台话术，
	// 不能夹带上游服务商的内部错误细节（这里埋了一个特征串用于回退期检测）。
	const upstreamFingerprint = "PROVIDER_INTERNAL_OOM_TRACE_xyz"
	body := `{"error":{"message":"` + upstreamFingerprint + ` upstream backend panic"}}`

	c, _ := newCompatErrorTestContext()
	var gotStatus int
	var gotMsg string
	writeError := func(_ *gin.Context, statusCode int, _ string, message string) {
		gotStatus, gotMsg = statusCode, message
	}
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	_, err := (&OpenAIGatewayService{}).handleCompatErrorResponse(resp, c, newCompatErrorAccount(), writeError)
	require.Error(t, err)
	require.Equal(t, http.StatusInternalServerError, gotStatus)
	require.NotContains(t, gotMsg, upstreamFingerprint, "通用错误不得下发上游原文")
	require.Equal(t, "Upstream request failed", gotMsg, "通用错误应回平台统一文案")
}

func TestHandleCompatErrorResponse_Deterministic400StillEchoed(t *testing.T) {
	t.Parallel()

	// 确定性 400 是客户端请求错误（含 code/param 帮助定位字段），属刻意保留的回显，
	// 不算「上游服务商内部日志」。回归：这条路径的 message 必须仍是上游原文。
	body := `{"error":{"type":"invalid_request_error","message":"Invalid schema for function 'foo': got None"}}`

	c, _ := newCompatErrorTestContext()
	var gotMsg string
	writeError := func(_ *gin.Context, _ int, _ string, message string) {
		gotMsg = message
	}
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	_, err := (&OpenAIGatewayService{}).handleCompatErrorResponse(resp, c, newCompatErrorAccount(), writeError)
	require.Error(t, err)
	require.Contains(t, gotMsg, "Invalid schema for function 'foo'")
}

func TestHandleCompatErrorResponse_UnhandledCodeUsesPlatformMessage(t *testing.T) {
	t.Parallel()

	// 账号未启用自定义错误码时，未命中状态码走「Upstream gateway error」平台文案，
	// 同样不得下发上游原文。
	const upstreamFingerprint = "PROVIDER_QUOTA_BACKEND_DETAIL"
	body := `{"error":{"message":"` + upstreamFingerprint + `"}}`

	c, _ := newCompatErrorTestContext()
	var gotMsg string
	writeError := func(_ *gin.Context, _ int, _ string, message string) {
		gotMsg = message
	}
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	// 自定义错误码开启但不含 500 → 命中「未处理状态码」早退分支，文案为平台话术。
	_, err := (&OpenAIGatewayService{}).handleCompatErrorResponse(resp, c, newCompatErrorAccountCustomCodesExcluding(500), writeError)
	require.Error(t, err)
	require.NotContains(t, gotMsg, upstreamFingerprint)
	require.Equal(t, "Upstream gateway error", gotMsg)
}
