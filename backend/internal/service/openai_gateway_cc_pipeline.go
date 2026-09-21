package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// 本文件收敛三个 CC（Chat Completions）forwarder 之间重复的 HTTP 管线与 SSE
// 循环骨架（PR #3802 遗留项）：
//
//   - forwardAsRawChatCompletions          （原生 CC 直转）
//   - forwardResponsesViaRawChatCompletions（/v1/responses → CC 回退）
//   - forwardAnthropicViaRawChatCompletions（/v1/messages → CC 回退）
//
// 以及 messages / chat_completions 两条 Responses 主路径中逐字相同的错误处理块。
// 所有 helper 都是对既有内联代码的等价提取，不改变任何行为；各路径的差异
// （GLM effort 归一化、fast policy、Grok 分支、ClientDisconnect 语义等）仍留在
// 调用方，属于有意保留的行为差异，不在此强行统一。

// newUpstreamSSEScanner 构造读取上游 SSE 流的行扫描器，按配置放大单行上限。
func (s *OpenAIGatewayService) newUpstreamSSEScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)
	return scanner
}

// newStreamHeaderWriter 返回幂等的 SSE 响应头写入闭包：首次调用时透传过滤后的
// 上游响应头并写入标准 SSE 头 + 200 状态码，后续调用为 no-op。延迟到首个事件
// 写出前才提交响应头，使上游早期失败仍可改走 failover 或非流式错误响应。
func (s *OpenAIGatewayService) newStreamHeaderWriter(c *gin.Context, upstream http.Header) func() {
	headersWritten := false
	return func() {
		if headersWritten {
			return
		}
		headersWritten = true
		if s.responseHeaderFilter != nil {
			responseheaders.WriteFilteredHeaders(c.Writer.Header(), upstream, s.responseHeaderFilter)
		}
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("X-Accel-Buffering", "no")
		c.Writer.WriteHeader(http.StatusOK)
	}
}

// readOpenAIUpstreamError 读取上游错误体并把 resp.Body 回卷为可重读的副本
// （下游 handleXxxErrorResponse 需要再次读取），返回原始错误体与脱敏后的
// 上游错误消息。
func (s *OpenAIGatewayService) readOpenAIUpstreamError(resp *http.Response) ([]byte, string) {
	respBody := s.readUpstreamErrorBody(resp)
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(respBody))

	upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(respBody))
	upstreamMsg = sanitizeUpstreamErrorMessage(upstreamMsg)
	return respBody, upstreamMsg
}

// failoverOpenAIUpstreamHTTPError 对 >=400 的上游响应做 failover 判定：命中时
// 记录 ops 事件、执行账号级错误处置并返回 *UpstreamFailoverError；未命中返回
// nil，调用方继续走各自端点格式的非 failover 错误处理链。
func (s *OpenAIGatewayService) failoverOpenAIUpstreamHTTPError(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	resp *http.Response,
	respBody []byte,
	upstreamMsg string,
	upstreamModel string,
) *UpstreamFailoverError {
	shouldFailover := s.shouldFailoverOpenAIUpstreamResponse(resp.StatusCode, upstreamMsg, respBody)
	tempUnscheduled := false
	if c != nil && account != nil && account.Platform != PlatformGrok && !shouldFailover && !IsResponseCommitted(c) && s.rateLimitService != nil {
		tempUnscheduled = s.rateLimitService.CheckErrorPolicy(ctx, account, resp.StatusCode, respBody, upstreamModel) == ErrorPolicyTempUnscheduled
		shouldFailover = tempUnscheduled
	}
	if account != nil && account.Platform == PlatformGrok {
		shouldFailover = s.shouldFailoverGrokUpstreamError(resp.StatusCode, respBody)
	}
	if account != nil && account.Platform == PlatformGrok {
		s.handleGrokAccountUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody)
	}
	if !shouldFailover {
		return nil
	}
	upstreamDetail := ""
	if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
		maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
		if maxBytes <= 0 {
			maxBytes = 2048
		}
		upstreamDetail = truncateString(string(respBody), maxBytes)
	}
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		ProxyID:            opsUpstreamProxyID(account),
		ProxyName:          opsUpstreamProxyName(account),
		Platform:           account.Platform,
		AccountID:          account.ID,
		AccountName:        account.Name,
		UpstreamStatusCode: resp.StatusCode,
		UpstreamRequestID:  resp.Header.Get("x-request-id"),
		Kind:               "failover",
		Message:            upstreamMsg,
		Detail:             upstreamDetail,
	})
	shouldDisable := tempUnscheduled
	if account.Platform != PlatformGrok && !tempUnscheduled {
		shouldDisable = s.handleOpenAIAccountUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody, upstreamModel)
	}
	return s.newOpenAIAccountFailoverError(
		account,
		resp.StatusCode,
		resp.Header,
		respBody,
		upstreamMsg,
		shouldDisable,
		!shouldDisable && account.IsPoolMode() && (account.IsPoolModeRetryableStatus(resp.StatusCode) || isOpenAITransientProcessingError(resp.StatusCode, upstreamMsg, respBody)),
	)
}

