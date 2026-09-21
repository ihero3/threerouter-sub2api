package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// ---------------------------------------------------------------------------
// CC 出口自愈的 HTTP 级验证。
//
// 验收标准（用户明确要求）：**为了支持 /v1/responses 而加的任何东西，都不得影响
// 既有的 /v1/chat/completions**。这里用真实发请求的方式把这条约束钉死：
//
//   - 客户端没发 json_schema 的 chat 请求 → 上游 400 时**一次都不重发**；
//   - 客户端发了 json_schema → 才允许重发一次，且第二次体必须是 json_object；
//   - 400 与 json_schema 无关 → 不重发；
//   - 2xx → 直接返回，不做任何额外动作；
//   - 读错误体做判定之后，响应体必须仍然可被调用方完整读取。
//
// 注意：本文件**不加 //go:build unit**。本仓库 -tags=unit 层当前整体编不过（既有
// 问题），带 tag 的测试根本跑不到，所以桩在这里自带，不复用 unit 层那套。
// ---------------------------------------------------------------------------

// ccSelfHealUpstreamRejectionBody 是"有 json_schema 概念但不实现"的上游返回的典型报文。
const ccSelfHealUpstreamRejectionBody = `{"error":{"code":"Bad Request","message":"Model 'deepseek/deepseek-v4.1-flash' does not support 'json_schema' response format. Supported formats: json_object.","type":"invalid_request_error"}}`

// ccSelfHealPlainBadRequestBody 与 json_schema 无关的 400。
const ccSelfHealPlainBadRequestBody = `{"error":{"code":"Bad Request","message":"model does not exist","type":"invalid_request_error"}}`

// ccSelfHealJSONSchemaBody 是"客户端确实要了结构化输出"的 CC 请求体。
const ccSelfHealJSONSchemaBody = `{"model":"deepseek-chat","messages":[{"role":"user","content":"hi"}],"response_format":{"type":"json_schema","json_schema":{"name":"p","schema":{"type":"object"}}}}`

// ccSelfHealPlainBody 是普通 chat 请求（没有 response_format）。
const ccSelfHealPlainBody = `{"model":"deepseek-chat","messages":[{"role":"user","content":"hi"}]}`

type ccSelfHealScriptedResponse struct {
	status int
	body   string
	// err 非空时模拟传输层失败（没有任何 HTTP 响应）。
	err error
	// nilBody 为 true 时返回 Body 为 nil 的响应，用于验证出口不会对 nil Body 崩溃。
	nilBody bool
}

// ccSelfHealUpstream 按调用顺序回放预置响应，并记录每次真正出站的请求体。
type ccSelfHealUpstream struct {
	mu     sync.Mutex
	script []ccSelfHealScriptedResponse
	calls  [][]byte
}

func (u *ccSelfHealUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	raw, _ := io.ReadAll(req.Body)

	u.mu.Lock()
	defer u.mu.Unlock()
	u.calls = append(u.calls, raw)
	index := len(u.calls) - 1
	if index >= len(u.script) {
		// 脚本用完后重复最后一条：多出来的调用会让断言先失败，而不是让桩崩掉。
		index = len(u.script) - 1
	}
	scripted := u.script[index]
	if scripted.err != nil {
		return nil, scripted.err
	}
	var body io.ReadCloser
	if !scripted.nilBody {
		body = io.NopCloser(strings.NewReader(scripted.body))
	}
	return &http.Response{
		StatusCode: scripted.status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       body,
		Request:    req,
	}, nil
}

func (u *ccSelfHealUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, concurrency)
}

func (u *ccSelfHealUpstream) callCount() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return len(u.calls)
}

func (u *ccSelfHealUpstream) bodyAt(index int) []byte {
	u.mu.Lock()
	defer u.mu.Unlock()
	if index >= len(u.calls) {
		return nil
	}
	return u.calls[index]
}

func newCCSelfHealService(script ...ccSelfHealScriptedResponse) (*OpenAIGatewayService, *ccSelfHealUpstream) {
	upstream := &ccSelfHealUpstream{script: script}
	// cfg 必须给：Responses 侧重发走 buildUpstreamRequest，其中会按 URL 白名单
	// 配置校验 base_url。零值 Config 表示"白名单关闭"，走纯格式校验（要求 https）。
	return &OpenAIGatewayService{
		httpUpstream: upstream,
		cfg:          &config.Config{},
	}, upstream
}

func newCCSelfHealContext() *gin.Context {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(nil))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func newCCSelfHealAccount() *Account {
	return &Account{
		ID:          9901,
		Name:        "cc-selfheal-test",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{},
	}
}

const ccSelfHealTargetURL = "http://upstream.example/v1/chat/completions"

func sendCCSelfHealRequest(t *testing.T, svc *OpenAIGatewayService, c *gin.Context, body []byte) *http.Response {
	t.Helper()
	resp, err := svc.sendCCUpstreamRequest(
		context.Background(), c, newCCSelfHealAccount(), ccSelfHealTargetURL, body, false, "sk-test", "", "",
	)
	require.NoError(t, err)
	require.NotNil(t, resp)
	return resp
}

