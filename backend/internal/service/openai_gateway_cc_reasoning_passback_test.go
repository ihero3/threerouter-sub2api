package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// ---------------------------------------------------------------------------
// CC 出口 reasoning_content 回传兼容的回归测试。
//
// 线上工单：请求 e975d24c-59f9-4f65-a763-0318234848e9（账号「deepseek顾」id 33，
// 平台 openai，入站 /v1/chat/completions → 上游 /v1/chat/completions，模型
// deepseek-v4-flash → deepseek-flash，流式 400）：
//
//	The `reasoning_content` in the thinking mode must be passed back to the API.
//
// 该用例锁死两层防御：预判回填（DeepSeek 系）与报错自愈（兜底），并锁死
// 「非适用请求逐字节不变」这条安全边界。
// ---------------------------------------------------------------------------

// ccPassbackIncidentAccount 复刻线上账号：platform=openai，但凭据 base_url 指向
// DeepSeek，模型名被映射成别名——单看任一信号都会漏判，必须多路并集才认得出来。
func ccPassbackIncidentAccount() *Account {
	return &Account{
		ID:          33,
		Name:        "deepseek顾",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"base_url": "https://api.deepseek.com"},
	}
}

// ccPassbackToolsBody 是「携带 tools + assistant 历史缺 reasoning_content」的真实
// 请求形态——正是触发 400 的那一类。
func ccPassbackToolsBody() string {
	return `{"model":"deepseek-flash","stream":true,"messages":[` +
		`{"role":"user","content":"查一下天气"},` +
		`{"role":"assistant","content":"","tool_calls":[{"id":"call_0","type":"function","function":{"name":"get_weather","arguments":"{}"}}]},` +
		`{"role":"tool","tool_call_id":"call_0","content":"{\"temp\":20}"}` +
		`],"tools":[{"type":"function","function":{"name":"get_weather","parameters":{"type":"object"}}}],"tool_choice":"auto"}`
}

// TestApplyCCReasoningPassbackBackfillsMissingReasoningContent 复刻线上场景：
// 账号/模型命中 DeepSeek 契约 + 携带 tools + assistant 缺字段 → 补单空格占位符。
func TestApplyCCReasoningPassbackBackfillsMissingReasoningContent(t *testing.T) {
	out, changed := applyCCReasoningPassback(ccPassbackIncidentAccount(), "deepseek-flash", []byte(ccPassbackToolsBody()))
	require.True(t, changed, "命中契约时必须发生改写")

	require.True(t, gjson.GetBytes(out, "messages.1.reasoning_content").Exists(),
		"assistant 消息必须被补上 reasoning_content")
	require.Equal(t, ccReasoningPassbackPlaceholder, gjson.GetBytes(out, "messages.1.reasoning_content").String(),
		"占位符必须是单空格（空字符串会被 DeepSeek V4-Pro 同样拒 400）")
	require.Equal(t, " ", ccReasoningPassbackPlaceholder)

	// 非 assistant 消息不得被注入字段。
	require.False(t, gjson.GetBytes(out, "messages.0.reasoning_content").Exists())
	require.False(t, gjson.GetBytes(out, "messages.2.reasoning_content").Exists())

	// 其余字段必须原样保留（只增不改）。
	require.Equal(t, "call_0", gjson.GetBytes(out, "messages.1.tool_calls.0.id").String())
	require.True(t, gjson.GetBytes(out, "stream").Bool())
}

