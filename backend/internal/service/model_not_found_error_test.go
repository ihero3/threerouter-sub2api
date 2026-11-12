package service

import (
	"net/http"
	"testing"
)

func TestIsUpstreamModelNotFoundError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       []byte
		want       bool
	}{
		{
			name:       "404 model not found message",
			statusCode: http.StatusNotFound,
			body:       []byte(`{"error":{"message":"model not found"}}`),
			want:       true,
		},
		{
			name:       "404 model_not_found code",
			statusCode: http.StatusNotFound,
			body:       []byte(`{"error":{"code":"model_not_found","message":"The requested model was not found"}}`),
			want:       true,
		},
		{
			name:       "404 unknown model message",
			statusCode: http.StatusNotFound,
			body:       []byte(`{"error":{"message":"unknown model gpt-5.4"}}`),
			want:       true,
		},
		{
			name:       "404 endpoint not found is not model specific",
			statusCode: http.StatusNotFound,
			body:       []byte(`{"error":{"message":"endpoint not found"}}`),
			want:       false,
		},
		{
			name:       "404 arbitrary body is not model specific",
			statusCode: http.StatusNotFound,
			body:       []byte(`404 page not found`),
			want:       false,
		},
		{
			name:       "non 404 does not match",
			statusCode: http.StatusBadRequest,
			body:       []byte(`{"error":{"message":"model not found"}}`),
			want:       false,
		},
		{
			name:       "403 model not found message (upstream returns 403 for missing model)",
			statusCode: http.StatusForbidden,
			body:       []byte(`{"error":{"message":"model not found"}}`),
			want:       true,
		},
		{
			name:       "403 chinese model not exist (deepseek-v3-flash 不存在)",
			statusCode: http.StatusForbidden,
			body:       []byte(`{"error":"模型 deepseek-v3-flash 不存在或暂时不可用"}`),
			want:       true,
		},
		{
			name:       "403 model does not exist (english)",
			statusCode: http.StatusForbidden,
			body:       []byte(`{"error":{"message":"Model 'gpt-5.6' does not exist"}}`),
			want:       true,
		},
		{
			name:       "403 account not exist is not model specific",
			statusCode: http.StatusForbidden,
			body:       []byte(`{"error":{"message":"账号不存在"}}`),
			want:       false,
		},
		{
			name:       "403 chinese token routing rule (deepseek-v4-flash 不在令牌路由规则中)",
			statusCode: http.StatusForbidden,
			body:       []byte(`{"error":"模型 deepseek-v4-flash 不在令牌路由规则中"}`),
			want:       true,
		},
		{
			name:       "403 chinese model unavailable without 暂时 prefix",
			statusCode: http.StatusForbidden,
			body:       []byte(`{"error":"模型 deepseek-v4-flash 不可用"}`),
			want:       true,
		},
		{
			name:       "403 english not in token routing rules",
			statusCode: http.StatusForbidden,
			body:       []byte(`{"error":{"message":"Model 'deepseek-v4-flash' is not in the token routing rules"}}`),
			want:       true,
		},
		{
			// 精度护栏：账号级禁用消息即便提及「模型」但无可用性否定信号，仍应判定为非模型级
			// （保持可停用），验证「模型 + 否定信号」启发式不会把纯账号故障误判为模型级而漏停用。
			name:       "403 account banned mentioning model but no negation signal stays terminal",
			statusCode: http.StatusForbidden,
			body:       []byte(`{"error":"您的账号已被禁用，无法使用该模型"}`),
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUpstreamModelNotFoundError(tt.statusCode, tt.body); got != tt.want {
				t.Fatalf("isUpstreamModelNotFoundError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAntigravityModelNotFoundKeepsBare404Fallback(t *testing.T) {
	if !isModelNotFoundError(http.StatusNotFound, []byte(`endpoint not found`)) {
		t.Fatal("antigravity model-not-found helper should keep bare 404 fallback")
	}
}

func TestIsOpenAICodexPlanGatedModelError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       []byte
		want       bool
	}{
		{
			name:       "400 codex plan gated detail payload",
			statusCode: http.StatusBadRequest,
			body:       []byte(`{"detail":"The 'gpt-5.6-sol' model is not supported when using Codex with a ChatGPT account."}`),
			want:       true,
		},
		{
			name:       "400 codex plan gated error message payload",
			statusCode: http.StatusBadRequest,
			body:       []byte(`{"error":{"message":"The 'gpt-5.4' model is not supported when using Codex with a ChatGPT account."}}`),
			want:       true,
		},
		{
			name:       "400 unrelated invalid request does not match",
			statusCode: http.StatusBadRequest,
			body:       []byte(`{"error":{"message":"Invalid schema for response_format 'agentic_plan'"}}`),
			want:       false,
		},
		{
			name:       "404 with plan gated message does not match",
			statusCode: http.StatusNotFound,
			body:       []byte(`{"detail":"The 'gpt-5.6-sol' model is not supported when using Codex with a ChatGPT account."}`),
			want:       false,
		},
		{
			name:       "400 empty body does not match",
			statusCode: http.StatusBadRequest,
			body:       nil,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isOpenAICodexPlanGatedModelError(tt.statusCode, tt.body); got != tt.want {
				t.Fatalf("isOpenAICodexPlanGatedModelError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsOpenAICompatibleModelNotFound400(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{name: "structured code", body: `{"error":{"code":"model_not_found","message":"No such deployment"}}`, want: true},
		{name: "unknown provider", body: `{"error":{"message":"Unknown provider for model claude-x"}}`, want: true},
		{name: "model not found", body: `{"error":{"message":"Model not found: claude-x"}}`, want: true},
		{name: "unknown model", body: `{"error":{"message":"unknown model: claude-x"}}`, want: true},
		{name: "model unsupported", body: `{"error":{"message":"The requested model is not supported"}}`, want: true},
		{name: "plain text compatible gateway", body: `unknown provider for model claude-x`, want: true},
		{name: "invalid parameter remains terminal", body: `{"error":{"code":"invalid_request_error","message":"Invalid value for temperature"}}`, want: false},
		{name: "structured non-matching code overrides model not found message", body: `{"error":{"code":"invalid_request_error","message":"Model not found: claude-x"}}`, want: false},
		{name: "structured non-matching error code overrides unsupported message", body: `{"error":{"code":"upstream_error","message":"The requested model is not supported"}}`, want: false},
		{name: "echoed phrase is ignored", body: `{"error":{"message":"Invalid request"},"echo":"model not found"}`, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isOpenAICompatibleModelNotFound400([]byte(tt.body)); got != tt.want {
				t.Fatalf("isOpenAICompatibleModelNotFound400() = %v, want %v", got, tt.want)
			}
		})
	}
}
