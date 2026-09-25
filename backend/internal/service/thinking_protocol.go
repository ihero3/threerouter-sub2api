package service

import "strings"

// ThinkingProtocol 描述上游对 thinking block 的处理契约。
// 不同上游对历史 thinking block 的语义要求是相反的：
//   - Anthropic 官方：要求 thinking block 携带有效 signature，否则 400
//     "thinking.signature: Field required"
//   - DeepSeek `/anthropic`、Kimi `/coding` 等第三方 Anthropic 兼容上游：
//     要求历史 thinking block 原样回传，否则 400
//     "The content[].thinking in the thinking mode must be passed back to the API"
//
// 见 .pensieve/short-term/knowledge/thinking-block-filter-third-party-upstream-inversion/
type ThinkingProtocol int

const (
	// ThinkingProtocolUnknown 表示无法识别协议族（默认保守不剥离）。
	ThinkingProtocolUnknown ThinkingProtocol = iota

	// ThinkingProtocolAnthropicStrict 表示 Anthropic 官方语义：
	// 历史 thinking block 必须携带有效 signature，缺失/非法签名应剥离。
	ThinkingProtocolAnthropicStrict

	// ThinkingProtocolPassbackRequired 表示第三方兼容上游语义：
	// 所有历史 thinking block 必须原样回传，预过滤会破坏契约。
	ThinkingProtocolPassbackRequired
)

// ResolveThinkingProtocol 根据「作为 thinking block 处理参考的模型 ID」推断 thinking 协议族。
//
// 传入参数的语义随调用路径不同：
//   - **Anthropic gateway**（转发原始 Anthropic 请求）：传 mappedModel（账号级 model mapping
//     后的上游 model ID）。例：用户配置「claude-sonnet-4-6 → deepseek-v4-pro」后，
//     传 deepseek-v4-pro 才能被正确判为 passback-required。
//   - **Gemini messages compat**（Anthropic body → Gemini upstream）：传 originalModel
//     （客户端 Anthropic 请求的 model ID）。原因：此场景下上游是 Gemini，但被剥
//     离的 body 是 Anthropic 格式，需按客户端请求的 Anthropic 子协议族判定剥离行为。
//
// 匹配规则按厂商前缀硬编码：
//   - anthropic-strict: claude-* / opus-* / sonnet-* / haiku-*
//   - passback-required: deepseek-* / kimi-* / moonshot-* / glm-* /
//     minimax-* / minimax-m* / (qwen-|qwen2-|qwen3-|qwen4-)*-thinking /
//     Kimi Code bare aliases k3 / k3-256k（精确匹配，避免宽泛 k3 前缀）
//   - unknown: 其他模型（保守不剥离）
//
// 已知局限：前缀贪婪匹配（如 `claudette-`、`claude-foreign-relay-` 也会被分类为
// strict）。当遇到伪装命名时改成显式名单匹配，但现实场景几乎不会出现。
//
// 不覆盖的厂商（截至 2026-04）：
//   - Doubao / Seed (ByteDance)：走 Volcano Engine OpenAI 协议，非 Anthropic 路径
//   - Hunyuan T1 (Tencent)：未提供 Anthropic 兼容端点
//   - 若未来出现这些厂商的 Anthropic 兼容代理，需扩展前缀列表
func ResolveThinkingProtocol(modelID string) ThinkingProtocol {
	if modelID == "" {
		return ThinkingProtocolUnknown
	}
	id := strings.ToLower(modelID)

	// Passback-required 优先匹配（特定厂商前缀），避免误判 claude-* 时也命中。
	// kimi-k3* 已由 kimi- 前缀覆盖；Kimi Code endpoint 的 bare model ID（k3 / k3-256k）
	// 无厂商前缀，仅精确匹配，避免 "foo-k3" 等未知型号被宽泛前缀误判。
	switch {
	case strings.HasPrefix(id, "deepseek-"),
		strings.HasPrefix(id, "kimi-"),
		strings.HasPrefix(id, "moonshot-"),
		strings.HasPrefix(id, "glm-"),
		id == "k3",
		id == "k3-256k":
		return ThinkingProtocolPassbackRequired
	}
	// MiniMax M 系列：走 https://api.minimax.io/anthropic 端点，
	// 官方明文要求 thinking block round-trip（interleaved thinking 协议）。
	// 实例：MiniMax-M2、MiniMax-M2.1、MiniMax-M2.5、MiniMax-M2.7、MiniMax-M2.7-highspeed
	// 大小写在 ToLower 后统一为 minimax-。
	if strings.HasPrefix(id, "minimax-m") {
		return ThinkingProtocolPassbackRequired
	}
	// Qwen thinking 变体：覆盖 qwen-/qwen2-/qwen3-/qwen4- 前缀 + 包含 -thinking
	// 实例：qwen3-235b-a22b-thinking-2507、qwen3-next-80b-a3b-thinking、qwen-3-72b-thinking
	if (strings.HasPrefix(id, "qwen-") ||
		strings.HasPrefix(id, "qwen2-") ||
		strings.HasPrefix(id, "qwen3-") ||
		strings.HasPrefix(id, "qwen4-")) && strings.Contains(id, "-thinking") {
		return ThinkingProtocolPassbackRequired
	}

	switch {
	case strings.HasPrefix(id, "claude-"),
		strings.HasPrefix(id, "opus-"),
		strings.HasPrefix(id, "sonnet-"),
		strings.HasPrefix(id, "haiku-"):
		return ThinkingProtocolAnthropicStrict
	}

	return ThinkingProtocolUnknown
}

