package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 本文件守住线上事故的两条不变量（/v1/messages 及其两条 CC 回退路径）：
//
//  1. 上游 SSE 流在终止信号之前断掉、且网关还没有向客户端写出任何语义内容时，
//     必须交出 *UpstreamFailoverError（RetryableOnSameAccount=false）由 handler
//     透明换号，而不是把 502 直接抛给客户端；
//  2. 无论能否换号，这次截断都必须给账号记一次降权（RateLimitService
//     .HandleStreamTruncation）。否则客户端重试会被调度器再次选中同一个坏账号，
//     线上表现就是"用户重试 5 次、每次都命中同一个上游源"。

// messagesTruncationCounterRecorder 只统计降权计数调用次数，返回 1（首帧不触发
// 临时降权，避免测试依赖账号库）。
type messagesTruncationCounterRecorder struct {
	increments int
}

func (r *messagesTruncationCounterRecorder) IncrementStreamTruncationCount(_ context.Context, _ int64, _ int) (int64, error) {
	r.increments++
	return 1, nil
}

func (r *messagesTruncationCounterRecorder) ResetStreamTruncationCount(_ context.Context, _ int64) error {
	return nil
}

// messagesTruncationSchedulingBlocker 记录"语义输出已下发给客户端后才断流"触发的
// 进程内调度冷却（RateLimitService.PenalizeClientVisibleStreamTruncation）。
type messagesTruncationSchedulingBlocker struct {
	blockedReasons []string
}

func (b *messagesTruncationSchedulingBlocker) BlockAccountScheduling(_ *Account, _ time.Time, reason string) {
	b.blockedReasons = append(b.blockedReasons, reason)
}

func (b *messagesTruncationSchedulingBlocker) ClearAccountSchedulingBlock(int64) {}

// messagesTruncationTestHarness 注入带截断计数器 + 调度冷却记录器的 RateLimitService：
// 用它们证明降权确实被调用，而不依赖真实 Redis / 账号库。
type messagesTruncationTestHarness struct {
	svc     *OpenAIGatewayService
	counter *messagesTruncationCounterRecorder
	blocker *messagesTruncationSchedulingBlocker
}

func newMessagesTruncationTestHarness() *messagesTruncationTestHarness {
	counter := &messagesTruncationCounterRecorder{}
	blocker := &messagesTruncationSchedulingBlocker{}
	rateLimitService := NewRateLimitService(nil, nil, &config.Config{}, nil, nil)
	rateLimitService.SetStreamTruncationCounterCache(counter)
	rateLimitService.SetAccountRuntimeBlocker(blocker)
	return &messagesTruncationTestHarness{
		svc: &OpenAIGatewayService{
			cfg:              messagesTruncationTestConfig(),
			rateLimitService: rateLimitService,
		},
		counter: counter,
		blocker: blocker,
	}
}

func messagesTruncationTestConfig() *config.Config {
	return &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{
				Enabled:           false,
				AllowInsecureHTTP: true,
			},
		},
	}
}

// messagesTruncationTestAccount 复刻事故里的上游源：第三方 OpenAI 兼容中转
// （APIKey 账号，只支持 /v1/chat/completions），模型映射 deepseek-v4-pro →
// mimo-v2.6-pro。
func messagesTruncationTestAccount() *Account {
	return &Account{
		ID:          911,
		Name:        "third-party-relay",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "http://upstream.example",
		},
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceChatCompletions),
		},
	}
}

// messagesTruncationBrokenReader 先吐出 data，再返回 err（而不是 io.EOF），
// 模拟上游连接在流中途被 reset。
type messagesTruncationBrokenReader struct {
	data []byte
	off  int
	err  error
}

func (r *messagesTruncationBrokenReader) Read(p []byte) (int, error) {
	if r.off < len(r.data) {
		n := copy(p, r.data[r.off:])
		r.off += n
		return n, nil
	}
	return 0, r.err
}

func (r *messagesTruncationBrokenReader) Close() error { return nil }

