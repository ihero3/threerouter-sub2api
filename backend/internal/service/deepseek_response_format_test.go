package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// DeepSeek 结构化输出降级回归：json_schema -> json_object，并保证 prompt 内出现
// DeepSeek 官方硬性要求的 "json" 关键词。
func TestNormalizeDeepSeekResponseFormat(t *testing.T) {
	t.Parallel()

	deepseekAccount := &Account{Name: "顺建捷deepseek", Platform: PlatformDeepseek, Type: AccountTypeAPIKey}
	openAIRelayAccount := &Account{Name: "relay", Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	kimiAccount := &Account{Name: "kimi", Platform: PlatformKimi, Type: AccountTypeAPIKey}

	jsonSchemaBody := `{"model":"deepseek-flash","max_tokens":4096,"messages":[{"role":"user","content":"提取这段文本的信息"}],` +
		`"response_format":{"type":"json_schema","json_schema":{"name":"extract","strict":true,` +
		`"schema":{"type":"object","properties":{"title":{"type":"string"}},"required":["title"]}}}}`

	t.Run("deepseek 账号：降级并注入 json 关键词与 schema", func(t *testing.T) {
		normalized, changed := NormalizeDeepSeekResponseFormat(deepseekAccount, "deepseek-flash", []byte(jsonSchemaBody))
		require.True(t, changed)

		require.Equal(t, "json_object", gjson.GetBytes(normalized, "response_format.type").String())
		require.False(t, gjson.GetBytes(normalized, "response_format.json_schema").Exists(),
			"json_schema 必须整体丢弃：DeepSeek 对 json_object 携带额外字段同样报错")

		// 其余字段原样保留
		require.Equal(t, "deepseek-flash", gjson.GetBytes(normalized, "model").String())
		require.EqualValues(t, 4096, gjson.GetBytes(normalized, "max_tokens").Int())

		// 前置一条 system 提示：含 "json" 关键词 + 客户端 schema
		require.Equal(t, "system", gjson.GetBytes(normalized, "messages.0.role").String())
		hint := gjson.GetBytes(normalized, "messages.0.content").String()
		require.Contains(t, hint, "JSON")
		require.Contains(t, hint, "title")
		require.Equal(t, "user", gjson.GetBytes(normalized, "messages.1.role").String())
		require.Equal(t, "提取这段文本的信息", gjson.GetBytes(normalized, "messages.1.content").String())
		require.EqualValues(t, 2, gjson.GetBytes(normalized, "messages.#").Int())
	})

	t.Run("openai 平台账号但上游模型是 deepseek-*：同样降级（截图场景）", func(t *testing.T) {
		normalized, changed := NormalizeDeepSeekResponseFormat(openAIRelayAccount, "deepseek-flash", []byte(jsonSchemaBody))
		require.True(t, changed)
		require.Equal(t, "json_object", gjson.GetBytes(normalized, "response_format.type").String())
	})

	t.Run("首条 system 已含 json：只降级，不改动 messages", func(t *testing.T) {
		body := `{"model":"deepseek-chat","messages":[{"role":"system","content":"仅输出 json"},` +
			`{"role":"user","content":"hi"}],"response_format":{"type":"json_schema","json_schema":{"schema":{"type":"object"}}}}`
		normalized, changed := NormalizeDeepSeekResponseFormat(deepseekAccount, "deepseek-chat", []byte(body))
		require.True(t, changed)
		require.Equal(t, "json_object", gjson.GetBytes(normalized, "response_format.type").String())
		require.EqualValues(t, 2, gjson.GetBytes(normalized, "messages.#").Int())
		require.Equal(t, "仅输出 json", gjson.GetBytes(normalized, "messages.0.content").String())
	})

	t.Run("首条 system 无 json：就地追加，不新增消息", func(t *testing.T) {
		body := `{"model":"deepseek-chat","messages":[{"role":"system","content":"你是助手"},` +
			`{"role":"user","content":"hi"}],"response_format":{"type":"json_schema","json_schema":{"schema":{"type":"object"}}}}`
		normalized, changed := NormalizeDeepSeekResponseFormat(deepseekAccount, "deepseek-chat", []byte(body))
		require.True(t, changed)
		require.EqualValues(t, 2, gjson.GetBytes(normalized, "messages.#").Int())
		require.Contains(t, gjson.GetBytes(normalized, "messages.0.content").String(), "JSON")
		require.Contains(t, gjson.GetBytes(normalized, "messages.0.content").String(), "你是助手")
	})

	t.Run("多模态 content 分段已含 json：不注入", func(t *testing.T) {
		body := `{"model":"deepseek-chat","messages":[{"role":"user","content":[{"type":"text","text":"输出 JSON 结果"},` +
			`{"type":"image_url","image_url":{"url":"data:image/png;base64,xx"}}]}],` +
			`"response_format":{"type":"json_schema","json_schema":{"schema":{"type":"object"}}}}`
		normalized, changed := NormalizeDeepSeekResponseFormat(deepseekAccount, "deepseek-chat", []byte(body))
		require.True(t, changed)
		require.EqualValues(t, 1, gjson.GetBytes(normalized, "messages.#").Int())
		require.Equal(t, "json_schema", gjson.Get(body, "response_format.type").String(), "入参不得被就地修改")
	})

	cases := []struct {
		name    string
		account *Account
		model   string
		body    string
	}{
		{
			name:    "kimi 原生支持 json_schema：不动",
			account: kimiAccount,
			model:   "kimi-k3",
			body:    jsonSchemaBody,
		},
		{
			name:    "已是 json_object：不动",
			account: deepseekAccount,
			model:   "deepseek-chat",
			body:    `{"model":"deepseek-chat","messages":[{"role":"user","content":"hi"}],"response_format":{"type":"json_object"}}`,
		},
		{
			name:    "无 response_format：不动",
			account: deepseekAccount,
			model:   "deepseek-chat",
			body:    `{"model":"deepseek-chat","messages":[{"role":"user","content":"hi"}]}`,
		},
		{
			name:    "非 json_schema 的其他类型：不动",
			account: deepseekAccount,
			model:   "deepseek-chat",
			body:    `{"model":"deepseek-chat","messages":[{"role":"user","content":"hi"}],"response_format":{"type":"text"}}`,
		},
		{
			name:    "非法 JSON：原样返回",
			account: deepseekAccount,
			model:   "deepseek-chat",
			body:    `{"model":"deepseek-chat",`,
		},
		{
			name:    "空 body：原样返回",
			account: deepseekAccount,
			model:   "deepseek-chat",
			body:    ``,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			normalized, changed := NormalizeDeepSeekResponseFormat(tc.account, tc.model, []byte(tc.body))
			require.False(t, changed)
			require.Equal(t, tc.body, string(normalized))
		})
	}
}
