package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"go.uber.org/zap"
)

// forwardResponsesViaRawChatCompletions serves /v1/responses clients through an
// upstream that only supports /v1/chat/completions.
func (s *OpenAIGatewayService) forwardResponsesViaRawChatCompletions(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	var responsesReq apicompat.ResponsesRequest
	if err := json.Unmarshal(body, &responsesReq); err != nil {
		writeOpenAIResponsesFallbackError(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return nil, fmt.Errorf("parse responses request: %w", err)
	}
	originalModel := strings.TrimSpace(responsesReq.Model)
	if originalModel == "" {
		writeOpenAIResponsesFallbackError(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return nil, fmt.Errorf("missing model in request")
	}

	clientStream := responsesReq.Stream
	// custom 工具（如 codex 的 exec）降级为 function 工具转发，回程需按名字还原为
	// custom_tool_call 项，先记下名字集合；tool_search 工具同理，回程还原为
	// tool_search_call 项；namespace 子工具（如 MCP 工具）摊平转发，回程按映射还原
	// 为带 namespace 字段的 function_call 项。
	effectiveTools, err := apicompat.EffectiveResponsesTools(&responsesReq)
	if err != nil {
		writeOpenAIResponsesFallbackError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return nil, fmt.Errorf("resolve responses tools: %w", err)
	}
	customTools := apicompat.CustomToolNames(effectiveTools)
	functionTools := apicompat.FunctionToolNames(effectiveTools)
	toolSearch := apicompat.HasToolSearchTool(effectiveTools)
	namespaceTools := apicompat.NamespaceToolNames(effectiveTools)

	// 自愈回写：历史里带明文 summary 的 reasoning item 刷新进缓存，覆盖 Redis
	// 被 flush / 跨实例漂移后同 id 的 encrypted-only 副本无法再取明文的情况。
	s.recacheReasoningItemsFromInput(responsesReq.Input)

	chatReq, err := apicompat.ResponsesToChatCompletionsRequestWithOptions(&responsesReq, &apicompat.ResponsesToChatOptions{
		ReasoningContentByID: s.reasoningContentByID,
	})
	if err != nil {
		writeOpenAIResponsesFallbackError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return nil, fmt.Errorf("convert responses to chat completions: %w", err)
	}

	billingModel := resolveOpenAIForwardModel(account, originalModel, "")
	upstreamModel := normalizeOpenAIModelForUpstream(account, billingModel)
	if err := validateGPT61SolCompatRequest(body, upstreamModel); err != nil {
		writeOpenAIResponsesFallbackError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return nil, err
	}
	if openai.RequiresResponsesForToolCalls(upstreamModel) && len(effectiveTools) > 0 {
		err := fmt.Errorf("%s requires Responses for tool calls; this account only supports Chat Completions", normalizeKnownOpenAICodexModel(upstreamModel))
		writeOpenAIResponsesFallbackError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return nil, err
	}
	reasoningEffort := extractOpenAIReasoningEffortFromBody(body, upstreamModel, billingModel, originalModel)
	// 国产模型默认 effort 补充：需要 mappedModel 判定，推迟到 billingModel 算出之后。
	reasoningEffort = ApplyThinkingEnabledFallback(reasoningEffort, body, billingModel)
	chatReq.Model = upstreamModel
	if clientStream {
		chatReq.StreamOptions = &apicompat.ChatStreamOptions{IncludeUsage: true}
	}

	chatBody, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal chat completions fallback request: %w", err)
	}
	// 本条路径此前整段缺失上游归一化，导致客户端用 Responses 结构化输出
	// （text.format=json_schema）时被转成 response_format=json_schema 原样发给只
	// 支持 json_object 的上游，直接 400（线上工单 f74e374a）。现在与另外两条
	// CC 出口共用同一组归一化，不再按入站端点分别复制。
	if normalizedBody, normalized := normalizeOpenAICCUpstreamBody(account, upstreamModel, chatBody); normalized {
		chatBody = normalizedBody
	}
	chatBody, err = s.applyOpenAIFastPolicyToBody(ctx, account, upstreamModel, chatBody)
	if err != nil {
		var blocked *OpenAIFastBlockedError
		if errors.As(err, &blocked) {
			writeOpenAIFastPolicyBlockedResponse(c, blocked)
		}
		return nil, err
	}
	// /v1/responses 降级到 raw CC 的出站与 forwardAsRawChatCompletions 共用同一个
	// 独立 Ollama Cloud token 钩子；chatReq.Model 已是模型映射后的 upstreamModel。
	chatBody = clampOllamaCloudUpstreamMaxTokens(account, chatBody)
	// Keep the final outbound tier for usage-time reconciliation. A policy
	// filter that removes the field therefore leaves this nil.
	serviceTier := extractOpenAIServiceTierFromBody(chatBody)

	logger.L().Debug("openai responses: forwarding via raw chat completions",
		zap.Int64("account_id", account.ID),
		zap.String("original_model", originalModel),
		zap.String("billing_model", billingModel),
		zap.String("upstream_model", upstreamModel),
		zap.Bool("stream", clientStream),
	)
	SetOpsUpstreamModel(c, upstreamModel)

	// Build and send upstream request via the shared CC pipeline
	apiKey, targetURL, err := s.resolveCCFallbackTarget(account)
	if err != nil {
		return nil, err
	}
	resp, err := s.sendCCUpstreamRequest(ctx, c, account, targetURL, chatBody, clientStream, apiKey, account.GetOpenAIUserAgent(), "")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBody, upstreamMsg := s.readOpenAIUpstreamError(resp)
		if foErr := s.failoverOpenAIUpstreamHTTPError(ctx, c, account, resp, respBody, upstreamMsg, upstreamModel); foErr != nil {
			return nil, foErr
		}
		return s.handleErrorResponse(ctx, resp, c, account, chatBody, billingModel)
	}

	if clientStream {
		return s.streamChatCompletionsAsResponses(c, account, resp, originalModel, customTools, functionTools, toolSearch, namespaceTools, billingModel, upstreamModel, reasoningEffort, serviceTier, startTime)
	}
	return s.bufferChatCompletionsAsResponses(c, resp, originalModel, customTools, functionTools, toolSearch, namespaceTools, billingModel, upstreamModel, reasoningEffort, serviceTier, startTime)
}

