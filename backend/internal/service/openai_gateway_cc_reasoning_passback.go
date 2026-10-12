package service

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// DeepSeek 思考模式的 reasoning_content 回传兼容。
//
// DeepSeek 官方文档（Thinking Mode → Tool Calls）明确规定：
//
//	携带了 tools 参数的请求，在后续所有请求中，必须完整回传 reasoning_content
//	给 API——即使该轮模型未实际进行工具调用。若未正确回传，API 会返回 400：
//	The `reasoning_content` in the thinking mode must be passed back to the API.
//
// 而 reasoning_content 不属于标准 OpenAI Chat Completions 消息字段，绝大多数
// OpenAI 兼容客户端（SDK / agent 框架）在重建历史时只保留自己认识的键
// （role / content / tool_calls / tool_call_id），该字段被静默丢弃；下一轮带上
// 工具结果再请求时就必然 400（opencode / claude-code-router / n8n 等均命中过同一问题）。
//
// 网关作为中转平台无法要求第三方客户端补齐字段，因此做两层防御：
//
//  1. **预判回填**（applyCCReasoningPassback，接在所有 CC 出口共用的
//     normalizeOpenAICCUpstreamBody 里）：识别到 DeepSeek 系上游且请求携带 tools
//     时，主动补齐字段，正常情况下请求根本不会失败。
//  2. **报错自愈**（isCCReasoningContentPassbackError + backfillCCReasoningContent）：
//     万一上游仍以此报 400（模型名被映射成无法识别的别名等），按上游报文再补一次
//     并重发一次。两道防线共用同一段回填逻辑。
//
// 线上工单：请求 e975d24c-59f9-4f65-a763-0318234848e9（账号「deepseek顾」id 33，
// 入站 /v1/chat/completions → 上游 /v1/chat/completions，模型 deepseek-v4-flash
// → deepseek-flash，流式，400）。该路径是 CC 直转（forwardAsRawChatCompletions），
// 与先前的 CC→Responses 桥接回注修复（ReasoningContentByCallID）是**两条不同的
// 链路**，因此那一处修复覆盖不到它。

// ccReasoningPassbackPlaceholder 是回填用的占位推理内容。
//
// 必须是**单个空格**而不是空字符串：DeepSeek V4-Pro 收紧校验后会拒绝空的
// reasoning_content（同样报 "must be passed back"），而单空格能同时满足
// 「字段必须存在」与「非空」两重检查，且不会把伪造的推理内容混入上下文
// （DeepSeek 在无工具调用的轮次本就会忽略该字段）。
//
// 参见 hermes-agent 回归测试 test_deepseek_reasoning_content_echo.py：
//   - reasoning_content 完全缺失 → 400
//   - reasoning_content: ""（空字符串）→ V4-Pro 400
//   - reasoning_content: " "（单空格）→ 200
const ccReasoningPassbackPlaceholder = " "

// applyCCReasoningPassback 在命中「思考模式强制回传」契约时，为携带 tools 的
// 请求体补齐 assistant 消息上的 reasoning_content 字段；未命中则逐字节原样返回。
//
// 返回改写后的 body 与是否发生改写。改写失败一律回退原 body（不阻断转发）。
func applyCCReasoningPassback(account *Account, upstreamModel string, body []byte) ([]byte, bool) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body, false
	}
	if !shouldBackfillCCReasoningContent(account, upstreamModel) {
		return body, false
	}
	// 官方契约只在「携带 tools 的会话」上成立：没有 tools 时既不会触发 400，
	// 也无工具调用产生的推理需要回传，此时补字段纯属伪造上下文。
	if !ccBodyHasTools(body) {
		return body, false
	}
	updated, backfilled, changed := backfillCCReasoningContent(body)
	if !changed {
		return body, false
	}
	logger.LegacyPrintf("service.openai_gateway",
		"[DeepSeek] reasoning_content backfilled on %d assistant message(s) to satisfy thinking-mode passback (account: %s, account_platform: %s, upstream_model: %s)",
		backfilled, accountDisplayName(account), accountDisplayPlatform(account), upstreamModel)
	return updated, true
}

// backfillCCReasoningContent 无条件对请求体执行字段回填（不做账号/模型判定，也不打
// 日志）。供报错自愈路径使用——此时是上游自己声明了该要求，无需再猜厂商；该路径的
// 账号与模型信息已由 sendCCUpstreamRequest 的自愈日志记录。
//
// 返回改写后的 body、回填条数与是否发生改写；没有可补的 assistant 消息时
// changed=false，从而让自愈链路收敛（不重发）。
func backfillCCReasoningContent(body []byte) ([]byte, int, bool) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body, 0, false
	}
	messages := gjson.GetBytes(body, "messages")
	if !messages.IsArray() || len(messages.Array()) == 0 {
		return body, 0, false
	}

	updated := body
	backfilled := 0
	for i, msg := range messages.Array() {
		if msg.Get("role").String() != "assistant" {
			continue
		}
		// 客户端已回传非空 reasoning_content：原样尊重，绝不覆盖。
		if _, ok := jsonNonEmptyString(msg.Get("reasoning_content")); ok {
			continue
		}
		// 部分 OpenAI 兼容客户端把推理放在别名键上（reasoning / thinking）。
		// 能取到真实推理就优先用它，取不到才落占位符——回填真实推理比占位更贴近
		// 上游期望的上下文。
		value := ccAssistantReasoningAlias(msg)
		if value == "" {
			value = ccReasoningPassbackPlaceholder
		}
		next, err := sjson.SetBytes(updated, "messages."+strconv.Itoa(i)+".reasoning_content", value)
		if err != nil {
			return body, 0, false
		}
		updated = next
		backfilled++
	}
	if backfilled == 0 {
		return body, 0, false
	}
	return updated, backfilled, true
}