// 核心断言：普通 chat 请求（没有 response_format）撞上 json_schema 措辞的 400，
// 也不能被重发。这是 /v1/chat/completions 行为不变的最直接证据。
func TestCCSelfHealDoesNotRetryWhenClientSentNoJSONSchema(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream := newCCSelfHealService(ccSelfHealScriptedResponse{status: http.StatusBadRequest, body: ccSelfHealUpstreamRejectionBody})

	resp := sendCCSelfHealRequest(t, svc, newCCSelfHealContext(), []byte(ccSelfHealPlainBody))

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Equal(t, 1, upstream.callCount(), "客户端没发 json_schema，绝不能重发：chat 行为必须一个字节都不变")
	require.Equal(t, ccSelfHealPlainBody, string(upstream.bodyAt(0)), "出站请求体也必须原样")
}

// 客户端确实要了 json_schema：允许自愈重发一次，第二次体降级为 json_object。
func TestCCSelfHealRetriesOnceWhenClientSentJSONSchema(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream := newCCSelfHealService(
		ccSelfHealScriptedResponse{status: http.StatusBadRequest, body: ccSelfHealUpstreamRejectionBody},
		ccSelfHealScriptedResponse{status: http.StatusOK, body: `{"id":"chatcmpl_ok","choices":[]}`},
	)

	resp := sendCCSelfHealRequest(t, svc, newCCSelfHealContext(), []byte(ccSelfHealJSONSchemaBody))

	require.Equal(t, http.StatusOK, resp.StatusCode, "重发成功时应把成功的响应交回调用方")
	require.Equal(t, 2, upstream.callCount(), "只允许重发一次，不允许递归")

	retried := upstream.bodyAt(1)
	require.Equal(t, "json_object", gjson.GetBytes(retried, "response_format.type").String())
	require.False(t, gjson.GetBytes(retried, "response_format.json_schema").Exists(), "json_schema 必须整体丢弃")
	require.Equal(t, "deepseek-chat", gjson.GetBytes(retried, "model").String(), "不应动到其它字段")
	require.Contains(t, strings.ToLower(gjson.GetBytes(retried, "messages.0.content").String()), "json",
		"降级后必须补上小写 json 关键词")
}

// 400 与 json_schema 无关：即使客户端发了 json_schema 也不重发（避免拿无关错误乱改请求）。
func TestCCSelfHealSkipsUnrelatedBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream := newCCSelfHealService(ccSelfHealScriptedResponse{status: http.StatusBadRequest, body: ccSelfHealPlainBadRequestBody})

	resp := sendCCSelfHealRequest(t, svc, newCCSelfHealContext(), []byte(ccSelfHealJSONSchemaBody))

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Equal(t, 1, upstream.callCount(), "与 json_schema 无关的 400 不得触发重发")
	require.Equal(t, ccSelfHealJSONSchemaBody, string(upstream.bodyAt(0)))
}

// 2xx 直通：不能因为加了自愈而对成功请求做任何多余动作。
func TestCCSelfHealLeavesSuccessUntouched(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream := newCCSelfHealService(ccSelfHealScriptedResponse{status: http.StatusOK, body: `{"id":"chatcmpl_ok","choices":[]}`})

	resp := sendCCSelfHealRequest(t, svc, newCCSelfHealContext(), []byte(ccSelfHealJSONSchemaBody))

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, 1, upstream.callCount())
	require.Equal(t, ccSelfHealJSONSchemaBody, string(upstream.bodyAt(0)), "成功路径上请求体必须原样上行")
}

// 流式成功响应（200 + SSE）必须原封不动地交给调用方。
//
// 这是 /v1/chat/completions 最主流的用法，也是自愈最容易搞坏的地方：为了判定 400
// 需要读错误体，一旦把"读"错做到成功响应上，SSE 流就会被吞掉或截断。自愈的读只
// 发生在 status==400 分支，本用例把这条边界钉死。
func TestCCSelfHealLeavesStreamingResponseBodyIntact(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sse := "data: {\"id\":\"c1\",\"choices\":[{\"delta\":{\"content\":\"你\"}}]}\n\n" +
		"data: {\"id\":\"c1\",\"choices\":[{\"delta\":{\"content\":\"好\"}}]}\n\n" +
		"data: [DONE]\n\n"
	svc, upstream := newCCSelfHealService(ccSelfHealScriptedResponse{status: http.StatusOK, body: sse})

	resp := sendCCSelfHealRequest(t, svc, newCCSelfHealContext(), []byte(ccSelfHealJSONSchemaBody))

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, 1, upstream.callCount(), "流式成功不得触发任何重发")
	require.Equal(t, ccSelfHealJSONSchemaBody, string(upstream.bodyAt(0)), "流式请求体必须原样上行")

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, sse, string(raw), "SSE 流必须逐字节完整，不被自愈吞掉或截断")
}

