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
		case service.ModelCapabilityImage:
			if !rewriteChatRequestForMedia(c) {
				return
			}
			runMediaAsChat(c, h.MediaGateway.CreateImages, model)
		case service.ModelCapabilityVideo, service.ModelCapabilityAudio:
			if !rewriteChatRequestForMedia(c) {
				return
			}
			runMediaAsChat(c, h.MediaGateway.Create, model)
		default:
			textHandler(c)
		}
	}
}

// rewriteChatRequestForMedia maps the last user text to the prompt field used
// by the unified media handlers. Chat clients send messages, while media
// adapters consume prompt; without this bridge the request is submitted with
// an empty prompt.
func rewriteChatRequestForMedia(c *gin.Context) bool {
	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil || len(body) == 0 {
		writeUnifiedError(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return false
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		writeUnifiedError(c, http.StatusBadRequest, "invalid_request_error", "Invalid JSON body")
		return false
	}
	// 媒体模型不支持流式：runMediaAsChat 恒返回非流式 chat.completion，
	// 而 stream=true 的客户端会按 SSE 逐行解析 JSON，直接报错。与其给一个
	// 客户端解析不了的格式，不如明确拒绝，提示关掉 stream。
	if stream, _ := payload["stream"].(bool); stream {
		writeUnifiedError(c, http.StatusBadRequest, "invalid_request_error",
			"streaming is not supported for media (image/video/audio) models; set stream=false")
		return false
	}

	changed := false
	// prompt：chat 客户端把提示词放在 messages 里，媒体适配器只认 prompt 字段。
	if prompt, ok := payload["prompt"].(string); !ok || strings.TrimSpace(prompt) == "" {
		if prompt := lastUserMessageText(payload["messages"]); prompt != "" {
			payload["prompt"] = prompt
			changed = true
		}
	}
	// 图生图：chat 客户端把参考图放在 message content 的 image_url parts，
	// 媒体链路适配器只认 image 字段。
	//
	// 两个必须注意的点：
	//  1. 提取不能依赖 prompt 是否为空——用户完全可以只发图不发文字，
	//     那时同样要带上参考图，否则静默退化成文生图。
	//  2. 用户显式传的 image 字段优先，不能用 messages 里的图覆盖它，
	//     否则用户的显式意图被静默改写。
	if _, exists := payload["image"]; !exists {
		if refs := lastUserMessageImageURLs(payload["messages"]); len(refs) > 0 {
			if len(refs) == 1 {
				payload["image"] = refs[0]
			} else {
				payload["image"] = refs
			}
			changed = true
		}
	}

	if !changed {
		resetRequestBody(c, body)
		return true
	}
	updated, err := json.Marshal(payload)
	if err != nil {
		writeUnifiedError(c, http.StatusBadRequest, "invalid_request_error", "Invalid request body")
		return false
	}
	resetRequestBody(c, updated)
	return true
}

func lastUserMessageText(raw any) string {
	messages, ok := raw.([]any)
	if !ok {
		return ""
	}
	for i := len(messages) - 1; i >= 0; i-- {
		message, ok := messages[i].(map[string]any)
		if !ok || strings.ToLower(strings.TrimSpace(fmt.Sprint(message["role"]))) != "user" {
			continue
		}
		if text := mediaMessageContentText(message["content"]); text != "" {
			return text
		}
	}
	return ""
}

// lastUserMessageImageURLs 提取最后一条 user 消息里的 image_url 参考图地址。
// 与 lastUserMessageText 共用同一"取最后一条 user 消息"的语义，保证图与文来自
// 同一条消息，避免把历史消息里的旧图错配到当前请求。
func lastUserMessageImageURLs(raw any) []string {
	messages, ok := raw.([]any)
	if !ok {
		return nil
	}
	for i := len(messages) - 1; i >= 0; i-- {
		message, ok := messages[i].(map[string]any)
		if !ok || strings.ToLower(strings.TrimSpace(fmt.Sprint(message["role"]))) != "user" {
			continue
		}
		refs := mediaMessageContentImageURLs(message["content"])
		if len(refs) > 0 {
			return refs
		}
	}
	return nil
}

func mediaMessageContentImageURLs(raw any) []string {
	parts, ok := raw.([]any)
	if !ok {
		return nil
	}
	var refs []string
	for _, part := range parts {
		partMap, ok := part.(map[string]any)
		if !ok || strings.ToLower(strings.TrimSpace(fmt.Sprint(partMap["type"]))) != "image_url" {
			continue
		}
		imageURL, ok := partMap["image_url"].(map[string]any)
		if !ok {
			continue
		}
		url, _ := imageURL["url"].(string)
		if strings.TrimSpace(url) != "" {
			refs = append(refs, strings.TrimSpace(url))
		}
	}
	return refs
}

func mediaMessageContentText(raw any) string {
	if text, ok := raw.(string); ok {
		return strings.TrimSpace(text)
	}
	parts, ok := raw.([]any)
	if !ok {
		return ""
	}
	var text strings.Builder
	for _, part := range parts {
		partMap, ok := part.(map[string]any)
		if !ok || strings.ToLower(strings.TrimSpace(fmt.Sprint(partMap["type"]))) != "text" {
			continue
		}
		if value, ok := partMap["text"].(string); ok && strings.TrimSpace(value) != "" {
			if text.Len() > 0 {
				text.WriteString("\n")
			}
			text.WriteString(strings.TrimSpace(value))
		}
	}
	return text.String()
}

// runMediaAsChat 执行媒体 handler，但把它的响应改写成 chat.completion 结构。
// 媒体链路返回 {id, status, url, urls}，而 chat 客户端期待 choices[].message.content，
// 直接透传会让 OpenAI SDK 抛解析异常。
func runMediaAsChat(c *gin.Context, mediaHandler gin.HandlerFunc, requestedModel string) {
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
	writeChatMediaCompletion(c, capture, requestedModel)
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
//
// 错误体有两种形态：OpenAI 风格 {"error":{"type":"...","message":"..."}} 与
// 纯字符串 {"error":"plain string"}。不能把两个字段都标成 json:"error"——重复
// tag 会让 encoding/json 同时忽略它们，导致这里恒返回空串（已实测确认）。
// 因此先按对象形态解一次，解不出 message 再退回字符串形态。
func (w *chatCompatWriter) ErrorMessage() string {
	var obj struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.buf.Bytes(), &obj); err == nil && obj.Error.Message != "" {
		return obj.Error.Message
	}
	var str struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w.buf.Bytes(), &str); err == nil && str.Error != "" {
		return str.Error
	}
	return ""
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
func writeChatMediaCompletion(c *gin.Context, capture *chatCompatWriter, requestedModel string) {
	var image struct {
		Data []struct {
			URL     string `json:"url"`
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(capture.buf.Bytes(), &image); err == nil && len(image.Data) > 0 {
		urls := make([]string, 0, len(image.Data))
		for _, item := range image.Data {
			if strings.TrimSpace(item.URL) != "" {
				urls = append(urls, item.URL)
			} else if strings.TrimSpace(item.B64JSON) != "" {
				urls = append(urls, "data:image/png;base64,"+strings.TrimSpace(item.B64JSON))
			}
		}
		if len(urls) > 0 {
			content := buildChatMediaContent(mediaTaskPayload{Model: requestedModel, URLs: urls})
			writeChatCompletion(c, "", requestedModel, content)
			return
		}
	}
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

	writeChatCompletion(c, media.ID, defaultString(media.Model, requestedModel), buildChatMediaContent(media))
}

func writeChatCompletion(c *gin.Context, id, model, content string) {
	if strings.TrimSpace(id) == "" {
		id = fmt.Sprintf("media-%d", time.Now().UnixNano())
	}
	c.JSON(http.StatusOK, gin.H{
		"id":      "chatcmpl-" + id,
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []gin.H{{
			"index": 0,
			"message": gin.H{
				"role":    "assistant",
				"content": content,
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