func (s *OpenAIGatewayService) bufferChatCompletionsAsResponses(
	c *gin.Context,
	resp *http.Response,
	originalModel string,
	customTools map[string]bool,
	functionTools map[string]bool,
	toolSearch bool,
	namespaceTools map[string]apicompat.NamespacedToolName,
	billingModel string,
	upstreamModel string,
	reasoningEffort *string,
	serviceTier *string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")
	ccResp, usage, err := s.readCCUpstreamJSONResponse(c, resp, writeOpenAIResponsesFallbackError)
	if err != nil {
		return nil, err
	}
	responsesResp := apicompat.ChatCompletionsResponseToResponses(ccResp, originalModel, customTools, functionTools, toolSearch, namespaceTools)
	s.cacheReasoningItemsFromOutput(responsesResp.Output)

	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	c.JSON(http.StatusOK, responsesResp)

	return &OpenAIForwardResult{
		RequestID:                   requestID,
		UpstreamHeaders:             resp.Header,
		Usage:                       usage,
		Model:                       originalModel,
		BillingModel:                billingModel,
		UpstreamModel:               upstreamModel,
		ReasoningEffort:             reasoningEffort,
		UpstreamResponseServiceTier: observedUpstreamResponseServiceTier(c),
		ServiceTier:                 resolvedOpenAIUpstreamServiceTier(c, serviceTier),
		Stream:                      false,
		Duration:                    time.Since(startTime),
	}, nil
}

