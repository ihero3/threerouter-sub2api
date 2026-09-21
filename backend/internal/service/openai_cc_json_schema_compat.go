package service

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/tidwall/gjson"
)

// 本文件是 CC 上游出口对"结构化输出（json_schema）不被上游支持"的自愈。
//
// 与 NormalizeDeepSeekResponseFormat 的分工：
//   - 后者是**预判**：按账号平台/模型名提前降级，零额外往返，但只覆盖我们已知的
//     厂商（DeepSeek），且依赖模型名命中；
//   - 本文件是**兜底**：由上游明确报错驱动，不认厂商、不认模型名、不认入站端点，
//     任何上游只要以 400 说"不支持 json_schema"，就把 json_schema 换成
//     json_object 重发一次。
//
// 两者并存的原因很实在：预判省钱省时间，兜底保证"以后不会再出现这类 400"——
// 包括我们还没听说过的新上游，以及将来新加的转发路径。

// upstreamJSONSchemaRejectionVerbs 是 400 错误里表示"这个能力我这里没有"的措辞。
//
// 刻意**不含** invalid / unknown 这两个词单独使用：它们通常出现在"schema 本身写错了"
// 或"某个字段不该出现"的场景，降级重试只会把真正的入参错误换个样子再报一次。
// 这里只认"能力缺失"。（"unknown parameter" 作为完整短语例外，见下方枚举清单的处理。）
var upstreamJSONSchemaRejectionVerbs = []string{
	"does not support",
	"doesn't support",
	"not support",
	"unsupported",
	"unavailable",
	"is not available",
	"not available",
	// 完整短语，区别于上面刻意排除的裸 'invalid'/'unknown'：这是"我这台机器没有这个
	// 入参"的标准说法，多见于白名单式中转网关（LiteLLM / one-api 一类）。
	// 它需要额外核对"被拒的那个参数是不是 structured-output 相关"，见
	// upstreamJSONSchemaUnknownParamTargetsOutput。
	"unknown parameter",
	"unrecognized parameter",
	"unknown field",
}

// upstreamJSONSchemaEnumVerbs 是**枚举式**拒绝措辞：这类报文会在冒号后面紧跟着
// 列出"我支持哪些取值"。
//
// 为什么单独拎出来：它们必须额外核对清单的维度。举例，下面这句是客户端该改的
// schema 内容错误，句子里同样有 "must be one of"：
//
//	response_format.json_schema: 'type' must be one of 'object', 'array'
//
// 而下面这句是真正的"不支持 json_schema"能力缺失：
//
//	response_format must be one of 'text', 'json_object'
//
// 两者的区分点非常干净：**清单里列出的是不是 response_format 这一维度的取值**。
// 见 upstreamJSONSchemaEnumListsFormatTypes。
var upstreamJSONSchemaEnumVerbs = []string{
	"must be one of",
	"supported values",
	"allowed values",
	"permitted values",
	"expected one of",
	"is one of",
}

// upstreamJSONSchemaFormatTypeNames 是 response_format.type 这一维度已知取值。
// 枚举清单里出现其中任意一个，就说明上游是在说"json_schema 这个**格式**我不支持"，
// 而不是在说客户端 schema 内容写错了。
var upstreamJSONSchemaFormatTypeNames = []string{"text", "json_object", "json_schema", "json_code"}

// upstreamJSONSchemaValueSeparator 切分枚举清单。
var upstreamJSONSchemaValueSeparator = regexp.MustCompile(`[^a-z0-9_+-]+`)

// upstreamJSONSchemaUnknownParamVerbs 是"我不认识这个入参"的说法。
var upstreamJSONSchemaUnknownParamVerbs = []string{
	"unknown parameter",
	"unrecognized parameter",
	"unknown field",
	"unrecognized request argument",
	"unknown request argument",
}

// upstreamJSONSchemaQuotedTokenPattern 取出句子里第一个被引号包裹的标记。
var upstreamJSONSchemaQuotedTokenPattern = regexp.MustCompile("[`'\"]([a-z0-9_.+-]{1,64})[`'\"]")

// upstreamJSONSchemaUnknownParamTargetsOutput 判断"unknown parameter"这类拒绝针对的
// 是不是结构化输出的参数本身。
//
// 为什么必须核：这类措辞同样出现在 **schema 内容错误**里：
//
//	json_schema: unknown parameter 'maxItems' in schema
//
// 那一行说的只是一个 JSON Schema 关键字，与"不支持 json_schema 这个格式"是两回事；
// 判成能力缺失会把严格结构悄悄降级掉。
//
// 判据：这类句子几乎一定把被拒的参数名引起来。若取到了引号里的名字，它必须指向
// response_format / json_schema / text.format 这一类目标（见 upstreamJSONSchemaTargetTokens）；
// 取不到名字（没有引号）时无法判别，按整句的目标词判定放行。
func upstreamJSONSchemaUnknownParamTargetsOutput(message string) bool {
	bestIndex, bestLen := -1, 0
	for _, verb := range upstreamJSONSchemaUnknownParamVerbs {
		if idx := strings.Index(message, verb); idx >= 0 && (bestIndex < 0 || idx < bestIndex) {
			bestIndex, bestLen = idx, len(verb)
		}
	}
	if bestIndex < 0 {
		return false
	}
	match := upstreamJSONSchemaQuotedTokenPattern.FindStringSubmatch(message[bestIndex+bestLen:])
	if match == nil {
		// 没给参数名（如 "Unrecognized request argument supplied: response_format"）：
		// 无从判别，交给调用方已通过的目标词判定。
		return true
	}
	quoted := match[1]
	for _, token := range upstreamJSONSchemaTargetTokens {
		if strings.Contains(quoted, token) {
			return true
		}
	}
	return false
}

