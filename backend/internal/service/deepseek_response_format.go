package service

import (
	"encoding/json"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/tidwall/gjson"
)

// DeepSeek 的 response_format 兼容适配。
//
// DeepSeek 官方 JSON Output 只实现了 response_format={"type":"json_object"}，
// 收到 OpenAI 新版结构化输出（{"type":"json_schema","json_schema":{...}}）会直接
// 返回 400：
//
//	{"error":{"message":"This response_format type is unavailable now",
//	          "type":"invalid_request_error"}}
//
// 而客户端（LangChain 的 with_structured_output、OpenAI SDK 的
// chat.completions.parse、Vercel AI SDK 的 generateObject、部分 IDE 插件等）
// 默认就会发送这种请求，因此网关侧做两件事：
//
//  1. response_format 降级为 {"type":"json_object"}。注意 json_schema 必须
//     **整体丢弃**：DeepSeek 只认这一个键，携带额外字段同样报错。
//  2. 官方要求 prompt 内出现 "json" 关键词（否则另报 "Prompt must contain the
//     word 'json'"）。缺失时补一条 system 提示，并把客户端的 JSON Schema 作为
//     参考一并带入，尽量贴近客户端的字段期望。
//
// 触发范围（两个信号取并集，因为 DeepSeek 上游可能挂在 deepseek 平台账号上，
// 也可能挂在 openai/composite 平台账号 + 模型映射到 deepseek-* 上）：
//   - 账号 platform = deepseek
//   - 上游模型名以 "deepseek" 开头
//
// Kimi 官方原生支持 json_schema（platform.moonshot.cn/docs/guide/response_format），
// 因此不做降级；GLM 尚未核实，暂不纳入。
const deepSeekJSONSchemaHintLimit = 1500

// deepSeekJSONKeywordSystemHintLead 是补进 prompt 的 JSON 输出提示。
//
// 必须**含小写 ascii 的 "json"**：上游的校验是"prompt 里要有 json 字样"，其报错与
// 文档引用的都是小写的 `json`（"Prompt must contain the word 'json'"）。早期这里写的
// 是大写 `JSON`，如果上游按小写字面匹配，这句提示形同没写，客户端会被再拒一次 400
// ——而那个 400 与本次工单（json_schema 类型被拒）长得完全不同，极难排查。
// 小写同时满足"按小写字面匹配"和"忽略大小写匹配"两种实现；反过来则不然。
const deepSeekJSONKeywordSystemHintLead = "请严格以 json 格式输出。"

// NormalizeDeepSeekResponseFormat 在命中 DeepSeek 结构化输出不兼容时改写请求体，
// 返回改写后的 body 与是否发生改写。未命中或改写失败时原样返回入参。
func NormalizeDeepSeekResponseFormat(account *Account, upstreamModel string, body []byte) ([]byte, bool) {
	if len(body) == 0 || !isDeepSeekResponseFormatTarget(account, upstreamModel) {
		return body, false
	}
	if strings.TrimSpace(gjson.GetBytes(body, "response_format.type").String()) != "json_schema" {
		return body, false
	}

	var reqBody map[string]any
	if err := decodeOpenAIJSONUseNumber(body, &reqBody); err != nil {
		// 解析失败说明请求体本身不合法，交给上游按原样报错，网关不介入。
		return body, false
	}

	schemaHint := deepSeekJSONSchemaHint(reqBody)
	reqBody["response_format"] = map[string]any{"type": "json_object"}
	injected := ensureDeepSeekJSONKeyword(reqBody, schemaHint)

	normalized, err := json.Marshal(reqBody)
	if err != nil {
		return body, false
	}
	logger.LegacyPrintf("service.openai_gateway",
		"[DeepSeek] response_format downgraded: json_schema -> json_object (account: %s, account_platform: %s, upstream_model: %s, json_keyword_injected: %t)",
		accountDisplayName(account), accountDisplayPlatform(account), upstreamModel, injected)
	return normalized, true
}

