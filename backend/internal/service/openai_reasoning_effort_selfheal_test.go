package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// ---------------------------------------------------------------------------
// 线上工单 b16d2f40 的回归：上游不接受客户端给的 reasoning_effort 取值。
//
// 账号「国外：阿里（moxing）」，qwen3.8-max → deepseek-v4-flash，
// 入站 /v1/chat/completions → 上游 /v1/chat/completions：
//
//	{"error":{"code":"invalid_value",
//	          "message":"'reasoning_effort' must be one of: 'low', 'medium', 'high', 'xhigh', 'max'",
//	          "param":null,"type":"invalid_request_error"}}
//
// 平台内部认可的档位是 minimal/low/medium/high/xhigh/max —— 上游集合少了 minimal。
// 自愈做法：**照上游报文里给出的清单就近对齐**后重发一次，不维护厂商表。
// ---------------------------------------------------------------------------

// ccSelfHealReasoningEffortRejectionBody 是线上真实报文（阿里 moxing 中转）。
const ccSelfHealReasoningEffortRejectionBody = `{"error":{"code":"invalid_value","message":"'reasoning_effort' must be one of: 'low', 'medium', 'high', 'xhigh', 'max'","param":null,"type":"invalid_request_error"},"request_id":"chatcmpl-cf9926a5-3068-40f8-89cf-d214397a7425"}`

// ccSelfHealReasoningEffortBody 是客户端发来的、会导致上面那个 400 的请求体。
const ccSelfHealReasoningEffortBody = `{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"hi"}],"reasoning_effort":"minimal"}`

func TestUpstreamReasoningEffortAllowedValues(t *testing.T) {
	cases := []struct {
		name    string
		message string
		want    []string
		wantOK  bool
	}{
		{
			// 线上真实报文：字段名在清单之前，绝不能被当成一个允许值。
			name:    "ticket_message",
			message: `'reasoning_effort' must be one of: 'low', 'medium', 'high', 'xhigh', 'max'`,
			want:    []string{"low", "medium", "high", "xhigh", "max"},
			wantOK:  true,
		},
		{
			name:    "double_quotes",
			message: `"reasoning_effort" must be one of: "low", "high"`,
			want:    []string{"low", "high"},
			wantOK:  true,
		},
		{
			name:    "backticks_and_dot_field",
			message: "`reasoning.effort` must be one of: `low`, `medium`",
			want:    []string{"low", "medium"},
			wantOK:  true,
		},
		{
			name:    "space_field_form",
			message: `reasoning effort must be one of: 'low', 'high'`,
			want:    []string{"low", "high"},
			wantOK:  true,
		},
		{
			name:    "allowed_values_wording",
			message: `invalid reasoning_effort; allowed values: 'low', 'max'`,
			want:    []string{"low", "max"},
			wantOK:  true,
		},
		{
			name:    "uppercase_message",
			message: `'REASONING_EFFORT' MUST BE ONE OF: 'LOW', 'HIGH'`,
			want:    []string{"low", "high"},
			wantOK:  true,
		},
		{
			// 没有档位清单：不能自行决定改成什么，必须原样交回上游错误。
			name:    "no_list",
			message: `'reasoning_effort' is not supported on this model`,
			wantOK:  false,
		},
		{
			// 清单里没有一个本平台认识的档位：无法判断相对高低，不介入。
			name:    "unrecognized_only",
			message: `'reasoning_effort' must be one of: 'auto', 'turbo'`,
			wantOK:  false,
		},
		{
			name:    "unrelated_message",
			message: `The model 'x' does not exist`,
			wantOK:  false,
		},
		{
			name:    "unrelated_invalid_value",
			message: `'temperature' must be one of: 'low', 'high'`,
			wantOK:  false,
		},
		{
			name:    "empty",
			message: ``,
			wantOK:  false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := upstreamReasoningEffortAllowedValues(tc.message)
			require.Equal(t, tc.wantOK, ok, "message=%q", tc.message)
			if tc.wantOK {
				require.Equal(t, tc.want, got)
			}
		})
	}
}