// 自愈判定要读错误体，但**必须原样回填**：否则调用方的 failover 判定与错误响应会拿到空体。
//
// 注意区分两个读法：调用方用的是 readUpstreamErrorBody（一次性读，自己不回填），
// 判定用的是 readOpenAIUpstreamError（读完会回填成可重读副本）。这里两种都验。
func TestCCSelfHealKeepsErrorBodyReadableForCaller(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, _ := newCCSelfHealService(ccSelfHealScriptedResponse{status: http.StatusBadRequest, body: ccSelfHealUpstreamRejectionBody})

	resp := sendCCSelfHealRequest(t, svc, newCCSelfHealContext(), []byte(ccSelfHealPlainBody))

	// 调用方的读法：能拿到完整报文。
	require.Contains(t, string(svc.readUpstreamErrorBody(resp)), "does not support 'json_schema'")
}

// 判定用的读法必须可重复读：否则"先判定、后交给 failover 处理"就会拿到空体。
func TestCCSelfHealErrorBodyRemainsRereadableForJudgement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, _ := newCCSelfHealService(ccSelfHealScriptedResponse{status: http.StatusBadRequest, body: ccSelfHealUpstreamRejectionBody})

	resp := sendCCSelfHealRequest(t, svc, newCCSelfHealContext(), []byte(ccSelfHealPlainBody))

	first, _ := svc.readOpenAIUpstreamError(resp)
	second, _ := svc.readOpenAIUpstreamError(resp)
	require.Contains(t, string(first), "does not support 'json_schema'")
	require.Equal(t, string(first), string(second), "回填用的必须是可重读副本")
}

// 重发仍失败时，交回调用方的必须是**原始响应**（语义与"没重试过"完全一致），
// 而不是重发那条错误——否则一个原本失败的 chat 请求会被换掉失败原因。
func TestCCSelfHealReturnsOriginalResponseWhenRetryAlsoFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream := newCCSelfHealService(
		ccSelfHealScriptedResponse{status: http.StatusBadRequest, body: ccSelfHealUpstreamRejectionBody},
		ccSelfHealScriptedResponse{status: http.StatusBadRequest, body: ccSelfHealPlainBadRequestBody},
	)

	resp := sendCCSelfHealRequest(t, svc, newCCSelfHealContext(), []byte(ccSelfHealJSONSchemaBody))

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Equal(t, 2, upstream.callCount(), "应该确实尝试过重发")
	require.Equal(t, ccSelfHealUpstreamRejectionBody, string(svc.readUpstreamErrorBody(resp)),
		"重发仍失败时必须交回原始响应：客户端看到的错误不能因为自愈而改变")
}

// 重发遇到传输层失败（没有 HTTP 响应）时，同样必须交回原始响应，不能把错误换成连接类错误。
func TestCCSelfHealReturnsOriginalResponseWhenRetryTransportFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream := newCCSelfHealService(
		ccSelfHealScriptedResponse{status: http.StatusBadRequest, body: ccSelfHealUpstreamRejectionBody},
		ccSelfHealScriptedResponse{err: context.DeadlineExceeded},
	)

	resp := sendCCSelfHealRequest(t, svc, newCCSelfHealContext(), []byte(ccSelfHealJSONSchemaBody))

	require.NotNil(t, resp)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Equal(t, 2, upstream.callCount())
	require.Equal(t, ccSelfHealUpstreamRejectionBody, string(svc.readUpstreamErrorBody(resp)),
		"重发传输失败时必须交回原始响应，不得变成连接类错误")
}

// 上游 400 说的是"你的 schema 写法有问题"（不是"我没这个能力"）时，绝不能重发。
//
// 这里用的是**前缀被丢掉**的形态：只有 OpenAI 官方会带 "Invalid schema for ..."，
// 中转站常把它重写成 "json_schema: 'anyOf' is not supported"。这种句子同样含
// "被拒字段 + not supported"，早先会被判成能力缺失而重发；重发往往成功，客户端于是
// 拿到 200 和不受约束的输出，而不是那条提示它改 schema 的 400。
func TestCCSelfHealDoesNotRetryOnSchemaContentError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for name, message := range map[string]string{
		"prefix_dropped_anyof":        `json_schema: 'anyOf' is not supported`,
		"prefix_dropped_ref":          `response_format.schema: '$ref' is not supported in strict mode`,
		"unquoted_keyword":            `Unsupported keyword 'additionalProperties' in response_format.json_schema`,
		"openai_full_form":            `Invalid schema for response_format 'p': In context=(), 'additionalProperties' is required to be supplied and to be false.`,
		"schema_not_valid_jsonschema": `response_format.json_schema.schema is not a valid JSON Schema`,
	} {
		t.Run(name, func(t *testing.T) {
			payload := `{"error":{"code":"invalid_request_error","message":` + strconv.Quote(message) + `,"type":"invalid_request_error"}}`
			svc, upstream := newCCSelfHealService(ccSelfHealScriptedResponse{status: http.StatusBadRequest, body: payload})

			resp := sendCCSelfHealRequest(t, svc, newCCSelfHealContext(), []byte(ccSelfHealJSONSchemaBody))

			require.Equal(t, http.StatusBadRequest, resp.StatusCode)
			require.Equal(t, 1, upstream.callCount(),
				"schema 内容错误不得重发：重发会成功并悄悄丢掉客户端的严格结构保证（message=%q）", message)
		})
	}
}

