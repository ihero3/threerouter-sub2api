package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/stretchr/testify/require"
)

// toolCallReasoningRecordingCache 嵌入完整接口空桩，仅记录 reasoning 读写。
type toolCallReasoningRecordingCache struct {
	stubGatewayCache
	sets    map[string]string
	getResp map[string]string
}

func (c *toolCallReasoningRecordingCache) SetReasoningContent(_ context.Context, itemID string, content string, _ time.Duration) error {
	if c.sets == nil {
		c.sets = make(map[string]string)
	}
	c.sets[itemID] = content
	return nil
}

func (c *toolCallReasoningRecordingCache) GetReasoningContent(_ context.Context, itemID string) (string, error) {
	if v, ok := c.getResp[itemID]; ok {
		return v, nil
	}
	return "", ErrReasoningContentNotFound
}

func (c *toolCallReasoningRecordingCache) snapshotSets() map[string]string {
	out := make(map[string]string, len(c.sets))
	for k, v := range c.sets {
		out[k] = v
	}
	return out
}

// reasoning item 紧随其后的 function_call 必须按 call_id 缓存推理明文——
// 这是 TRAE 这类只存 Chat 历史的客户端在后续轮次回注 reasoning_text 的数据
// 来源（修复 DeepSeek/Kimi thinking mode 400 的写入侧）。
func TestCacheToolCallReasoningFromOutput_KeysReasoningByCallID(t *testing.T) {
	cache := &toolCallReasoningRecordingCache{}
	svc := &OpenAIGatewayService{cache: cache}

	svc.cacheToolCallReasoningFromOutput([]apicompat.ResponsesOutput{
		{
			Type:    "reasoning",
			ID:      "rs_1",
			Summary: []apicompat.ResponsesSummary{{Type: "summary_text", Text: "plan edit"}},
		},
		{Type: "function_call", CallID: "call_a", Name: "edit_file", Arguments: "{}"},
		{Type: "message", Role: "assistant", Content: []apicompat.ResponsesContentPart{{Type: "output_text", Text: "done"}}},
	})

	sets := cache.snapshotSets()
	require.Equal(t, "plan edit", sets["cc_tool_call:call_a"])
	// rs_ item id and call id live in disjoint namespaces
	require.NotContains(t, sets, "call_a")
	// reasoning without a following tool call is not cached
	require.NotContains(t, sets, "rs_1")
}

// 同一轮并行工具调用共享同一段推理：每个 function_call 都应被缓存。
func TestCacheToolCallReasoningFromOutput_ParallelToolCallsShareReasoning(t *testing.T) {
	cache := &toolCallReasoningRecordingCache{}
	svc := &OpenAIGatewayService{cache: cache}

	svc.cacheToolCallReasoningFromOutput([]apicompat.ResponsesOutput{
		{Type: "reasoning", Summary: []apicompat.ResponsesSummary{{Type: "summary_text", Text: "shared thought"}}},
		{Type: "function_call", CallID: "call_p1"},
		{Type: "function_call", CallID: "call_p2"},
	})

	sets := cache.snapshotSets()
	require.Equal(t, "shared thought", sets["cc_tool_call:call_p1"])
	require.Equal(t, "shared thought", sets["cc_tool_call:call_p2"])
}

// 同一 output 属同一轮：message item 位于 reasoning 与 function_call 之间
// （非流式聚合器的合成顺序）时，function_call 仍必须关联到该推理。
// 跨轮隔离由每个请求各自的 pending 局部变量保证，无需 message 重置。
func TestCacheToolCallReasoningFromOutput_MessageDoesNotResetPending(t *testing.T) {
	cache := &toolCallReasoningRecordingCache{}
	svc := &OpenAIGatewayService{cache: cache}

	svc.cacheToolCallReasoningFromOutput([]apicompat.ResponsesOutput{
		{Type: "reasoning", Summary: []apicompat.ResponsesSummary{{Type: "summary_text", Text: "think then speak then call"}}},
		{Type: "message", Role: "assistant", Content: []apicompat.ResponsesContentPart{{Type: "output_text", Text: "let me run it"}}},
		{Type: "function_call", CallID: "call_after_message"},
	})

	sets := cache.snapshotSets()
	require.Equal(t, "think then speak then call", sets["cc_tool_call:call_after_message"])
}

// 整轮没有 reasoning item（普通非思考模型）时 fail-open：不伪造任何缓存条目。
func TestCacheToolCallReasoningFromOutput_NoReasoningCachesNothing(t *testing.T) {
	cache := &toolCallReasoningRecordingCache{}
	svc := &OpenAIGatewayService{cache: cache}

	svc.cacheToolCallReasoningFromOutput([]apicompat.ResponsesOutput{
		{Type: "function_call", CallID: "call_plain"},
	})

	require.Empty(t, cache.snapshotSets())
}

// summary 缺失时回退提取 content[].reasoning_text（部分上游只给原文）。
func TestCacheToolCallReasoningFromOutput_FallsBackToReasoningTextContent(t *testing.T) {
	cache := &toolCallReasoningRecordingCache{}
	svc := &OpenAIGatewayService{cache: cache}

	svc.cacheToolCallReasoningFromOutput([]apicompat.ResponsesOutput{
		{
			Type: "reasoning",
			Content: []apicompat.ResponsesContentPart{
				{Type: "reasoning_text", Text: "raw reasoning text"},
			},
		},
		{Type: "function_call", CallID: "call_raw"},
	})

	require.Equal(t, "raw reasoning text", cache.snapshotSets()["cc_tool_call:call_raw"])
}

// reasoningContentByCallID 通过同一前缀读回，供去程转换回注。
func TestReasoningContentByCallID_ReadThrough(t *testing.T) {
	cache := &toolCallReasoningRecordingCache{
		getResp: map[string]string{"cc_tool_call:call_77": "stored thought"},
	}
	svc := &OpenAIGatewayService{cache: cache}

	require.Equal(t, "stored thought", svc.reasoningContentByCallID("call_77"))
	require.Equal(t, "", svc.reasoningContentByCallID("call_missing"))
	require.Equal(t, "", svc.reasoningContentByCallID("   "))
}