// TestApplyCCReasoningPassbackPreservesExistingReasoningContent 客户端已回传非空
// reasoning_content 时必须逐字节保留，绝不覆盖为占位符。
func TestApplyCCReasoningPassbackPreservesExistingReasoningContent(t *testing.T) {
	body := `{"model":"deepseek-flash","messages":[` +
		`{"role":"user","content":"hi"},` +
		`{"role":"assistant","content":"","reasoning_content":"真实推理链","tool_calls":[{"id":"c0","type":"function","function":{"name":"f","arguments":"{}"}}]},` +
		`{"role":"tool","tool_call_id":"c0","content":"{}"}` +
		`],"tools":[{"type":"function","function":{"name":"f","parameters":{"type":"object"}}}]}`

	out, changed := applyCCReasoningPassback(ccPassbackIncidentAccount(), "deepseek-flash", []byte(body))
	require.False(t, changed, "已有非空 reasoning_content 时不应报告改写")
	require.Equal(t, body, string(out), "必须逐字节原样上行，不得覆盖客户端回传的推理")
}

// TestApplyCCReasoningPassbackReusesReasoningAlias 部分 OpenAI 兼容客户端把推理放在
// 别名键（reasoning / thinking）上；能取到真实推理时优先用它，而不是落占位符。
func TestApplyCCReasoningPassbackReusesReasoningAlias(t *testing.T) {
	for _, tc := range []struct {
		name  string
		alias string
		want  string
	}{
		{"reasoning 别名", `"reasoning":"来自 reasoning 键的推理"`, "来自 reasoning 键的推理"},
		{"thinking 别名", `"thinking":"来自 thinking 键的推理"`, "来自 thinking 键的推理"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"model":"deepseek-flash","messages":[` +
				`{"role":"user","content":"hi"},` +
				`{"role":"assistant","content":"",` + tc.alias + `,"tool_calls":[{"id":"c0","type":"function","function":{"name":"f","arguments":"{}"}}]},` +
				`{"role":"tool","tool_call_id":"c0","content":"{}"}` +
				`],"tools":[{"type":"function","function":{"name":"f","parameters":{"type":"object"}}}]}`

			out, changed := applyCCReasoningPassback(ccPassbackIncidentAccount(), "deepseek-flash", []byte(body))
			require.True(t, changed)
			require.Equal(t, tc.want, gjson.GetBytes(out, "messages.1.reasoning_content").String(),
				"应优先复用别名键里的真实推理")
		})
	}
}

// TestApplyCCReasoningPassbackSkipsWithoutTools 官方契约只在「携带 tools 的会话」
// 上成立：没有 tools 时既不会 400，也无推理需要回传，补字段纯属伪造上下文。
func TestApplyCCReasoningPassbackSkipsWithoutTools(t *testing.T) {
	body := `{"model":"deepseek-flash","messages":[` +
		`{"role":"user","content":"hi"},` +
		`{"role":"assistant","content":"上一轮回答"}` +
		`]}`

	out, changed := applyCCReasoningPassback(ccPassbackIncidentAccount(), "deepseek-flash", []byte(body))
	require.False(t, changed, "无 tools 时必须逐字节不变")
	require.Equal(t, body, string(out))
}

// TestApplyCCReasoningPassbackSkipsNonDeepSeekAccounts 预判回填只认 DeepSeek。
// 其余厂商（含 kimi/glm/minimax/qwen*-thinking 等 thinking 族）一律不预判注入，
// 改由报错自愈处理——这是「上游说了算」范式，防止凭厂商表猜而误补。
func TestApplyCCReasoningPassbackSkipsNonDeepSeekAccounts(t *testing.T) {
	noAssistantReasoning := `{"model":"x","messages":[` +
		`{"role":"user","content":"hi"},` +
		`{"role":"assistant","content":"","tool_calls":[{"id":"c0","type":"function","function":{"name":"f","arguments":"{}"}}]}` +
		`],"tools":[{"type":"function","function":{"name":"f","parameters":{"type":"object"}}}]}`

	cases := []struct {
		name    string
		account *Account
		model   string
	}{
		{"nil 账号", nil, "deepseek-flash"},
		{"grok 平台", &Account{ID: 1, Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{}}, "deepseek-flash"},
		{"openai 中转 + GPT", &Account{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{}}, "gpt-5.4"},
		{"anthropic 官方模型", &Account{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{}}, "claude-opus-4.8"},
		{"kimi 不预判", &Account{ID: 4, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{}}, "kimi-k2.6"},
		{"glm 不预判", &Account{ID: 5, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{}}, "glm-4.7"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, changed := applyCCReasoningPassback(tc.account, tc.model, []byte(noAssistantReasoning))
			require.False(t, changed, "非 DeepSeek 契约不得预判注入字段")
			require.Equal(t, noAssistantReasoning, string(out))
		})
	}
}

// TestApplyCCReasoningPassbackDeepSeekSignals 三路 DeepSeek 信号各自独立成立。
func TestApplyCCReasoningPassbackDeepSeekSignals(t *testing.T) {
	body := ccPassbackToolsBody()
	cases := []struct {
		name    string
		account *Account
		model   string
	}{
		{"deepseek 平台账号", &Account{ID: 1, Platform: PlatformDeepseek, Type: AccountTypeAPIKey, Credentials: map[string]any{}}, "some-alias"},
		{"模型名含 deepseek", &Account{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{}}, "deepseek-chat"},
		{"base_url 指向 deepseek", ccPassbackIncidentAccount(), "some-alias"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, changed := applyCCReasoningPassback(tc.account, tc.model, []byte(body))
			require.True(t, changed, "任一路信号命中即可回填")
			require.True(t, gjson.GetBytes(out, "messages.1.reasoning_content").Exists())
		})
	}
}

// TestBackfillCCReasoningContentIsIdempotent 回填必须收敛：补过一次之后再补不再变，
// 否则自愈链路会无限重发。
func TestBackfillCCReasoningContentIsIdempotent(t *testing.T) {
	first, filled, changed := backfillCCReasoningContent([]byte(ccPassbackToolsBody()))
	require.True(t, changed)
	require.Equal(t, 1, filled)

	second, filled2, changed2 := backfillCCReasoningContent(first)
	require.False(t, changed2, "已补齐的请求体再次回填必须报告无改写")
	require.Equal(t, 0, filled2)
	require.Equal(t, string(first), string(second))
}

// TestBackfillCCReasoningContentFillsOnlyAssistantMessages 只补 assistant 且只补缺字段的。
func TestBackfillCCReasoningContentFillsOnlyAssistantMessages(t *testing.T) {
	body := `{"model":"x","messages":[` +
		`{"role":"system","content":"sys"},` +
		`{"role":"assistant","content":"a1"},` +
		`{"role":"assistant","content":"a2","reasoning_content":"已存在"},` +
		`{"role":"assistant","content":"a3"}` +
		`]}`

	out, filled, changed := backfillCCReasoningContent([]byte(body))
	require.True(t, changed)
	require.Equal(t, 2, filled, "只补两条缺字段的 assistant")
	require.Equal(t, " ", gjson.GetBytes(out, "messages.1.reasoning_content").String())
	require.Equal(t, "已存在", gjson.GetBytes(out, "messages.2.reasoning_content").String(), "既有值不得覆盖")
	require.Equal(t, " ", gjson.GetBytes(out, "messages.3.reasoning_content").String())
	require.False(t, gjson.GetBytes(out, "messages.0.reasoning_content").Exists())
}

// TestBackfillCCReasoningContentRejectsMalformed 畸形/空请求体一律原样返回。
func TestBackfillCCReasoningContentRejectsMalformed(t *testing.T) {
	for _, body := range []string{"", "{", `{"model":"x"}`, `{"messages":[]}`} {
		out, filled, changed := backfillCCReasoningContent([]byte(body))
		require.False(t, changed, "body=%q", body)
		require.Equal(t, 0, filled)
		require.Equal(t, body, string(out))
	}
}

// TestIsCCReasoningContentPassbackError 判定只认 reasoning_content + passed back 同时出现。
func TestIsCCReasoningContentPassbackError(t *testing.T) {
	match := []string{
		`{"error":{"message":"The ` + "`reasoning_content`" + ` in the thinking mode must be passed back to the API. (request_id: 769a8ca7-b0af-4713-9282-34ef31441ac4)","type":"invalid_request_error"}}`,
		`{"error":{"message":"reasoning_content must be passed back","type":"invalid_request_error"}}`,
		// error 不是对象时的整包退化匹配。
		`{"detail":"The reasoning_content in the thinking mode must be passed back to the API"}`,
	}
	for _, body := range match {
		require.True(t, isCCReasoningContentPassbackError([]byte(body)), "应命中：%s", body)
	}

	noMatch := []string{
		``,
		`{"error":{"message":"Upstream rejected the request","type":"invalid_request_error"}}`,
		`{"error":{"message":"Invalid request: unknown field 'foo'"}}`,
		// Responses 协议变体报 reasoning_text，属另一条链路，不得误判。
		`{"error":{"message":"The ` + "`reasoning_text`" + ` in the thinking mode must be passed back to the API."}}`,
		// 只出现半个关键词。
		`{"error":{"message":"reasoning_content is invalid"}}`,
		`{"error":{"message":"something must be passed back"}}`,
	}
	for _, body := range noMatch {
		require.False(t, isCCReasoningContentPassbackError([]byte(body)), "不应命中：%s", body)
	}
}

// TestCCUpstreamSelfHealBodyHandlesReasoningPassback 报错自愈链路：上游以该 400 报错
// 时（模型名被映射成无法识别的别名等预判漏判情形），补齐字段并重发一次。
func TestCCUpstreamSelfHealBodyHandlesReasoningPassback(t *testing.T) {
	errorBody := []byte(`{"error":{"message":"The ` + "`reasoning_content`" + ` in the thinking mode must be passed back to the API.","type":"invalid_request_error"}}`)

	filled, reason, changed := ccUpstreamSelfHealBody([]byte(ccPassbackToolsBody()), errorBody)
	require.True(t, changed)
	require.Equal(t, "reasoning_content passback required", reason)
	require.True(t, gjson.GetBytes(filled, "messages.1.reasoning_content").Exists())

	// 无可补内容时（已带非空 reasoning_content）必须收敛为不改写。
	alreadyOK := `{"model":"deepseek-flash","messages":[` +
		`{"role":"assistant","content":"","reasoning_content":"有","tool_calls":[{"id":"c0","type":"function","function":{"name":"f","arguments":"{}"}}]}` +
		`],"tools":[{"type":"function","function":{"name":"f","parameters":{"type":"object"}}}]}`
	_, _, changed = ccUpstreamSelfHealBody([]byte(alreadyOK), errorBody)
	require.False(t, changed, "无可补内容时不得报告改写（否则自愈会无限重发）")

	// 上游没报这件事时，自愈不得介入。
	_, _, changed = ccUpstreamSelfHealBody([]byte(ccPassbackToolsBody()),
		[]byte(`{"error":{"message":"Invalid request: unknown field"}}`))
	require.False(t, changed)
}

// TestNormalizeOpenAICCUpstreamBodyBackfillsDeepSeekReasoning 端到端：走公共出口
// 归一化即可救活线上工单那类请求。
func TestNormalizeOpenAICCUpstreamBodyBackfillsDeepSeekReasoning(t *testing.T) {
	out, changed := normalizeOpenAICCUpstreamBody(ccPassbackIncidentAccount(), "deepseek-flash", []byte(ccPassbackToolsBody()))
	require.True(t, changed)
	require.Equal(t, " ", gjson.GetBytes(out, "messages.1.reasoning_content").String())

	// 再次归一化必须幂等（不报告改写），保证上层不会重复改写。
	_, changedAgain := normalizeOpenAICCUpstreamBody(ccPassbackIncidentAccount(), "deepseek-flash", out)
	require.False(t, changedAgain, "归一化必须幂等")
}
