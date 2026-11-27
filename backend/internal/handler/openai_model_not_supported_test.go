package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func newModelNotSupportedHandlerContext(path string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, path, nil)
	setOpsRequestContext(c, "claude-3-5-haiku-20241022", false)
	return c, rec
}

func newModelNotSupportedFailoverError() *service.UpstreamFailoverError {
	return &service.UpstreamFailoverError{
		StatusCode:       http.StatusBadRequest,
		ResponseBody:     []byte(`{"error":{"code":"400","message":"Unsupported model deepseek-v4-pro"}}`),
		Reason:           service.OpenAIModelNotSupportedReason,
		ClientStatusCode: http.StatusNotFound,
		ClientMessage:    service.OpenAIModelNotSupportedClientMessage,
	}
}

// /v1/messages（Anthropic 形态）换号耗尽后必须返回明确的 model_not_found，
// 而不是 502/400 的通用 "Upstream request failed / rejected"。
func TestHandleAnthropicFailoverExhausted_ModelNotSupported(t *testing.T) {
	c, rec := newModelNotSupportedHandlerContext("/v1/messages")

	(&OpenAIGatewayHandler{}).handleAnthropicFailoverExhausted(c, newModelNotSupportedFailoverError(), false)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, "error", gjson.Get(rec.Body.String(), "type").String())
	require.Equal(t, "model_not_found", gjson.Get(rec.Body.String(), "error.type").String())
	require.Equal(t, `Model "claude-3-5-haiku-20241022" is not supported by any available upstream account`,
		gjson.Get(rec.Body.String(), "error.message").String())
	require.NotContains(t, rec.Body.String(), "deepseek", "不得泄露账号映射后的内部上游模型名")
	require.NotContains(t, rec.Body.String(), "Upstream rejected the request")
}

// OpenAI 形态端点（/v1/responses 等）同样返回 404 + model_not_found。
func TestHandleFailoverExhausted_ModelNotSupported(t *testing.T) {
	c, rec := newModelNotSupportedHandlerContext("/v1/responses")

	(&OpenAIGatewayHandler{}).handleFailoverExhausted(c, newModelNotSupportedFailoverError(), false)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, "model_not_found", gjson.Get(rec.Body.String(), "error.type").String())
	require.Equal(t, `Model "claude-3-5-haiku-20241022" is not supported by any available upstream account`,
		gjson.Get(rec.Body.String(), "error.message").String())
	require.NotContains(t, rec.Body.String(), "deepseek")
}

// 只有可识别的模型级 400 才走该分支；普通参数 400 仍保持原语义（不谎报
// model_not_found）。
func TestHandleFailoverExhausted_Generic400Unchanged(t *testing.T) {
	c, rec := newModelNotSupportedHandlerContext("/v1/responses")
	err := &service.UpstreamFailoverError{
		StatusCode:   http.StatusBadRequest,
		ResponseBody: []byte(`{"error":{"code":"invalid_request_error","message":"Invalid value for temperature"}}`),
	}

	(&OpenAIGatewayHandler{}).handleFailoverExhausted(c, err, false)

	require.NotEqual(t, http.StatusNotFound, rec.Code)
	require.NotEqual(t, "model_not_found", gjson.Get(rec.Body.String(), "error.type").String())
}
