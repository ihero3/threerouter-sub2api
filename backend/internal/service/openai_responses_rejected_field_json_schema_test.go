package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// ---------------------------------------------------------------------------
// 原生 Responses 出口的 json_schema 自愈。
//
// 这是工单 f74e374a 的"另一半"：CC 出口的自愈只覆盖「上游是 /v1/chat/completions」
// 的场景，而入站 CC 自适应转 Responses、入站 Responses 原生转发、passthrough、
// WS ingress、WS HTTP bridge 这五条出口走的是 /v1/responses，被拒时同样 400。
// 它们共用 normalizeOpenAIResponsesRejectedFieldRetryBody，所以只需在这一处判定。
// ---------------------------------------------------------------------------

// responsesJSONSchemaUpstreamRejection 是"有 json_schema 概念但不实现"的上游返回的
// 典型报文（措辞取自线上 DeepSeek 中转）。
const responsesJSONSchemaUpstreamRejection = `{"error":{"code":"invalid_request_error","message":"Model 'deepseek/deepseek-v4.1-flash' does not support 'json_schema' response format. Supported formats: json_object.","type":"invalid_request_error"}}`

func TestNormalizeOpenAIResponsesRejectedJSONSchemaFormat(t *testing.T) {
	body := []byte(responsesJSONSchemaRequestBody)
	require.Equal(t, "json_schema", gjson.GetBytes(body, "text.format.type").String())

	retryBody, reason, changed, err := normalizeOpenAIResponsesRejectedFieldRetryBody(
		http.StatusBadRequest, body, []byte(responsesJSONSchemaUpstreamRejection))
	require.NoError(t, err)
	require.True(t, changed, "上游明确拒绝了 json_schema，必须产出重试体")
	require.Equal(t, "json_schema text format rejection", reason)

	require.Equal(t, "json_object", gjson.GetBytes(retryBody, "text.format.type").String())
	require.False(t, gjson.GetBytes(retryBody, "text.format.schema").Exists(),
		"schema 必须整体丢弃：只支持 json_object 的上游见到多余字段同样报错")
	require.False(t, gjson.GetBytes(retryBody, "text.format.strict").Exists())
	require.Contains(t, gjson.GetBytes(retryBody, "instructions").String(), "json",
		"json_object 模式要求 prompt/instructions 内出现 json 关键词（小写字面，见 deepSeekJSONKeywordSystemHintLead）")

	// 其余字段不能被动到
	require.Equal(t, "claude-opus-4.8", gjson.GetBytes(retryBody, "model").String())
	require.Equal(t, "Extract the person.", gjson.GetBytes(retryBody, "input.0.content.0.text").String())
}

// 幂等是防死循环的关键：重试预算虽然只放行"没见过的 body"，但降级函数本身也必须是
// 一次性的——否则同一份 body 会被反复改写。
func TestNormalizeOpenAIResponsesRejectedJSONSchemaFormatIsIdempotent(t *testing.T) {
	body := []byte(responsesJSONSchemaRequestBody)
	retryBody, _, changed, err := normalizeOpenAIResponsesRejectedFieldRetryBody(
		http.StatusBadRequest, body, []byte(responsesJSONSchemaUpstreamRejection))
	require.NoError(t, err)
	require.True(t, changed)

	_, _, changedAgain, err := normalizeOpenAIResponsesRejectedFieldRetryBody(
		http.StatusBadRequest, retryBody, []byte(responsesJSONSchemaUpstreamRejection))
	require.NoError(t, err)
	require.False(t, changedAgain, "已经降级成 json_object 后不能再触发")
}