func newMessagesTruncationTestContext(t *testing.T, path string, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	var requestBody io.Reader
	if body != nil {
		requestBody = bytes.NewReader(body)
	}
	c.Request = httptest.NewRequest(http.MethodPost, path, requestBody)
	c.Request.Header.Set("Content-Type", "application/json")
	return c, rec
}

func messagesTruncationStreamResponse(requestID string, body io.ReadCloser) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
			"x-request-id": []string{requestID},
		},
		Body: body,
	}
}

// 上游一个字节都没发就被 reset：必须换号（RetryableOnSameAccount=false 表示不要
// 在原地重试同一条坏通道），并且换号前给账号记一次降权。
func TestHandleAnthropicStreamingResponse_ReadErrorBeforeOutputFailsOver(t *testing.T) {
	h := newMessagesTruncationTestHarness()
	c, rec := newMessagesTruncationTestContext(t, "/v1/messages", nil)

	resp := messagesTruncationStreamResponse("rid_msgs_pre_output", &messagesTruncationBrokenReader{
		err: errors.New("read tcp 10.0.0.1:443: read: connection reset by peer"),
	})

	result, err := h.svc.handleAnthropicStreamingResponse(
		resp, c, messagesTruncationTestAccount(),
		"deepseek-v4-pro", "deepseek-v4-pro", "mimo-v2.6-pro", time.Now(),
	)

	require.Nil(t, result, "failover 错误不得携带 result，避免重复计费")
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.True(t, failoverErr.ShouldRetryNextAccount(), "客户端应被透明切到下一个账号")
	require.False(t, failoverErr.RetryableOnSameAccount, "刚刚断开的通道原地重试只会再失败一次")
	require.False(t, failoverErr.SafeToFailoverAfterWrite, "一个字节都没写出时不应占用首包换号预算")
	require.Contains(t, string(failoverErr.ResponseBody), OpenAIUpstreamStreamReadErrorCode)
	_, _, ok := OpenAIUpstreamStreamReadErrorDetails(err)
	require.False(t, ok, "failover 错误不得伪装成客户端侧读取错误")
	require.Empty(t, rec.Body.String(), "换号前不得向客户端写出任何字节")
	require.Equal(t, 1, h.counter.increments, "换号前必须同步降权，否则客户端重试会再次命中同一账号")
	require.Empty(t, h.blocker.blockedReasons, "尚未写出语义内容的截断走透明换号，无需进程内冷却")
}

// 只写出过 event: ping 心跳帧（响应头已被提交为 200）之后上游断流：仍然必须换号，
// 且 SafeToFailoverAfterWrite=true —— heartbeat 字节不含模型内容，不能锁死换号。
func TestHandleAnthropicStreamingResponse_PingOnlyThenReadErrorFailsOver(t *testing.T) {
	h := newMessagesTruncationTestHarness()
	c, rec := newMessagesTruncationTestContext(t, "/v1/messages", nil)

	// 模拟 keepalive ticker 已经写出 Anthropic ping 帧（service 内部按字节记账，
	// 见 openai_gateway_messages.go 的 recordOpenAIStreamKeepaliveBytes 调用）。
	pingFrame := "event: ping\ndata: {\"type\":\"ping\"}\n\n"
	written, werr := c.Writer.WriteString(pingFrame)
	require.NoError(t, werr)
	c.Writer.Flush()
	recordOpenAIStreamKeepaliveBytes(c, written)
	require.Equal(t, -1, OpenAICompactKeepaliveAdjustedWrittenSize(c),
		"纯心跳字节必须被扣除并归一化为未写出，否则 handler 侧换号闸门会被一帧心跳永久关闭")

	resp := messagesTruncationStreamResponse("rid_msgs_ping", &messagesTruncationBrokenReader{
		err: errors.New("stream error: stream ID 3; INTERNAL_ERROR; received from peer"),
	})

	result, err := h.svc.handleAnthropicStreamingResponse(
		resp, c, messagesTruncationTestAccount(),
		"deepseek-v4-pro", "deepseek-v4-pro", "mimo-v2.6-pro", time.Now(),
	)

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.True(t, failoverErr.ShouldRetryNextAccount())
	require.False(t, failoverErr.RetryableOnSameAccount)
	require.True(t, failoverErr.SafeToFailoverAfterWrite, "心跳已提交 200，但仍允许在同一客户端流内换号")
	require.Contains(t, string(failoverErr.ResponseBody), OpenAIUpstreamHTTP2StreamErrorCode)
	require.Equal(t, pingFrame, rec.Body.String(), "心跳之后换号前不得再写出其他内容")
	require.Equal(t, 1, h.counter.increments)
	require.Empty(t, h.blocker.blockedReasons, "尚未写出语义内容的截断走透明换号，无需进程内冷却")
}

