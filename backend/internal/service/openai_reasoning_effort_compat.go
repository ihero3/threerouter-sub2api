package service

import (
	"regexp"
	"sort"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// 本文件处理「上游不接受客户端给的 reasoning_effort 取值」这一类 400。
//
// 线上工单 b16d2f40（账号「国外：阿里（moxing）」，qwen3.8-max → deepseek-v4-flash，
// 入站 /v1/chat/completions → 上游 /v1/chat/completions）：
//
//	{"error":{"code":"invalid_value",
//	          "message":"'reasoning_effort' must be one of: 'low', 'medium', 'high', 'xhigh', 'max'",
//	          "param":null,"type":"invalid_request_error"}}
//
// 为什么网关必须兜：平台内部认可的档位集合是
// minimal/low/medium/high/xhigh/max（见 openAIReasoningEffortValues），
// 但**各家上游的集合并不一致**——本例上游就没有 minimal。客户端发出
// minimal / none 这类值时网关原样转发，上游 400，而客户端拿到的是一个它无法
// 自行修复的错误：它并不知道这台上游支持哪些档位，也无从在运行时探测。
//
// 做法是「上游说了算」：这条 400 报文里**已经列出了它接受的档位**，照它给的集合
// 就近对齐后重发一次。刻意不维护厂商档位表——表会过期，而且对没听过的新上游无效；
// 报文里的清单是上游自己声明的，永远最新。

// upstreamReasoningEffortFieldTokens 判断报文是不是在说 effort 字段本身。
// 四种写法都出现过：下划线 / 点 / 空格，以及把字段名放在 error.param 里的厂商
// 只写嵌套字段名 effort。
var upstreamReasoningEffortFieldTokens = []string{
	"reasoning_effort",
	"reasoning.effort",
	"reasoning effort",
	// 裸 effort：某些上游报嵌套字段时只写 'effort'。
	//
	// 放宽这一步是安全的，因为后面还有三重收窄：① 必须出现"清单措辞"
	// （must be one of / supported values …）；② 清单里的取值必须**全部**是
	// reasoningEffortRank 认得的档位或关闭档才保留；③ 真正决定是否改写的是
	// clampReasoningEffortToAllowedValues ——只有客户端确实发了 effort 字段才会动。
	"effort",
}

// upstreamReasoningEffortMentionsField 判断文本里是否在说 effort 字段。
func upstreamReasoningEffortMentionsField(text string) bool {
	for _, token := range upstreamReasoningEffortFieldTokens {
		if strings.Contains(text, token) {
			return true
		}
	}
	return false
}

// upstreamReasoningEffortDisableValues 是上游用来表达「不做推理」的字面写法。
//
// 必须单独保留、不能当"认不出的取值"丢掉：
//   - 它们**可判语义**（就是关闭），不像 auto/turbo 这种无法判断相对高低；
//   - 丢掉会造成**语义反转**——客户端明确要关闭推理、上游恰恰支持这一档，而网关会
//     因为"认不出"而把取值向上贴到最低的启用档 low，等于替客户端把推理打开。
//
// 平台里 'none' 是真实存在的值：Codex 模型名后缀解析（openai_compat_model.go
// 的 openAICompatSplitCodexReasoning，switch 里 "none" 与 "minimal" 同属一档、
// 都不产出 effort）已经把它当作既有的语义使用。
var upstreamReasoningEffortDisableValues = []string{"none", "off", "disabled"}

// upstreamReasoningEffortIsDisableValue 判断某个字面写法是不是"不做推理"。
func upstreamReasoningEffortIsDisableValue(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return false
	}
	for _, candidate := range upstreamReasoningEffortDisableValues {
		if value == candidate {
			return true
		}
	}
	return false
}

// upstreamReasoningEffortListMarkers 是「后面跟着允许值清单」的措辞。
//
// 必须有这层约束：只说「reasoning_effort 不受支持」的报文（没有清单）是不能
// 自行决定改成什么的，那种情况必须原样交回上游错误。
var upstreamReasoningEffortListMarkers = []string{
	"one of",
	"allowed values",
	"allowed value",
	"allowed:",
	"permitted values",
	"supported values",
}