func (s *OpenAIGatewayService) streamChatCompletionsAsResponses(
	c *gin.Context,
	account *Account,
	resp *http.Response,
	originalModel string,
	customTools map[string]bool,
	functionTools map[string]bool,
	toolSearch bool,
	namespaceTools map[string]apicompat.NamespacedToolName,
	billingModel string,
	upstreamModel string,
	reasoningEffort *string,
	serviceTier *string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")
	writeStreamHeaders := s.newStreamHeaderWriter(c, resp.Header)

	state := apicompat.NewChatCompletionsToResponsesStreamState(originalModel)
	state.CustomTools = customTools
	state.FunctionTools = functionTools
	state.ToolSearchDeclared = toolSearch
	state.NamespaceTools = namespaceTools
	clientDisconnected := false
	clientOutputStarted := false

	writeEvents := func(events []apicompat.ResponsesStreamEvent) {
		if clientDisconnected || len(events) == 0 {
			return
		}
		writeStreamHeaders()
		for _, event := range events {
			sse, err := apicompat.ResponsesEventToSSE(event)
			if err != nil {
				logger.L().Warn("openai responses chat fallback: failed to marshal stream event",
					zap.Error(err),
					zap.String("request_id", requestID),
				)
				continue
			}
			if _, err := fmt.Fprint(c.Writer, sse); err != nil {
				clientDisconnected = true
				logger.L().Debug("openai responses chat fallback: client disconnected, continuing to drain upstream for billing",
					zap.Error(err),
					zap.String("request_id", requestID),
				)
				return
			}
			clientOutputStarted = true
		}
		c.Writer.Flush()
	}

	scan := s.scanCCStream(c, resp, "openai responses chat fallback", requestID, startTime, func(chunk *apicompat.ChatCompletionsChunk) {
		events := apicompat.ChatCompletionsChunkToResponsesEvents(chunk, state)
		s.cacheReasoningItemsFromEvents(events)
		writeEvents(events)
	})

	if scan.Err != nil {
		result := &OpenAIForwardResult{
			RequestID:                   requestID,
			UpstreamHeaders:             resp.Header,
			Usage:                       scan.Usage,
			Model:                       originalModel,
			BillingModel:                billingModel,
			UpstreamModel:               upstreamModel,
			ReasoningEffort:             reasoningEffort,
			UpstreamResponseServiceTier: observedUpstreamResponseServiceTier(c),
			ServiceTier:                 resolvedOpenAIUpstreamServiceTier(c, serviceTier),
			Stream:                      true,
			Duration:                    time.Since(startTime),
			FirstTokenMs:                scan.FirstTokenMs,
		}
		// 客户端断开/取消、本地行长限制（bufio.ErrTooLong）、响应体超限都与上游账号
		// 健康无关：保持既有直返错误语义，不换号也不降权。
		if clientDisconnected || errors.Is(scan.Err, bufio.ErrTooLong) ||
			!shouldClassifyOpenAIUpstreamStreamReadError(scan.Err, ginRequestContext(c)) {
			return result, fmt.Errorf("stream usage incomplete: %w", scan.Err)
		}
		if !openAIStreamClientOutputStarted(c, clientOutputStarted) {
			// 尚未向客户端写出语义事件：可以透明换号。此前这里一律直返普通错误，
			// 于是"上游 200 + 半截 SSE"的同一个坏账号会被客户端重试反复命中
			// （线上事故：重试 5 次全部命中同一个上游源）。
			return nil, s.newStreamReadErrorFailoverError(c, account, upstreamModel, requestID, scan.Err)
		}
		// 已写出语义事件：换号会把两个上游的回答拼进同一条流，只能返回稳定的截断
		// 错误码；但必须降权（用户已看到半截回答，客户端几乎必然立刻重试），否则
		// 客户端重试会继续被调度到同一个坏账号。
		s.penalizeClientVisibleOpenAIStreamTruncation(c, account, upstreamModel)
		return result, newOpenAIUpstreamStreamReadError(scan.Err)
	}
	if err := state.ValidateToolCallArguments(); err != nil {
		return &OpenAIForwardResult{
			RequestID:                   requestID,
			UpstreamHeaders:             resp.Header,
			Usage:                       scan.Usage,
			Model:                       originalModel,
			BillingModel:                billingModel,
			UpstreamModel:               upstreamModel,
			ReasoningEffort:             reasoningEffort,
			UpstreamResponseServiceTier: observedUpstreamResponseServiceTier(c),
			ServiceTier:                 resolvedOpenAIUpstreamServiceTier(c, serviceTier),
			Stream:                      true,
			Duration:                    time.Since(startTime),
			FirstTokenMs:                scan.FirstTokenMs,
		}, fmt.Errorf("invalid tool call arguments from upstream: %w", err)
	}

	finalEvents := apicompat.FinalizeChatCompletionsResponsesStream(state)
	s.cacheReasoningItemsFromEvents(finalEvents)
	writeEvents(finalEvents)
	if !clientDisconnected {
		writeStreamHeaders()
		if _, err := fmt.Fprint(c.Writer, "data: [DONE]\n\n"); err != nil {
			clientDisconnected = true
		}
		if !clientDisconnected {
			c.Writer.Flush()
		}
	}
	if !scan.SawDone {
		logCCStreamMissingDoneSentinel("openai responses chat fallback", requestID)
	}

	return &OpenAIForwardResult{
		RequestID:                   requestID,
		UpstreamHeaders:             resp.Header,
		Usage:                       scan.Usage,
		Model:                       originalModel,
		BillingModel:                billingModel,
		UpstreamModel:               upstreamModel,
		ReasoningEffort:             reasoningEffort,
		UpstreamResponseServiceTier: observedUpstreamResponseServiceTier(c),
		ServiceTier:                 resolvedOpenAIUpstreamServiceTier(c, serviceTier),
		Stream:                      true,
		Duration:                    time.Since(startTime),
		FirstTokenMs:                scan.FirstTokenMs,
	}, nil
}