// 已写出语义内容后上游断流：不能换号（会把两条回答拼进一条流），但必须降权，
// 并返回带类型的截断错误码（而不是泛化错误）。
func TestHandleAnthropicStreamingResponse_ReadErrorAfterOutputPenalizesWithoutFailover(t *testing.T) {
	h := newMessagesTruncationTestHarness()
	c, rec := newMessagesTruncationTestContext(t, "/v1/messages", nil)

	partial := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_1","model":"deepseek-v4-pro","status":"in_progress","output":[]}}`,
		"",
		`data: {"type":"response.output_text.delta","delta":"partial"}`,
		"",
		"",
	}, "\n")
	resp := messagesTruncationStreamResponse("rid_msgs_post_output", &messagesTruncationBrokenReader{
		data: []byte(partial),
		err:  errors.New("stream error: stream ID 5; INTERNAL_ERROR; received from peer"),
	})

	result, err := h.svc.handleAnthropicStreamingResponse(
		resp, c, messagesTruncationTestAccount(),
		"deepseek-v4-pro", "deepseek-v4-pro", "mimo-v2.6-pro", time.Now(),
	)

	require.NotNil(t, result)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr), "已写出语义内容后不得再换号")
	code, _, ok := OpenAIUpstreamStreamReadErrorDetails(err)
	require.True(t, ok, "必须返回带类型的上游流读取错误")
	require.Equal(t, OpenAIUpstreamHTTP2StreamErrorCode, code)
	require.Contains(t, rec.Body.String(), "partial", "断流前已写出的内容必须到达客户端")
	require.Equal(t, 1, h.counter.increments, "无法换号的截断同样必须降权")
	require.Len(t, h.blocker.blockedReasons, 1, "客户端已经看到语义输出的截断必须立刻做进程内冷却，避免紧跟的重试命中同一账号")
}

// 干净 EOF + 缺终止事件（事故的直接形态）：未写出语义内容时换号，并降权。
func TestHandleAnthropicStreamingResponse_MissingTerminalBeforeOutputFailsOverAndPenalizes(t *testing.T) {
	h := newMessagesTruncationTestHarness()
	c, _ := newMessagesTruncationTestContext(t, "/v1/messages", nil)

	resp := messagesTruncationStreamResponse("rid_msgs_missing_terminal", io.NopCloser(strings.NewReader("data: [DONE]\n\n")))

	result, err := h.svc.handleAnthropicStreamingResponse(
		resp, c, messagesTruncationTestAccount(),
		"deepseek-v4-pro", "deepseek-v4-pro", "mimo-v2.6-pro", time.Now(),
	)

	require.NotNil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.True(t, failoverErr.ShouldRetryNextAccount())
	require.Contains(t, string(failoverErr.ResponseBody), "terminal event")
	require.Equal(t, 1, h.counter.increments, "未写出语义内容的截断同样必须降权")
	require.Empty(t, h.blocker.blockedReasons, "尚未写出语义内容的截断走透明换号，无需进程内冷却")
}