// openAIChatCompletionsTargetURL 解析账号的（非 Grok）Chat Completions 上游端点。
func (s *OpenAIGatewayService) openAIChatCompletionsTargetURL(account *Account) (string, error) {
	baseURL := account.GetOpenAIBaseURL()
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base_url: %w", err)
	}
	return buildOpenAIChatCompletionsURL(validatedURL), nil
}

// resolveCCFallbackTarget 解析两条 CC 回退路径共用的账号凭证与上游端点
// （回退路径仅面向 APIKey 账号，凭证恒为 openai api_key）。
func (s *OpenAIGatewayService) resolveCCFallbackTarget(account *Account) (apiKey string, targetURL string, err error) {
	apiKey = strings.TrimSpace(account.GetOpenAIProtocolAPIKey())
	if apiKey == "" {
		return "", "", fmt.Errorf("account %d missing api_key", account.ID)
	}
	targetURL, err = s.openAIChatCompletionsTargetURL(account)
	if err != nil {
		return "", "", err
	}
	return apiKey, targetURL, nil
}

// normalizeOpenAICCUpstreamBody 是所有 CC 上游出口共用的请求体归一化。
//
// 为什么必须集中在这里：这些归一化此前是**按入站端点各自复制**的，结果只加在了
// 入站 CC 直转与 /v1/messages 回退两条路径上，**入站 /v1/responses → 上游 CC 的
// 回退路径整段缺失**。客户端用 Responses 结构化输出（text.format=json_schema）时，
// 转换层会产出 response_format=json_schema，原样发给只支持 json_object 的上游即
// 400（线上工单 f74e374a，账号「星图」）。集中到公共出口后，今后新增任何 CC 出口
// 都不可能再漏掉其中任何一项。
//
// 注意顺序：先 GLM effort 归一化，再 DeepSeek response_format 归一化——后者会改
// response_format 并可能注入 system 提示，放在最后可确保它看到的是最终形态。
func normalizeOpenAICCUpstreamBody(account *Account, upstreamModel string, body []byte) ([]byte, bool) {
	if len(body) == 0 {
		return body, false
	}
	changed := false
	if normalized, ok := NormalizeGLMOpenAIReasoningEffort(body, upstreamModel); ok {
		body, changed = normalized, true
	}
	if normalized, ok := NormalizeDeepSeekResponseFormat(account, upstreamModel, body); ok {
		body, changed = normalized, true
	}
	return body, changed
}