func chatChunkStartsResponsesOutput(chunk *apicompat.ChatCompletionsChunk) bool {
	if chunk == nil {
		return false
	}
	for _, choice := range chunk.Choices {
		if choice.Delta.Content != nil || choice.Delta.ReasoningContent != nil || len(choice.Delta.ToolCalls) > 0 {
			return true
		}
	}
	return false
}

// responsesReasoningCacheTTL 是 reasoning 缓存（按 reasoning item id）的过期时间。
// Codex 会话可能跨多天恢复历史，取 7 天。
const responsesReasoningCacheTTL = 7 * 24 * time.Hour

// reasoningContentByID 按 reasoning item id 回查缓存的 reasoning 全文，供
// Responses→CC 桥接在客户端不回传明文 summary（encrypted-only reasoning
// item）时回注 reasoning_content。任何失败都 fail-open 返回 ""（维持桥接原
// 行为），因为缓存只是优化而非正确性前提。
func (s *OpenAIGatewayService) reasoningContentByID(itemID string) string {
	if s == nil || s.cache == nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	content, err := s.cache.GetReasoningContent(ctx, itemID)
	if err != nil {
		return ""
	}
	return content
}

// recacheReasoningItemsFromInput 把请求历史里带明文 summary 的 reasoning item
// 重新写入缓存（best-effort）。Codex 多数时候会原样回传明文 summary，借机
// 刷新 TTL 并自愈 Redis 被 flush / 跨实例漂移造成的缓存缺失。
func (s *OpenAIGatewayService) recacheReasoningItemsFromInput(inputRaw json.RawMessage) {
	if s == nil || s.cache == nil {
		return
	}
	inputRaw = bytes.TrimSpace(inputRaw)
	if len(inputRaw) == 0 || inputRaw[0] != '[' {
		return
	}
	var items []json.RawMessage
	if err := json.Unmarshal(inputRaw, &items); err != nil {
		return
	}
	for _, raw := range items {
		id, text, ok := apicompat.ExtractResponsesReasoningItem(raw)
		if !ok || id == "" || text == "" {
			continue
		}
		s.setReasoningContent(id, text)
	}
}

// cacheReasoningItemsFromEvents 从 Responses 流事件里提取完成的 reasoning
// item 写入缓存（覆盖一个流中的多个 reasoning item）。
func (s *OpenAIGatewayService) cacheReasoningItemsFromEvents(events []apicompat.ResponsesStreamEvent) {
	for _, event := range events {
		if event.Type != "response.output_item.done" || event.Item == nil {
			continue
		}
		s.cacheReasoningItem(event.Item)
	}
}

// cacheReasoningItemsFromOutput 从非流式 Responses 响应的 output 里提取
// reasoning item 写入缓存。
func (s *OpenAIGatewayService) cacheReasoningItemsFromOutput(output []apicompat.ResponsesOutput) {
	for i := range output {
		s.cacheReasoningItem(&output[i])
	}
}

