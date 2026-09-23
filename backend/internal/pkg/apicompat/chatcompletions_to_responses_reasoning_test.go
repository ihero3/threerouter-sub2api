package apicompat

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// 工具调用轮次：客户端原样回传了 reasoning_content 时，转换必须把它发成独立
// 的 reasoning item（content[].reasoning_text + summary），且位于
// function_call 之前，而不是包进可见的 <thinking> 文本——否则 DeepSeek/Kimi
// Responses 端点返回 400 "The `reasoning_text` in the thinking mode must be
// passed back to the API"。
func TestChatCompletionsToResponses_ToolCallReplaysReasoningItem(t *testing.T) {
	req := &ChatCompletionsRequest{
		Model: "deepseek-v4",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"edit the file"`)},
			{
				Role:             "assistant",
				ReasoningContent: "plan the edit",
				ToolCalls: []ChatToolCall{{
					ID:   "call_1",
					Type: "function",
					Function: ChatFunctionCall{
						Name:      "edit_file",
						Arguments: `{"path":"a.ts"}`,
					},
				}},
			},
			{Role: "tool", ToolCallID: "call_1", Content: json.RawMessage(`"ok"`)},
		},
	}

	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)

	body, err := json.Marshal(resp)
	require.NoError(t, err)

	// input: [user message, reasoning item, function_call, function_call_output]
	reasoning := gjson.GetBytes(body, "input.1")
	require.Equal(t, "reasoning", reasoning.Get("type").String())
	require.True(t, strings.HasPrefix(reasoning.Get("id").String(), "rs_"), "reasoning item id must start with rs_, got %q", reasoning.Get("id").String())
	require.Equal(t, "summary_text", reasoning.Get("summary.0.type").String())
	require.Equal(t, "plan the edit", reasoning.Get("summary.0.text").String())
	require.Equal(t, "reasoning_text", reasoning.Get("content.0.type").String())
	require.Equal(t, "plan the edit", reasoning.Get("content.0.text").String())

	fc := gjson.GetBytes(body, "input.2")
	require.Equal(t, "function_call", fc.Get("type").String())
	require.Equal(t, "call_1", fc.Get("call_id").String())
	require.Equal(t, "edit_file", fc.Get("name").String())

	// reasoning must not also leak into any visible assistant message
	require.NotContains(t, string(body), "<thinking>")
}

// 工具调用轮次但客户端（如 TRAE）未回传 reasoning_content：opts 回调按
// tool_call id 从网关缓存取回明文，同样生成 reasoning item。
func TestChatCompletionsToResponses_ToolCallRestoresReasoningByCallID(t *testing.T) {
	var queriedCallIDs []string
	opts := &ChatCompletionsToResponsesOptions{
		ReasoningContentByCallID: func(callID string) string {
			queriedCallIDs = append(queriedCallIDs, callID)
			if callID == "call_42" {
				return "cached reasoning"
			}
			return ""
		},
	}
	req := &ChatCompletionsRequest{
		Model: "kimi-k2",
		Messages: []ChatMessage{
			{
				Role: "assistant",
				ToolCalls: []ChatToolCall{{
					ID:   "call_42",
					Type: "function",
					Function: ChatFunctionCall{Name: "run", Arguments: "{}"},
				}},
			},
			{Role: "tool", ToolCallID: "call_42", Content: json.RawMessage(`"done"`)},
		},
	}

	resp, err := ChatCompletionsToResponsesWithOptions(req, opts)
	require.NoError(t, err)
	require.Equal(t, []string{"call_42"}, queriedCallIDs)

	body, err := json.Marshal(resp)
	require.NoError(t, err)

	reasoning := gjson.GetBytes(body, "input.0")
	require.Equal(t, "reasoning", reasoning.Get("type").String())
	require.Equal(t, "cached reasoning", reasoning.Get("content.0.text").String())
	require.Equal(t, "cached reasoning", reasoning.Get("summary.0.text").String())
	require.Equal(t, "function_call", gjson.GetBytes(body, "input.1.type").String())
}

// 没有任何可用 reasoning 明文时不伪造 reasoning item（fail-open，保持旧形态）。
func TestChatCompletionsToResponses_ToolCallWithoutReasoningSkipsItem(t *testing.T) {
	req := &ChatCompletionsRequest{
		Model: "deepseek-v4",
		Messages: []ChatMessage{
			{
				Role: "assistant",
				ToolCalls: []ChatToolCall{{
					ID:   "call_9",
					Type: "function",
					Function: ChatFunctionCall{Name: "run", Arguments: "{}"},
				}},
			},
			{Role: "tool", ToolCallID: "call_9", Content: json.RawMessage(`"x"`)},
		},
	}

	resp, err := ChatCompletionsToResponsesWithOptions(req, nil)
	require.NoError(t, err)

	body, err := json.Marshal(resp)
	require.NoError(t, err)

	require.Equal(t, "function_call", gjson.GetBytes(body, "input.0.type").String())
	for _, item := range gjson.GetBytes(body, "input").Array() {
		require.NotEqual(t, "reasoning", item.Get("type").String())
	}
}

// 无工具调用的 assistant reasoning 维持历史行为：作为 <thinking> 标签包裹的
// 可见文本随 assistant 消息发送。
func TestChatCompletionsToResponses_NonToolReasoningStaysAsThinkingText(t *testing.T) {
	req := &ChatCompletionsRequest{
		Model: "deepseek-v4",
		Messages: []ChatMessage{
			{
				Role:             "assistant",
				Content:          json.RawMessage(`"the answer is 42"`),
				ReasoningContent: "some thoughts",
			},
		},
	}

	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)

	body, err := json.Marshal(resp)
	require.NoError(t, err)

	require.Equal(t, "assistant", gjson.GetBytes(body, "input.0.role").String())
	text := gjson.GetBytes(body, "input.0.content.0.text").String()
	require.Contains(t, text, "<thinking>some thoughts</thinking>")
	require.Contains(t, text, "the answer is 42")
	for _, item := range gjson.GetBytes(body, "input").Array() {
		require.NotEqual(t, "reasoning", item.Get("type").String())
	}
}

// 同一轮既有可见正文又有工具调用：reasoning 走独立 item，正文只包含可见内容，
// 顺序为 reasoning → assistant message → function_call。
func TestChatCompletionsToResponses_ToolCallReasoningItemKeepsMessageClean(t *testing.T) {
	req := &ChatCompletionsRequest{
		Model: "deepseek-v4",
		Messages: []ChatMessage{
			{
				Role:             "assistant",
				Content:          json.RawMessage(`"I will edit now"`),
				ReasoningContent: "edit reasoning",
				ToolCalls: []ChatToolCall{{
					ID:   "call_2",
					Type: "function",
					Function: ChatFunctionCall{Name: "edit_file", Arguments: "{}"},
				}},
			},
		},
	}

	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)

	body, err := json.Marshal(resp)
	require.NoError(t, err)

	items := gjson.GetBytes(body, "input").Array()
	require.Len(t, items, 3)
	require.Equal(t, "reasoning", items[0].Get("type").String())
	require.Equal(t, "assistant", items[1].Get("role").String())
	require.Equal(t, "I will edit now", items[1].Get("content.0.text").String())
	require.NotContains(t, items[1].Raw, "thinking")
	require.Equal(t, "function_call", items[2].Get("type").String())
	require.Equal(t, "call_2", items[2].Get("call_id").String())
}