// 上游 400 且 Body 为 nil 时不得崩溃：自定义 HTTPUpstream 实现可能返回这种响应。
// 早先出口会直接对 nil Body 调 ReadAll。
func TestCCSelfHealDoesNotPanicOnNilErrorBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream := newCCSelfHealService(ccSelfHealScriptedResponse{status: http.StatusBadRequest, nilBody: true})

	require.NotPanics(t, func() {
		resp := sendCCSelfHealRequest(t, svc, newCCSelfHealContext(), []byte(ccSelfHealJSONSchemaBody))
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
	require.Equal(t, 1, upstream.callCount(), "没有错误体就没有判定依据，不得重发")
}

// 自愈判定要读错误体，读上限必须与调用方 readUpstreamErrorBody **同口径**：
// 若这里用一个更小的自定上限，超大错误体回填后就会比改动前调用方能读到的更短，
// 等于悄悄改了调用方的错误处理输入（failover 判定与错误响应都基于它）。
func TestCCSelfHealPreservesCallerErrorBodyLengthOnHugeBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// 构造一个超过读上限、且与 json_schema 无关的 400 报文（避免触发重发）。
	padding := strings.Repeat("x", int(openAIUpstreamErrorBodyReadLimit)+4096-len(ccSelfHealPlainBadRequestBody))
	huge := ccSelfHealPlainBadRequestBody[:len(ccSelfHealPlainBadRequestBody)-2] + `,"padding":"` + padding + `"}}`

	svc, upstream := newCCSelfHealService(ccSelfHealScriptedResponse{status: http.StatusBadRequest, body: huge})
	resp := sendCCSelfHealRequest(t, svc, newCCSelfHealContext(), []byte(ccSelfHealJSONSchemaBody))

	require.Equal(t, 1, upstream.callCount(), "与 json_schema 无关的 400 不得重发")

	limit := openAIUpstreamErrorBodyReadLimitForConfig(svc.cfg)
	got := svc.readUpstreamErrorBody(resp)
	require.Equal(t, limit, int64(len(got)),
		"调用方应能读到与改动前同样的字节数（上限口径一致）")
	require.Equal(t, huge[:limit], string(got), "回填内容必须是原始错误体的等长前缀")
}

// 重发拿到 400 且该响应 Body 为 nil 时不得崩溃：失败路径要丢弃重发结果并交回原始响应。
func TestCCSelfHealDoesNotPanicWhenRetryResponseHasNilBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream := newCCSelfHealService(
		ccSelfHealScriptedResponse{status: http.StatusBadRequest, body: ccSelfHealUpstreamRejectionBody},
		ccSelfHealScriptedResponse{status: http.StatusBadRequest, nilBody: true},
	)

	var resp *http.Response
	require.NotPanics(t, func() {
		resp = sendCCSelfHealRequest(t, svc, newCCSelfHealContext(), []byte(ccSelfHealJSONSchemaBody))
	})
	require.Equal(t, 2, upstream.callCount(), "应该确实尝试过重发")
	require.Equal(t, ccSelfHealUpstreamRejectionBody, string(svc.readUpstreamErrorBody(resp)),
		"重发仍失败必须交回原始响应")
}

// ---------------------------------------------------------------------------
// 请求体归一化的**逐字节等价性**（chat 正常路径不变的正面证据）。
//
// 上面那些用例证明的是"不会多发一次请求"；这里证明的是更基础的一件事：
// **客户端没要 json_schema 时，出站请求体必须与加这些功能之前逐字节一致。**
//
// 做法照搬取证时的矩阵法（曾用同一张矩阵在 git HEAD 基线上对跑、792 例中差异
// 全部落在 json_schema 请求上，0 例落在普通请求上），这里把它固化成断言。
// ---------------------------------------------------------------------------

// ccSelfHealNonJSONSchemaBodies 是"客户端没要结构化输出"的各类真实请求体。
var ccSelfHealNonJSONSchemaBodies = map[string]string{
	"plain":          `{"model":"x","messages":[{"role":"user","content":"hi"}]}`,
	"json_object":    `{"model":"x","messages":[{"role":"user","content":"hi"}],"response_format":{"type":"json_object"}}`,
	"text_format":    `{"model":"x","messages":[{"role":"user","content":"hi"}],"response_format":{"type":"text"}}`,
	"tools":          `{"model":"x","messages":[{"role":"user","content":"hi"}],"tools":[{"type":"function","function":{"name":"f","parameters":{"type":"object"}}}],"tool_choice":"auto"}`,
	"numbers":        `{"model":"x","temperature":0.5,"max_tokens":1024,"top_p":1,"messages":[{"role":"user","content":"hi"}]}`,
	"multimodal":     `{"model":"x","messages":[{"role":"user","content":[{"type":"text","text":"see"},{"type":"image_url","image_url":{"url":"http://a/b.png"}}]}]}`,
	"system":         `{"model":"x","messages":[{"role":"system","content":"You are helpful."},{"role":"user","content":"hi"}]}`,
	"stream":         `{"model":"x","stream":true,"messages":[{"role":"user","content":"hi"}]}`,
	"effort_high":    `{"model":"x","messages":[{"role":"user","content":"hi"}],"reasoning_effort":"high"}`,
	"nested_effort":  `{"model":"x","messages":[{"role":"user","content":"hi"}],"reasoning":{"effort":"xhigh"}}`,
	"uppercase_type": `{"model":"x","messages":[{"role":"user","content":"hi"}],"response_format":{"type":"JSON_SCHEMA","json_schema":{"name":"p","schema":{"g":7}}}}`,
	"invalid_json":   `{`,
	"empty":          ``,
}

