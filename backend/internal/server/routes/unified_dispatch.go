package routes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// unified_dispatch.go — 统一入口：客户端只给 base_url + api_key + model，
// 由服务端按模型名判定模态并分派到对应链路。
//
// 设计约束：
//  1. 不改动任何既有链路的内部实现，只在路由层做分派与响应适配；
//  2. 判定顺序与 MediaKindFromModel 一致（图片优先于视频），避免误判；
//  3. 无法识别的模型一律按文本处理——文本是默认链路，让上游给出权威错误信息
//     比本地猜测更安全。

// dispatchModelFromBody 读取并还原请求体，返回其中的 model。
// 复用 compositeRequestModelFromBody 以兼容 multipart（图片 edits 场景）。
func dispatchModelFromBody(c *gin.Context) string {
	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil || len(body) == 0 {
		return ""
	}
	model := compositeRequestModelFromBody(c.GetHeader("Content-Type"), body)
	resetRequestBody(c, body)
	return model
}

// generationsHandler 是 /v1/generations 万能入口：接受任意模型，
// 按模型能力分派到文本或统一媒体链路。
// textHandler 由注册方传入（chat 链路本身要按分组平台二次分派）。
func generationsHandler(h *handler.Handlers, textHandler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		model := dispatchModelFromBody(c)
		if strings.TrimSpace(model) == "" {
			writeUnifiedError(c, http.StatusBadRequest, "invalid_request_error", "model is required")
			return
		}
		switch service.DispatchModelCapability(model) {
		case service.ModelCapabilityImage, service.ModelCapabilityVideo, service.ModelCapabilityAudio:
			h.MediaGateway.Create(c)
		default:
			textHandler(c)
		}
	}
}

// chatCompletionsWithDispatch 包装 chat 入口：收到媒体类模型时转到对应链路，
// 并把结果包装成 chat 兼容格式，避免 OpenAI SDK 解析崩溃。
func chatCompletionsWithDispatch(h *handler.Handlers, textHandler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		model := dispatchModelFromBody(c)
		switch service.DispatchModelCapability(model) {
		case service.ModelCapabilityImage, service.ModelCapabilityVideo, service.ModelCapabilityAudio:
			runMediaAsChat(c, h.MediaGateway.Create)
		default:
			textHandler(c)
		}
	}
}

// runMediaAsChat 执行媒体 handler，但把它的响应改写成 chat.completion 结构。
// 媒体链路返回 {id, status, url, urls}，而 chat 客户端期待 choices[].message.content，
// 直接透传会让 OpenAI SDK 抛解析异常。
func runMediaAsChat(c *gin.Context, mediaHandler gin.HandlerFunc) {
	capture := &chatCompatWriter{ResponseWriter: c.Writer}
	c.Writer = capture
	mediaHandler(c)
	c.Writer = capture.ResponseWriter

	status := capture.Status()
	if status == 0 {
		status = http.StatusOK
	}
	// 上游失败：转成 OpenAI 错误结构，让 SDK 抛出可读异常。
	if status >= 400 {
		message := strings.TrimSpace(capture.ErrorMessage())
		if message == "" {
			message = http.StatusText(status)
		}
		writeUnifiedError(c, status, "upstream_error", message)
		return
	}
	writeChatMediaCompletion(c, capture)
}

// chatCompatWriter 缓冲下游响应体，便于改写后再发给客户端。
// 只覆盖写相关方法，其余沿用被包装的 ResponseWriter。
type chatCompatWriter struct {
	gin.ResponseWriter
	buf    bytes.Buffer
	status int
	size   int
}

// Write / WriteString 只入缓冲，不落到底层连接。
func (w *chatCompatWriter) Write(b []byte) (int, error) {
	w.size += len(b)
	return w.buf.Write(b)
}

func (w *chatCompatWriter) WriteString(s string) (int, error) {
	w.size += len(s)
	return w.buf.WriteString(s)
}

// WriteHeader / WriteHeaderNow 必须拦住：一旦真实写出状态码，
// 改写后的响应就无法再设置自己的状态码，gin 也会报 headers already written。
func (w *chatCompatWriter) WriteHeader(code int) { w.status = code }
func (w *chatCompatWriter) WriteHeaderNow()      {}

func (w *chatCompatWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *chatCompatWriter) Size() int     { return w.size }
func (w *chatCompatWriter) Written() bool { return w.buf.Len() > 0 }

// ErrorMessage 从缓冲的错误响应里提取 message 字段。
func (w *chatCompatWriter) ErrorMessage() string {
	var payload struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		ErrorString string `json:"error"`
	}
	if err := json.Unmarshal(w.buf.Bytes(), &payload); err != nil {
		return ""
	}
	if payload.Error.Message != "" {
		return payload.Error.Message
	}
	return payload.ErrorString
}

// mediaTaskPayload 是 /v1/media 创建响应的结构（与 handler 内 mediaTaskResponse 对齐）。
type mediaTaskPayload struct {
	ID           string   `json:"id"`
	Status       string   `json:"status"`
	Model        string   `json:"model"`
	URL          string   `json:"url"`
	URLs         []string `json:"urls"`
	ThumbnailURL string   `json:"thumbnail_url"`
	Resolution   string   `json:"resolution"`
	DurationSec  int      `json:"duration_sec"`
	Error        string   `json:"error"`
}

// writeChatMediaCompletion 把媒体任务结果包装成 chat.completion 响应。
func writeChatMediaCompletion(c *gin.Context, capture *chatCompatWriter) {
	var media mediaTaskPayload
	if err := json.Unmarshal(capture.buf.Bytes(), &media); err != nil {
		// 非预期结构时不猜测，交给客户端看原始错误。
		writeUnifiedError(c, http.StatusBadGateway, "upstream_error", "unrecognized media response")
		return
	}
	if msg := strings.TrimSpace(media.Error); msg != "" {
		writeUnifiedError(c, http.StatusBadGateway, "upstream_error", msg)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      "chatcmpl-" + media.ID,
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   media.Model,
		"choices": []gin.H{{
			"index": 0,
			"message": gin.H{
				"role":    "assistant",
				"content": buildChatMediaContent(media),
			},
			"finish_reason": "stop",
		}},
		"usage": gin.H{
			"prompt_tokens":     0,
			"completion_tokens": 0,
			"total_tokens":      0,
		},
	})
}

// buildChatMediaContent 把媒体结果渲染成 Markdown 图片 + 元信息，
// 保证任何 chat 客户端都能直接展示，同时保留程序可解析的 URL。
func buildChatMediaContent(media mediaTaskPayload) string {
	urls := media.URLs
	if len(urls) == 0 && media.URL != "" {
		urls = []string{media.URL}
	}
	if len(urls) == 0 {
		// 异步任务（视频为主）还没出结果，给出可轮询的线索。
		return fmt.Sprintf(
			"任务已提交，当前状态：%s。任务 ID：%s（可用 GET /v1/media/%s 查询结果）。",
			defaultString(media.Status, "processing"), media.ID, media.ID,
		)
	}
	var b strings.Builder
	for i, u := range urls {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(fmt.Sprintf("![result-%d](%s)", i+1, u))
	}
	if media.Resolution != "" {
		b.WriteString(fmt.Sprintf("\n\n分辨率：%s", media.Resolution))
	}
	if media.DurationSec > 0 {
		b.WriteString(fmt.Sprintf("\n时长：%d 秒", media.DurationSec))
	}
	return b.String()
}

func writeUnifiedError(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"message": message,
			"type":    errType,
			"code":    errType,
		},
	})
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