// upstreamJSONSchemaEnumListsFormatTypes 判断枚举清单是不是 response_format 维度。
//
// 只看最靠前的那个枚举措辞之后的内容：清单一定紧跟在措辞后面。
func upstreamJSONSchemaEnumListsFormatTypes(message string) bool {
	bestIndex, bestLen := -1, 0
	for _, verb := range upstreamJSONSchemaEnumVerbs {
		if idx := strings.Index(message, verb); idx >= 0 && (bestIndex < 0 || idx < bestIndex) {
			bestIndex, bestLen = idx, len(verb)
		}
	}
	if bestIndex < 0 {
		return false
	}
	segment := message[bestIndex+bestLen:]
	for _, part := range upstreamJSONSchemaValueSeparator.Split(segment, -1) {
		value := strings.Trim(part, "._-")
		if value == "" {
			continue
		}
		for _, name := range upstreamJSONSchemaFormatTypeNames {
			if value == name {
				return true
			}
		}
	}
	return false
}

// upstreamJSONSchemaTargetTokens 是 400 错误里"被拒绝的那个东西"的说法。
//
// 必须放宽的原因很实在：DeepSeek 官方的拒绝信息里**根本不出现 json_schema 这四个
// 字**（"This response_format type is unavailable now"），只说 response_format；
// 另有上游写 "response format"（空格）或 "structured output"。
// 放宽不会带来误伤——真正决定要不要重试的是降级函数是否返回 changed：**只有本次
// 请求确实发了 json_schema 才会重发**，所以这里宽一点只是"多判一次"，不会把无关
// 请求改写成另一种语义。
var upstreamJSONSchemaTargetTokens = []string{
	"json_schema",
	"json-schema",
	"json schema",
	"response_format",
	"response format",
	// Responses 侧的字段名。上游可能只报"不支持的参数 text.format"，整句里既没有
	// json_schema 也没有 response_format，漏掉它就会整类漏判。
	"text.format",
	"structured output",
}

// upstreamJSONSchemaContentErrorTokens 表示"你传的 schema 内容有问题"，而不是
// "我没这个功能"。命中时不重试：那属于客户端该修的错误，降级成 json_object 只会
// 让它换个样子再报一次，并且悄悄丢掉客户端本来要的严格结构保证。
//
// 为什么不能只列 "invalid schema" 这一族：只有 OpenAI 官方会规规矩矩带上
// "Invalid schema for response_format '...'" 前缀，中转站/自建网关经常把前缀
// 丢掉或重新包装成 "json_schema: 'anyOf' is not supported" 这类句子。此时句子
// 里同时出现"被拒的字段"和"not supported"，会与"能力缺失"长得一模一样，被判成
// 能力缺失后重发——重发通常会**成功**，于是客户端拿到 200 和不受约束的输出，
// 而不是那条本该提示它改 schema 的 400。
//
// 所以这里补的是**schema 内部构造**的标记。它们不可能出现在能力缺失报文里：
// 说"我不支持 json_schema 这个格式"的上游，没有理由去点某一个 JSON Schema 关键字
// 或某个 schema 内部路径。
//
// 两侧失败模式**不对称**，这是取舍依据：
//   - 排多了（把真的能力缺失也判成内容错误）→ 客户端拿到 400，等于回到本次改动
//     之前的状态，是"新功能失效"，不产生错误行为；
//   - 排少了（把内容错误判成能力缺失）→ 静默降级并返回 200 给客户端一个不受约束的
//     结果，是**悄悄改变语义**。
//
// 因此这里宁可排得宽一点。
var upstreamJSONSchemaContentErrorTokens = []string{
	// OpenAI 前缀形式
	"invalid schema",
	"invalid json_schema",
	"schema is invalid",
	"invalid 'schema'",
	"invalid \"schema\"",
	// OpenAI schema 报错里的位置标记（"In context=('properties', 'x')"）
	"in context=",
	// 明确说"关键字"：能力缺失说的是 parameter/field/format，不会说 keyword
	"keyword",
	// JSON Schema 关键字名：只可能出现在"你的 schema 写法有问题"的语境里
	"additionalproperties",
	"$ref",
	"'anyof'",
	"'oneof'",
	"'allof'",
	"patternproperties",
	// 上面几个 JSON Schema 关键字的**带空格写法**（并行/camelCase 之外的倾向）。
	// 少了它们会漏掉真实场景："response_format.json_schema: additional properties
	// are not supported" 同时命中目标词与"不支持"动词，会被当成能力缺失降级重发，
	// 而重发必然成功（schema 被整个丢掉了）→ 客户端静默失去严格结构保证。
	"additional properties",
	"pattern properties",
}