// 非 GLM 模型：GLM 的 effort 归一是既有独立行为，需单独验（见下一个用例），
// 混进等价性断言会把"既有行为"误判成"新行为"。
var ccSelfHealIdentityModels = []string{
	"deepseek", "deepseek-chat", "deepseek-v4.1-flash",
	"DeepSeek/DeepSeek-V3", "deepseek_v3", "deepseek/deepseek-v4.1-flash",
	"gpt-5.4", "kimi-k2.6", "claude-opus-4.8", "",
}

func TestNormalizeOpenAICCUpstreamBodyIsIdentityForNonJSONSchemaRequests(t *testing.T) {
	accounts := map[string]*Account{
		"nil":                  nil,
		"deepseek-platform":    {ID: 1, Platform: PlatformDeepseek, Type: AccountTypeAPIKey, Credentials: map[string]any{}},
		"openai-relay":         {ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{}},
		"deepseek-platform-oa": {ID: 3, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{}},
	}
	checked := 0
	for accountKey, account := range accounts {
		for _, model := range ccSelfHealIdentityModels {
			for bodyKey, body := range ccSelfHealNonJSONSchemaBodies {
				out, changed := normalizeOpenAICCUpstreamBody(account, model, []byte(body))
				require.Equal(t, body, string(out),
					"未要 json_schema 的请求体必须逐字节原样上行（account=%s model=%q body=%s）", accountKey, model, bodyKey)
				require.False(t, changed,
					"未要 json_schema 时不得报告任何改写（account=%s model=%q body=%s）", accountKey, model, bodyKey)
				checked++
			}
		}
	}
	require.Equal(t, len(accounts)*len(ccSelfHealIdentityModels)*len(ccSelfHealNonJSONSchemaBodies), checked)
	t.Logf("已逐字节核对 %d 组普通请求体", checked)
}

// GLM 的 effort 归一是本函数提取进来时**必须保住的既有行为**；同时验证它与
// DeepSeek 降级能在同一个请求体上先后生效（即两步都被执行、顺序没丢）。
func TestNormalizeOpenAICCUpstreamBodyKeepsGLMEffortNormalization(t *testing.T) {
	glmAccount := &Account{ID: 1, Platform: PlatformDeepseek, Type: AccountTypeAPIKey, Credentials: map[string]any{}}

	// 只带 effort：GLM 映射必须仍然生效。
	effortOnly := []byte(`{"model":"x","messages":[{"role":"user","content":"hi"}],"reasoning_effort":"xhigh"}`)
	out, changed := normalizeOpenAICCUpstreamBody(glmAccount, "glm-4.7", effortOnly)
	require.True(t, changed)
	require.Equal(t, "max", gjson.GetBytes(out, "reasoning_effort").String(), "GLM effort 归一化不得因提取而丢失")

	// effort + json_schema 同时出现：两步都必须生效，证明组合顺序没被破坏。
	combined := []byte(`{"model":"x","messages":[{"role":"user","content":"hi"}],"reasoning_effort":"xhigh","response_format":{"type":"json_schema","json_schema":{"name":"p","schema":{"type":"object"}}}}`)
	out, changed = normalizeOpenAICCUpstreamBody(glmAccount, "glm-4.7", combined)
	require.True(t, changed)
	require.Equal(t, "max", gjson.GetBytes(out, "reasoning_effort").String(), "GLM effort 步必须先生效")
	require.Equal(t, "json_object", gjson.GetBytes(out, "response_format.type").String(), "DeepSeek 降级步必须在后生效")
	require.False(t, gjson.GetBytes(out, "response_format.json_schema").Exists())
	require.Contains(t, gjson.GetBytes(out, "messages.0.content").String(), "json")

	// 非 GLM 模型不得被 GLM 步碰到。
	out, changed = normalizeOpenAICCUpstreamBody(glmAccount, "gpt-5.4", effortOnly)
	require.False(t, changed)
	require.Equal(t, string(effortOnly), string(out))
}

// ---------------------------------------------------------------------------
// Responses 侧重发钩子的契约。
//
// 它挂在 /v1/chat/completions（自适应账号转 Responses）的链路上，因此必须同时满足两件事：
//  1. 只在 json_schema 被拒时重发，其它"被拒字段"原因一律不介入；
//  2. 重发没成功就交回原始响应——失败路径零行为变化。
// ---------------------------------------------------------------------------

func newResponsesRetryTestContext() *gin.Context {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(nil))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func newResponsesRetryTestAccount() *Account {
	return &Account{
		ID:       9902,
		Name:     "responses-retry-test",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			// 必须 https：新搭建的 Config 零值 = 白名单关闭 + 禁止明文 http，
			// http:// 会被 validateUpstreamBaseURL 直接拒掉，测不到重发路径。
			"base_url": "https://upstream.example",
		},
	}
}

func responsesRetryInitialResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// 重发成功：交回新响应（原本必然 400 的请求被救活）。
func TestResponsesRejectedFieldRetryOnceReturnsRetryResponseOnSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream := newCCSelfHealService(ccSelfHealScriptedResponse{status: http.StatusOK, body: `{"id":"resp_ok","output":[]}`})
	account := newResponsesRetryTestAccount()
	initial := responsesRetryInitialResponse(ccSelfHealUpstreamRejectionBody)

	got := svc.retryOpenAIResponsesRejectedFieldOnce(
		context.Background(), newResponsesRetryTestContext(), account,
		initial, []byte(responsesJSONSchemaRequestBody), "tok", "", "", true, false,
	)

	require.NotNil(t, got)
	require.Equal(t, http.StatusOK, got.StatusCode)
	require.Equal(t, 1, upstream.callCount())
	require.Equal(t, "json_object", gjson.GetBytes(upstream.bodyAt(0), "text.format.type").String(),
		"重发必须用降级后的 body")
	require.Contains(t, gjson.GetBytes(upstream.bodyAt(0), "instructions").String(), "json")
}

// 重发仍失败：必须交回**原始**响应，不能让失败原因被换掉。
func TestResponsesRejectedFieldRetryOnceKeepsOriginalOnRetryFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream := newCCSelfHealService(ccSelfHealScriptedResponse{status: http.StatusBadRequest, body: ccSelfHealPlainBadRequestBody})
	account := newResponsesRetryTestAccount()
	initial := responsesRetryInitialResponse(ccSelfHealUpstreamRejectionBody)

	got := svc.retryOpenAIResponsesRejectedFieldOnce(
		context.Background(), newResponsesRetryTestContext(), account,
		initial, []byte(responsesJSONSchemaRequestBody), "tok", "", "", true, false,
	)

	require.Same(t, initial, got, "重发失败必须原样交回入参响应，调用方错误处理链不变")
	require.Equal(t, 1, upstream.callCount())
}

// 重发传输失败：同样交回原始响应，不得变成连接类错误。
func TestResponsesRejectedFieldRetryOnceKeepsOriginalOnTransportFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream := newCCSelfHealService(ccSelfHealScriptedResponse{err: context.DeadlineExceeded})
	account := newResponsesRetryTestAccount()
	initial := responsesRetryInitialResponse(ccSelfHealUpstreamRejectionBody)

	got := svc.retryOpenAIResponsesRejectedFieldOnce(
		context.Background(), newResponsesRetryTestContext(), account,
		initial, []byte(responsesJSONSchemaRequestBody), "tok", "", "", true, false,
	)

	require.Same(t, initial, got)
	require.Equal(t, 1, upstream.callCount())
}

// 白名单的**行为级**证据：非 json_schema 原因（这里用 truncation）即使判定命中，
// 也绝不允许在本函数里重发——否则 max_output_tokens / truncation 等既有 chat 行为
// 会被这次改动顺带改变。
func TestResponsesRejectedFieldRetryOnceIgnoresNonJSONSchemaReasons(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream := newCCSelfHealService(ccSelfHealScriptedResponse{status: http.StatusOK, body: `{"id":"should_not_be_used"}`})
	account := newResponsesRetryTestAccount()
	initial := responsesRetryInitialResponse(`{"error":{"code":"unknown_parameter","message":"Unsupported parameter: 'truncation'","param":"truncation"}}`)

	body := []byte(`{"model":"gpt-5.4","input":"hi","truncation":"auto","text":{"format":{"type":"json_object"}}}`)

	got := svc.retryOpenAIResponsesRejectedFieldOnce(
		context.Background(), newResponsesRetryTestContext(), account, initial, body, "tok", "", "", true, false,
	)

	require.Same(t, initial, got, "非 json_schema 原因不得重发，必须原样交回入参响应")
	require.Equal(t, 0, upstream.callCount(), "一次上游请求都不允许发出")
}

// 客户端没发 json_schema（没有 text.format）时，本函数必须是彻底的 no-op。
func TestResponsesRejectedFieldRetryOnceIsNoOpWithoutTextFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream := newCCSelfHealService(ccSelfHealScriptedResponse{status: http.StatusOK, body: `{"id":"should_not_be_used"}`})
	account := newResponsesRetryTestAccount()
	initial := responsesRetryInitialResponse(ccSelfHealUpstreamRejectionBody)

	got := svc.retryOpenAIResponsesRejectedFieldOnce(
		context.Background(), newResponsesRetryTestContext(), account,
		initial, []byte(ccSelfHealPlainBody), "tok", "", "", true, false,
	)

	require.Same(t, initial, got)
	require.Equal(t, 0, upstream.callCount(), "普通 chat 请求不得触发任何额外上游调用")
}

