package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const maxOpenAIResponsesRejectedFieldRetries = 6

// openAIResponsesReasoningEffortValueRejectionReason 是"effort 取值不被接受"这一分支
// 的重试原因。与 json_schema 一样属于"本平台主动支持"的自愈，因此也在 chat 链路的
// 白名单里（见 openAIResponsesChatPathRetryReasons）。
const openAIResponsesReasoningEffortValueRejectionReason = "reasoning_effort value rejection"

// openAIResponsesJSONSchemaFormatRejectionReason 是"结构化输出被拒"这一分支的重试原因。
const openAIResponsesJSONSchemaFormatRejectionReason = "json_schema text format rejection"

// openAIResponsesChatPathRetryReasons 是允许在**入站 /v1/chat/completions** 链路上
// 触发重发的原因白名单。
//
// 为什么需要白名单：retryOpenAIResponsesRejectedFieldOnce 挂在
// forwardAsChatCompletions 上，而那个函数同时承担 chat 的既有行为。同一个
// normalizeOpenAIResponsesRejectedFieldRetryBody 还服务另外四条 Responses 出口
// （原生 Forward / passthrough / WS ingress / WS HTTP bridge），它们的重试原因集合
// （max_output_tokens / truncation / namespace / status / 工具参数…）是既有行为，
// **不允许**因为本次改动被顺带带上 chat 链路。
//
// 新增原因时必须在这里显式登记，并同时补上对应的"不该改的没改"测试。
var openAIResponsesChatPathRetryReasons = map[string]struct{}{
	openAIResponsesJSONSchemaFormatRejectionReason:     {},
	openAIResponsesReasoningEffortValueRejectionReason: {},
}

// isOpenAIResponsesChatPathRetryReason 判断重试原因是否允许在 chat 链路上触发重发。
func isOpenAIResponsesChatPathRetryReason(reason string) bool {
	_, ok := openAIResponsesChatPathRetryReasons[reason]
	return ok
}

var (
	openAIResponsesRejectedNamespaceParamPattern  = regexp.MustCompile(`(?i)^input\[(\d+)\]\.namespace$`)
	openAIResponsesRejectedStatusParamPattern     = regexp.MustCompile(`(?i)^input\[(\d+)\]\.status$`)
	openAIResponsesRejectedContentParamPattern    = regexp.MustCompile(`(?i)^input\[(\d+)\]\.content$`)
	openAIResponsesRejectedCacheParamPattern      = regexp.MustCompile(`(?i)^input\[(\d+)\]\.prompt_cache_breakpoint$`)
	openAIResponsesRejectedMessageParamPattern    = regexp.MustCompile(`(?i)(?:unknown|unsupported)[ _-]+parameter\s*(?::|=|is)?\s*["']?(max_output_tokens|truncation|input\[\d+\]\.(?:namespace|status))(?:["']|\b)`)
	openAIResponsesInvalidTypeMessageParamPattern = regexp.MustCompile(`(?i)invalid[ _-]+type\s+for\s+["']?(input\[\d+\]\.content)(?:["']|\b)[^\n]*\b(?:got|received)\s+null\b`)
	openAIResponsesMaxZeroContentMessagePattern   = regexp.MustCompile(`(?i)invalid\s+["']?(input\[\d+\]\.content)["']?\s*:\s*array too long\.[^\n]*maximum length 0\b`)
	openAIResponsesCacheModelRejectionPattern     = regexp.MustCompile(`(?i)["']?(prompt_cache_breakpoint|input\[\d+\]\.prompt_cache_breakpoint)["']?\s+is\s+not\s+supported\s+on\s+this\s+model\b`)
	openAIResponsesToolParametersParamPattern     = regexp.MustCompile(`(?i)^(?:tools|input)\[\d+\](?:\.tools\[\d+\])*(?:\.function)?\.parameters$`)
	openAIResponsesMissingSchemaTypePattern       = regexp.MustCompile(`(?i)\bgot\s+["']?type\s*:\s*["']?none["']?`)
)

type openAIResponsesRejectedFieldRetryState struct {
	mu             sync.Mutex
	budget         *openAIResponsesRejectedFieldRetryBudget
	seenBodyHashes map[[sha256.Size]byte]struct{}
}