func (s *OpenAIGatewayService) cacheReasoningItem(item *apicompat.ResponsesOutput) {
	if item == nil || item.Type != "reasoning" || item.ID == "" {
		return
	}
	var parts []string
	for _, sum := range item.Summary {
		if t := strings.TrimSpace(sum.Text); t != "" {
			parts = append(parts, t)
		}
	}
	if len(parts) == 0 {
		return
	}
	s.setReasoningContent(item.ID, strings.Join(parts, "\n"))
}

// setReasoningContent 写入缓存，使用 detached ctx：客户端断连后仍在 drain
// 上游流（计费需要），此时的 reasoning 也是后续轮次回注所依赖的，不能随
// 请求 ctx 一起取消。失败仅记日志，不影响转发。
func (s *OpenAIGatewayService) setReasoningContent(itemID, content string) {
	if s == nil || s.cache == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.cache.SetReasoningContent(ctx, itemID, content, responsesReasoningCacheTTL); err != nil {
		logger.L().Warn("openai responses chat fallback: cache reasoning content failed",
			zap.Error(err),
			zap.String("item_id", itemID),
		)
	}
}

// reasoningContentCallIDPrefix 把 Chat 侧 tool call id 与 Responses reasoning
// item id（rs_ 前缀）隔离在同一缓存命名空间内，避免客户端伪造的 call id
// 与网关上一轮缓存的 reasoning item 相互覆盖。
const reasoningContentCallIDPrefix = "cc_tool_call:"

// reasoningContentTenantScope 返回 reasoning 缓存的租户维度前缀。
//
// call_id 完全由客户端决定，大量客户端用的是 call_0 / call_1 / toolu_1 这类
// 自增短 id——不同用户会反复撞到同一个 key。若不按租户隔离，用户 B 的下一轮
// 请求会读到用户 A 缓存的思考内容，并被回注进 B 的上游请求里（跨租户串扰）。
// 缓存 TTL 是 7 天，撞车窗口很长。
//
// 返回形如 "u42:" 的前缀；取不到身份时返回 ""，退化为原来的全局命名空间，
// 不引入新的失败模式。
func reasoningContentTenantScope(c *gin.Context) string {
	if c == nil {
		return ""
	}
	v, exists := c.Get("api_key")
	if !exists {
		return ""
	}
	apiKey, ok := v.(*APIKey)
	if !ok || apiKey == nil {
		return ""
	}
	// 优先按 user 隔离：同一用户换 API Key 重放历史时仍要能命中缓存，
	// 否则会退化成「回注失败 → 上游 400 复现」。
	if apiKey.UserID > 0 {
		return "u" + strconv.FormatInt(apiKey.UserID, 10) + ":"
	}
	if apiKey.ID > 0 {
		return "k" + strconv.FormatInt(apiKey.ID, 10) + ":"
	}
	return ""
}

// reasoningContentByCallID 按 Chat 侧 tool_call id 回查产生该调用的 reasoning
// 全文，供 Chat Completions→Responses 桥接在客户端（如 TRAE）不回传
// reasoning_content 时回注 reasoning item。任何失败 fail-open 返回 ""。
//
// 无租户维度的版本仅用于不经过 HTTP 上下文的调用（单测等）。生产路径一律走
// reasoningContentByCallIDInScope。
func (s *OpenAIGatewayService) reasoningContentByCallID(callID string) string {
	return s.reasoningContentByCallIDInScope("", callID)
}

// reasoningContentByCallIDInScope 同 reasoningContentByCallID，但把缓存 key
// 限制在 scope 租户内。scope 为空时与前者完全等价。
func (s *OpenAIGatewayService) reasoningContentByCallIDInScope(scope, callID string) string {
	callID = strings.TrimSpace(callID)
	if callID == "" {
		return ""
	}
	return s.reasoningContentByID(reasoningContentCallIDPrefix + scope + callID)
}