// isCCReasoningContentPassbackError 判断上游 400 报文是不是「思考模式必须回传
// reasoning_content」这一类。
//
// 判定只认「reasoning_content」与「passed back」同时出现，避免把无关 400 误判。
// 注意 DeepSeek 的 Responses 协议变体报的是 `reasoning_text`（不同链路），
// 本函数服务 CC 出口，因此只认 reasoning_content。
func isCCReasoningContentPassbackError(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "error.message").String()))
	if message == "" {
		// 少数上游把原因放在顶层或 error 不是对象，退化到整包匹配。
		message = strings.ToLower(string(body))
	}
	return strings.Contains(message, "reasoning_content") && strings.Contains(message, "passed back")
}

// shouldBackfillCCReasoningContent 判断该账号/上游模型是否是**已知必须**回传
// reasoning_content 的 DeepSeek 上游。判定取「账号平台 / 模型名 / base_url」三路
// 信号的并集，因为同一个 DeepSeek 上游既可能挂在 deepseek 平台账号上，也可能挂在
// openai/composite 平台账号 + 自定义 base_url 上（模型名还可能被映射成别名，
// 如线上工单的 deepseek-v4-flash → deepseek-flash）。
//
// **为什么预判只覆盖 DeepSeek，而不捎带其它 thinking 厂商**：
//
// 预判回填会往请求体里**注入一个上游没要求的字段**，这属于有破坏风险的方向
// （误补：把本可正常工作的请求改坏）。只有对「契约已被权威确认（官方文档 + 多份
// 独立实现回归测试）、且要求是硬性 400」的厂商才值得冒这个风险——目前只有
// DeepSeek 满足。kimi / glm / minimax / qwen*-thinking 等虽然也可能要求回传，但
// 我们没有一个能站住脚的依据去断言「它们一定硬性要求」以及「占位符 ' ' 一定被
// 接受」，贸然预判注入就是凭厂商表猜——正是本项目明令禁止的做法
// （见 openai_reasoning_effort_compat.go 的「上游说了算」范式）。
//
// 对这些厂商改用**报错自愈**（isCCReasoningContentPassbackError +
// backfillCCReasoningContent）：上游真的报「pass back reasoning_content」时再补，
// 首轮多一次上游往返，但客户端仍拿到成功响应，且失败路径零行为变化。
//
// 排除项都是**有破坏风险**的方向，宁可不补也不误补：
//   - Grok/xAI：没有 reasoning_content 回传契约，注入陌生字段可能被拒；
//   - Anthropic 官方语义（claude-/opus-/sonnet-/haiku-）：其 thinking block 走
//     signature 校验，与本字段无关。
func shouldBackfillCCReasoningContent(account *Account, upstreamModel string) bool {
	if account == nil {
		return false
	}
	if account.Platform == PlatformGrok {
		return false
	}
	if ResolveThinkingProtocol(upstreamModel) == ThinkingProtocolAnthropicStrict {
		return false
	}
	if account.Platform == PlatformDeepseek || IsDeepSeekModelID(upstreamModel) {
		return true
	}
	return accountBaseURLLooksDeepSeek(account)
}

// accountBaseURLLooksDeepSeek 通过账号 base_url 识别 DeepSeek 上游。
// 覆盖「platform=openai + 自定义 base_url 指向 DeepSeek + 模型名被映射成不含
// deepseek 的别名」这类配置——此时仅靠模型名判定会整体漏判。
func accountBaseURLLooksDeepSeek(account *Account) bool {
	if account == nil || account.Credentials == nil {
		return false
	}
	baseURL, _ := account.Credentials["base_url"].(string)
	return strings.Contains(strings.ToLower(baseURL), "deepseek")
}

// ccBodyHasTools 判断请求体是否携带工具声明（tools 或旧版 functions）。
func ccBodyHasTools(body []byte) bool {
	for _, path := range []string{"tools", "functions"} {
		value := gjson.GetBytes(body, path)
		if value.IsArray() && len(value.Array()) > 0 {
			return true
		}
	}
	return false
}

// ccAssistantReasoningAlias 从 assistant 消息的别名键里取回真实推理内容。
func ccAssistantReasoningAlias(msg gjson.Result) string {
	for _, key := range []string{"reasoning", "thinking"} {
		if value, ok := jsonNonEmptyString(msg.Get(key)); ok {
			return value
		}
	}
	return ""
}