// upstreamReasoningEffortMarkerIndex 找清单措辞的起始位置，找不到返回 -1。
//
// 刻意跳过**被 not 否定**的措辞：
// "reasoning_effort 'minimal' is not allowed: low, medium" 里 allowed: 后面跟的是
// **被禁止**的取值，当成允许集来对齐会把请求改成一个上游明确不要的值。
// 这种形态少见但有害，跳过它只是"这条报文不介入"，安全。
func upstreamReasoningEffortMarkerIndex(normalized string) int {
	markerIndex := -1
	for _, marker := range upstreamReasoningEffortListMarkers {
		idx := strings.Index(normalized, marker)
		if idx < 0 {
			continue
		}
		if strings.HasSuffix(normalized[:idx], "not ") {
			continue
		}
		if markerIndex < 0 || idx < markerIndex {
			markerIndex = idx
		}
	}
	return markerIndex
}

const (
	upstreamReasoningEffortMaxValues   = 16
	upstreamReasoningEffortMaxValueLen = 32
	// upstreamReasoningEffortMinBareWords 是裸词清单（未加引号）被采信所需的最少
	// 档位数。见 upstreamReasoningEffortAllowedValues 里的取舍说明。
	upstreamReasoningEffortMinBareWords = 2
)

// upstreamQuotedValuePattern 提取清单里的取值。只认引号包裹的短标识符，
// 避免把句子里的普通词当成档位。
var upstreamQuotedValuePattern = regexp.MustCompile("[`'\"]([a-z][a-z0-9_.+-]{0,31})[`'\"]")

// upstreamReasoningEffortAllowedValues 从上游错误消息里解析它接受的 effort 档位。
//
// 返回 false 表示「无法据此安全改写」——包括没有清单、清单里没有一个档位是我们
// 认得的（那种报文很可能是别的语义，宁可不介入）。
//
// 只保留**本平台认识的档位**（reasoningEffortRank 可判秩），这既是安全网也是
// 改写目标集合：认不出的取值（如某些上游自定义的 auto）不作为改写目标，
// 因为无法判断它与其它档位的相对高低。
func upstreamReasoningEffortAllowedValues(message string) ([]string, bool) {
	return upstreamReasoningEffortAllowedValuesWithParam(message, "")
}

// upstreamReasoningEffortAllowedValuesWithParam 同 upstreamReasoningEffortAllowedValues，
// 额外接受上游**单独**给出的字段名（error.param）。
//
// 为什么必须接这一格：相当多上游（含 OpenAI 自己）把"被拒的字段名"放在 error.param，
// 只在 error.message 里写取值本身，例如
//
//	{"error":{"message":"Invalid value 'minimal'. Supported values are: 'low', 'medium',
//	          "param":"reasoning_effort"}}
//
// 只看 message 会导致这一整类报文判成"与我无关"而失去自愈——用户仍然拿到 400。
func upstreamReasoningEffortAllowedValuesWithParam(message, param string) ([]string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(message))
	if normalized == "" {
		return nil, false
	}

	// 字段名的证据有两个来源，取并集：message 里提到，或 param 明确指向 effort。
	if !upstreamReasoningEffortMentionsField(normalized) &&
		!upstreamReasoningEffortMentionsField(strings.ToLower(strings.TrimSpace(param))) {
		return nil, false
	}

	// 只在清单措辞**之后**取值：字段名本身（'reasoning_effort'）出现在清单之前，
	// 不切分会把它也当成一个允许值。
	markerIndex := upstreamReasoningEffortMarkerIndex(normalized)
	if markerIndex < 0 {
		return nil, false
	}

	// 引号解析：优先路径。上游（OpenAI 及多数中转站）会用引号包裹枚举取值，
	// 这是精度最高的形态——引号本身就是"这是一个取值"的声明。
	matches := upstreamQuotedValuePattern.FindAllStringSubmatch(normalized[markerIndex:], upstreamReasoningEffortMaxValues+1)
	values := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		value := strings.TrimSpace(match[1])
		if value == "" || len(value) > upstreamReasoningEffortMaxValueLen {
			continue
		}
		if _, ok := reasoningEffortRank(value); !ok && !upstreamReasoningEffortIsDisableValue(value) {
			// 认不出的取值不作为改写目标（见函数注释）。
			// 例外：**"不做推理"这一档要留下**——它语义明确（关闭），只是无法参与
			// 秩比较；丢掉会让"客户端要关闭"被反向改写成最低启用档。
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
		if len(values) >= upstreamReasoningEffortMaxValues {
			break
		}
	}
	if len(values) > 0 {
		return values, true
	}
	// 兜底：清单**未加引号**的写法（"must be one of: low, medium"）。
	//
	// 这不是假想的形态——OpenAI 会加引号，但相当多中转站/自建网关直接输出裸词。
	// 只在引号解析一无所获时才尝试，并且要求**至少命中 2 个本平台认得的档位**：
	// 真正的枚举清单几乎不会只给一个取值，而"句子里恰好飘过一个 low / high 这类
	// 英文词"的误抓概率要高得多。
	//
	// 取舍与别处一致，但这里更严：抓错会把客户端明确要求的档位**悄悄改低或改高**
	// （例如把 max 改成 low），是静默改变语义，比"没救活"严重得多。所以宁可漏。
	bare := upstreamReasoningEffortBareWordValues(normalized[markerIndex:])
	if len(bare) == 0 {
		return nil, false
	}
	return bare, true
}

