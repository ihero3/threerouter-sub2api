package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// OpenAIUpstreamHTTP2StreamErrorCode is returned to OpenAI-compatible clients
	// when an upstream HTTP/2 response stream is reset after the request started.
	OpenAIUpstreamHTTP2StreamErrorCode = "upstream_http2_stream_error"
	OpenAIUpstreamStreamReadErrorCode  = "upstream_stream_read_error"
	// OpenAIUpstreamStreamTruncatedCode is returned when an upstream SSE stream
	// closes *cleanly* before delivering any terminal signal. A clean EOF carries
	// no transport error, so without this classification a truncated generation is
	// indistinguishable from a successful one.
	OpenAIUpstreamStreamTruncatedCode = "upstream_stream_truncated"
)

// ErrOpenAIUpstreamStreamTruncated marks an upstream SSE stream that ended at
// EOF — without a read error — before any terminal signal arrived.
var ErrOpenAIUpstreamStreamTruncated = errors.New("upstream stream ended before any terminal chunk")

type openAIUpstreamStreamReadError struct {
	cause         error
	clientCode    string
	clientMessage string
}

func (e *openAIUpstreamStreamReadError) Error() string {
	return fmt.Sprintf("stream usage incomplete: %v", e.cause)
}

func (e *openAIUpstreamStreamReadError) Unwrap() error { return e.cause }

func newOpenAIUpstreamStreamReadError(err error) error {
	code, message := classifyOpenAIUpstreamStreamReadError(err)
	return &openAIUpstreamStreamReadError{
		cause:         err,
		clientCode:    code,
		clientMessage: message,
	}
}

// newStreamReadErrorFailoverError 在返回透明换号错误之前，先按账号级截断降权
// 计数（与 raw 直转路径一致）：同一账号 5 分钟内累计 2 次上游流中断 → 临时降权
// 2 分钟。单次抖动不降权；连续失败才会把账号移出调度，让客户端的自动重试落到
// 其他上游源，而不是反复命中同一个坏账号。
func (s *OpenAIGatewayService) newStreamReadErrorFailoverError(
	c *gin.Context,
	account *Account,
	upstreamModel string,
	upstreamRequestID string,
	cause error,
) *UpstreamFailoverError {
	s.penalizeOpenAIStreamTruncation(c, account, upstreamModel)
	return newOpenAIStreamReadErrorFailoverError(c, account, upstreamRequestID, cause)
}

// penalizeOpenAIStreamTruncation 递增账号级截断计数；窗口内重复命中时把账号
// 临时移出调度。已写出部分内容、无法就地换号的截断同样要计数，否则同一坏账号
// 会在客户端的后续重试里被反复选中。
func (s *OpenAIGatewayService) penalizeOpenAIStreamTruncation(c *gin.Context, account *Account, upstreamModel string) {
	if s == nil || s.rateLimitService == nil || c == nil || c.Request == nil {
		return
	}
	// 客户端断开后 c.Request.Context() 也会被取消：降权计数与落库必须用
	// WithoutCancel，否则这次降权会被一并取消，账号永远不会被摘除。
	penaltyCtx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 3*time.Second)
	s.rateLimitService.HandleStreamTruncation(penaltyCtx, account, upstreamModel)
	cancel()
}

// penalizeClientVisibleOpenAIStreamTruncation 用于"语义输出已经下发给客户端之后
// 才断流"的形态：用户已经看到被截断的回答，客户端几乎必然立刻重试，而重试请求
// 通常带着相同的内容指纹、会被粘性会话绑回同一个上游账号。这里在常规的窗口计数
// 之外立刻做一次进程内冷却，保证紧跟着的重试不会再命中同一个坏源。
func (s *OpenAIGatewayService) penalizeClientVisibleOpenAIStreamTruncation(c *gin.Context, account *Account, upstreamModel string) {
	if s == nil || s.rateLimitService == nil || c == nil || c.Request == nil {
		return
	}
	penaltyCtx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 3*time.Second)
	s.rateLimitService.PenalizeClientVisibleStreamTruncation(penaltyCtx, account, upstreamModel)
	cancel()
}

