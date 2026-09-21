package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// ---------------------------------------------------------------------------
// 线上工单 f74e374a 的回归：客户端用 Responses 结构化输出（text.format=
// json_schema），账号「星图」（platform=openai，中转 DeepSeek）走
// 入站 /v1/responses → 上游 /v1/chat/completions 的回退路径，
// 因该路径未接上游归一化而被上游 400 拒绝。
// ---------------------------------------------------------------------------

// responsesJSONSchemaRequestBody 是客户端（Codex / SDK 结构化输出）会发的形态。
const responsesJSONSchemaRequestBody = `{
  "model": "claude-opus-4.8",
  "stream": false,
  "input": [{"role": "user", "content": [{"type": "input_text", "text": "Extract the person."}]}],
  "text": {
    "format": {
      "type": "json_schema",
      "name": "person",
      "strict": true,
      "schema": {"type": "object", "properties": {"name": {"type": "string"}}, "required": ["name"]}
    }
  }
}`

func TestIsDeepSeekModelID(t *testing.T) {
	cases := []struct {
		model string
		want  bool
	}{
		{"deepseek", true},
		{"deepseek-chat", true},
		{"deepseek-v4.1-flash", true},
		{"DeepSeek-V3", true},
		{"deepseek_v3", true},
		// 中转站的 vendor/model 斜杠形式：线上上游报错里回显的模型名就是它。
		{"deepseek/deepseek-v4.1-flash", true},
		{"DeepSeek/DeepSeek-V3", true},
		{"openai/gpt-5.4", false},
		{"gpt-5.4", false},
		{"deep-seek", false},
		{"", false},
		{"/", false},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, IsDeepSeekModelID(tc.model), "model=%q", tc.model)
	}
}

// 复现整条链路：Responses 结构化输出 → 转成 CC → 上游归一化。
func TestCCUpstreamBodyNormalizesDeepSeekJSONSchema(t *testing.T) {
	var req apicompat.ResponsesRequest
	require.NoError(t, json.Unmarshal([]byte(responsesJSONSchemaRequestBody), &req))

	chatReq, err := apicompat.ResponsesToChatCompletionsRequestWithOptions(&req, &apicompat.ResponsesToChatOptions{})
	require.NoError(t, err)
	chatBody, err := json.Marshal(chatReq)
	require.NoError(t, err)

	// 转换层本身是对的：Responses 的 text.format=json_schema 会落成 CC 的
	// response_format=json_schema。问题出在这之后没人做上游兼容降级。
	require.Equal(t, "json_schema", gjson.GetBytes(chatBody, "response_format.type").String())

	// 账号 platform=openai（中转站）+ 斜杠形式模型名：
	// 早期实现两个条件都不匹配前置的 DeepSeek 判定，因此整段漏判。
	account := &Account{Platform: PlatformOpenAI, Name: "星图"}
	normalized, changed := normalizeOpenAICCUpstreamBody(account, "deepseek/deepseek-v4.1-flash", chatBody)
	require.True(t, changed, "DeepSeek 系模型 + json_schema 必须触发降级")
	require.Equal(t, "json_object", gjson.GetBytes(normalized, "response_format.type").String())
	require.False(t, gjson.GetBytes(normalized, "response_format.json_schema").Exists(),
		"json_schema 必须整体丢弃：只支持 json_object 的上游见到多余字段同样报错")
	require.Contains(t, strings.ToLower(gjson.GetBytes(normalized, "messages.0.content").String()), "json",
		"DeepSeek 要求 prompt 内出现 json 关键词")

	// platform=deepseek 的账号同样要降级
	native := &Account{Platform: PlatformDeepseek, Name: "ds"}
	_, changed = normalizeOpenAICCUpstreamBody(native, "deepseek-chat", chatBody)
	require.True(t, changed)
}

func TestCCUpstreamBodyLeavesNonDeepSeekUntouched(t *testing.T) {
	var req apicompat.ResponsesRequest
	require.NoError(t, json.Unmarshal([]byte(responsesJSONSchemaRequestBody), &req))
	chatReq, err := apicompat.ResponsesToChatCompletionsRequestWithOptions(&req, &apicompat.ResponsesToChatOptions{})
	require.NoError(t, err)
	chatBody, err := json.Marshal(chatReq)
	require.NoError(t, err)

	// Kimi 官方原生支持 json_schema，不能降级（降级会丢掉严格结构保证）。
	account := &Account{Platform: PlatformOpenAI, Name: "kimi-relay"}
	normalized, changed := normalizeOpenAICCUpstreamBody(account, "kimi-k2.6", chatBody)
	require.False(t, changed, "非 DeepSeek 上游不应被降级")
	require.Equal(t, string(chatBody), string(normalized))
}

