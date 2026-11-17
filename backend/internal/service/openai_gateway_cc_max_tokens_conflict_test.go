package service

import (
	"io"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// ---------------------------------------------------------------------------
// CC 出口自愈：max_tokens 与 max_completion_tokens 同传的 400。
//
// 验收标准与其余四类自愈一致：
//   - 上游明确报"两个不能同传"→ 允许重发一次，第二次体只保留 max_completion_tokens；
//   - 请求体里两个字段不齐 → 一次都不重发（那种 400 不是本问题的成因）；
//   - 400 与本问题无关 → 不重发；
//   - 重发仍失败 → 交回**原始**响应，失败路径零行为变化。
//
// 注意：本文件**不加 //go:build unit**（与 openai_cc_json_schema_selfheal_test.go
// 同口径），HTTP 桩直接复用那里的 ccSelfHealUpstream。
// ---------------------------------------------------------------------------

// ccMaxTokensConflictRejectionBody 是线上工单 725d5416 的上游原文（type=api_error，
// param 为空，message 点名 max_tokens 与 max_completion_tokens 互斥）。
const ccMaxTokensConflictRejectionBody = `{"error":{"code":"BadRequest","message":"The parameter ` + "`max_tokens`" + ` specified in the request are not valid: max_tokens and max_completion_tokens cannot be set at the same time.","param":"","type":"api_error"}}`

// ccMaxTokensBothBody 同时下发新旧两个额度字段（客户端为兼容新旧模型时的常见写法）。
const ccMaxTokensBothBody = `{"model":"deepseek-v4.1-flash-1017","messages":[{"role":"user","content":"hi"}],"max_tokens":4096,"max_completion_tokens":4096}`

// ccMaxTokensOnlyBody 只发旧字段。
const ccMaxTokensOnlyBody = `{"model":"deepseek-v4.1-flash-1017","messages":[{"role":"user","content":"hi"}],"max_tokens":4096}`

// 客户端同传两个字段：按上游说明删掉 max_tokens 后重发一次，成功响应交回调用方。
func TestCCMaxTokensConflictSelfHealRetriesOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream := newCCSelfHealService(
		ccSelfHealScriptedResponse{status: http.StatusBadRequest, body: ccMaxTokensConflictRejectionBody},
		ccSelfHealScriptedResponse{status: http.StatusOK, body: `{"id":"chatcmpl_ok","choices":[]}`},
	)

	resp := sendCCSelfHealRequest(t, svc, newCCSelfHealContext(), []byte(ccMaxTokensBothBody))

	require.Equal(t, http.StatusOK, resp.StatusCode, "重发成功时应把成功的响应交回调用方")
	require.Equal(t, 2, upstream.callCount(), "只允许重发一次，不允许递归")

	retried := upstream.bodyAt(1)
	require.False(t, gjson.GetBytes(retried, "max_tokens").Exists(), "max_tokens 必须被删除")
	require.Equal(t, int64(4096), gjson.GetBytes(retried, "max_completion_tokens").Int(),
		"max_completion_tokens 必须保留且取值不变")
	require.Equal(t, "deepseek-v4.1-flash-1017", gjson.GetBytes(retried, "model").String(), "不应动到其它字段")
	require.Equal(t, "hi", gjson.GetBytes(retried, "messages.0.content").String(), "不应动到其它字段")
}

// 上游点名了互斥、但客户端只发了一个字段：请求体没有可改写的东西，不重发。
// 这条钉住"只有 max_tokens 超过模型上限"那类同类措辞的 400 不被误伤。
func TestCCMaxTokensConflictSelfHealRequiresBothFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream := newCCSelfHealService(
		ccSelfHealScriptedResponse{status: http.StatusBadRequest, body: ccMaxTokensConflictRejectionBody},
	)

	resp := sendCCSelfHealRequest(t, svc, newCCSelfHealContext(), []byte(ccMaxTokensOnlyBody))

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Equal(t, 1, upstream.callCount(), "请求体只有一个字段，绝不能重发")
	require.Equal(t, ccMaxTokensOnlyBody, string(upstream.bodyAt(0)), "出站请求体必须原样")
}

// 400 与额度字段无关：即使客户端同传了两个字段也不重发（不能拿无关错误乱改请求）。
func TestCCMaxTokensConflictSelfHealSkipsUnrelatedBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream := newCCSelfHealService(
		ccSelfHealScriptedResponse{status: http.StatusBadRequest, body: ccSelfHealPlainBadRequestBody},
	)

	resp := sendCCSelfHealRequest(t, svc, newCCSelfHealContext(), []byte(ccMaxTokensBothBody))

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Equal(t, 1, upstream.callCount(), "与额度字段冲突无关的 400 不得触发重发")
	require.Equal(t, ccMaxTokensBothBody, string(upstream.bodyAt(0)))
}

