package service

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"go.uber.org/zap"
)

// 上游在已经向客户端下发部分回答之后断流（读取错误，或在终止信号前干净 EOF）时，
// 直接返回 502 会让用户看到半截回答 + 报错。这里改用「自动续写」补救：
// 把已经下发的内容作为 assistant 前缀重新请求一次上游，只把后续增量接着写进
// 同一条 SSE 响应，客户端最终拿到完整回答且不会看到错误帧。
//
// 约束（刻意保守）：
//   - 每个请求最多续写一次；续写仍失败则沿用原有的 502 语义，不做无限重试。
//   - 只有已经下发过语义内容时才续写（没有前缀就无法无缝拼接）。
//   - 续写消耗的 token 正常累加进本次请求 usage，不静默免费。
const openAIStreamContinuationInstruction = "上一条回复因传输中断被截断。请从中断处直接继续输出剩余内容：不要重复已经输出的任何内容，不要重新开头，也不要解释。"

// openAIStreamContinuationChatBody 把「已下发的 assistant 前缀 + 继续指令」追加到
// Chat Completions messages 末尾，得到续写请求体。返回 ok=false 表示无法续写
// （前缀为空或请求体不是 messages 结构的 CC 请求）。
func openAIStreamContinuationChatBody(chatBody []byte, assistantPrefix string) ([]byte, bool, error) {
	prefix := strings.TrimSpace(assistantPrefix)
	if prefix == "" || !gjson.GetBytes(chatBody, "messages").IsArray() {
		return nil, false, nil
	}
	body, err := sjson.SetBytes(chatBody, "messages.-1", map[string]string{
		"role":    "assistant",
		"content": assistantPrefix,
	})
	if err != nil {
		return nil, false, err
	}
	body, err = sjson.SetBytes(body, "messages.-1", map[string]string{
		"role":    "user",
		"content": openAIStreamContinuationInstruction,
	})
	if err != nil {
		return nil, false, err
	}
	return body, true, nil
}

// openAIStreamContinuationDelta 判断续写流的 data 帧是否含有需要继续下发的增量。
// 只放行 content / reasoning_content / tool_calls：role 起始帧、finish_reason 帧
// 与空帧都会被丢弃，避免客户端在同一条响应里看到第二个开头或提前结束。
func openAIStreamContinuationDelta(payload string) bool {
	if content := gjson.Get(payload, "choices.0.delta.content"); content.Exists() && content.String() != "" {
		return true
	}
	if reasoning := gjson.Get(payload, "choices.0.delta.reasoning_content"); reasoning.Exists() && reasoning.String() != "" {
		return true
	}
	if toolCalls := gjson.Get(payload, "choices.0.delta.tool_calls"); toolCalls.IsArray() && len(toolCalls.Array()) > 0 {
		return true
	}
	return false
}

// rewriteOpenAIStreamContinuationIDs 把续写 chunk 的 id/created/model 改回首条流的
// 取值，让客户端仍认为这些增量属于同一个 completion。
func rewriteOpenAIStreamContinuationIDs(payload, id, model string, created int64) string {
	if id != "" {
		if updated, err := sjson.Set(payload, "id", id); err == nil {
			payload = updated
		}
	}
	if model != "" {
		if updated, err := sjson.Set(payload, "model", model); err == nil {
			payload = updated
		}
	}
	if created > 0 {
		if updated, err := sjson.Set(payload, "created", created); err == nil {
			payload = updated
		}
	}
	return payload
}

// resumeRawChatCompletions 对「已下发语义内容后被截断」的 raw Chat Completions
// 流做一次自动续写，把新流的内容增量接着写进同一个 SSE 响应。返回 true 表示
// 续写成功（客户端已收到 [DONE]），调用方应把本次请求当作成功收尾；false 表示
// 续写失败，调用方沿用原本的截断/502 处理。
func (s *OpenAIGatewayService) resumeRawChatCompletions(
	c *gin.Context,
	send func(prefix string) (*http.Response, error),
	prefix string,
	id string,
	model string,
	created int64,
	usage *OpenAIUsage,
) bool {
	resp, err := send(prefix)
	if err != nil || resp == nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return false
	}

	scanner := s.newUpstreamSSEScanner(resp.Body)
	sawDone := false
	for scanner.Scan() {
		payload, ok := extractOpenAISSEDataLine(scanner.Text())
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(payload)
		if trimmed == "" {
			continue
		}
		if trimmed == "[DONE]" {
			sawDone = true
			break
		}
		if u := extractCCStreamUsage(trimmed); u != nil {
			addOpenAIUsage(usage, *u)
		}
		if !openAIStreamContinuationDelta(trimmed) {
			continue
		}
		line := rewriteOpenAIStreamContinuationIDs(trimmed, id, model, created)
		if _, werr := c.Writer.WriteString("data: " + line + "\n\n"); werr != nil {
			// 客户端已经断开：不必再补错误帧，按成功收尾（计费仍按已收集 usage）。
			return true
		}
		c.Writer.Flush()
	}
	if scanner.Err() != nil {
		return false
	}
	if !sawDone {
		return false
	}
	if _, err := c.Writer.WriteString("data: [DONE]\n\n"); err != nil {
		return true
	}
	c.Writer.Flush()
	return true
}

// logOpenAIStreamContinuation 统一记录续写结果，便于在 Ops 里区分「续写成功」
// 与「续写失败后仍 502」两种线上形态。
func logOpenAIStreamContinuation(logPrefix, requestID string, account *Account, upstreamModel string, resumed bool) {
	fields := []zap.Field{
		zap.String("request_id", requestID),
		zap.String("upstream_model", upstreamModel),
		zap.Bool("resumed", resumed),
	}
	if account != nil {
		fields = append(fields, zap.Int64("account_id", account.ID))
	}
	logger.L().Info(logPrefix+": truncated stream continuation attempted", fields...)
}