// upstreamReasoningEffortBareWordSeparator 切分未加引号的清单。
var upstreamReasoningEffortBareWordSeparator = regexp.MustCompile(`[^a-z0-9_+-]+`)

// upstreamReasoningEffortBareWordValues 从裸词清单里提取档位，找不到成规模的
// 清单时返回 nil（调用方据此判定"不介入"）。见上游调用处的取舍说明。
func upstreamReasoningEffortBareWordValues(segment string) []string {
	parts := upstreamReasoningEffortBareWordSeparator.Split(strings.ToLower(segment), -1)
	values := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		value := strings.Trim(part, "._-")
		if value == "" || len(value) > upstreamReasoningEffortMaxValueLen {
			continue
		}
		if _, ok := reasoningEffortRank(value); !ok && !upstreamReasoningEffortIsDisableValue(value) {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
		if len(values) >= upstreamReasoningEffortMaxValues {
			break
		}
	}
	// 少于 2 个不采信：单个取值既可能是枚举，也更可能是句子里恰好出现的英文单词。
	if len(values) < upstreamReasoningEffortMinBareWords {
		return nil
	}
	return values
}

// upstreamReasoningEffortAllowedValuesFromErrorBody 从上游错误体里取消息再解析。
// 与 isCCJSONSchemaUnsupportedError 同口径：优先取 error.message，
// 取不到时退化到整包匹配。
func upstreamReasoningEffortAllowedValuesFromErrorBody(errorBody []byte) ([]string, bool) {
	if len(errorBody) == 0 {
		return nil, false
	}
	message := strings.TrimSpace(gjson.GetBytes(errorBody, "error.message").String())
	param := strings.TrimSpace(gjson.GetBytes(errorBody, "error.param").String())
	if message == "" {
		// 少数上游把原因放在顶层或 error 不是对象，退化到整包匹配。
		message = string(errorBody)
	}
	return upstreamReasoningEffortAllowedValuesWithParam(message, param)
}