// 非 400 直接返回，连错误体都不读——尤其要保证 200 的响应体（流式则为 SSE 流）
// 没有被碰过：本钩子挂在 /v1/chat/completions 的成功链路上，读出/回填 body 一旦
// 越界就会破坏流式转发。
func TestResponsesRejectedFieldRetryOnceSkipsNonBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream := newCCSelfHealService(ccSelfHealScriptedResponse{status: http.StatusOK, body: `{}`})
	account := newResponsesRetryTestAccount()
	sse := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hi\"}\n\ndata: [DONE]\n\n"
	initial := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(sse)),
	}

	got := svc.retryOpenAIResponsesRejectedFieldOnce(
		context.Background(), newResponsesRetryTestContext(), account,
		initial, []byte(responsesJSONSchemaRequestBody), "tok", "", "", true, false,
	)

	require.Same(t, initial, got)
	require.Equal(t, 0, upstream.callCount())

	raw, err := io.ReadAll(got.Body)
	require.NoError(t, err)
	require.Equal(t, sse, string(raw), "非 400 响应体必须原封不动：成功/流式路径不得被本钩子读取或改写")
}

// 400 且响应体为 nil 时不得崩溃（自定义 HTTPUpstream 实现可能返回这种响应）。
func TestResponsesRejectedFieldRetryOnceDoesNotPanicOnNilErrorBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream := newCCSelfHealService(ccSelfHealScriptedResponse{status: http.StatusBadRequest, nilBody: true})
	account := newResponsesRetryTestAccount()
	initial := &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       nil,
	}

	var got *http.Response
	require.NotPanics(t, func() {
		got = svc.retryOpenAIResponsesRejectedFieldOnce(
			context.Background(), newResponsesRetryTestContext(), account,
			initial, []byte(responsesJSONSchemaRequestBody), "tok", "", "", true, false,
		)
	})
	require.Same(t, initial, got, "没有响应体就没有判定依据，必须原样交回")
	require.Equal(t, 0, upstream.callCount())
}

// 重发拿到 400 且该响应 Body 为 nil 时不得崩溃：必须原样交回入参响应。
func TestResponsesRejectedFieldRetryOnceDoesNotPanicWhenRetryResponseHasNilBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream := newCCSelfHealService(ccSelfHealScriptedResponse{status: http.StatusBadRequest, nilBody: true})
	account := newResponsesRetryTestAccount()
	initial := responsesRetryInitialResponse(ccSelfHealUpstreamRejectionBody)

	var got *http.Response
	require.NotPanics(t, func() {
		got = svc.retryOpenAIResponsesRejectedFieldOnce(
			context.Background(), newResponsesRetryTestContext(), account,
			initial, []byte(responsesJSONSchemaRequestBody), "tok", "", "", true, false,
		)
	})
	require.Same(t, initial, got, "重发仍失败必须原样交回入参响应")
	require.Equal(t, 1, upstream.callCount())
}

// 挂在 /v1/chat/completions 链路上的 Responses 侧重发，原因白名单必须只放行本平台
// 主动支持的自愈原因（json_schema / reasoning_effort 取值）。否则 max_output_tokens /
// truncation / namespace / status / 工具参数等既有 chat 行为会被本改动顺带改变。
func TestOpenAIResponsesChatPathRetryReasonWhitelist(t *testing.T) {
	require.True(t, isOpenAIResponsesChatPathRetryReason(openAIResponsesJSONSchemaFormatRejectionReason))
	require.True(t, isOpenAIResponsesChatPathRetryReason(openAIResponsesReasoningEffortValueRejectionReason))

	otherReasons := []string{
		"",
		"tool parameter root type rejection",
		"prompt_cache_breakpoint parameter rejection",
		"indexed namespace parameter rejection",
		"indexed status parameter rejection",
		"max_output_tokens parameter rejection",
		"truncation parameter rejection",
		"indexed prompt_cache_breakpoint parameter rejection",
		"indexed reasoning null content rejection",
		"indexed message null content rejection",
		"indexed reasoning content maximum-length rejection",
	}
	for _, reason := range otherReasons {
		require.False(t, isOpenAIResponsesChatPathRetryReason(reason),
			"未登记的原因不得在 chat 链路上触发重发: %q", reason)
	}
}

// 降级会把整份请求体重新序列化（map → json.Marshal）。这条守的是**静默数值损坏**：
// 解码若走了 float64 而不是 json.Number，超过 2^53 的整数会被悄悄改写，且降级后
// 请求仍能成功，客户端完全无从察觉。仓库里存在 nonce / seed 这类大整数字段，
// 因此这里必须钉住。
func TestDowngradeCCJSONSchemaPreservesRequestPayload(t *testing.T) {
	body := []byte(`{"model":"m","seed":9007199254740993,"top_p":0.30000000000000004,"presence_penalty":1e-7,` +
		`"custom_unknown_field":{"x":1},"a":{"b":[1,2,null]},` +
		`"response_format":{"type":"json_schema","json_schema":{"name":"x","strict":true,"schema":{"type":"object"}}}}`)

	out, changed := downgradeCCJSONSchemaResponseFormat(body)
	require.True(t, changed)
	require.Equal(t, "json_object", gjson.GetBytes(out, "response_format.type").String())

	// 注意：数值比较一律用 .Raw——gjson 的 .String() 会把数字过一遍 float64 再格式化，
	// "1e-7" 会被显示成 "0.0000001"，那是读取端的格式化而非出站字节被改动。
	require.Equal(t, "9007199254740993", gjson.GetBytes(out, "seed").Raw)
	// 小数与科学计数不得被改写。
	require.Equal(t, "0.30000000000000004", gjson.GetBytes(out, "top_p").Raw)
	require.Equal(t, "1e-7", gjson.GetBytes(out, "presence_penalty").Raw)
	// 未知字段与嵌套结构不得丢失。
	require.Equal(t, "1", gjson.GetBytes(out, "custom_unknown_field.x").String())
	require.Equal(t, `[1,2,null]`, gjson.GetBytes(out, "a.b").String())
}