func TestUpstreamReasoningEffortAllowedValuesNeverIncludesFieldName(t *testing.T) {
	allowed, ok := upstreamReasoningEffortAllowedValues(`'reasoning_effort' must be one of: 'low', 'medium', 'high', 'xhigh', 'max'`)
	require.True(t, ok)
	for _, value := range allowed {
		require.NotEqual(t, "reasoning_effort", value, "字段名本身不得被当成允许档位")
		require.NotEqual(t, "reasoning.effort", value)
	}
}

func TestClampReasoningEffortToAllowedValues(t *testing.T) {
	// 线上那台上游的允许集。
	allowed := []string{"low", "medium", "high", "xhigh", "max"}

	cases := []struct {
		name       string
		body       string
		want       string
		wantPath   string
		wantChange bool
	}{
		// 核心场景：minimal 不在上游集合里 → 向上贴到最低档。
		{"minimal_to_low", `{"reasoning_effort":"minimal"}`, "low", "reasoning_effort", true},
		// none 是本平台不认识的取值（上游也没有"关闭"档）→ 同样贴最低档。
		{"none_to_low", `{"reasoning_effort":"none"}`, "low", "reasoning_effort", true},
		// 拼写错误/未知取值 → 贴最低档。
		{"unknown_to_low", `{"reasoning_effort":"banana"}`, "low", "reasoning_effort", true},
		// 已在允许集里 → 一个字节都不动（避免无意义重发）。
		{"medium_unchanged", `{"reasoning_effort":"medium"}`, "medium", "reasoning_effort", false},
		{"high_unchanged", `{"reasoning_effort":"high"}`, "high", "reasoning_effort", false},
		{"xhigh_unchanged", `{"reasoning_effort":"xhigh"}`, "xhigh", "reasoning_effort", false},
		{"max_unchanged", `{"reasoning_effort":"max"}`, "max", "reasoning_effort", false},
		// 同义写法（extrahigh 归一为 xhigh）：上游未必认同义词，落到字面档位。
		{"extrahigh_to_xhigh", `{"reasoning_effort":"extrahigh"}`, "xhigh", "reasoning_effort", true},
		// 嵌套字段（Responses 形态）。
		{"nested_minimal_to_low", `{"reasoning":{"effort":"minimal"}}`, "low", "reasoning.effort", true},
		// 没有该字段 → 不改（无关 400 不得被乱改）。
		{"absent_field", `{"temperature":0.5}`, "", "", false},
		// 字段不是字符串 → 不改。
		{"non_string_field", `{"reasoning_effort":3}`, "", "", false},
		// 空字符串 → 不改。
		{"empty_field", `{"reasoning_effort":""}`, "", "", false},
		// 空体 / 空允许集 → 不改。
		{"empty_body", ``, "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, changed := clampReasoningEffortToAllowedValues([]byte(tc.body), allowed)
			require.Equal(t, tc.wantChange, changed, "body=%s", tc.body)
			if !tc.wantChange {
				require.Equal(t, tc.body, string(out), "不改时必须是原字节")
				return
			}
			require.Equal(t, tc.want, gjson.GetBytes(out, tc.wantPath).String())
		})
	}
}

// 允许集比平台档位表**窄**时，要按上游给的区间夹紧，而不是套用平台档位。
func TestClampReasoningEffortToAllowedValuesRespectsNarrowUpstreamSets(t *testing.T) {
	// 只支持 high/max 的上游：minimal 要贴到 high（而不是平台里最近的 low）。
	out, changed := clampReasoningEffortToAllowedValues([]byte(`{"reasoning_effort":"minimal"}`), []string{"high", "max"})
	require.True(t, changed)
	require.Equal(t, "high", gjson.GetBytes(out, "reasoning_effort").String())

	// 只支持 low/medium/high 的上游：max 要向下夹到 high。
	out, changed = clampReasoningEffortToAllowedValues([]byte(`{"reasoning_effort":"max"}`), []string{"low", "medium", "high"})
	require.True(t, changed)
	require.Equal(t, "high", gjson.GetBytes(out, "reasoning_effort").String())

	// 区间内取秩最近，并列时取下限（更省推理）。
	out, changed = clampReasoningEffortToAllowedValues([]byte(`{"reasoning_effort":"minimal"}`), []string{"medium", "high"})
	require.True(t, changed)
	require.Equal(t, "medium", gjson.GetBytes(out, "reasoning_effort").String())

	// 允许集为空 → 不改。
	_, changed = clampReasoningEffortToAllowedValues([]byte(`{"reasoning_effort":"minimal"}`), nil)
	require.False(t, changed)
}