// cacheToolCallReasoningFromOutput correlates plaintext reasoning items with
// the function_call items they produced in a (non-streaming) Responses output
// and persists each pair keyed by the call_id. The call_id is the only
// identifier that round-trips in Chat Completions history, so it is what the
// next-turn Chat→Responses conversion can look up. Best-effort: cache errors
// are logged inside setReasoningContent and never fail the forward.
func (s *OpenAIGatewayService) cacheToolCallReasoningFromOutput(output []apicompat.ResponsesOutput) {
	s.cacheToolCallReasoningFromOutputInScope("", output)
}

// cacheToolCallReasoningFromOutputInScope 同 cacheToolCallReasoningFromOutput，
// 但把结果写入 scope 租户的命名空间。scope 为空时与前者完全等价。
func (s *OpenAIGatewayService) cacheToolCallReasoningFromOutputInScope(scope string, output []apicompat.ResponsesOutput) {
	var pending string
	for i := range output {
		pending = s.cacheToolCallReasoningFromItemInScope(scope, &output[i], pending)
	}
}

// cacheToolCallReasoningFromEvents is the streaming counterpart of
// cacheToolCallReasoningFromOutput. The caller owns the pending reasoning
// string and passes the value returned by the previous invocation.
func (s *OpenAIGatewayService) cacheToolCallReasoningFromEvents(events []apicompat.ResponsesStreamEvent, pending string) string {
	return s.cacheToolCallReasoningFromEventsInScope("", events, pending)
}

// cacheToolCallReasoningFromEventsInScope 同 cacheToolCallReasoningFromEvents，
// 但把结果写入 scope 租户的命名空间。scope 为空时与前者完全等价。
func (s *OpenAIGatewayService) cacheToolCallReasoningFromEventsInScope(scope string, events []apicompat.ResponsesStreamEvent, pending string) string {
	for i := range events {
		if events[i].Type != "response.output_item.done" || events[i].Item == nil {
			continue
		}
		pending = s.cacheToolCallReasoningFromItemInScope(scope, events[i].Item, pending)
	}
	return pending
}

// cacheToolCallReasoningFromItem folds one completed output item into the
// reasoning→tool_call correlation state. All items in one Responses output
// belong to the same turn, so pending reasoning is never reset mid-output:
//   - reasoning item: its plaintext becomes the pending reasoning;
//   - function_call item: the pending reasoning is cached under its call_id
//     (pending is kept so parallel tool calls — and synthesized accumulator
//     outputs that place message before function_call — share it).
//
// Cross-turn isolation needs no explicit reset: every request owns its own
// pending variable, so the next response starts empty.
func (s *OpenAIGatewayService) cacheToolCallReasoningFromItem(item *apicompat.ResponsesOutput, pending string) string {
	return s.cacheToolCallReasoningFromItemInScope("", item, pending)
}

// cacheToolCallReasoningFromItemInScope 同 cacheToolCallReasoningFromItem，
// 但把结果写入 scope 租户的命名空间。scope 为空时与前者完全等价。
func (s *OpenAIGatewayService) cacheToolCallReasoningFromItemInScope(scope string, item *apicompat.ResponsesOutput, pending string) string {
	if item == nil {
		return pending
	}
	switch item.Type {
	case "reasoning":
		if text := extractToolCallReasoningText(item); text != "" {
			return text
		}
	case "function_call":
		if pending != "" {
			if callID := strings.TrimSpace(item.CallID); callID != "" {
				s.setReasoningContent(reasoningContentCallIDPrefix+scope+callID, pending)
			}
		}
	}
	return pending
}

// extractToolCallReasoningText returns the plaintext reasoning of a reasoning
// output item, preferring the portable summary and falling back to the raw
// reasoning_text content parts.
func extractToolCallReasoningText(item *apicompat.ResponsesOutput) string {
	var parts []string
	for _, sum := range item.Summary {
		if t := strings.TrimSpace(sum.Text); t != "" {
			parts = append(parts, t)
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, "\n")
	}
	for _, p := range item.Content {
		if p.Type == "reasoning_text" {
			if t := strings.TrimSpace(p.Text); t != "" {
				parts = append(parts, t)
			}
		}
	}
	return strings.Join(parts, "\n")
}