type openAIResponsesRejectedFieldRetryBudget struct {
	mu       sync.Mutex
	attempts int
}

const openAIResponsesRejectedFieldRetryBudgetContextKey = "openai_responses_rejected_field_retry_budget"

// openAIResponsesRejectedFieldRetryStateForRequest returns a fresh loop guard
// for one account attempt backed by the inbound request's shared retry budget.
// A later account may apply the same compatibility transform, while all account
// attempts together remain bounded.
func openAIResponsesRejectedFieldRetryStateForRequest(c *gin.Context, initialBody []byte) *openAIResponsesRejectedFieldRetryState {
	var budget *openAIResponsesRejectedFieldRetryBudget
	if c != nil {
		if existing, ok := c.Get(openAIResponsesRejectedFieldRetryBudgetContextKey); ok {
			budget, _ = existing.(*openAIResponsesRejectedFieldRetryBudget)
		}
	}
	if budget == nil {
		budget = &openAIResponsesRejectedFieldRetryBudget{}
		if c != nil {
			c.Set(openAIResponsesRejectedFieldRetryBudgetContextKey, budget)
		}
	}
	return newOpenAIResponsesRejectedFieldRetryStateWithBudget(initialBody, budget)
}

func newOpenAIResponsesRejectedFieldRetryState(initialBody []byte) *openAIResponsesRejectedFieldRetryState {
	return newOpenAIResponsesRejectedFieldRetryStateWithBudget(initialBody, &openAIResponsesRejectedFieldRetryBudget{})
}

func newOpenAIResponsesRejectedFieldRetryStateWithBudget(initialBody []byte, budget *openAIResponsesRejectedFieldRetryBudget) *openAIResponsesRejectedFieldRetryState {
	state := &openAIResponsesRejectedFieldRetryState{
		budget:         budget,
		seenBodyHashes: make(map[[sha256.Size]byte]struct{}, maxOpenAIResponsesRejectedFieldRetries+1),
	}
	state.remember(initialBody)
	return state
}

func (s *openAIResponsesRejectedFieldRetryState) Allow(nextBody []byte) bool {
	if s == nil || s.budget == nil || len(nextBody) == 0 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	bodyHash := sha256.Sum256(nextBody)
	if _, seen := s.seenBodyHashes[bodyHash]; seen {
		return false
	}
	s.budget.mu.Lock()
	defer s.budget.mu.Unlock()
	if s.budget.attempts >= maxOpenAIResponsesRejectedFieldRetries {
		return false
	}
	s.seenBodyHashes[bodyHash] = struct{}{}
	s.budget.attempts++
	return true
}

func (s *openAIResponsesRejectedFieldRetryState) remember(body []byte) {
	if s == nil || len(body) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rememberLocked(body)
}

func (s *openAIResponsesRejectedFieldRetryState) rememberLocked(body []byte) {
	if s.seenBodyHashes == nil {
		s.seenBodyHashes = make(map[[sha256.Size]byte]struct{}, maxOpenAIResponsesRejectedFieldRetries+1)
	}
	s.seenBodyHashes[sha256.Sum256(body)] = struct{}{}
}