// 取值**已被上游字面列出**时不得改写，哪怕存在同义档位。
//
// 这条守的是同义档位并列的情形：上游若同时列出 extrahigh 与 xhigh（秩相同），
// 只按"秩最近"挑目标会挑到同义的另一个字面值，把上游明明接受的值换掉，
// 白白多发一次请求。必须先用字面匹配放行。
func TestClampReasoningEffortToAllowedValuesKeepsAlreadyAcceptedLiteral(t *testing.T) {
	// 两种书写顺序都要覆盖：结果不能随上游清单顺序抖动。
	for _, allowed := range [][]string{
		{"xhigh", "extrahigh"},
		{"extrahigh", "xhigh"},
	} {
		out, changed := clampReasoningEffortToAllowedValues([]byte(`{"reasoning_effort":"xhigh"}`), allowed)
		require.False(t, changed, "上游已列出该字面值，不得改写：allowed=%v", allowed)
		require.Equal(t, `{"reasoning_effort":"xhigh"}`, string(out))
	}

	// 对照：同义但**未被列出**的写法仍要归一到上游列出的字面值。
	out, changed := clampReasoningEffortToAllowedValues([]byte(`{"reasoning_effort":"extrahigh"}`), []string{"xhigh", "max"})
	require.True(t, changed)
	require.Equal(t, "xhigh", gjson.GetBytes(out, "reasoning_effort").String())
}

// 端到端：CC 出口的自愈调度器必须同时认得两类原因，且无关 400 一律不改。
func TestCCUpstreamSelfHealBodyReasonSelection(t *testing.T) {
	effortBody := []byte(ccSelfHealReasoningEffortBody)

	// 1) effort 取值被拒 → 改写 effort，返回对应原因。
	adjusted, reason, changed := ccUpstreamSelfHealBody(effortBody, []byte(ccSelfHealReasoningEffortRejectionBody))
	require.True(t, changed)
	require.Equal(t, "reasoning_effort value rejected", reason)
	require.Equal(t, "low", gjson.GetBytes(adjusted, "reasoning_effort").String())

	// 2) json_schema 被拒（原有分支不能被这次改动破坏）。
	schemaBody := []byte(ccSelfHealJSONSchemaBody)
	adjusted, reason, changed = ccUpstreamSelfHealBody(schemaBody, []byte(ccSelfHealUpstreamRejectionBody))
	require.True(t, changed)
	require.Equal(t, "json_schema response_format not supported", reason)
	require.Equal(t, "json_object", gjson.GetBytes(adjusted, "response_format.type").String())

	// 3) 无关 400 → 不改。
	_, _, changed = ccUpstreamSelfHealBody(effortBody, []byte(ccSelfHealPlainBadRequestBody))
	require.False(t, changed)

	// 4) 报文说了 effort 但**没有档位清单** → 不改（不能凭空决定改成什么）。
	_, _, changed = ccUpstreamSelfHealBody(effortBody, []byte(`{"error":{"message":"'reasoning_effort' is not supported on this model"}}`))
	require.False(t, changed)

	// 5) 有档位清单但请求体里没有该字段 → 不改。
	_, _, changed = ccUpstreamSelfHealBody([]byte(ccSelfHealPlainBody), []byte(ccSelfHealReasoningEffortRejectionBody))
	require.False(t, changed)

	// 6) 取值本来就合法 → 不改（不产生无意义重发）。
	legal := []byte(`{"model":"deepseek-v4-flash","messages":[],"reasoning_effort":"medium"}`)
	_, _, changed = ccUpstreamSelfHealBody(legal, []byte(ccSelfHealReasoningEffortRejectionBody))
	require.False(t, changed)
}

