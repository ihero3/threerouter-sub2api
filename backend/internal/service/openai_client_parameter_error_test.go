package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// 线上真实报文（#161382，2026-11-27 02:14，account A1-未智数 2 / neutoken）：
// 上游把请求参数错误包成 bad_response_status_code，正文只有一句
// "Invalid request parameters."。这类失败与账号健康无关。
const openAIClientParameterErrorRealBody = `{"error":{"code":"bad_response_status_code","message":"Invalid request parameters. (request id: 202611261814037118484978268d9d65xhJOItA)","param":"","type":"bad_response_status_code"}}`

const openAIDeepSeekSupportedModelListBody = `{"error":{"code":"invalid_request_error","message":"The supported API model names are deepseek-flash, deepseek-v4-pro, but you passed claude-opus-5. (request_id: 91bcaec4-af90-48d2-8a54-349f2695e9c7)","param":null,"type":"invalid_request_error"}}`

func TestIsOpenAICompatibleClientParameterError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		want       bool
	}{
		{
			name:       "neutoken invalid request parameters",
			statusCode: http.StatusBadRequest,
			body:       openAIClientParameterErrorRealBody,
			want:       true,
		},
		{
			name:       "invalid value for parameter",
			statusCode: http.StatusBadRequest,
			body:       `{"error":{"code":"invalid_request_error","message":"Invalid value for temperature"}}`,
			want:       true,
		},
		{
			name:       "unknown parameter",
			statusCode: http.StatusBadRequest,
			body:       `{"error":{"message":"Unknown parameter: 'input[0].status'.","param":"input[0].status"}}`,
			want:       true,
		},
		{
			name:       "invalid schema",
			statusCode: http.StatusBadRequest,
			body:       `{"error":{"type":"invalid_request_error","message":"Invalid schema for function 'foo': got None"}}`,
			want:       true,
		},
		{
			name:       "model not found stays model scoped",
			statusCode: http.StatusBadRequest,
			body:       `{"error":{"code":"model_not_found","message":"model not found"}}`,
			want:       false,
		},
		{
			name:       "unsupported model stays model scoped",
			statusCode: http.StatusBadRequest,
			body:       `{"error":{"code":"400","message":"Unsupported model deepseek-v4-pro"}}`,
			want:       false,
		},
		{
			name:       "deepseek supported model list stays model scoped",
			statusCode: http.StatusBadRequest,
			body:       openAIDeepSeekSupportedModelListBody,
			want:       false,
		},
		{
			name:       "context window stays terminal",
			statusCode: http.StatusBadRequest,
			body:       `{"error":{"code":"context_length_exceeded","message":"This model's maximum context length is 8192 tokens"}}`,
			want:       false,
		},
		{
			name:       "max completion tokens stays failover eligible",
			statusCode: http.StatusBadRequest,
			body:       `{"error":{"param":"max_completion_tokens","message":"max_completion_tokens is too large: 250000. This model supports at most 131072"}}`,
			want:       false,
		},
		{
			name:       "organization disabled is account level",
			statusCode: http.StatusBadRequest,
			body:       `{"error":{"type":"invalid_request_error","message":"organization has been disabled"}}`,
			want:       false,
		},
		{
			name:       "identity verification is account level",
			statusCode: http.StatusBadRequest,
			body:       `{"error":{"type":"invalid_request_error","message":"Identity verification is required"}}`,
			want:       false,
		},
		{
			name:       "generic 400 stays unclassified",
			statusCode: http.StatusBadRequest,
			body:       `{"error":{"message":"something went wrong"}}`,
			want:       false,
		},
		{
			name:       "non 400 never matches",
			statusCode: http.StatusInternalServerError,
			body:       openAIClientParameterErrorRealBody,
			want:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isOpenAICompatibleClientParameterError(tt.statusCode, []byte(tt.body)))
		})
	}
}

// 参数错误不得触发 failover：同一份请求体换号也只会得到同一个 400。
func TestShouldFailover_ClientParameterErrorStaysTerminal(t *testing.T) {
	svc := &OpenAIGatewayService{accountRepo: &modelNotFoundManagedAccountRepo{}}
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	require.False(t, svc.shouldFailoverOpenAIUpstreamResponse(
		account, http.StatusBadRequest, "Invalid request parameters.", []byte(openAIClientParameterErrorRealBody),
	), "参数错误换号不会成功，不得放大成 N 次上游 400")
	require.False(t, shouldFailoverOpenAIPassthroughResponse(
		account, http.StatusBadRequest, []byte(openAIClientParameterErrorRealBody),
	))
}

type clientParameterAccountRepoStub struct {
	AccountRepository
	tempCalls  int
	modelCalls int
}

