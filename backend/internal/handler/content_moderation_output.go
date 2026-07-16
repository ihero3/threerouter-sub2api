package handler

import (
	"bufio"
	"bytes"
	"strings"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// maxCapturedOutputBytes 限制事后审核捕获的响应体大小，避免长响应占用过多内存。
// 超过后停止累积（审核仅需前若干 KB 文本足以判定风险）。
const maxCapturedOutputBytes = 256 * 1024

// outputCaptureWriter 包装 gin.ResponseWriter，把写入客户端的字节同时旁路到一个有界缓冲区，
// 用于响应结束后对模型输出做异步事后审核（合规方案 0.4）。不改变正常写入行为。
type outputCaptureWriter struct {
	gin.ResponseWriter
	buf       bytes.Buffer
	truncated bool
}

func (w *outputCaptureWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	if n > 0 && !w.truncated {
		remaining := maxCapturedOutputBytes - w.buf.Len()
		if remaining > 0 {
			chunk := data[:n]
			if len(chunk) > remaining {
				chunk = chunk[:remaining]
				w.truncated = true
			}
			w.buf.Write(chunk)
		} else {
			w.truncated = true
		}
	}
	return n, err
}

func (w *outputCaptureWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *outputCaptureWriter) captured() []byte {
	return w.buf.Bytes()
}

// extractChatCompletionsOutputText 从 OpenAI Chat Completions 响应体中提取助手输出文本。
// 同时兼容流式（SSE，多行 data: chunk，聚合 choices[].delta.content）与非流式（单个 JSON，choices[].message.content）。
func extractChatCompletionsOutputText(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	trimmed := bytes.TrimSpace(body)
	// 非流式：整体是一个 JSON 对象。
	if len(trimmed) > 0 && trimmed[0] == '{' && !bytes.Contains(trimmed, []byte("data:")) {
		return aggregateChatCompletionsMessageContent(trimmed)
	}
	return aggregateChatCompletionsStreamContent(body)
}

func aggregateChatCompletionsMessageContent(jsonBody []byte) string {
	var sb strings.Builder
	gjson.GetBytes(jsonBody, "choices").ForEach(func(_, choice gjson.Result) bool {
		if content := choice.Get("message.content"); content.Exists() {
			sb.WriteString(content.String())
		}
		return true
	})
	return strings.TrimSpace(sb.String())
}

func aggregateChatCompletionsStreamContent(body []byte) string {
	var sb strings.Builder
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64*1024), maxCapturedOutputBytes)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		gjson.Get(payload, "choices").ForEach(func(_, choice gjson.Result) bool {
			if content := choice.Get("delta.content"); content.Exists() {
				sb.WriteString(content.String())
			}
			return true
		})
	}
	return strings.TrimSpace(sb.String())
}

// postModerateCapturedOutput 在响应结束后，从捕获的响应体中提取助手输出并触发异步事后审核。
// 该调用永不阻断（响应已发送完毕），仅记录/计入违规。
func (h *GatewayHandler) postModerateCapturedOutput(c *gin.Context, cw *outputCaptureWriter, apiKey *service.APIKey, subject middleware2.AuthSubject, model string, requestBody []byte) {
	if h == nil || h.contentModerationService == nil || cw == nil {
		return
	}
	output := extractChatCompletionsOutputText(cw.captured())
	if output == "" {
		return
	}
	h.postModerateOutput(c, apiKey, subject, service.ContentModerationProtocolOpenAIChat, model, requestBody, output)
}