func normalizeOpenAIResponsesRejectedFieldRetryBody(statusCode int, body, responseBody []byte) ([]byte, string, bool, error) {
	if statusCode != http.StatusBadRequest || len(body) == 0 || len(responseBody) == 0 {
		return nil, "", false, nil
	}

	code := strings.ToLower(strings.TrimSpace(extractUpstreamErrorCode(responseBody)))
	message := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(responseBody)))
	param := strings.ToLower(strings.TrimSpace(gjson.GetBytes(responseBody, "error.param").String()))
	if code == "invalid_function_parameters" &&
		openAIResponsesToolParametersParamPattern.MatchString(param) &&
		openAIResponsesMissingSchemaTypePattern.MatchString(message) {
		retryBody, changed, err := sanitizeOpenAIResponsesToolParameterTypes(body)
		if err != nil {
			return nil, "", false, fmt.Errorf("repair rejected tool parameter root type: %w", err)
		}
		if changed {
			return retryBody, "tool parameter root type rejection", true, nil
		}
	}
	// 结构化输出（text.format=json_schema）被上游拒绝：降级为 json_object 重试。
	//
	// 与 CC 出口的 openai_cc_json_schema_compat.go 是同一类问题的两端——入站
	// 到底是 CC 还是 Responses 不重要，只要上游是"有 json_schema 概念但不实现"
	// 的厂商，就会以 400 拒绝。上游出口不同，判定必须同源，否则又会只修一半。
	//
	// 覆盖范围：本函数被 Forward（原生 Responses）、passthrough、WS ingress、
	// WS HTTP bridge 以及入站 CC 自适应转 Responses 的路径共用，因此这一处
	// 分支同时补上全部五条出口，且自动继承上面的重试预算与 body 去重。
	if isUpstreamJSONSchemaUnsupportedMessage(message) {
		retryBody, changed, err := downgradeOpenAIResponsesJSONSchemaFormat(body)
		if err != nil {
			return nil, "", false, fmt.Errorf("downgrade rejected json_schema text format: %w", err)
		}
		if changed {
			return retryBody, openAIResponsesJSONSchemaFormatRejectionReason, true, nil
		}
	}
	// reasoning_effort 取值不被上游接受：按上游自己给出的可接受档位就近对齐后重试。
	//
	// 与 CC 出口的 openai_reasoning_effort_compat.go 同一类问题的两端（线上工单
	// b16d2f40 是 CC 侧）：上游在各家实现的档位集合并不一致，而平台内部认可
	// minimal/low/medium/high/xhigh/max。上游拒绝时会**在报文里列出它接受的档位**，
	// 照它给的清单对齐即可，不必维护厂商表。
	//
	// 位置放在这里（其余"被拒字段"分支之前）：effort 取值错误与那些字段级拒绝
	// 互斥，且本分支只在报文明确给出档位清单 + 请求体里真有可改写字段时才返回 changed，
	// 不命中会自然下落到下面的既有分支。
	// 字段名可能在 error.param 里而不在 message 里（与 CC 出口同一处 gap）：
	// 两边必须同源，否则又变成只修一半。
	if allowed, ok := upstreamReasoningEffortAllowedValuesWithParam(message, param); ok {
		if adjusted, changed := clampReasoningEffortToAllowedValues(body, allowed); changed {
			return adjusted, openAIResponsesReasoningEffortValueRejectionReason, true, nil
		}
	}
	cacheMessageParam := openAIResponsesCacheModelRejectionParamFromMessage(message)
	cacheParam := param
	if cacheParam == "" {
		cacheParam = cacheMessageParam
	}
	cacheParamMatchesMessage := cacheMessageParam == "" || cacheParam == cacheMessageParam
	cacheModelRejection := code == "invalid_parameter" || cacheMessageParam != ""
	if cacheParam != "" && cacheParamMatchesMessage && cacheModelRejection {
		if cacheParam == "prompt_cache_breakpoint" && gjson.GetBytes(body, cacheParam).Exists() {
			retryBody, err := sjson.DeleteBytes(body, cacheParam)
			if err != nil {
				return nil, "", false, fmt.Errorf("delete rejected prompt_cache_breakpoint: %w", err)
			}
			return retryBody, "prompt_cache_breakpoint parameter rejection", true, nil
		}
		if index, ok := openAIResponsesRejectedCacheIndex(cacheParam); ok {
			return removeOpenAIResponsesRejectedCacheAtIndex(body, index)
		}
	}
	if isExplicitOpenAIResponsesFieldRejection(code, message) {
		messageParam := openAIResponsesRejectedParamFromMessage(message)
		if param != "" && messageParam != "" && param != messageParam {
			return nil, "", false, nil
		}
		if param == "" {
			param = messageParam
		}
		if index, ok := openAIResponsesRejectedNamespaceIndex(param); ok {
			return removeOpenAIResponsesRejectedNamespaceAtIndex(body, index)
		}
		if index, ok := openAIResponsesRejectedStatusIndex(param); ok {
			return removeOpenAIResponsesRejectedStatusAtIndex(body, index)
		}
		if param == "max_output_tokens" && gjson.GetBytes(body, "max_output_tokens").Exists() {
			retryBody, err := sjson.DeleteBytes(body, "max_output_tokens")
			if err != nil {
				return nil, "", false, fmt.Errorf("delete rejected max_output_tokens: %w", err)
			}
			return retryBody, "max_output_tokens parameter rejection", true, nil
		}
		if param == "truncation" && gjson.GetBytes(body, "truncation").Exists() {
			retryBody, err := sjson.DeleteBytes(body, "truncation")
			if err != nil {
				return nil, "", false, fmt.Errorf("delete rejected truncation: %w", err)
			}
			return retryBody, "truncation parameter rejection", true, nil
		}
	}

	messageContentParam := openAIResponsesInvalidTypeParamFromMessage(message)
	contentParam := param
	if contentParam == "" {
		contentParam = messageContentParam
	}
	if index, ok := openAIResponsesRejectedContentIndex(contentParam); ok &&
		contentParam == messageContentParam && isExplicitOpenAIResponsesNullContentRejection(code, message) {
		return normalizeOpenAIResponsesRejectedNullContentAtIndex(body, index)
	}
	maxZeroContentParam := openAIResponsesMaxZeroContentParamFromMessage(message)
	if index, ok := openAIResponsesRejectedContentIndex(param); ok &&
		param == maxZeroContentParam && code == "array_above_max_length" {
		return removeOpenAIResponsesRejectedReasoningContentAtIndex(body, index)
	}
	return nil, "", false, nil
}