func TestIsCCJSONSchemaUnsupportedError(t *testing.T) {
	// 线上真实报文（工单 f74e374a）
	production := []byte(`{"error":{"code":"Bad Request","message":"[trace_id: 80f43d0f] Model 'deepseek/deepseek-v4.1-flash' does not support 'json_schema' response format. Supported formats: json_object.","type":"invalid_request_error"}}`)
	require.True(t, isCCJSONSchemaUnsupportedError(production))

	// DeepSeek 官方早期措辞
	require.True(t, isCCJSONSchemaUnsupportedError([]byte(`{"error":{"message":"This response_format type is unavailable now","type":"invalid_request_error"}}`)))

	require.False(t, isCCJSONSchemaUnsupportedError(nil))
	// schema 本身写错，不是能力缺失——不该触发自愈重试（否则掩盖客户端问题）
	require.False(t, isCCJSONSchemaUnsupportedError([]byte(`{"error":{"message":"Invalid schema for response_format 'person': missing type"}}`)))
	// 与 json_schema 无关的 400
	require.False(t, isCCJSONSchemaUnsupportedError([]byte(`{"error":{"message":"model does not exist"}}`)))
}

// 枚举式拒绝措辞：DTO SUPPORTED VALUES / MUST BE ONE OF 一类。
//
// 关键是**维度核对**：必须确认清单里列的是 response_format 这一维度的取值，
// 否则 "json_schema: 'type' must be one of 'object','array'" 这类**客户端 schema
// 内容错误**会被当成能力缺失降级重发，而重发必然成功（schema 被整个丢掉），
// 客户端静默失去严格结构保证。
func TestIsUpstreamJSONSchemaUnsupportedMessageEnumVerbs(t *testing.T) {
	capability := []string{
		"response_format must be one of 'text', 'json_object'",
		"Invalid value 'json_schema' for parameter 'response_format.type'. Supported values are: 'text', 'json_object'",
		"`structured output` must be one of: `text`, `json_object`",
	}
	for _, message := range capability {
		require.True(t, isUpstreamJSONSchemaUnsupportedMessage(message), message)
	}

	// 同一句措辞，但清单是**别的维度** → 不得判成能力缺失。
	notCapability := []string{
		"response_format.json_schema: 'type' must be one of 'object', 'array'",
		"response_format.json_schema: supported values are 'object', 'array', 'string'",
	}
	for _, message := range notCapability {
		require.False(t, isUpstreamJSONSchemaUnsupportedMessage(message), message)
	}
}

// 带空格的 Schema 关键字写法（additional properties / pattern properties）：
// 只认并行写法（additionalproperties）时会漏，漏了就是"静默降级"。
func TestIsUpstreamJSONSchemaUnsupportedMessageRejectsSpacedSchemaKeywords(t *testing.T) {
	for _, message := range []string{
		"response_format.json_schema: additional properties are not supported",
		"json_schema: pattern properties are not supported on this model",
		"Invalid 'schema': Additional properties are not allowed in strict mode",
	} {
		require.False(t, isUpstreamJSONSchemaUnsupportedMessage(message), message)
	}
}

// "unknown parameter" 必须核对被拒的参数本身是不是 structured-output 相关：
// "json_schema: unknown parameter 'maxItems' in schema" 说的是一个 Schema 关键字，
// 判成能力缺失会把严格结构悄悄降级掉。
func TestUpstreamJSONSchemaUnknownParamTargetsOutput(t *testing.T) {
	// 针对结构化输出参数 → 放行。
	for _, message := range []string{
		"Unknown parameter: 'response_format.json_schema'",
		"unknown parameter 'text.format' supplied for this model",
		// 没给参数名（无引号）→ 无从判别，按已有目标词判定放行。
		"Unrecognized request argument supplied: response_format",
	} {
		require.True(t, isUpstreamJSONSchemaUnsupportedMessage(message), message)
	}
	// 针对 Schema 内部的未知关键字 → 不放行。
	require.False(t, isUpstreamJSONSchemaUnsupportedMessage("json_schema: unknown parameter 'maxItems' in schema"))
	require.False(t, isUpstreamJSONSchemaUnsupportedMessage("json_schema: unknown parameter 'pattern' in schema"))
}

func TestDowngradeCCJSONSchemaResponseFormat(t *testing.T) {
	body := []byte(`{"model":"deepseek-chat","messages":[{"role":"user","content":"hi"}],"response_format":{"type":"json_schema","json_schema":{"name":"p","schema":{"type":"object"}}}}`)

	downgraded, changed := downgradeCCJSONSchemaResponseFormat(body)
	require.True(t, changed)
	require.Equal(t, "json_object", gjson.GetBytes(downgraded, "response_format.type").String())
	require.False(t, gjson.GetBytes(downgraded, "response_format.json_schema").Exists())
	require.Equal(t, "deepseek-chat", gjson.GetBytes(downgraded, "model").String(), "不应动到其它字段")

	// 已经是 json_object / 没有 response_format：不重复降级（否则自愈会死循环重试）
	_, changed = downgradeCCJSONSchemaResponseFormat(downgraded)
	require.False(t, changed)
	_, changed = downgradeCCJSONSchemaResponseFormat([]byte(`{"model":"x","messages":[]}`))
	require.False(t, changed)

	// 非法 JSON 不介入
	garbage := []byte(`{not json`)
	out, changed := downgradeCCJSONSchemaResponseFormat(garbage)
	require.False(t, changed)
	require.Equal(t, garbage, out)
}