// sendCCUpstreamRequest 构建并发送 CC 上游请求：分离的上游 context、OpenAI HTTP
// profile、标准头（含流式 Accept 切换）、客户端 header 白名单透传、自定义 UA 与
// 账号级 header 覆写，最后经代理发出。传输层失败（DNS/TCP/TLS，无 HTTP 响应）
// 统一由 handleOpenAIUpstreamTransportError 归一为 failover。
//
// userAgent 为空时保留默认 UA；Grok 的默认 UA 兜底由调用方解析后传入。
//
// 另含一层错误驱动自愈：上游以 400 明确说明"这次请求哪里不接受"时，按其说明改写
// 请求体后**重试一次**（判定与改写见 ccUpstreamSelfHealBody）。目前两类：
//   - 不支持 json_schema 结构化输出 → 降级为 json_object（isCCJSONSchemaUnsupportedError）
//   - 不接受客户端给的 reasoning_effort 取值 → 按上游自己给的可接受档位就近对齐
//     （upstreamReasoningEffortAllowedValuesFromErrorBody）
//
// 自愈的变更范围被刻意压到最小——**只有重发拿到 <400 才改变结果**：
//   - 重发成功：交回成功的响应（原本必然 400 的请求被救活，纯收益）；
//   - 重发仍是 4xx/5xx，或重发根本没拿到响应（传输失败）：**丢弃重发结果，交回原始
//     响应**。客户端因此看到的仍是它原本就会看到的那个错误，失败路径零行为变化。
//
// 这条规则是硬要求：本函数同时服务 /v1/chat/completions，任何"失败时换个错误给客户端"
// 的行为都算破坏既有接口。
func (s *OpenAIGatewayService) sendCCUpstreamRequest(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	targetURL string,
	body []byte,
	stream bool,
	bearerToken string,
	userAgent string,
	grokCacheIdentity string,
) (*http.Response, error) {
	resp, err := s.doCCUpstreamRequest(ctx, c, account, targetURL, body, stream, bearerToken, userAgent, grokCacheIdentity)
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.StatusCode != http.StatusBadRequest {
		return resp, nil
	}
	// 400 但上游没给 body：没有任何判定依据，直接原样交回。这同时避免了对
	// nil Body 调 ReadAll 的崩溃——本函数是唯一会在拿到响应后立刻读 body 的
	// CC 出口，而自定义 HTTPUpstream 实现是可能返回 Body 为 nil 的响应的。
	if resp.Body == nil {
		return resp, nil
	}
	// 读错误体判定，但必须原样塞回：调用方随后还要用 resp.Body 做 failover
	// 判定与错误响应。
	//
	// 读上限刻意与调用方的 readUpstreamErrorBody 取**同一个**配置口径：若这里
	// 用一个更小的自定上限，超大错误体（>上限）回填后就会比改动前调用方能读到的
	// 更短，等于悄悄改了调用方的错误处理输入。
	errorBody, readErr := io.ReadAll(io.LimitReader(resp.Body, openAIUpstreamErrorBodyReadLimitForConfig(s.cfg)))
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(errorBody))
	if readErr != nil {
		return resp, nil
	}
	retryBody, selfHealReason, changed := ccUpstreamSelfHealBody(body, errorBody)
	if !changed {
		return resp, nil
	}
	// 与其余出口共用同一套重试预算与 body 去重。
	//
	// 本函数会被**每个候选账号各调一次**（账号 failover / 凭证 failover），不接预算
	// 就是"每账号各重试一次"，会把一次入站请求的上游调用数按账号数放大成 2N；
	// 上游若对这条请求稳定 400，放大出来的全是无效调用。
	// Responses 侧（retryOpenAIResponsesRejectedFieldOnce）刻意接了同一个预算，
	// 这里必须同口径，否则同一种自愈在两条链路上松紧不一。
	retryState := openAIResponsesRejectedFieldRetryStateForRequest(c, body)
	if !retryState.Allow(retryBody) {
		return resp, nil
	}
	logger.LegacyPrintf("service.openai_gateway",
		"[CC] upstream rejected request (%s), retrying with adjusted body (account: %s, upstream_model_hint: %s, stream: %t)",
		selfHealReason, accountDisplayName(account), ccResponseFormatModelHint(body), stream)
	retryResp, retryErr := s.doCCUpstreamRequest(ctx, c, account, targetURL, retryBody, stream, bearerToken, userAgent, grokCacheIdentity)
	if retryErr != nil || retryResp == nil {
		// 重发没拿到响应（传输层失败）：丢弃它，交回原始 400。自愈只在"重发成功"时
		// 改变结果，失败路径必须与没重试过完全一致——否则客户端会从"上游拒绝了
		// 这条请求体"变成看到一条连接类错误，那是行为破坏。
		logger.LegacyPrintf("service.openai_gateway",
			"[CC] self-heal retry (%s) did not produce a response, keeping original error (account: %s, err: %v)",
			selfHealReason, accountDisplayName(account), retryErr)
		return resp, nil
	}
	if retryResp.StatusCode >= 400 {
		// 重发仍失败：同样丢弃，让调用方按原始错误走既有链路（failover 判定、错误响应、
		// 账号处置都基于原始响应，语义与本次改动前一致）。
		if retryResp.Body != nil {
			_ = retryResp.Body.Close()
		}
		logger.LegacyPrintf("service.openai_gateway",
			"[CC] self-heal retry (%s) still failed with %d, keeping original error (account: %s)",
			selfHealReason, retryResp.StatusCode, accountDisplayName(account))
		return resp, nil
	}
	return retryResp, nil
}