// retryOpenAIResponsesRejectedFieldOnce 给"一次性"的上游 Responses 出口补一次被拒
// 字段重发。
//
// 适用范围：没有重试循环的出口，例如入站 /v1/chat/completions 自适应转 Responses 的
// forwardAsChatCompletions。它只会发一次请求，上游 400 时既没有 rejectedFieldRetry
// 预算也没有重发机会，是同类出口里唯一漏网的。Forward / passthrough / WS ingress /
// WS HTTP bridge 都有自己的循环，不需要走这里。
//
// **严格白名单**：只放行本平台主动支持的两类自愈原因
// （见 openAIResponsesChatPathRetryReasons）。本函数挂在 /v1/chat/completions 的
// 链路上，其余"被拒字段"原因（max_output_tokens / truncation / namespace / status /
// 工具参数类型等）**一律不重发**——那些是既有的 chat 行为，不允许被本次改动顺带改变。
//
// 契约：命中且重发成功（<400）时返回新响应；其余任何情况（未命中、原因不在白名单、
// 构建失败、传输失败、重发仍失败）都原样返回入参 resp，调用方既有的错误处理链完全
// 不受影响。
func (s *OpenAIGatewayService) retryOpenAIResponsesRejectedFieldOnce(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	resp *http.Response,
	body []byte,
	token string,
	promptCacheKey string,
	proxyURL string,
	isStream bool,
	compatPromptCacheTenantIsolated bool,
) *http.Response {
	if resp == nil || resp.StatusCode != http.StatusBadRequest || len(body) == 0 {
		return resp
	}
	// 400 但没有响应体：既没有判定依据，也避免下面 readOpenAIUpstreamError 对
	// nil Body 调 Close 崩溃（自定义 HTTPUpstream 实现可能返回这种响应）。
	if resp.Body == nil {
		return resp
	}
	// readOpenAIUpstreamError 会把 body 回卷成可重读的副本，调用方随后照常读取，
	// 这里先判定不会破坏它的错误处理。
	respBody, _ := s.readOpenAIUpstreamError(resp)
	retryBody, reason, changed, retryErr := normalizeOpenAIResponsesRejectedFieldRetryBody(resp.StatusCode, body, respBody)
	if retryErr != nil || !changed {
		return resp
	}
	// 白名单：本函数挂在 /v1/chat/completions 链路上，只允许登记过的自愈原因
	// 触发重发，避免顺带改变 chat 的既有行为。
	if !isOpenAIResponsesChatPathRetryReason(reason) {
		return resp
	}
	// 与其余四条出口共用同一套预算与 body 去重：本函数会被每个候选账号各调一次
	// （账号 failover），不接预算就是"每账号各重试一次"，虽然仍以账号数为上界，
	// 但比其余出口的"单次入站请求合计 6 次"宽松，没必要不一致。
	retryState := openAIResponsesRejectedFieldRetryStateForRequest(c, body)
	if !retryState.Allow(retryBody) {
		return resp
	}

	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	retryReq, err := s.buildUpstreamRequest(upstreamCtx, c, account, retryBody, token, isStream, promptCacheKey, false)
	releaseUpstreamCtx()
	if err != nil {
		return resp
	}
	if strings.TrimSpace(promptCacheKey) != "" {
		apiKeyID := getAPIKeyIDFromContext(c)
		sessionKey := promptCacheKey
		if !compatPromptCacheTenantIsolated {
			sessionKey = isolateOpenAIUpstreamSessionID(apiKeyID, codexAccountIdentitySource(c, account), promptCacheKey)
		}
		retryReq.Header.Set("session_id", generateSessionUUID(sessionKey))
	}

	retryResp, err := s.doOpenAIUpstream(retryReq, proxyURL, account)
	if err != nil {
		return resp
	}
	if retryResp == nil {
		return resp
	}
	if retryResp.StatusCode >= 400 {
		// 重发仍然失败：丢弃它，把原响应交给既有错误处理，语义与"没重试过"一致。
		if retryResp.Body != nil {
			_ = retryResp.Body.Close()
		}
		return resp
	}
	logger.LegacyPrintf("service.openai_gateway",
		"[OpenAI] Retried responses request after %s (account: %s)", reason, account.Name)
	return retryResp
}