func isDeepSeekResponseFormatTarget(account *Account, upstreamModel string) bool {
	if account != nil && account.Platform == PlatformDeepseek {
		return true
	}
	return IsDeepSeekModelID(upstreamModel)
}

// IsDeepSeekModelID 识别 DeepSeek 系模型名。
//
// 覆盖三种写法，其中第三种是线上真实出现过的：
//   - deepseek
//   - deepseek-<xxx>
//   - <vendor>/deepseek-<xxx> —— 中转站（如本次的「星图」）普遍用 vendor/model
//     斜杠形式，且其错误回显里的模型名就是 "deepseek/deepseek-v4.1-flash"。
//     早期实现只认前两种，斜杠形式会整个漏判。
func IsDeepSeekModelID(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	if normalized == "" {
		return false
	}
	if idx := strings.LastIndex(normalized, "/"); idx >= 0 {
		normalized = normalized[idx+1:]
	}
	if normalized == "" {
		return false
	}
	return normalized == "deepseek" ||
		strings.HasPrefix(normalized, "deepseek-") ||
		strings.HasPrefix(normalized, "deepseek_")
}

func accountDisplayName(account *Account) string {
	if account == nil {
		return ""
	}
	return account.Name
}

func accountDisplayPlatform(account *Account) string {
	if account == nil {
		return ""
	}
	return account.Platform
}

// deepSeekJSONSchemaHint 提取客户端 schema 的可读文本，供 system 提示引用。
// 优先取 json_schema.schema（真正的结构定义），退化到整个 json_schema 对象。
func deepSeekJSONSchemaHint(reqBody map[string]any) string {
	format, ok := reqBody["response_format"].(map[string]any)
	if !ok {
		return ""
	}
	payload := format["json_schema"]
	if schema, ok := format["json_schema"].(map[string]any); ok {
		if inner, exists := schema["schema"]; exists {
			payload = inner
		}
	}
	if payload == nil {
		return ""
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return truncateString(string(raw), deepSeekJSONSchemaHintLimit)
}

// ensureDeepSeekJSONKeyword 保证 messages 内出现 "json" 关键词（DeepSeek 的硬性
// 要求）。返回是否注入了提示。已有 system 且 content 为字符串时就地追加，其余
// 情况前置一条 system 消息。
func ensureDeepSeekJSONKeyword(reqBody map[string]any, schemaHint string) bool {
	messages, ok := reqBody["messages"].([]any)
	if !ok || len(messages) == 0 {
		return false
	}
	if deepSeekMessagesMentionJSON(messages) {
		return false
	}

	hint := deepSeekJSONKeywordSystemHintLead
	if schemaHint != "" {
		hint += "\n输出须满足以下 JSON Schema：" + schemaHint
	}

	if first, ok := messages[0].(map[string]any); ok &&
		strings.EqualFold(strings.TrimSpace(stringValue(first["role"])), "system") {
		if content, ok := first["content"].(string); ok {
			first["content"] = content + "\n\n" + hint
			return true
		}
	}

	reqBody["messages"] = append([]any{map[string]any{"role": "system", "content": hint}}, messages...)
	return true
}

func deepSeekMessagesMentionJSON(messages []any) bool {
	for _, message := range messages {
		obj, ok := message.(map[string]any)
		if !ok {
			continue
		}
		if deepSeekContentMentionsJSON(obj["content"]) {
			return true
		}
	}
	return false
}

// deepSeekContentMentionsJSON 覆盖字符串 content 与多模态分段数组（text 分段）。
func deepSeekContentMentionsJSON(content any) bool {
	switch value := content.(type) {
	case string:
		return strings.Contains(strings.ToLower(value), "json")
	case []any:
		for _, part := range value {
			obj, ok := part.(map[string]any)
			if !ok {
				continue
			}
			if deepSeekContentMentionsJSON(obj["text"]) {
				return true
			}
		}
	}
	return false
}