// isUpstreamJSONSchemaUnsupportedMessage 判断上游错误消息是否表示"不支持
// json_schema 结构化输出"。CC 出口与原生 Responses 出口共用同一判定，避免两处
// 判定各自演化、再次出现"一条路径漏接"的问题。
func isUpstreamJSONSchemaUnsupportedMessage(message string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	if message == "" {
		return false
	}
	// "schema 内容有问题"优先判否：这类消息同样会出现 not supported 之类的措辞
	// （例如 "'anyOf' is not supported in strict mode"），但它属于客户端该修的错误，
	// 降级成 json_object 只会让它换个样子再报一次，还会悄悄丢掉严格结构保证。
	for _, token := range upstreamJSONSchemaContentErrorTokens {
		if strings.Contains(message, token) {
			return false
		}
	}
	mentionsTarget := false
	for _, token := range upstreamJSONSchemaTargetTokens {
		if strings.Contains(message, token) {
			mentionsTarget = true
			break
		}
	}
	if !mentionsTarget {
		return false
	}
	// 枚举式措辞优先走维度核对：清单里必须出现 response_format 这一维度的取值，
	// 否则那更可能是"客户端 schema 内容写错"的枚举错误（见 upstreamJSONSchemaEnumVerbs）。
	for _, verb := range upstreamJSONSchemaEnumVerbs {
		if strings.Contains(message, verb) {
			return upstreamJSONSchemaEnumListsFormatTypes(message)
		}
	}
	// "我不认识这个入参"必须额外核对被拒的确实是 structured-output 相关参数，
	// 否则 "json_schema: unknown parameter 'maxItems'" 这类 schema 内容错误会被
	// 误判成能力缺失而静默降级。
	for _, verb := range upstreamJSONSchemaUnknownParamVerbs {
		if strings.Contains(message, verb) {
			return upstreamJSONSchemaUnknownParamTargetsOutput(message)
		}
	}
	for _, verb := range upstreamJSONSchemaRejectionVerbs {
		if strings.Contains(message, verb) {
			return true
		}
	}
	return false
}

// isCCJSONSchemaUnsupportedError 判断 CC 上游 400 是否因为不支持 json_schema。
func isCCJSONSchemaUnsupportedError(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	message := strings.TrimSpace(gjson.GetBytes(body, "error.message").String())
	if message == "" {
		// 少数上游把原因放在顶层或 error 不是对象，退化到整包匹配。
		message = string(body)
	}
	return isUpstreamJSONSchemaUnsupportedMessage(message)
}

// downgradeCCJSONSchemaResponseFormat 把 CC 请求体的结构化输出降级为 json_object。
//
// json_schema 必须**整体丢弃**：只支持 json_object 的上游（如 DeepSeek）见到
// 额外的 json_schema 键同样报错。schema 的文本会作为提示保留下来，尽量让输出
// 仍贴近客户端期望的字段结构。
func downgradeCCJSONSchemaResponseFormat(body []byte) ([]byte, bool) {
	if len(body) == 0 {
		return body, false
	}
	// 用 EqualFold 而不是 !=：与 Responses 侧的 downgradeOpenAIResponsesJSONSchemaFormat
	// 同口径。上游对 type 的大小写容忍度不一致（有的一律小写归一、有的原样比对），
	// 客户端写 "JSON_SCHEMA" 时若这里用大小写敏感比较，同一条 400 在 CC 出口得不到
	// 自愈、在 Responses 出口却能得到，又变成"只修一半"。
	if !strings.EqualFold(strings.TrimSpace(gjson.GetBytes(body, "response_format.type").String()), "json_schema") {
		return body, false
	}
	var reqBody map[string]any
	if err := decodeOpenAIJSONUseNumber(body, &reqBody); err != nil {
		// 解析失败说明请求体本身不合法，交给上游按原样报错，网关不介入。
		return body, false
	}
	schemaHint := deepSeekJSONSchemaHint(reqBody)
	reqBody["response_format"] = map[string]any{"type": "json_object"}
	// DeepSeek 另有"prompt 必须含 json 关键词"的硬性要求，顺手满足；
	// 对没有这个要求的上游，多一句提示无害。
	ensureDeepSeekJSONKeyword(reqBody, schemaHint)
	normalized, err := json.Marshal(reqBody)
	if err != nil {
		return body, false
	}
	return normalized, true
}

// ccResponseFormatModelHint 取请求体里的模型名，仅用于日志定位。
func ccResponseFormatModelHint(body []byte) string {
	return strings.TrimSpace(gjson.GetBytes(body, "model").String())
}
