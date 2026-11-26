package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestBuildOpenAIEmbeddingsURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		base string
		want string
	}{
		{"bare domain", "https://api.openai.com", "https://api.openai.com/v1/embeddings"},
		{"bare /v1", "https://api.openai.com/v1", "https://api.openai.com/v1/embeddings"},
		{"already embeddings", "https://api.openai.com/v1/embeddings", "https://api.openai.com/v1/embeddings"},
		{"third-party versioned path", "https://open.bigmodel.cn/api/paas/v4", "https://open.bigmodel.cn/api/paas/v4/embeddings"},
		// 2026-11-26 事故回归：base_url 配成完整 chat 端点（chat 请求可用的写法）时，
		// embeddings 不得拼出 /v1/chat/completions/v1/embeddings（曾致同组账号被 404 扫禁）。
		{"base is chat completions endpoint", "https://neutoken.ai/v1/chat/completions", "https://neutoken.ai/v1/embeddings"},
		{"base is completions endpoint", "https://host.example/v1/completions", "https://host.example/v1/embeddings"},
		{"base is responses endpoint", "https://host.example/v1/responses", "https://host.example/v1/embeddings"},
		{"base is responses input_tokens endpoint", "https://host.example/v1/responses/input_tokens", "https://host.example/v1/embeddings"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, buildOpenAIEmbeddingsURL(tt.base))
		})
	}
}

func TestForwardEmbeddings_APIKeyPassthroughRecordsUsageAndBatchInput(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reqBody := []byte(`{
		"model":"nowledge-embedding",
		"input":["hello","world"],
		"encoding_format":"float",
		"dimensions":256
	}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Request-Id": []string{"emb-rid"},
		},
		Body: io.NopCloser(strings.NewReader(`{
			"object":"list",
			"data":[
				{"object":"embedding","index":0,"embedding":[0.1,0.2]},
				{"object":"embedding","index":1,"embedding":[0.3,0.4]}
			],
			"model":"jina-embeddings-v5-text-small",
			"usage":{"prompt_tokens":13,"total_tokens":13}
		}`)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:       42,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://api.jina.ai",
			"model_mapping": map[string]any{
				"nowledge-embedding": "jina-embeddings-v5-text-small",
			},
		},
	}

	result, err := svc.ForwardEmbeddings(context.Background(), c, account, reqBody, "")

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, result)
	require.Equal(t, "emb-rid", result.RequestID)
	require.Equal(t, "nowledge-embedding", result.Model)
	require.Equal(t, "jina-embeddings-v5-text-small", result.BillingModel)
	require.Equal(t, "jina-embeddings-v5-text-small", result.UpstreamModel)
	require.Equal(t, 13, result.Usage.InputTokens)
	require.Equal(t, 0, result.Usage.OutputTokens)
	require.Equal(t, "https://api.jina.ai/v1/embeddings", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer sk-test", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "jina-embeddings-v5-text-small", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, int64(2), gjson.GetBytes(upstream.lastBody, "input.#").Int())
	require.Equal(t, "hello", gjson.GetBytes(upstream.lastBody, "input.0").String())
	require.Equal(t, "world", gjson.GetBytes(upstream.lastBody, "input.1").String())
	require.Equal(t, "float", gjson.GetBytes(upstream.lastBody, "encoding_format").String())
	require.Equal(t, int64(256), gjson.GetBytes(upstream.lastBody, "dimensions").Int())
}

func TestForwardEmbeddings_AccessStateUsesTypedFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reqBody := []byte(`{"model":"text-embedding-3-small","input":"hello"}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(reqBody))

	upstreamBody := []byte(`{"error":{"code":"deactivated_workspace","message":"Workspace is deactivated"}}`)
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusForbidden,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Request-Id": []string{"req_embeddings_access_state"},
		},
		Body: io.NopCloser(bytes.NewReader(upstreamBody)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{
		ID:       43,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
	}

	result, err := svc.ForwardEmbeddings(context.Background(), c, account, reqBody, "")

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusForbidden, failoverErr.StatusCode)
	require.Equal(t, GatewayFailureStageAccountAuth, failoverErr.Stage)
	require.Equal(t, GatewayFailureScopeAccount, failoverErr.Scope)
	require.Equal(t, OpenAIUpstreamAccessStateReason, failoverErr.Reason)
	require.Equal(t, NextAccountRetry, failoverErr.NextAccountAction)
	require.Equal(t, http.StatusBadGateway, failoverErr.ClientStatusCode)
	require.Equal(t, openAIUpstreamAccessUnavailableClientMessage, failoverErr.ClientMessage)
	require.False(t, failoverErr.RetryableOnSameAccount)
	require.Equal(t, "req_embeddings_access_state", failoverErr.ResponseHeaders.Get("x-request-id"))
	require.False(t, c.Writer.Written())
}

func TestForwardEmbeddings_NonAccessFailoverKeepsLegacyShape(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reqBody := []byte(`{"model":"text-embedding-3-small","input":"hello"}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(reqBody))

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"rate limited"}}`)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{
		ID:       44,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
	}

	result, err := svc.ForwardEmbeddings(context.Background(), c, account, reqBody, "")

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
	require.Empty(t, failoverErr.Stage)
	require.Empty(t, failoverErr.Scope)
	require.Empty(t, failoverErr.Reason)
	require.Zero(t, failoverErr.ClientStatusCode)
	require.Empty(t, failoverErr.ClientMessage)
	require.Nil(t, failoverErr.ResponseHeaders)
	require.False(t, c.Writer.Written())
}