// 重发仍 400：丢弃重发结果，交回**原始**响应（状态码与 body 都是第一次那个）。
// 这是 /v1/chat/completions 行为不变的关键：自愈只救活，不改写失败原因。
func TestCCMaxTokensConflictSelfHealKeepsOriginalErrorWhenRetryFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream := newCCSelfHealService(
		ccSelfHealScriptedResponse{status: http.StatusBadRequest, body: ccMaxTokensConflictRejectionBody},
		ccSelfHealScriptedResponse{status: http.StatusBadRequest, body: `{"error":{"message":"something else","type":"api_error"}}`},
	)

	resp := sendCCSelfHealRequest(t, svc, newCCSelfHealContext(), []byte(ccMaxTokensBothBody))

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Equal(t, 2, upstream.callCount())
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, ccMaxTokensConflictRejectionBody, string(raw),
		"重发失败必须交回原始响应体，客户端看到的错误与没自愈时完全一致")
}

func TestIsCCMaxTokensConflictError(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "线上工单原文",
			body: ccMaxTokensConflictRejectionBody,
			want: true,
		},
		{
			name: "反向措辞 cannot be used together",
			body: `{"error":{"message":"max_tokens and max_completion_tokens cannot be used together"}}`,
			want: true,
		},
		{
			name: "mutually exclusive",
			body: `{"error":{"message":"max_tokens and max_completion_tokens are mutually exclusive"}}`,
			want: true,
		},
		{
			name: "only one of",
			body: `{"error":{"message":"only one of max_tokens, max_completion_tokens may be specified"}}`,
			want: true,
		},
		{
			name: "error 不是对象时退化整包匹配",
			body: `{"message":"The parameter max_tokens is not valid: max_tokens and max_completion_tokens cannot be set at the same time."}`,
			want: true,
		},
		{
			name: "只提 max_completion_tokens 超上限：不是本次成因",
			body: `{"error":{"message":"The parameter max_completion_tokens specified in the request are not valid: integer above maximum value, expected a value <= 131072","param":"max_completion_tokens"}}`,
			want: false,
		},
		{
			name: "只提 max_tokens 超上限：不是本次成因",
			body: `{"error":{"message":"max_tokens (256000) exceeds model's maximum output tokens (65536)"}}`,
			want: false,
		},
		{
			name: "两个字段名都出现但无语义关联",
			body: `{"error":{"message":"unknown field max_completion_tokens; max_tokens is deprecated"}}`,
			want: false,
		},
		{
			name: "空 body",
			body: "",
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, isCCMaxTokensConflictError([]byte(test.body)))
		})
	}
}

func TestDropCCConflictingMaxTokens(t *testing.T) {
	t.Run("两个字段同传时只删 max_tokens", func(t *testing.T) {
		out, changed := dropCCConflictingMaxTokens([]byte(ccMaxTokensBothBody))
		require.True(t, changed)
		require.Equal(t,
			`{"model":"deepseek-v4.1-flash-1017","messages":[{"role":"user","content":"hi"}],"max_completion_tokens":4096}`,
			string(out), "定点删除：除被删的键外字节级不变")
	})

	t.Run("只有 max_tokens 时不动手", func(t *testing.T) {
		out, changed := dropCCConflictingMaxTokens([]byte(ccMaxTokensOnlyBody))
		require.False(t, changed)
		require.Equal(t, ccMaxTokensOnlyBody, string(out))
	})

	t.Run("只有 max_completion_tokens 时不动手", func(t *testing.T) {
		body := `{"model":"m","max_completion_tokens":4096}`
		out, changed := dropCCConflictingMaxTokens([]byte(body))
		require.False(t, changed)
		require.Equal(t, body, string(out))
	})

	t.Run("两者都不存在时不动手", func(t *testing.T) {
		body := `{"model":"m","messages":[]}`
		out, changed := dropCCConflictingMaxTokens([]byte(body))
		require.False(t, changed)
		require.Equal(t, body, string(out))
	})

	t.Run("非法 JSON 原样返回", func(t *testing.T) {
		body := `{"model":"m","max_tokens":`
		out, changed := dropCCConflictingMaxTokens([]byte(body))
		require.False(t, changed)
		require.Equal(t, body, string(out))
	})

	t.Run("空 body 原样返回", func(t *testing.T) {
		out, changed := dropCCConflictingMaxTokens(nil)
		require.False(t, changed)
		require.Nil(t, out)
	})
}