// downgradeOpenAIResponsesJSONSchemaFormat 把 Responses 的结构化输出降级为
// json_object，与 CC 侧的 downgradeCCJSONSchemaResponseFormat 对称。
//
// json_schema 必须**整体丢弃**：只支持 json_object 的上游见到 text.format 里多余
// 的 name / strict / schema 键同样报错，所以这里整个替换 text.format 而不是只改
// type 字段。schema 文本会保留进 instructions，尽量让输出仍贴近客户端期望。
func downgradeOpenAIResponsesJSONSchemaFormat(body []byte) ([]byte, bool, error) {
	if len(body) == 0 {
		return body, false, nil
	}
	if !strings.EqualFold(strings.TrimSpace(gjson.GetBytes(body, "text.format.type").String()), "json_schema") {
		return body, false, nil
	}
	schemaHint := openAIResponsesJSONSchemaHint(body)
	retryBody, err := sjson.SetBytes(body, "text.format", map[string]any{"type": "json_object"})
	if err != nil {
		return nil, false, fmt.Errorf("rewrite rejected text.format: %w", err)
	}
	return ensureOpenAIResponsesJSONKeyword(retryBody, schemaHint), true, nil
}

// openAIResponsesJSONSchemaHint 提取客户端 schema 的可读文本，供 instructions 引用。
func openAIResponsesJSONSchemaHint(body []byte) string {
	if !gjson.GetBytes(body, "text.format").Exists() {
		return ""
	}
	if schema := gjson.GetBytes(body, "text.format.schema"); schema.Exists() {
		return truncateString(schema.Raw, deepSeekJSONSchemaHintLimit)
	}
	return truncateString(gjson.GetBytes(body, "text.format").Raw, deepSeekJSONSchemaHintLimit)
}

// ensureOpenAIResponsesJSONKeyword 保证 instructions 内出现 "json" 关键词。
//
// 只支持 json_object 的上游（DeepSeek 在 Chat Completions 上是硬性要求，Responses
// 侧同源）在 prompt 里没有 json 字样时会另报一个 400。这里补一句提示，并把客户端
// 的 schema 作为参考带入。
//
// instructions 不是字符串（例如新版 API 的数组形态）时不做任何改写——宁可让上游
// 按原样报错，也不能把结构写坏。
func ensureOpenAIResponsesJSONKeyword(body []byte, schemaHint string) []byte {
	existing := gjson.GetBytes(body, "instructions")
	if existing.Exists() && existing.Type != gjson.String {
		return body
	}
	instructions := strings.TrimSpace(existing.String())
	if strings.Contains(strings.ToLower(instructions), "json") {
		return body
	}
	hint := deepSeekJSONKeywordSystemHintLead
	if schemaHint != "" {
		hint += "\n输出须满足以下 JSON Schema：" + schemaHint
	}
	if instructions != "" {
		hint = instructions + "\n\n" + hint
	}
	updated, err := sjson.SetBytes(body, "instructions", hint)
	if err != nil {
		return body
	}
	return updated
}