// 端到端 HTTP 级：客户端发 minimal，上游 400 → 自愈重发一次并带上 low，交回成功响应。
func TestCCSelfHealRetriesOnceOnReasoningEffortValueRejection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream := newCCSelfHealService(
		ccSelfHealScriptedResponse{status: http.StatusBadRequest, body: ccSelfHealReasoningEffortRejectionBody},
		ccSelfHealScriptedResponse{status: http.StatusOK, body: `{"id":"chatcmpl_ok","choices":[]}`},
	)

	resp := sendCCSelfHealRequest(t, svc, newCCSelfHealContext(), []byte(ccSelfHealReasoningEffortBody))

	require.Equal(t, http.StatusOK, resp.StatusCode, "自愈成功后应把成功响应交回调用方")
	require.Equal(t, 2, upstream.callCount(), "只允许重发一次")

	retried := upstream.bodyAt(1)
	require.Equal(t, "low", gjson.GetBytes(retried, "reasoning_effort").String(), "重发必须用对齐后的档位")
	require.Equal(t, "deepseek-v4-flash", gjson.GetBytes(retried, "model").String(), "不得动到其它字段")
	require.Equal(t, "hi", gjson.GetBytes(retried, "messages.0.content").String(), "不得动到 messages")
}

// 重发仍失败时交回**原始**响应：effort 自愈同样不许更换失败原因。
func TestCCSelfHealKeepsOriginalWhenReasoningEffortRetryFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream := newCCSelfHealService(
		ccSelfHealScriptedResponse{status: http.StatusBadRequest, body: ccSelfHealReasoningEffortRejectionBody},
		ccSelfHealScriptedResponse{status: http.StatusBadRequest, body: ccSelfHealPlainBadRequestBody},
	)

	resp := sendCCSelfHealRequest(t, svc, newCCSelfHealContext(), []byte(ccSelfHealReasoningEffortBody))

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Equal(t, 2, upstream.callCount())
	require.Equal(t, ccSelfHealReasoningEffortRejectionBody, string(svc.readUpstreamErrorBody(resp)),
		"重发仍失败必须交回原始响应")
}

// 客户端没发 reasoning_effort 时，同一条 400 不得触发任何重发（chat 行为不变）。
// 取值语义已经合法、只是**写法不同**（大小写 / 多余空白）时必须归一到上游的字面写法。
//
// 这条守的是一个能让自愈**永久失效**的组合：判据会先把两边的大小写抹平再比较，
// 于是 " medium " / "MEDIUM" 会被判成"已经合规"而直接放过——可大小写敏感的上游
// 照样拒，客户端就会一直拿 400，而这个功能正是为了救这类请求才存在的。
func TestClampReasoningEffortNormalizesWrittenForm(t *testing.T) {
	allowed := []string{"low", "medium", "high"}
	cases := []struct {
		name string
		body string
		path string
		want string
	}{
		{"trailing_space_top_level", `{"reasoning_effort":" medium "}`, "reasoning_effort", "medium"},
		{"uppercase_top_level", `{"reasoning_effort":"MEDIUM"}`, "reasoning_effort", "medium"},
		{"mixed_case_top_level", `{"reasoning_effort":"Medium"}`, "reasoning_effort", "medium"},
		{"trailing_space_nested", `{"reasoning":{"effort":" medium "}}`, "reasoning.effort", "medium"},
		{"uppercase_nested", `{"reasoning":{"effort":"HIGH"}}`, "reasoning.effort", "high"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, changed := clampReasoningEffortToAllowedValues([]byte(tc.body), allowed)
			require.True(t, changed, "写法不同必须归一：%s", tc.body)
			require.Equal(t, tc.want, gjson.GetBytes(out, tc.path).String())
		})
	}

	// 写法本来就规范 → 一个字节都不动（避免无意义重发）。
	for _, body := range []string{
		`{"reasoning_effort":"medium"}`,
		`{"reasoning":{"effort":"high"}}`,
	} {
		out, changed := clampReasoningEffortToAllowedValues([]byte(body), allowed)
		require.False(t, changed, "body=%s", body)
		require.Equal(t, body, string(out))
	}
}