// ccUpstreamSelfHealBody 决定这条 400 是否可以被"改写请求体后重发一次"救活。
//
// 返回 (改写后的请求体, 原因, 是否发生改写)。**只要 changed 为 false 就必须原样
// 交回上游响应**——自愈的语义边界是"只救活、不改写失败原因"，这是硬要求，
// 因为本函数服务的 /v1/chat/completions 上还有大量既有错误处理链
// （failover 判定、账号处置、错误响应格式）都基于那个原始响应。
//
// 每个分支都必须**同时**满足两个条件才会返回 changed：
//  1. 上游错误体明确指向这件事（而不是泛指某类错误）；
//  2. 请求体里真的有可改写的东西（否则就是无关 400，乱改会把入参错误换个样子再报）。
func ccUpstreamSelfHealBody(body, errorBody []byte) ([]byte, string, bool) {
	if len(body) == 0 || len(errorBody) == 0 {
		return body, "", false
	}
	if isCCJSONSchemaUnsupportedError(errorBody) {
		if downgraded, changed := downgradeCCJSONSchemaResponseFormat(body); changed {
			return downgraded, "json_schema response_format not supported", true
		}
	}
	if allowed, ok := upstreamReasoningEffortAllowedValuesFromErrorBody(errorBody); ok {
		if adjusted, changed := clampReasoningEffortToAllowedValues(body, allowed); changed {
			return adjusted, "reasoning_effort value rejected", true
		}
	}
	return body, "", false
}

func (s *OpenAIGatewayService) doCCUpstreamRequest(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	targetURL string,
	body []byte,
	stream bool,
	bearerToken string,
	userAgent string,
	grokCacheIdentity string,
) (*http.Response, error) {
	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	upstreamReq, err := http.NewRequestWithContext(upstreamCtx, http.MethodPost, targetURL, bytes.NewReader(body))
	releaseUpstreamCtx()
	if err != nil {
		return nil, fmt.Errorf("build upstream request: %w", err)
	}
	// 记录本次实际选择的协议端点，供错误日志和用量日志在没有
	// OpenAIForwardResult（例如 503/传输失败）时使用。每次发送都覆盖，
	// 避免 Gin context 在账号 failover 尝试之间残留旧端点。
	SetActualOpenAIUpstreamEndpoint(c, "/v1/chat/completions")
	upstreamReq = upstreamReq.WithContext(WithHTTPUpstreamProfile(upstreamReq.Context(), HTTPUpstreamProfileOpenAI))
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Authorization", "Bearer "+bearerToken)
	if stream {
		upstreamReq.Header.Set("Accept", "text/event-stream")
	} else {
		upstreamReq.Header.Set("Accept", "application/json")
	}

	// 透传白名单中的客户端 header。详见 openaiCCRawAllowedHeaders 的设计说明。
	for key, values := range c.Request.Header {
		lowerKey := strings.ToLower(key)
		if openaiCCRawAllowedHeaders[lowerKey] {
			for _, v := range values {
				upstreamReq.Header.Add(key, v)
			}
		}
	}
	if userAgent != "" {
		upstreamReq.Header.Set("user-agent", userAgent)
	}

	if account.Platform == PlatformGrok {
		if account.IsGrokOAuth() {
			applyGrokCLIHeaders(upstreamReq.Header)
		}
		applyGrokCacheHeaders(upstreamReq.Header, grokCacheIdentity)
	}
	// 账号级请求头覆写：放在所有内置默认头（含 Grok CLI 身份头）之后应用，
	// 使配置值获得除共享传输层强制头之外的最高优先级。
	account.ApplyHeaderOverrides(upstreamReq.Header)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.doOpenAIUpstream(upstreamReq, proxyURL, account)
	if err != nil {
		return nil, s.handleOpenAIUpstreamTransportError(ctx, c, account, err, false)
	}
	return resp, nil
}

// ccStreamScanState 是 scanCCStream 返回的读取状态快照。
type ccStreamScanState struct {
	// Usage 为 include_usage chunk 中最近一次出现的用量（上游可能重复发送，
	// 总是保留最新值）；终态事件中的用量由调用方在 finalize 阶段自行覆盖。
	Usage OpenAIUsage
	// FirstTokenMs 为首个实际输出 chunk（排除 usage-only chunk）的到达时延。
	FirstTokenMs *int
	// SawDone 表示上游发出了 [DONE] 哨兵。
	SawDone bool
	// Err 为 scanner 读错误（客户端 context 取消不属于此类，会原样带出）。
	// 非 nil 时调用方必须跳过 finalize 并返回 usage-incomplete 错误，避免
	// 把上游截断伪装成正常收尾。
	Err error
}