func isExplicitOpenAIResponsesFieldRejection(code, message string) bool {
	switch strings.TrimSpace(code) {
	case "unknown_parameter", "unsupported_parameter":
		return true
	}
	return strings.Contains(message, "unknown parameter") ||
		strings.Contains(message, "unsupported parameter")
}

func openAIResponsesRejectedParamFromMessage(message string) string {
	match := openAIResponsesRejectedMessageParamPattern.FindStringSubmatch(strings.TrimSpace(message))
	if len(match) != 2 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(match[1]))
}

func openAIResponsesMaxZeroContentParamFromMessage(message string) string {
	match := openAIResponsesMaxZeroContentMessagePattern.FindStringSubmatch(strings.TrimSpace(message))
	if len(match) != 2 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(match[1]))
}

func openAIResponsesInvalidTypeParamFromMessage(message string) string {
	match := openAIResponsesInvalidTypeMessageParamPattern.FindStringSubmatch(strings.TrimSpace(message))
	if len(match) != 2 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(match[1]))
}

func openAIResponsesCacheModelRejectionParamFromMessage(message string) string {
	match := openAIResponsesCacheModelRejectionPattern.FindStringSubmatch(strings.TrimSpace(message))
	if len(match) != 2 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(match[1]))
}

func isExplicitOpenAIResponsesNullContentRejection(code, message string) bool {
	code = strings.TrimSpace(code)
	return (code == "invalid_type" || code == "invalid_request_error" || code == "") &&
		openAIResponsesInvalidTypeMessageParamPattern.MatchString(strings.TrimSpace(message))
}

func openAIResponsesRejectedNamespaceIndex(param string) (int, bool) {
	return openAIResponsesRejectedInputIndex(openAIResponsesRejectedNamespaceParamPattern, param)
}

func openAIResponsesRejectedStatusIndex(param string) (int, bool) {
	return openAIResponsesRejectedInputIndex(openAIResponsesRejectedStatusParamPattern, param)
}

func openAIResponsesRejectedContentIndex(param string) (int, bool) {
	return openAIResponsesRejectedInputIndex(openAIResponsesRejectedContentParamPattern, param)
}

func openAIResponsesRejectedCacheIndex(param string) (int, bool) {
	return openAIResponsesRejectedInputIndex(openAIResponsesRejectedCacheParamPattern, param)
}

func openAIResponsesRejectedInputIndex(pattern *regexp.Regexp, param string) (int, bool) {
	match := pattern.FindStringSubmatch(strings.TrimSpace(param))
	if len(match) != 2 {
		return 0, false
	}
	index, err := strconv.Atoi(match[1])
	if err == nil && index >= 0 {
		return index, true
	}
	return 0, false
}

// removeOpenAIResponsesRejectedStatusAtIndex drops the status field the
// upstream rejected, and the status of every other input item sharing the
// rejected item's type.
//
// The upstream names one offending index per response, but a replayed
// conversation routinely carries dozens of items of the same type, each with a
// status its schema does not accept. Clearing one index per round trip would
// need one retry per item and exhaust the bounded retry budget long before the
// request could succeed. Items of other types keep their status: the rejection
// only proves that this type has no status field.
func removeOpenAIResponsesRejectedStatusAtIndex(body []byte, index int) ([]byte, string, bool, error) {
	itemPath := fmt.Sprintf("input.%d", index)
	rejected := gjson.GetBytes(body, itemPath)
	if !rejected.IsObject() {
		return nil, "", false, nil
	}
	if !gjson.GetBytes(body, itemPath+".status").Exists() {
		return nil, "", false, nil
	}

	retryBody := body
	cleared := 0
	rejectedType := strings.TrimSpace(rejected.Get("type").String())
	if input := gjson.GetBytes(body, "input"); rejectedType != "" && input.IsArray() {
		// Deleting a field never shifts array indexes, so positions read from
		// the original body stay valid against the rewritten one.
		for itemIndex, item := range input.Array() {
			if !item.IsObject() || strings.TrimSpace(item.Get("type").String()) != rejectedType {
				continue
			}
			statusPath := fmt.Sprintf("input.%d.status", itemIndex)
			if !gjson.GetBytes(retryBody, statusPath).Exists() {
				continue
			}
			next, err := sjson.DeleteBytes(retryBody, statusPath)
			if err != nil {
				return nil, "", false, fmt.Errorf("delete rejected status at input[%d]: %w", itemIndex, err)
			}
			retryBody = next
			cleared++
		}
	}
	if cleared == 0 {
		// The rejected item carries no type to match on; fall back to clearing
		// just the index the upstream named.
		next, err := sjson.DeleteBytes(retryBody, itemPath+".status")
		if err != nil {
			return nil, "", false, fmt.Errorf("delete rejected status at input[%d]: %w", index, err)
		}
		retryBody = next
	}
	return retryBody, "indexed status parameter rejection", true, nil
}