// ShouldReplayToolCallReasoning 判断 Chat Completions→Responses 桥接是否应
// 把「产生 tool call 的那段推理」回注成 reasoning item
// （content[].reasoning_text + summary）。
//
// 这是 CC→Responses 桥接专用的判定，粒度比 ResolveThinkingProtocol 粗一层：
// 只排除 Anthropic 官方语义（thinking block 必须带有效 signature，reasoning
// item 回注不适用），其余一律启用——包括 ThinkingProtocolUnknown。
//
// 不能收窄成 `== ThinkingProtocolPassbackRequired`：ResolveThinkingProtocol
// 靠硬编码厂商前缀匹配（deepseek- / kimi- / glm- ...），而运营后台添加的
// 模型名不受控（glm4.6、deepseek_flash、自定义别名等），匹配不上就会静默
// 退回旧行为——把推理塞进 <thinking> 明文，上游照样 400。
//
// 放宽是安全的：只有请求里确实存在推理内容（客户端回传 reasoning_content，
// 或网关缓存里按 tool call id 命中）时才会生成 reasoning item；把推理作为
// reasoning item 回传是 Responses 协议的标准契约，比塞进可见明文更正确。
func ShouldReplayToolCallReasoning(modelID string) bool {
	return ResolveThinkingProtocol(modelID) != ThinkingProtocolAnthropicStrict
}

// ShouldPreFilterThinkingBlocks 判断是否应在转发前剥离无效 thinking block。
// 仅 anthropic-strict 协议族需要预过滤；passback-required/unknown 都跳过，
// 因为「保留 thinking block」对 anthropic-strict 之外的上游一律更安全。
func ShouldPreFilterThinkingBlocks(modelID string) bool {
	return ResolveThinkingProtocol(modelID) == ThinkingProtocolAnthropicStrict
}

// ShouldRectifyThinkingSignatureError 判断是否应在 400 后触发 thinking 签名整流 retry。
// 仅 anthropic-strict 触发；passback-required 路径的 400 一般不是签名缺失问题，
// retry 任何 thinking 变形都不会修好，反而会破坏契约。unknown 同理保守不 retry。
func ShouldRectifyThinkingSignatureError(modelID string) bool {
	return ResolveThinkingProtocol(modelID) == ThinkingProtocolAnthropicStrict
}

// ShouldApplyRetryFilters 判断是否应执行 retry 路径的 thinking/tool block 整流。
// 与预过滤保持对称：仅 anthropic-strict 走变形；passback-required 与 unknown
// 一律返回原 body 不变形——避免在不熟悉的上游上做出可能破坏契约的猜测。
func ShouldApplyRetryFilters(modelID string) bool {
	return ResolveThinkingProtocol(modelID) == ThinkingProtocolAnthropicStrict
}