// 上游把"不做推理"这一档列进允许集时，必须保留它——不能因为"认不出"就丢掉。
//
// 丢掉的后果是**语义反转**：客户端明确要关闭推理、上游恰恰支持这一档，而网关会
// 因取值无法判秩而向上贴到最低的启用档 low，等于替客户端把推理打开了。
// 反过来，"认不出"的拼写错误（banana）仍必须按既有规则贴到最低启用档，绝不许
// 被当成"要关闭"——那样会让一个笔误把推理直接关掉。
func TestUpstreamReasoningEffortAllowedValuesKeepsDisableValues(t *testing.T) {
	// 'none' 语义明确（关闭），必须留下，即使它无法参与秩比较。
	allowed, ok := upstreamReasoningEffortAllowedValues(
		`Invalid 'reasoning_effort': 'minimal'. Supported values are: 'none', 'low', 'medium', 'high'.`)
	require.True(t, ok)
	require.Contains(t, allowed, "none")

	// 认不出的取值仍旧丢弃：只留下 none 时说明其余全是生词，无从对齐。
	_, ok = upstreamReasoningEffortAllowedValues(`'reasoning_effort' must be one of: 'auto', 'turbo'`)
	require.False(t, ok, "既不判秩又不表示'关闭'的生词不得作为改写目标")
}

func TestClampReasoningEffortHonorsDisableSemantics(t *testing.T) {
	allowed := []string{"none", "low", "medium", "high"}

	// 客户端明确要关闭（含大小写/同义写法）→ 用上游列出的那一档。
	for _, body := range []string{
		`{"reasoning_effort":"off"}`,
		`{"reasoning_effort":"NONE"}`,
		`{"reasoning_effort":" disabled "}`,
	} {
		out, changed := clampReasoningEffortToAllowedValues([]byte(body), allowed)
		require.True(t, changed, "body=%s 必须保留'关闭'语义", body)
		require.Equal(t, "none", gjson.GetBytes(out, "reasoning_effort").String(), "body=%s", body)
	}

	// 刻意的分界线：客户端要的是**启用档**（minimal）时，即使上游列出了 none，
	// 也不许替它把推理关掉——保留"推理仍然存在"的最接近档 low。
	out, changed := clampReasoningEffortToAllowedValues([]byte(`{"reasoning_effort":"minimal"}`), allowed)
	require.True(t, changed)
	require.Equal(t, "low", gjson.GetBytes(out, "reasoning_effort").String())

	// 拼写错误同样不许被当成"要关闭"，仍旧贴到最低启用档。
	out, changed = clampReasoningEffortToAllowedValues([]byte(`{"reasoning_effort":"banana"}`), allowed)
	require.True(t, changed)
	require.Equal(t, "low", gjson.GetBytes(out, "reasoning_effort").String())
}

// 允许集里一个可判秩的启用档都没有（只有关闭档）时不得越界、也不得乱改。
func TestClampReasoningEffortWithDisableOnlyUpstreamSet(t *testing.T) {
	for _, body := range []string{
		`{"reasoning_effort":"minimal"}`,
		`{"reasoning_effort":"banana"}`,
		`{"reasoning_effort":"max"}`,
	} {
		out, changed := clampReasoningEffortToAllowedValues([]byte(body), []string{"none"})
		require.False(t, changed, "只有关闭档时无法做秩对齐：body=%s", body)
		require.Equal(t, body, string(out))
	}

	// 但要关闭的请求仍然能被救活。
	out, changed := clampReasoningEffortToAllowedValues([]byte(`{"reasoning_effort":"off"}`), []string{"none"})
	require.True(t, changed)
	require.Equal(t, "none", gjson.GetBytes(out, "reasoning_effort").String())
}