func TestNormalizeOpenAIResponsesRejectedFieldIgnoresSchemaContentErrors(t *testing.T) {
	body := []byte(responsesJSONSchemaRequestBody)

	// schema 内容写错 ≠ 上游不支持。降级会悄悄丢掉客户端要的严格结构保证，
	// 所以这类 400 必须原样透出，让客户端自己修。
	cases := []string{
		`{"error":{"code":"invalid_request_error","message":"Invalid schema for response_format 'person': missing type"}}`,
		`{"error":{"code":"invalid_request_error","message":"Invalid schema: 'additionalProperties' must be false"}}`,
		`{"error":{"code":"invalid_request_error","message":"Invalid json_schema: schema is invalid"}}`,
		// 关键用例：同时含"能力缺失"措辞（not supported）与"内容错误"措辞
		// （Invalid schema）。必须判否——它是客户端 schema 写错，不是上游不支持。
		`{"error":{"code":"invalid_request_error","message":"Invalid schema for response_format 'person': 'anyOf' is not supported in strict mode"}}`,
	}
	for _, raw := range cases {
		_, _, changed, err := normalizeOpenAIResponsesRejectedFieldRetryBody(http.StatusBadRequest, body, []byte(raw))
		require.NoError(t, err)
		require.False(t, changed, "schema 内容错误不应触发降级: %s", raw)
	}
}

// 请求体本身没发 json_schema 时，即便上游措辞命中也不能改写——否则会把一个无关
// 请求悄悄改成另一种语义。
func TestNormalizeOpenAIResponsesRejectedFieldSkipsWhenNoJSONSchemaSent(t *testing.T) {
	body := []byte(`{"model":"gpt-5.4","input":[{"role":"user","content":"hi"}]}`)
	_, _, changed, err := normalizeOpenAIResponsesRejectedFieldRetryBody(
		http.StatusBadRequest, body, []byte(responsesJSONSchemaUpstreamRejection))
	require.NoError(t, err)
	require.False(t, changed)
}

// 非 400 不进入任何重试分支。
func TestNormalizeOpenAIResponsesRejectedFieldSkipsNonBadRequest(t *testing.T) {
	_, _, changed, err := normalizeOpenAIResponsesRejectedFieldRetryBody(
		http.StatusTooManyRequests, []byte(responsesJSONSchemaRequestBody), []byte(responsesJSONSchemaUpstreamRejection))
	require.NoError(t, err)
	require.False(t, changed)
}

// instructions 为非字符串形态（新版 Responses API 允许数组）时不能改写，宁可让上游
// 按原样报错也不能把结构写坏。
func TestEnsureOpenAIResponsesJSONKeywordSkipsNonStringInstructions(t *testing.T) {
	body := []byte(`{"model":"m","instructions":[{"type":"input_text","text":"be brief"}],"text":{"format":{"type":"json_object"}}}`)
	updated := ensureOpenAIResponsesJSONKeyword(body, "")
	require.Equal(t, string(body), string(updated))
}

// 已有 instructions 且含 json 时不重复注入。
func TestEnsureOpenAIResponsesJSONKeywordKeepsExistingJSONMention(t *testing.T) {
	body := []byte(`{"model":"m","instructions":"Return JSON only.","text":{"format":{"type":"json_object"}}}`)
	updated := ensureOpenAIResponsesJSONKeyword(body, "")
	require.Equal(t, "Return JSON only.", gjson.GetBytes(updated, "instructions").String())
}

// 已有 instructions 但不含 json：追加而不是覆盖。
func TestEnsureOpenAIResponsesJSONKeywordAppendsToExistingInstructions(t *testing.T) {
	body := []byte(`{"model":"m","instructions":"Be concise.","text":{"format":{"type":"json_object"}}}`)
	updated := ensureOpenAIResponsesJSONKeyword(body, "")
	instructions := gjson.GetBytes(updated, "instructions").String()
	require.Contains(t, instructions, "Be concise.", "原 instructions 不能被覆盖")
	require.Contains(t, instructions, "json")
}

