package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// service 侧的心跳字节记账 key（service.openAIStreamKeepaliveBytesKey，未导出）。
// 这里跨包黑盒地模拟 /v1/messages 的 event: ping 记账；key 若被改名，下面的断言会
// 如实失败（adjusted size 不再被扣减），不会静默变成假通过。
const openAIStreamKeepaliveBytesContextKey = "openai_stream_keepalive_bytes"

// TestOpenAIMessagesFailoverGateIgnoresKeepaliveOnlyBytes 守住线上事故里第二个卡点：
// /v1/messages 的换号闸门此前直接比较裸 c.Writer.Size()。上游只要先静默一个心跳
// 周期，网关就会写出 event: ping 帧把响应头提交为 200，此后这一帧心跳会永久关掉
// 换号预算——上游紧接着断流时，请求被直接判成 502，客户端重试 5 次全部被调度到
// 同一个坏账号。
//
// 现在闸门与 /v1/responses 主循环同口径：快照用
// service.OpenAICompactKeepaliveAdjustedWrittenSize（扣除心跳字节后归一化为 -1），
// 判定走 openAIForwardMayFailover。只写出过心跳时仍可透明换号。
func TestOpenAIMessagesFailoverGateIgnoresKeepaliveOnlyBytes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	writerSizeBeforeForward := service.OpenAICompactKeepaliveAdjustedWrittenSize(c)
	require.Equal(t, -1, writerSizeBeforeForward, "转发前没有任何字节：快照应为 gin 的未写出哨兵值")

	// 模拟 /v1/messages 的心跳：写出 Anthropic ping 帧并按 service 的方式记账。
	pingFrame := "event: ping\ndata: {\"type\":\"ping\"}\n\n"
	written, err := c.Writer.WriteString(pingFrame)
	require.NoError(t, err)
	c.Writer.Flush()
	require.True(t, c.Writer.Written(), "心跳帧会把响应头提交为 200")
	c.Set(openAIStreamKeepaliveBytesContextKey, written)

	require.Equal(t, writerSizeBeforeForward, service.OpenAICompactKeepaliveAdjustedWrittenSize(c),
		"心跳字节必须被扣减，否则换号闸门会被一帧心跳永久关闭")
	require.True(t, openAIForwardMayFailover(c, writerSizeBeforeForward, &service.UpstreamFailoverError{}),
		"只有心跳字节写出时必须允许换号，即便 failover 错误没有声明 SafeToFailoverAfterWrite")
}

// TestOpenAIMessagesFailoverGateBlocksAfterSemanticOutput 保证换号闸门对真正的语义
// 输出仍然生效：写出 message_start/content_block_delta 之后不得中途切换账号
// （否则客户端会看到两个上游拼起来的回答）。
func TestOpenAIMessagesFailoverGateBlocksAfterSemanticOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	writerSizeBeforeForward := service.OpenAICompactKeepaliveAdjustedWrittenSize(c)

	// 先写心跳（记账），再写真正的内容帧。
	pingFrame := "event: ping\ndata: {\"type\":\"ping\"}\n\n"
	written, err := c.Writer.WriteString(pingFrame)
	require.NoError(t, err)
	c.Set(openAIStreamKeepaliveBytesContextKey, written)

	semanticFrame := "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"hi\"}}\n\n"
	_, err = c.Writer.WriteString(semanticFrame)
	require.NoError(t, err)
	c.Writer.Flush()

	require.False(t, openAIForwardMayFailover(c, writerSizeBeforeForward, &service.UpstreamFailoverError{}),
		"已写出语义内容后不得中途换号")
}