// 客户端压根没发 reasoning_effort 时，同一条 400 不得触发任何重发（chat 行为不变）。
//
// 注意用例名的措辞：这里的 "Omitted" 指的是**客户端没有发这个字段**，而不是发了一个
// 值为 "none" 的字段——后者由下面的 TestCCSelfHealHonorsDisableIntentWhenUpstreamListsNone 覆盖。
func TestCCSelfHealDoesNotRetryReasoningEffortWhenClientOmittedField(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream := newCCSelfHealService(ccSelfHealScriptedResponse{status: http.StatusBadRequest, body: ccSelfHealReasoningEffortRejectionBody})

	resp := sendCCSelfHealRequest(t, svc, newCCSelfHealContext(), []byte(ccSelfHealPlainBody))

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Equal(t, 1, upstream.callCount(), "客户端没发该字段，绝不能重发")
	require.Equal(t, ccSelfHealPlainBody, string(upstream.bodyAt(0)), "出站请求体必须原样")
}

// 端到端：客户端明确要关闭推理（"off"），而上游**确实列出**了关闭档 'none'。
//
// 守的是语义反转：'none' 无法参与秩比较，过去会被当成生词丢掉，于是"要关闭"会被
// 反向改写成最低的启用档 low——等于替客户端把推理打开（多花推理 token、结果也不同）。
func TestCCSelfHealHonorsDisableIntentWhenUpstreamListsNone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rejection := `{"error":{"code":"invalid_value","message":"Invalid 'reasoning_effort': 'off'. Supported values are: 'none', 'low', 'medium', 'high'","param":null,"type":"invalid_request_error"}}`
	svc, upstream := newCCSelfHealService(
		ccSelfHealScriptedResponse{status: http.StatusBadRequest, body: rejection},
		ccSelfHealScriptedResponse{status: http.StatusOK, body: `{"id":"chatcmpl_ok","choices":[]}`},
	)

	body := []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}],"reasoning_effort":"off"}`)
	resp := sendCCSelfHealRequest(t, svc, newCCSelfHealContext(), body)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, 2, upstream.callCount())
	require.Equal(t, "none", gjson.GetBytes(upstream.bodyAt(1), "reasoning_effort").String(),
		"客户端要关闭、上游有这一档时，必须保留'关闭'语义而不是抬到 low")
}

// ---------------------------------------------------------------------------
// Responses 侧：同一个判定要覆盖另外四条重试循环与 chat 链路那个一次性钩子。
// ---------------------------------------------------------------------------

const responsesReasoningEffortBody = `{"model":"gpt-5.4","input":"hi","reasoning":{"effort":"minimal"}}`

func TestResponsesRetryBodyClampsRejectedReasoningEffort(t *testing.T) {
	retryBody, reason, changed, err := normalizeOpenAIResponsesRejectedFieldRetryBody(
		http.StatusBadRequest, []byte(responsesReasoningEffortBody), []byte(ccSelfHealReasoningEffortRejectionBody))
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, openAIResponsesReasoningEffortValueRejectionReason, reason)
	require.Equal(t, "low", gjson.GetBytes(retryBody, "reasoning.effort").String())
	require.Equal(t, "hi", gjson.GetBytes(retryBody, "input").String(), "不得动到其它字段")

	// 再次判定必须收敛：改后的值已合法 → 不再产生改写（防重试循环打转）。
	_, _, changedAgain, err := normalizeOpenAIResponsesRejectedFieldRetryBody(
		http.StatusBadRequest, retryBody, []byte(ccSelfHealReasoningEffortRejectionBody))
	require.NoError(t, err)
	require.False(t, changedAgain, "对齐后必须收敛，否则会在重试预算内空转")
}

// CC 出口与 Responses 出口必须同源：字段名同样可能只出现在 error.param 里。
// 这条锁住 openai_responses_rejected_field_retry.go 里那处调用用的是
// WithParam 而不是只看 message——两边不同源就会退化成"只修一半"。
func TestResponsesRetryBodyClampsReasoningEffortNamedByParam(t *testing.T) {
	errorBody := []byte(`{"error":{"code":"invalid_value",` +
		`"message":"Invalid value 'minimal'. Supported values are: 'low', 'medium', 'high'",` +
		`"param":"reasoning.effort","type":"invalid_request_error"}}`)
	retryBody, reason, changed, err := normalizeOpenAIResponsesRejectedFieldRetryBody(
		http.StatusBadRequest, []byte(responsesReasoningEffortBody), errorBody)
	require.NoError(t, err)
	require.True(t, changed, "字段名在 error.param 里也必须能定位到 effort")
	require.Equal(t, openAIResponsesReasoningEffortValueRejectionReason, reason)
	require.Equal(t, "low", gjson.GetBytes(retryBody, "reasoning.effort").String())
}

// chat 链路上那个一次性钩子：effort 原因已在白名单内，重发成功要交回新响应。
func TestResponsesRetryOnceHealsReasoningEffortValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream := newCCSelfHealService(ccSelfHealScriptedResponse{status: http.StatusOK, body: `{"id":"resp_ok","output":[]}`})
	account := newResponsesRetryTestAccount()
	initial := responsesRetryInitialResponse(ccSelfHealReasoningEffortRejectionBody)

	got := svc.retryOpenAIResponsesRejectedFieldOnce(
		context.Background(), newResponsesRetryTestContext(), account,
		initial, []byte(responsesReasoningEffortBody), "tok", "", "", true, false,
	)

	require.NotNil(t, got)
	require.Equal(t, http.StatusOK, got.StatusCode)
	require.Equal(t, 1, upstream.callCount())
	require.Equal(t, "low", gjson.GetBytes(upstream.bodyAt(0), "reasoning.effort").String(),
		"重发必须用对齐后的档位")
}

// 客户端没发 effort 时，这条 400 在 chat 链路上仍是彻底的 no-op。
func TestResponsesRetryOnceIsNoOpForReasoningEffortWithoutField(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, upstream := newCCSelfHealService(ccSelfHealScriptedResponse{status: http.StatusOK, body: `{"id":"should_not_be_used"}`})
	account := newResponsesRetryTestAccount()
	initial := responsesRetryInitialResponse(ccSelfHealReasoningEffortRejectionBody)

	got := svc.retryOpenAIResponsesRejectedFieldOnce(
		context.Background(), newResponsesRetryTestContext(), account,
		initial, []byte(`{"model":"gpt-5.4","input":"hi"}`), "tok", "", "", true, false,
	)

	require.Same(t, initial, got)
	require.Equal(t, 0, upstream.callCount(), "没有该字段时不得发出任何重发")
}

// 清单**未加引号**的写法（"must be one of: low, medium"）必须也能解析。
//
// OpenAI 会加引号，但相当多中转站/自建网关直接输出裸词；只认引号会让这一类上游
// 完全拿不到自愈，而客户端照样拿 400。
func TestUpstreamReasoningEffortAllowedValuesBareWordList(t *testing.T) {
	allowed, ok := upstreamReasoningEffortAllowedValues(`reasoning effort must be one of: low, medium`)
	require.True(t, ok)
	require.Equal(t, []string{"low", "medium"}, allowed)

	allowed, ok = upstreamReasoningEffortAllowedValues(`'reasoning_effort' must be one of: minimal/low/medium/high`)
	require.True(t, ok)
	require.Contains(t, allowed, "low")
	require.Contains(t, allowed, "minimal")
}

// 裸词兜底的**精度闸门**：命中数不足 2 个一律不采信。
//
// 这是本文件里最要紧的一条约束。抓错的代价远大于漏掉：漏掉只是回到改动前
// （客户端拿 400），抓错会把客户端明确要求的档位**悄悄改低或改高**并成功返回，
// 是静默改变语义。句子里飘过一个 low/high 这类英文词是很常见的，必须挡住。
func TestUpstreamReasoningEffortBareWordFallbackRequiresMultipleValues(t *testing.T) {
	// 单个裸词：更可能是"恰好出现的英文单词"而不是枚举清单。
	_, ok := upstreamReasoningEffortAllowedValues(`'reasoning_effort' must be one of: too low`)
	require.False(t, ok, "只命中 1 个裸词不得采信")

	// 没有清单措辞 → 无论有没有裸词都不介入。
	_, ok = upstreamReasoningEffortAllowedValues(`'reasoning_effort' is not supported`)
	require.False(t, ok)

	// 说的是别的字段 → 不介入。
	_, ok = upstreamReasoningEffortAllowedValues(`'max_output_tokens' must be one of: low, medium`)
	require.False(t, ok)

	// 清单里全是生词 → 引号解析与裸词解析都无所获 → 不介入。
	_, ok = upstreamReasoningEffortAllowedValues(`'reasoning_effort' must be one of: 'auto','turbo'`)
	require.False(t, ok)
}

// 字段名被上游放在 error.param 里、message 里只写取值本身 —— OpenAI 与多数
// 中转站的标准形态。只看 message 会让这一整类报文判成"与我无关"而失去自愈。
func TestUpstreamReasoningEffortFieldNameCanComeFromErrorParam(t *testing.T) {
	// message 完全没提字段名，param 提了。
	allowed, ok := upstreamReasoningEffortAllowedValuesWithParam(
		"Invalid value 'minimal'. Supported values are: 'low', 'medium', 'high'",
		"reasoning_effort")
	require.True(t, ok)
	require.Equal(t, []string{"low", "medium", "high"}, allowed)

	// param 指向别的字段、message 也没提 effort → 仍不介入。
	_, ok = upstreamReasoningEffortAllowedValuesWithParam(
		"Invalid value 'minimal'. Supported values are: 'low', 'medium'",
		"max_output_tokens")
	require.False(t, ok)

	// 从整包错误体里取（包含 retryato 之类的噪声位置也要能命中）。
	body := []byte(`{"error":{"code":"invalid_value",` +
		`"message":"Invalid value 'minimal'. Supported values are: 'low', 'medium', 'high'",` +
		`"param":"reasoning_effort","type":"invalid_request_error"}}`)
	allowed, ok = upstreamReasoningEffortAllowedValuesFromErrorBody(body)
	require.True(t, ok)
	require.Equal(t, []string{"low", "medium", "high"}, allowed)

	// 同一条报文只拿 message 时不认 —— 这条断言锁住"必须接 param"这件事，
	// 防止有人日后为了简化把 param 那一路删掉却没测试变红。
	_, ok = upstreamReasoningEffortAllowedValues(`Invalid value 'minimal'. Supported values are: 'low', 'medium', 'high'`)
	require.False(t, ok)

	// param 指向嵌套字段。
	allowed, ok = upstreamReasoningEffortAllowedValuesWithParam(
		"must be one of: 'low', 'medium'", "reasoning.effort")
	require.True(t, ok)
	require.Equal(t, []string{"low", "medium"}, allowed)
}

// 上游报嵌套字段时只写 'effort'：这时本该从字段词里认出来。
// 放宽是安全的，因为清单措辞 + "取值必须是本平台认得的档位" + "客户端确实发了
// effort 字段才改写"这三重闸门仍在。
func TestUpstreamReasoningEffortAcceptsBareEffortFieldName(t *testing.T) {
	allowed, ok := upstreamReasoningEffortAllowedValues(`'effort' must be one of: 'low', 'medium'`)
	require.True(t, ok)
	require.Equal(t, []string{"low", "medium"}, allowed)

	// 说的是别的字段且不含 effort → 不放宽后误判。
	_, ok = upstreamReasoningEffortAllowedValues(`'verbosity' must be one of: 'low', 'medium', 'high'`)
	require.False(t, ok)
}

// allowed: 这种短写法要认；但被 not 否定（写的是**禁止**取值）时必须跳过，
// 否则会把请求改成一个上游明确不要的值。
func TestUpstreamReasoningEffortIgnoresNegatedAllowedList(t *testing.T) {
	allowed, ok := upstreamReasoningEffortAllowedValues(
		"reasoning_effort: invalid value 'minimal'; allowed: low|medium|high")
	require.True(t, ok)
	require.Equal(t, []string{"low", "medium", "high"}, allowed)

	// "'minimal' is not allowed: low, medium" —— 后面跟的是被禁止的取值。
	_, ok = upstreamReasoningEffortAllowedValues(`'reasoning_effort' 'minimal' is not allowed: low, medium`)
	require.False(t, ok, "被 not 否定的清单不得当作允许集")
}
