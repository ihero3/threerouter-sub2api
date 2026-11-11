package service

import (
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// CC 上游：max_tokens 与 max_completion_tokens 同传导致的 400。
//
// 背景：max_completion_tokens 是 OpenAI 为推理模型引入的新字段，旧字段 max_tokens
// 仍被广泛支持。不少客户端（以及 SDK 组合、用户手写的 config）为了"新旧模型都能跑"
// 会把两个字段一起发出去。多数 OpenAI 兼容上游对此宽容（取其一），但有一部分把它
// 当互斥入参直接 400：
//
//	The parameter `max_tokens` specified in the request are not valid:
//	max_tokens and max_completion_tokens cannot be set at the same time.
//
// 线上工单：请求 725d5416-49ca-4834-af56-a9a2dfeae028（账号「A1-趋境--词元甄选1」
// id 63，入站 /v1/chat/completions → 上游 /v1/chat/completions，模型
// deepseek-v4-pro → deepseek-v4.1-flash-1017，流式，400）。
//
// 为什么只做"报错自愈"而不做"预判合流"：预判性地删掉其中一个字段，对**忽略**该字段
// 的上游等于悄悄取消了输出上限（不少老版本 vLLM / SGLang / one-api 派生只认
// max_tokens，把 max_completion_tokens 当未知键丢弃），可能产生超长输出与额外计费。
// 这种"静默失效"比一次可见的 400 更难排查，正是本仓库一贯拒绝的副作用类型。因此
// 只在**上游明确说"两个不能同传"**时才动手，与其余四类自愈同构。

// ccMaxTokensConflictPhrases 是上游表达"两个额度字段互斥"的常见措辞。
//
// 判定同时要求报文里出现 max_tokens 与 max_completion_tokens 两个字段名，再叠加
// 这里任意一个措辞——单靠措辞会把"max_tokens 超过模型上限"这类无关 400 也算进来。
var ccMaxTokensConflictPhrases = []string{
	"at the same time",
	"together",
	"mutually exclusive",
	"only one of",
}

// isCCMaxTokensConflictError 判断 CC 上游 400 是否因为 max_tokens 与
// max_completion_tokens 同时下发（上游视为互斥入参）。
//
// 注意 "max_completion_tokens" 并不包含子串 "max_tokens"，两者可独立判定。
func isCCMaxTokensConflictError(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "error.message").String()))
	if message == "" {
		// 少数上游把原因放在顶层或 error 不是对象，退化到整包匹配。
		message = strings.ToLower(string(body))
	}
	if !strings.Contains(message, "max_tokens") || !strings.Contains(message, "max_completion_tokens") {
		return false
	}
	for _, phrase := range ccMaxTokensConflictPhrases {
		if strings.Contains(message, phrase) {
			return true
		}
	}
	return false
}

// dropCCConflictingMaxTokens 删除 CC 请求体里的 max_tokens，保留 max_completion_tokens。
//
// 保留 max_completion_tokens 的理由与转换链既有口径一致：apicompat 的
// max_tokens / max_completion_tokens → max_output_tokens 转换同样是
// "prefer max_completion_tokens"（chatcompletions_to_responses.go），
// Antigravity 兼容层亦为 max_completion_tokens takes precedence。两条链路对同一个
// 冲突请求给出同一个解释，避免"CC 直转与协议转换取的值不一样"。
//
// 调用前提是上游已经明确拒绝"两者同传"：该上游既然点名了两个字段，就必然认识
// max_completion_tokens，因此不会出现"删掉 max_tokens 后上限被静默忽略"的情况。
//
// 请求体里两个字段不齐时不动手（返回 false）：那种 400 与本次修复的成因无关，
// 乱改只会把入参错误换个样子再报一遍。
func dropCCConflictingMaxTokens(body []byte) ([]byte, bool) {
	if len(body) == 0 {
		return body, false
	}
	if !gjson.GetBytes(body, "max_completion_tokens").Exists() || !gjson.GetBytes(body, "max_tokens").Exists() {
		return body, false
	}
	// 用 sjson 定点删除而不是 unmarshal/marshal：字段顺序与其它字节保持不变，
	// 与 stripThinkingFromBody 同口径，重发体的 diff 只有被删掉的这一个键。
	adjusted, err := sjson.DeleteBytes(body, "max_tokens")
	if err != nil {
		return body, false
	}
	return adjusted, true
}