// 干净 EOF + 缺终止事件 + 已写出语义内容：不换号，但要降权。
func TestHandleAnthropicStreamingResponse_MissingTerminalAfterOutputPenalizes(t *testing.T) {
	h := newMessagesTruncationTestHarness()
	c, rec := newMessagesTruncationTestContext(t, "/v1/messages", nil)

	partial := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_1","model":"deepseek-v4-pro","status":"in_progress","output":[]}}`,
		"",
		`data: {"type":"response.output_text.delta","delta":"partial"}`,
		"",
		"",
	}, "\n")
	resp := messagesTruncationStreamResponse("rid_msgs_missing_terminal_post", io.NopCloser(strings.NewReader(partial)))

	result, err := h.svc.handleAnthropicStreamingResponse(
		resp, c, messagesTruncationTestAccount(),
		"deepseek-v4-pro", "deepseek-v4-pro", "mimo-v2.6-pro", time.Now(),
	)

	require.NotNil(t, result)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr))
	require.Contains(t, err.Error(), "missing terminal event")
	require.Contains(t, rec.Body.String(), "partial")
	require.Equal(t, 1, h.counter.increments, "已写出语义内容的缺终止事件同样必须降权")
	require.Len(t, h.blocker.blockedReasons, 1, "客户端已经看到语义输出的截断必须立刻做进程内冷却，避免紧跟的重试命中同一账号")
}

// 客户端取消/断开：与上游截断不可区分，保持既有语义——不换号、不降权。
func TestHandleAnthropicStreamingResponse_ClientCanceledReadErrorNoFailoverNoPenalty(t *testing.T) {
	h := newMessagesTruncationTestHarness()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(canceledCtx)

	resp := messagesTruncationStreamResponse("rid_msgs_canceled", &messagesTruncationBrokenReader{
		err: errors.New("read tcp: connection reset by peer"),
	})

	result, err := h.svc.handleAnthropicStreamingResponse(
		resp, c, messagesTruncationTestAccount(),
		"deepseek-v4-pro", "deepseek-v4-pro", "mimo-v2.6-pro", time.Now(),
	)

	require.NotNil(t, result, "客户端断开时按已收到的 usage 正常收尾")
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr), "客户端取消不得触发换号")
	require.Equal(t, 0, h.counter.increments, "客户端取消不得降权上游账号")
	require.Empty(t, h.blocker.blockedReasons, "客户端取消不得做账号调度冷却")
}

// ── /v1/messages 的 CC 回退（streamChatCompletionsAsAnthropic）──

func TestForwardAsAnthropic_ChatFallbackStreamReadErrorBeforeOutputFailsOver(t *testing.T) {
	h := newMessagesTruncationTestHarness()
	body := []byte(`{"model":"deepseek-v4-pro","max_tokens":64,"messages":[{"role":"user","content":"hello"}],"stream":true}`)
	c, rec := newMessagesTruncationTestContext(t, "/v1/messages", body)

	h.svc.httpUpstream = &httpUpstreamRecorder{resp: messagesTruncationStreamResponse("rid_cc_pre", &messagesTruncationBrokenReader{
		err: errors.New("read tcp: connection reset by peer"),
	})}

	result, err := h.svc.ForwardAsAnthropic(context.Background(), c, messagesTruncationTestAccount(), body, "", "")

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.True(t, failoverErr.ShouldRetryNextAccount())
	require.False(t, failoverErr.RetryableOnSameAccount)
	require.Empty(t, rec.Body.String(), "换号前不得向客户端写出任何字节")
	require.Equal(t, 1, h.counter.increments)
	require.Empty(t, h.blocker.blockedReasons, "尚未写出语义内容的截断走透明换号，无需进程内冷却")
}