func removeOpenAIResponsesRejectedCacheAtIndex(body []byte, index int) ([]byte, string, bool, error) {
	itemPath := fmt.Sprintf("input.%d", index)
	if !gjson.GetBytes(body, itemPath).IsObject() {
		return nil, "", false, nil
	}
	cachePath := itemPath + ".prompt_cache_breakpoint"
	if !gjson.GetBytes(body, cachePath).Exists() {
		return nil, "", false, nil
	}
	retryBody, err := sjson.DeleteBytes(body, cachePath)
	if err != nil {
		return nil, "", false, fmt.Errorf("delete rejected prompt_cache_breakpoint at input[%d]: %w", index, err)
	}
	return retryBody, "indexed prompt_cache_breakpoint parameter rejection", true, nil
}

func normalizeOpenAIResponsesRejectedNullContentAtIndex(body []byte, index int) ([]byte, string, bool, error) {
	itemPath := fmt.Sprintf("input.%d", index)
	item := gjson.GetBytes(body, itemPath)
	content := gjson.GetBytes(body, itemPath+".content")
	if !item.IsObject() || !content.Exists() || content.Type != gjson.Null {
		return nil, "", false, nil
	}

	itemType := strings.ToLower(strings.TrimSpace(item.Get("type").String()))
	role := strings.TrimSpace(item.Get("role").String())
	contentPath := itemPath + ".content"
	switch {
	case itemType == "reasoning":
		retryBody, err := sjson.DeleteBytes(body, contentPath)
		if err != nil {
			return nil, "", false, fmt.Errorf("delete rejected null content at input[%d]: %w", index, err)
		}
		return retryBody, "indexed reasoning null content rejection", true, nil
	case itemType == "message" || role != "":
		retryBody, err := sjson.SetBytes(body, contentPath, "")
		if err != nil {
			return nil, "", false, fmt.Errorf("normalize rejected null content at input[%d]: %w", index, err)
		}
		return retryBody, "indexed message null content rejection", true, nil
	default:
		return nil, "", false, nil
	}
}

func removeOpenAIResponsesRejectedReasoningContentAtIndex(body []byte, index int) ([]byte, string, bool, error) {
	itemPath := fmt.Sprintf("input.%d", index)
	item := gjson.GetBytes(body, itemPath)
	content := item.Get("content")
	if !item.IsObject() || strings.TrimSpace(item.Get("type").String()) != "reasoning" || !content.IsArray() || len(content.Array()) == 0 {
		return nil, "", false, nil
	}
	retryBody, err := sjson.DeleteBytes(body, itemPath+".content")
	if err != nil {
		return nil, "", false, fmt.Errorf("delete rejected reasoning content at input[%d]: %w", index, err)
	}
	return retryBody, "indexed reasoning content maximum-length rejection", true, nil
}

func removeOpenAIResponsesRejectedNamespaceAtIndex(body []byte, index int) ([]byte, string, bool, error) {
	itemPath := fmt.Sprintf("input.%d", index)
	itemType := strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, itemPath+".type").String()))
	switch itemType {
	case "function_call", "tool_call", "custom_tool_call", "mcp_tool_call":
	default:
		return nil, "", false, nil
	}

	namespacePath := itemPath + ".namespace"
	if !gjson.GetBytes(body, namespacePath).Exists() {
		return nil, "", false, nil
	}
	retryBody, err := sjson.DeleteBytes(body, namespacePath)
	if err != nil {
		return nil, "", false, fmt.Errorf("delete rejected namespace at input[%d]: %w", index, err)
	}
	return retryBody, "indexed namespace parameter rejection", true, nil
}