// deepSeekChatReasoningPlaceholderText 是 Chat Completions 侧 thinking-mode
// 占位明文。必须是单个空格：DeepSeek 拒绝空串，非空即可通过；LiteLLM 同样注入
// 单个空格。
const deepSeekChatReasoningPlaceholderText = " "

// targetsDeepSeekAPIHost 报告该账号实际上游是否是 DeepSeek 官方 API。
// platform=deepseek 直接命中；platform=openai 但 base_url 指向
// api.deepseek.com 的映射账号同样命中（Codex 把 GPT 模型名映射到 DeepSeek
// 时的典型接入方式）。
func targetsDeepSeekAPIHost(account *Account) bool {
	if account == nil {
		return false
	}
	if account.Platform == PlatformDeepseek {
		return true
	}
	u, err := url.Parse(strings.TrimSpace(account.GetOpenAIBaseURL()))
	if err != nil {
		return false
	}
	ds, err := url.Parse(DefaultDeepseekBaseURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Hostname(), ds.Hostname())
}

// targetsOpenCodeZenUpstream 报告该账号上游是否是官方 OpenCode Zen / Go 网关。
// platform=opencode_go 直接命中；其他平台按 base_url 主机判定，覆盖用 API Key
// 直接把 openai 平台账号指向 opencode.ai 的接入方式。
func targetsOpenCodeZenUpstream(account *Account) bool {
	if account == nil {
		return false
	}
	if account.IsOpenCodeGo() {
		return true
	}
	return isOfficialOpenCodeHost(account.GetOpenAIBaseURL())
}

// isDeepSeekCatalogModel 判定（剥掉 opencode 前缀后的）模型 ID 是否属于
// DeepSeek 系列，例如 deepseek-v4-flash / deepseek-v4.1-flash / deepseek-chat。
func isDeepSeekCatalogModel(model string) bool {
	return strings.HasPrefix(normalizeOpenCodeGoModelID(model), "deepseek")
}

// requiresDeepSeekChatReasoning 报告该 Chat Completions 请求的上游是否按
// DeepSeek thinking mode 语义校验 reasoning_content。
//
// DeepSeek 官方 API 直接命中。OpenCode Zen / Go 是转发型订阅网关：deepseek-*
// 模型由 DeepSeek 实际承载，同一条 400 原文会被一字不差地回吐，所以按
// 「官方 OpenCode 上游 + deepseek-* 模型」补一条判定。
//
// 不放宽成「任意上游的 deepseek-* 模型名」：占位符是写进请求体的额外字段，
// 对没有这条约束的上游属于无谓改写，且可能触发未知字段校验。
func requiresDeepSeekChatReasoning(account *Account, body []byte) bool {
	if targetsDeepSeekAPIHost(account) {
		return true
	}
	if !targetsOpenCodeZenUpstream(account) {
		return false
	}
	return isDeepSeekCatalogModel(gjson.GetBytes(body, "model").String())
}

// ensureDeepSeekChatReasoningPlaceholders 给缺 reasoning_content 的 assistant
// 消息补单个空格占位。DeepSeek thinking mode 要求历史里每条产生过思维的
// assistant 消息都回传该字段，否则 400
// "The `reasoning_content` in the thinking mode must be passed back to the API"。
//
// 桥接会从 summary / 缓存回注真实明文；这里只填仍为空的缺口，不覆盖已有内容。
// 判定见 requiresDeepSeekChatReasoning，不命中的上游原样返回（字节不变）。
func ensureDeepSeekChatReasoningPlaceholders(account *Account, body []byte) []byte {
	if !requiresDeepSeekChatReasoning(account, body) {
		return body
	}
	messages := gjson.GetBytes(body, "messages")
	if !messages.IsArray() {
		return body
	}
	updated := body
	changed := false
	for i, msg := range messages.Array() {
		if strings.TrimSpace(msg.Get("role").String()) != "assistant" {
			continue
		}
		if msg.Get("reasoning_content").String() != "" {
			continue
		}
		next, err := sjson.SetBytes(updated, "messages."+strconv.Itoa(i)+".reasoning_content", deepSeekChatReasoningPlaceholderText)
		if err != nil {
			return body
		}
		updated = next
		changed = true
	}
	if !changed {
		return body
	}
	return updated
}