func TestForwardAsAnthropic_ChatFallbackStreamReadErrorAfterOutputPenalizesWithoutFailover(t *testing.T) {
	h := newMessagesTruncationTestHarness()
	body := []byte(`{"model":"deepseek-v4-pro","max_tokens":64,"messages":[{"role":"user","content":"hello"}],"stream":true}`)
	c, rec := newMessagesTruncationTestContext(t, "/v1/messages", body)

	partial := strings.Join([]string{
		`data: {"id":"chatcmpl_e","object":"chat.completion.chunk","model":"mimo-v2.6-pro","choices":[{"index":0,"delta":{"role":"assistant","content":"he"},"finish_reason":null}]}`,
		"",
		"",
	}, "\n")
	h.svc.httpUpstream = &httpUpstreamRecorder{resp: messagesTruncationStreamResponse("rid_cc_post", &messagesTruncationBrokenReader{
		data: []byte(partial),
		err:  errors.New("read tcp: connection reset by peer"),
	})}

	result, err := h.svc.ForwardAsAnthropic(context.Background(), c, messagesTruncationTestAccount(), body, "", "")

	require.NotNil(t, result)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr), "已写出语义内容后不得再换号")
	code, _, ok := OpenAIUpstreamStreamReadErrorDetails(err)
	require.True(t, ok)
	require.Equal(t, OpenAIUpstreamStreamReadErrorCode, code)
	require.Contains(t, rec.Body.String(), `"text":"he"`)
	require.NotContains(t, rec.Body.String(), "event: message_stop", "不得凭空补一个完成帧掩盖截断")
	require.Equal(t, 1, h.counter.increments)
	require.Len(t, h.blocker.blockedReasons, 1, "客户端已经看到语义输出的截断必须立刻做进程内冷却，避免紧跟的重试命中同一账号")
}

// ── /v1/responses 的 CC 回退（streamChatCompletionsAsResponses）──

func TestForwardResponses_ChatFallbackStreamReadErrorBeforeOutputFailsOver(t *testing.T) {
	h := newMessagesTruncationTestHarness()
	body := []byte(`{"model":"deepseek-v4-pro","input":"hello","stream":true}`)
	c, rec := newMessagesTruncationTestContext(t, "/v1/responses", body)

	h.svc.httpUpstream = &httpUpstreamRecorder{resp: messagesTruncationStreamResponse("rid_resp_cc_pre", &messagesTruncationBrokenReader{
		err: errors.New("read tcp: connection reset by peer"),
	})}

	result, err := h.svc.Forward(context.Background(), c, messagesTruncationTestAccount(), body)

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.True(t, failoverErr.ShouldRetryNextAccount())
	require.False(t, failoverErr.RetryableOnSameAccount)
	require.Empty(t, rec.Body.String())
	require.Equal(t, 1, h.counter.increments)
	require.Empty(t, h.blocker.blockedReasons, "尚未写出语义内容的截断走透明换号，无需进程内冷却")
}

func TestForwardResponses_ChatFallbackStreamReadErrorAfterOutputPenalizesWithoutFailover(t *testing.T) {
	h := newMessagesTruncationTestHarness()
	body := []byte(`{"model":"deepseek-v4-pro","input":"hello","stream":true}`)
	c, rec := newMessagesTruncationTestContext(t, "/v1/responses", body)

	partial := strings.Join([]string{
		`data: {"id":"chatcmpl_e","object":"chat.completion.chunk","model":"mimo-v2.6-pro","choices":[{"index":0,"delta":{"role":"assistant","content":"he"},"finish_reason":null}]}`,
		"",
		"",
	}, "\n")
	h.svc.httpUpstream = &httpUpstreamRecorder{resp: messagesTruncationStreamResponse("rid_resp_cc_post", &messagesTruncationBrokenReader{
		data: []byte(partial),
		err:  errors.New("read tcp: connection reset by peer"),
	})}

	result, err := h.svc.Forward(context.Background(), c, messagesTruncationTestAccount(), body)

	require.NotNil(t, result)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr), "已写出语义事件后不得再换号")
	code, _, ok := OpenAIUpstreamStreamReadErrorDetails(err)
	require.True(t, ok)
	require.Equal(t, OpenAIUpstreamStreamReadErrorCode, code)
	require.NotEmpty(t, rec.Body.String())
	require.Equal(t, 1, h.counter.increments)
	require.Len(t, h.blocker.blockedReasons, 1, "客户端已经看到语义输出的截断必须立刻做进程内冷却，避免紧跟的重试命中同一账号")
}