func (r *clientParameterAccountRepoStub) SetTempUnschedulable(context.Context, int64, time.Time, string) error {
	r.tempCalls++
	return nil
}

func (r *clientParameterAccountRepoStub) SetModelRateLimit(context.Context, int64, string, time.Time, ...string) error {
	r.modelCalls++
	return nil
}

func newClientParameterTempRuleAccount() *Account {
	return &Account{
		ID:          17,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"temp_unschedulable_enabled": true,
			"temp_unschedulable_rules": []any{
				map[string]any{
					"error_code":       float64(http.StatusBadRequest),
					"keywords":         []any{"invalid request"},
					"duration_minutes": float64(30),
				},
			},
		},
	}
}

// 即使管理员给 400 配了临时不可调度规则（关键词命中 "invalid request"），
// 参数错误也不得冷却/停用上游账号或写 (account, model) 冷却：其他用户仍要用它。
func TestClientParameterError_DoesNotCoolOrDisableAccount(t *testing.T) {
	body := []byte(openAIClientParameterErrorRealBody)
	account := newClientParameterTempRuleAccount()
	repo := &clientParameterAccountRepoStub{}
	svc := &RateLimitService{accountRepo: repo}

	require.NotEmpty(t, matchTempUnschedulableRules(account, http.StatusBadRequest, body),
		"前提：账号的 400 关键字规则确实会命中该报文")

	require.False(t, svc.HandleUpstreamError(context.Background(), account, http.StatusBadRequest, nil, body, "kimi-k3"))
	require.Zero(t, repo.tempCalls, "参数错误不得写账号级临时冷却")
	require.Zero(t, repo.modelCalls, "参数错误不得写模型级冷却")

	require.False(t, svc.HandleTempUnschedulable(context.Background(), account, http.StatusBadRequest, body, "kimi-k3"))
	require.Zero(t, repo.tempCalls)
	require.Zero(t, repo.modelCalls)

	require.False(t, svc.tryTempUnschedulable(context.Background(), account, http.StatusBadRequest, body, "kimi-k3"))
	require.Zero(t, repo.tempCalls)
	require.Zero(t, repo.modelCalls)
}

// 客户端必须拿到可操作的参数提示（纯英文、不含上游原文），Ops 归因给客户端
// 而不是 provider，避免一次用户参数错误污染上游错误率/触发上游冷却看板。
func TestHandleErrorResponse_ClientParameterErrorGivesPromptAndClientAttribution(t *testing.T) {
	c, rec := newOpenAIUpstreamErrorTestContext(t)
	svc := &OpenAIGatewayService{cfg: &config.Config{}}

	_, err := svc.handleErrorResponse(
		context.Background(),
		newOpenAIUpstreamErrorResponse(http.StatusBadRequest, openAIClientParameterErrorRealBody),
		c, newOpenAIUpstreamErrorTestAccount(), nil,
	)
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	out := rec.Body.String()
	require.Equal(t, "invalid_request_error", gjson.Get(out, "error.type").String())
	require.Equal(t, OpenAIClientParameterErrorClientMessage, gjson.Get(out, "error.message").String())
	require.Contains(t, gjson.Get(out, "error.message").String(), "Invalid request parameters")
	require.NotContains(t, out, "202611261814037118484978268d9d65xhJOItA", "不得回显上游 request id")
	require.NotContains(t, out, "Upstream request failed")

	require.True(t, IsOpsRequestScopedError(c), "参数错误必须按请求级错误归因")
	errType, _ := OpsLocalErrorType(c)
	require.Equal(t, "invalid_request_error", errType)
}

// DeepSeek 官方把「模型不在支持清单」包在 invalid_request_error 里，必须仍按
// 模型级能力错误处理（可换号、给 model_not_found），不能被参数错误分类吞掉。
func TestOpenAIDeepSeekSupportedModelListMismatchIsModelScoped(t *testing.T) {
	body := []byte(openAIDeepSeekSupportedModelListBody)
	require.False(t, isOpenAICompatibleClientParameterError(http.StatusBadRequest, body))
	require.True(t, isOpenAICompatibleModelNotFoundBody(body))

	svc := &OpenAIGatewayService{accountRepo: &modelNotFoundManagedAccountRepo{}}
	account := &Account{ID: 48, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	require.True(t, svc.shouldFailoverOpenAIUpstreamResponse(account, http.StatusBadRequest, "", body))

	failoverErr := newOpenAIUpstreamFailoverError(http.StatusBadRequest, http.Header{}, body, "", false)
	require.Equal(t, OpenAIModelNotSupportedReason, failoverErr.Reason)
	require.Equal(t, http.StatusNotFound, failoverErr.ClientStatusCode)
	require.NotContains(t, failoverErr.ClientMessage, "deepseek")
}