// scanCCStream 驱动两条 CC 回退路径共享的 SSE 读循环：提取 data 行、在 [DONE]
// 哨兵处停止、保留最新 usage、记录首 token 时延，并把每个解析成功的 chunk 交给
// emit 回调做各自的协议转换与写出。读错误按既有约定过滤 context 取消类噪声后
// 记入 Warn 日志。
func (s *OpenAIGatewayService) scanCCStream(
	c *gin.Context,
	resp *http.Response,
	logPrefix string,
	requestID string,
	startTime time.Time,
	emit func(*apicompat.ChatCompletionsChunk),
) ccStreamScanState {
	var st ccStreamScanState

	scanner := s.newUpstreamSSEScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		payload, ok := extractOpenAISSEDataLine(line)
		if !ok {
			continue
		}
		payload = strings.TrimSpace(payload)
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			st.SawDone = true
			break
		}
		// 观察上游 CC chunk 回显的 model / service_tier（计费以回显为准）。
		// CC chunk 无 type 字段，按 untyped payload 观察（上游约束：只有终止
		// 事件与无类型 body 报告实际处理档位）。
		if observer := upstreamResponseModelObserverFromContext(c); observer != nil {
			observer.ObserveOpenAI([]byte(payload), "")
		}

		if u := extractCCStreamUsage(payload); u != nil {
			st.Usage = *u
		}

		var chunk apicompat.ChatCompletionsChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			logger.L().Warn(logPrefix+": failed to parse chat stream chunk",
				zap.Error(err),
				zap.String("request_id", requestID),
			)
			continue
		}
		if st.FirstTokenMs == nil && !isOpenAIChatUsageOnlyStreamChunk(payload) && chatChunkStartsResponsesOutput(&chunk) {
			ms := int(time.Since(startTime).Milliseconds())
			st.FirstTokenMs = &ms
		}
		emit(&chunk)
	}

	if err := scanner.Err(); err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			logger.L().Warn(logPrefix+": stream read error",
				zap.Error(err),
				zap.String("request_id", requestID),
			)
		}
		st.Err = err
	}
	return st
}

// logCCStreamMissingDoneSentinel 记录"上游未发 [DONE] 哨兵即结束"的 debug 日志。
func logCCStreamMissingDoneSentinel(logPrefix, requestID string) {
	logger.L().Debug(logPrefix+": upstream stream ended without done sentinel",
		zap.String("request_id", requestID),
	)
}

// readCCUpstreamJSONResponse 读取并解析 CC 非流式 JSON 响应，失败时以调用方
// 端点格式回写错误；成功时顺带提取 usage。
func (s *OpenAIGatewayService) readCCUpstreamJSONResponse(
	c *gin.Context,
	resp *http.Response,
	writeError compatErrorWriter,
) (*apicompat.ChatCompletionsResponse, OpenAIUsage, error) {
	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		if !errors.Is(err, ErrUpstreamResponseBodyTooLarge) {
			writeError(c, http.StatusBadGateway, "api_error", "Failed to read upstream response")
		}
		return nil, OpenAIUsage{}, fmt.Errorf("read upstream body: %w", err)
	}

	var ccResp apicompat.ChatCompletionsResponse
	if err := json.Unmarshal(respBody, &ccResp); err != nil {
		writeError(c, http.StatusBadGateway, "api_error", "Failed to parse upstream response")
		return nil, OpenAIUsage{}, fmt.Errorf("parse chat completions response: %w", err)
	}
	// 观察上游 CC JSON 回显的 model / service_tier（计费以回显为准）。
	// CC JSON 无 type 字段，按 untyped payload 观察（上游约束）。
	if observer := upstreamResponseModelObserverFromContext(c); observer != nil {
		observer.ObserveOpenAI(respBody, "")
	}

	usage := OpenAIUsage{}
	if parsed, ok := extractOpenAIUsageFromJSONBytes(respBody); ok {
		usage = parsed
	}
	return &ccResp, usage, nil
}

// writeOpenAIResponsesFallbackError 以 /v1/responses 回退路径的既有错误格式回写
// （裸 error 对象；不调用 MarkResponseCommitted，与原内联写法保持一致）。
func writeOpenAIResponsesFallbackError(c *gin.Context, statusCode int, errType, message string) {
	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}