// newOpenAIStreamReadErrorFailoverError 处理"上游 SSE 流读取失败且尚未向客户端
// 写出任何语义字节"的情况：可以透明换号重试，客户端不会看到半截流。
// RetryableOnSameAccount 保持零值（false）——刚刚断开的传输通道原地重试只会再
// 失败一次，直接切换到下一个可用账号。
//
// SafeToFailoverAfterWrite：只要本次转发还没有写出语义字节，就已经写出的字节
// 只可能是不含语义的 SSE 注释（handleChatStreamingResponse 在 keepalive 间隔到点
// 时写出的 `:\n\n`，或并发槽位等待期的 ping）。这些字节会把响应头提交为 200，
// 但没有任何模型内容；若 handler 只按 writer size 判定，就会误判成"已经开始向
// 客户端输出"而拒绝换号，把本可自愈的截断直接变成 502——线上表现就是同一个坏
// 账号被客户端一次次重试反复命中。没有写出任何字节时不置位，让该次请求保留
// 完整的换号预算（置位会被 handler 的首包换号上限约束为 1 次）。
func newOpenAIStreamReadErrorFailoverError(
	c *gin.Context,
	account *Account,
	upstreamRequestID string,
	cause error,
) *UpstreamFailoverError {
	recordOpenAIStreamReadFailure(c, account, upstreamRequestID, cause)

	headers := http.Header{}
	if id := strings.TrimSpace(upstreamRequestID); id != "" {
		headers.Set("x-request-id", id)
	}
	return &UpstreamFailoverError{
		StatusCode:               http.StatusBadGateway,
		ResponseBody:             openAIUpstreamStreamReadErrorBody(cause),
		ResponseHeaders:          headers,
		SafeToFailoverAfterWrite: c != nil && c.Writer != nil && c.Writer.Written(),
	}
}

// recordOpenAIStreamReadFailure 把换号前的上游流读取失败记入 ops 上下文，
// 使其在错误日志与账号健康度中可见。
func recordOpenAIStreamReadFailure(
	c *gin.Context,
	account *Account,
	upstreamRequestID string,
	cause error,
) {
	if c == nil {
		return
	}
	code, clientMessage := classifyOpenAIUpstreamStreamReadError(cause)
	message := clientMessage
	if cause != nil && !errors.Is(cause, ErrOpenAIUpstreamStreamTruncated) {
		message = clientMessage + ": " + cause.Error()
	}
	platform := PlatformOpenAI
	accountID := int64(0)
	accountName := ""
	if account != nil {
		platform = account.Platform
		accountID = account.ID
		accountName = account.Name
	}

	setOpsUpstreamError(c, http.StatusBadGateway, message, code)
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		ProxyID:            opsUpstreamProxyID(account),
		ProxyName:          opsUpstreamProxyName(account),
		Platform:           platform,
		AccountID:          accountID,
		AccountName:        accountName,
		UpstreamStatusCode: http.StatusBadGateway,
		UpstreamRequestID:  strings.TrimSpace(upstreamRequestID),
		Kind:               "failover",
		Message:            message,
	})
}

// shouldClassifyOpenAIUpstreamStreamReadError excludes cancellation and
// response-size enforcement from upstream retry.
func shouldClassifyOpenAIUpstreamStreamReadError(err error, contexts ...context.Context) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrUpstreamResponseBodyTooLarge) {
		return false
	}
	for _, ctx := range contexts {
		if ctx != nil && ctx.Err() != nil {
			return false
		}
	}
	return true
}

// OpenAIUpstreamStreamReadErrorDetails returns the stable, sanitized client
// classification attached to an upstream stream read failure.
func OpenAIUpstreamStreamReadErrorDetails(err error) (code, message string, ok bool) {
	var streamErr *openAIUpstreamStreamReadError
	if !errors.As(err, &streamErr) || streamErr == nil {
		return "", "", false
	}
	return streamErr.clientCode, streamErr.clientMessage, true
}

func classifyOpenAIUpstreamStreamReadError(err error) (code, message string) {
	if err != nil {
		if errors.Is(err, ErrOpenAIUpstreamStreamTruncated) {
			return OpenAIUpstreamStreamTruncatedCode, "Upstream response stream ended before completion"
		}
		lower := strings.ToLower(err.Error())
		// net/http's HTTP/2 stream error is unexported. Its stable text contains
		// "stream error: stream ID ..."; match only the transport signature and
		// never pass the original text to the client.
		if strings.Contains(lower, "stream error: stream id ") ||
			(strings.Contains(lower, "http2:") && strings.Contains(lower, "stream")) {
			return OpenAIUpstreamHTTP2StreamErrorCode, "Upstream HTTP/2 stream failed"
		}
	}
	return OpenAIUpstreamStreamReadErrorCode, "Upstream response stream was interrupted"
}