// clampReasoningEffortToAllowedValues 把请求体里的 effort 取值就近对齐到上游允许集。
//
// 判定顺序（对每个存在的 effort 字段各判一次）：
//  1. **写法归一**：语义已是允许集里某个值、只是大小写/空白不同 → 改成上游的字面写法
//     （如 " medium " / "MEDIUM" → "medium"，否则大小写敏感的上游会一直拒、
//     而本判据又会误判成"已合法"从而永久放过，自愈失效）；
//  2. **客户端明确要关闭**（none/off/disabled）且上游确实列出了关闭档 → 用那一档
//     （见 upstreamReasoningEffortDisableValues：丢掉会造成"要关闭却被打开"的语义反转）；
//  3. 认得的取值低于允许集下限 → 贴到最低档（保序）；高于上限 → 贴到最高档；
//  4. 落在区间内 → 贴到秩最近的一档，并列时取下限（更省推理）；
//  5. **认不出的取值**（拼写错误）→ 按「比最低档还低」处理，向上贴到最低档。
//     理由：任何选择都会开启推理，贴最低档最接近客户端"要更少推理"的意图
//     （删字段会退回上游默认，通常反而更高）。
//
// 字段名两种都覆盖：CC 用扁平的 reasoning_effort，Responses 用嵌套的 reasoning.effort。
func clampReasoningEffortToAllowedValues(body []byte, allowed []string) ([]byte, bool) {
	if len(body) == 0 || len(allowed) == 0 {
		return body, false
	}

	type effortCandidate struct {
		value string
		rank  int
	}
	// 可判秩的启用档是"就近对齐"的候选；不参与秩比较的关闭档单独放。
	candidates := make([]effortCandidate, 0, len(allowed))
	// canonicalOf 记「小写写法 → 上游在报文里给出的字面写法」。
	// 允许集里可能同时有可判秩档与关闭档，二者都要能被"写法归一"命中。
	canonicalOf := make(map[string]string, len(allowed))
	disableLiteral := ""
	for _, value := range allowed {
		literal := strings.TrimSpace(value)
		if literal == "" {
			continue
		}
		if _, exists := canonicalOf[strings.ToLower(literal)]; !exists {
			canonicalOf[strings.ToLower(literal)] = literal
		}
		if rank, ok := reasoningEffortRank(literal); ok {
			candidates = append(candidates, effortCandidate{value: literal, rank: rank})
			continue
		}
		if disableLiteral == "" && upstreamReasoningEffortIsDisableValue(literal) {
			disableLiteral = literal
		}
	}
	// 既没有可判秩的启用档、也没有关闭档 → 没有任何可对齐的目标。
	if len(candidates) == 0 && disableLiteral == "" {
		return body, false
	}
	// 升序排列：让"秩最近且并列"时稳定地取下限，与上游清单的书写顺序无关。
	// 必须用 SliceStable：同义档位（如 extrahigh 与 xhigh）秩相同，
	// sort.Slice 不稳定会让并列时的选择随输入顺序抖动。
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].rank < candidates[j].rank })

	out := body
	changed := false
	for _, path := range []string{"reasoning.effort", "reasoning_effort"} {
		field := gjson.GetBytes(out, path)
		if !field.Exists() || field.Type != gjson.String {
			continue
		}
		raw := field.String()
		original := strings.TrimSpace(raw)
		if original == "" {
			continue
		}

		// (1) 写法归一，同时充当"已经合法就不动"的闸门。
		//
		// 这道闸门对**同义档位**也是必需的：例如上游同时列出了 extrahigh 与 xhigh
		// （两者秩相同，都是 xhigh 档），客户端给的正是 xhigh。此时"就近对齐"会在
		// 两个等距候选中挑一个，可能挑中 extrahigh 从而产生一次毫无意义的改写
		// （甚至会因为上游只认字面量而再次 400）。先做字面量命中判断即可短路。
		if canonical, ok := canonicalOf[strings.ToLower(original)]; ok {
			if raw == canonical {
				continue
			}
			updated, err := sjson.SetBytes(out, path, canonical)
			if err != nil {
				continue
			}
			out, changed = updated, true
			continue
		}

		target := ""
		switch {
		// (2) 客户端明确要关闭且上游有这一档 → 保留"关闭"语义。
		case upstreamReasoningEffortIsDisableValue(original) && disableLiteral != "":
			target = disableLiteral
		// 允许集里只有关闭档、没有任何可判秩的启用档：除上面的情形外无从对齐，动不得。
		case len(candidates) == 0:
			continue
		default:
			rank, rankable := reasoningEffortRank(original)
			if !rankable {
				rank = candidates[0].rank - 1
			}
			switch {
			case rank < candidates[0].rank:
				target = candidates[0].value
			case rank > candidates[len(candidates)-1].rank:
				target = candidates[len(candidates)-1].value
			default:
				bestDelta := -1
				for _, candidate := range candidates {
					delta := candidate.rank - rank
					if delta < 0 {
						delta = -delta
					}
					if bestDelta < 0 || delta < bestDelta {
						target, bestDelta = candidate.value, delta
					}
				}
			}
		}
		if target == "" || target == original {
			continue
		}

		updated, err := sjson.SetBytes(out, path, target)
		if err != nil {
			continue
		}
		out = updated
		changed = true
	}
	return out, changed
}