// 注入的关键词必须是**小写 ascii 的 "json"**。
//
// 上游的校验是"prompt 里要有 json 字样"，其报错引用的就是小写 `json`
// （"Prompt must contain the word 'json'"）。若只给大写 `JSON`，按小写字面匹配的
// 实现依然会拒——而且报的是另一个 400，与 json_schema 被拒长得完全不同，很难排查。
// 这里把"必须含小写 json"钉死，防止将来有人把文案改回大写。
func TestInjectedJSONKeywordIsLowercase(t *testing.T) {
	require.Contains(t, deepSeekJSONKeywordSystemHintLead, "json")

	// Responses 侧
	body := []byte(`{"model":"m","text":{"format":{"type":"json_schema","schema":{"type":"object"}}}}`)
	updated := ensureOpenAIResponsesJSONKeyword(body, "")
	require.Contains(t, gjson.GetBytes(updated, "instructions").String(), "json")

	// CC 侧（DeepSeek 预判降级路径注入的是同一条文案）
	chatBody := []byte(`{"model":"deepseek-chat","messages":[{"role":"user","content":"hi"}],"response_format":{"type":"json_schema","json_schema":{"name":"p","schema":{"type":"object"}}}}`)
	downgraded, changed := downgradeCCJSONSchemaResponseFormat(chatBody)
	require.True(t, changed)
	require.Contains(t, gjson.GetBytes(downgraded, "messages.0.content").String(), "json")
}

// retryOpenAIResponsesRejectedFieldOnce 依赖"同一入站请求内的多次账号尝试共享一份
// 重试预算"。这里把该语义钉住：不同账号尝试各自 new 一个 state，但 Allow 共用同一个
// 计数器，并且见过的 body 不再放行（去重）。
//
// 若将来有人把 openAIResponsesRejectedFieldRetryStateForRequest 改成每次新建预算，
// 这个用例会红——那意味着"每账号各重试一次"的失控行为又回来了。
func TestResponsesRejectedFieldRetryBudgetIsSharedAcrossAccountAttempts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	initial := []byte(`{"model":"m","text":{"format":{"type":"json_schema"}}}`)
	attemptA := openAIResponsesRejectedFieldRetryStateForRequest(c, initial)
	attemptB := openAIResponsesRejectedFieldRetryStateForRequest(c, initial)

	require.True(t, attemptA.Allow([]byte(`{"retry":1}`)), "第一个账号尝试应放行")
	require.True(t, attemptB.Allow([]byte(`{"retry":2}`)), "第二个账号尝试共用同一预算，也应放行")
	require.False(t, attemptA.Allow([]byte(`{"retry":1}`)), "同一 body 不得重放（去重）")
}

func TestIsUpstreamJSONSchemaUnsupportedMessage(t *testing.T) {
	supported := []string{
		"Model 'deepseek/deepseek-v4.1-flash' does not support 'json_schema' response format. Supported formats: json_object.",
		"This response_format type is unavailable now",
		"json_schema is not supported on this model",
		"structured outputs are unsupported for this model",
		"response format is not available",
		// Responses 侧字段名：整句里既无 json_schema 也无 response_format
		"Unsupported parameter: 'text.format' is not supported for this model",
	}
	for _, message := range supported {
		require.True(t, isUpstreamJSONSchemaUnsupportedMessage(message), "应命中: %q", message)
	}

	ignored := []string{
		"",
		"model does not exist",
		"Invalid schema for response_format 'person': missing type",
		"Invalid schema for response_format 'person': 'anyOf' is not supported in strict mode",
		"json_schema is invalid",
		"rate limit exceeded",
		"unsupported model",
		// 前缀被中转站丢掉/重写后的形态：句子里同样出现"被拒字段 + not supported"，
		// 若被判成能力缺失就会静默降级，让客户端失去它要的严格结构保证。
		// 这组是 2026-09-21 实测出的漏判，必须一直为 false。
		"response_format.schema: '$ref' is not supported in strict mode",
		"json_schema: 'anyOf' is not supported",
		"Unsupported keyword 'additionalProperties' in response_format.json_schema",
		"Unsupported schema keyword: additionalProperties",
		"Property 'name' is not supported in strict mode",
		"In context=(), 'additionalProperties' is required to be supplied and to be false.",
		"'$ref' is not supported in strict mode.",
		"response_format.json_schema: 'patternProperties' is not supported",
	}
	for _, message := range ignored {
		require.False(t, isUpstreamJSONSchemaUnsupportedMessage(message), "不应命中: %q", message)
	}
}