// 注入 system 提示词时的边界形态：不得 panic，也不得把非数组的 messages 改坏。
func TestDowngradeCCJSONSchemaSystemHintEdgeCases(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"no_messages_key", `{"model":"m","response_format":{"type":"json_schema","json_schema":{"name":"x","schema":{"type":"object"}}}}`},
		{"empty_messages", `{"model":"m","messages":[],"response_format":{"type":"json_schema","json_schema":{"name":"x","schema":{"type":"object"}}}}`},
		{"messages_not_array", `{"model":"m","messages":"not-an-array","response_format":{"type":"json_schema","json_schema":{"name":"x","schema":{"type":"object"}}}}`},
		{"existing_system_message", `{"model":"m","messages":[{"role":"system","content":"旧的"}],"response_format":{"type":"json_schema","json_schema":{"name":"x","schema":{"type":"object"}}}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, changed := downgradeCCJSONSchemaResponseFormat([]byte(tc.body))
			require.True(t, changed)
			require.Equal(t, "json_object", gjson.GetBytes(out, "response_format.type").String())
			// 必须始终是合法、可被再次解码的 JSON（写过座位即崩的守卫）。
			require.True(t, gjson.Valid(string(out)), "降级结果必须是合法 JSON: %s", string(out))
		})
	}
}

// 客户端把 type 写成大写（JSON_SCHEMA）时也必须能自愈。
//
// 守的是"同一条 400 在两条链路上松紧不一"：Responses 侧的
// downgradeOpenAIResponsesJSONSchemaFormat 用的是 strings.EqualFold，CC 侧若用
// 大小写敏感的 !=，客户端换个写法就会在 CC 出口拿不到自愈、在 Responses 出口却
// 拿得到——又是"只修一半"。
func TestCCSelfHealDowngradesUppercaseJSONSchemaType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream := newCCSelfHealService(
		ccSelfHealScriptedResponse{status: http.StatusBadRequest, body: ccSelfHealUpstreamRejectionBody},
		ccSelfHealScriptedResponse{status: http.StatusOK, body: `{"id":"chatcmpl_ok","choices":[]}`},
	)
	body := []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}],"response_format":{"type":"JSON_SCHEMA","json_schema":{"name":"x","schema":{"type":"object"}}}}`)

	resp := sendCCSelfHealRequest(t, svc, newCCSelfHealContext(), body)

	require.Equal(t, http.StatusOK, resp.StatusCode, "type 的大小写写法不该决定能否自愈")
	require.Equal(t, 2, upstream.callCount())
	require.Equal(t, "json_object", gjson.GetBytes(upstream.bodyAt(1), "response_format.type").String())
}

// CC 自愈必须受**本次入站请求共享的重试预算**约束。
//
// sendCCUpstreamRequest 会被每个候选账号各调一次（账号 failover / 凭证 failover）；
// 不接预算就是"每账号各重发一次"，一次入站请求的上游调用数会按账号数放大成 2N。
// Responses 侧（retryOpenAIResponsesRejectedFieldOnce）刻意接了同一个预算并写明
// 了这个理由，CC 侧必须同口径。
func TestCCSelfHealSharesRetryBudgetAcrossAccountAttempts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const attempts = 8
	// 上游对这条请求稳定 400：最能暴露"放大"的场景。
	script := make([]ccSelfHealScriptedResponse, 0, attempts*2)
	for i := 0; i < attempts*2; i++ {
		script = append(script, ccSelfHealScriptedResponse{status: http.StatusBadRequest, body: ccSelfHealUpstreamRejectionBody})
	}
	svc, upstream := newCCSelfHealService(script...)

	// 关键：同一个 gin.Context 串起全部账号尝试，预算正是按它共享的。
	c := newCCSelfHealContext()
	body := []byte(ccSelfHealJSONSchemaBody)
	for i := 0; i < attempts; i++ {
		resp, err := svc.sendCCUpstreamRequest(
			context.Background(), c, newCCSelfHealAccount(), ccSelfHealTargetURL, body, false, "sk-test", "", "")
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		if resp.Body != nil {
			_ = resp.Body.Close()
		}
	}

	// 前 maxOpenAIResponsesRejectedFieldRetries 次尝试各重发 1 次（每次 2 次上游调用），
	// 预算耗尽后每次尝试只剩 1 次调用。
	want := maxOpenAIResponsesRejectedFieldRetries*2 + (attempts - maxOpenAIResponsesRejectedFieldRetries)
	require.Equal(t, want, upstream.callCount(),
		"自愈重发必须受共享预算约束：期望 %d 次上游调用，实际 %d 次", want, upstream.callCount())
}
