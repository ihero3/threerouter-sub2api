package routes

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestLastUserMessageTextUsesLatestTextMessage(t *testing.T) {
	messages := []any{
		map[string]any{"role": "user", "content": "old prompt"},
		map[string]any{"role": "assistant", "content": "ignored"},
		map[string]any{
			"role": "user",
			"content": []any{
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/ref.png"}},
				map[string]any{"type": "text", "text": "draw this as a watercolor"},
			},
		},
	}

	require.Equal(t, "draw this as a watercolor", lastUserMessageText(messages))
}

func TestLastUserMessageImageURLsCollectsReferenceImages(t *testing.T) {
	messages := []any{
		map[string]any{"role": "user", "content": "first"},
		map[string]any{
			"role": "user",
			"content": []any{
				map[string]any{"type": "text", "text": "edit this"},
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/a.png"}},
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/b.png"}},
			},
		},
	}

	refs := lastUserMessageImageURLs(messages)
	require.Len(t, refs, 2)
	require.Equal(t, "https://example.com/a.png", refs[0])
	require.Equal(t, "https://example.com/b.png", refs[1])
}

func TestMediaMessageContentImageURLsIgnoresNonImageParts(t *testing.T) {
	parts := []any{
		map[string]any{"type": "text", "text": "hi"},
		map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/c.png"}},
	}
	require.Equal(t, []string{"https://example.com/c.png"}, mediaMessageContentImageURLs(parts))
	require.Nil(t, mediaMessageContentImageURLs("not-a-list"))
}

func TestRewriteChatRequestForMediaBridgesReferenceImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"model":"qwen-image-3.0","messages":[{"role":"user","content":[
		{"type":"image_url","image_url":{"url":"https://example.com/ref.png"}},
		{"type":"text","text":"recolor this"}
	]}]}`
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	context.Request.Header.Set("Content-Type", "application/json")

	require.True(t, rewriteChatRequestForMedia(context))

	// 改写后的 body 必须带上 prompt 与 image 字段。
	rewritten, _ := io.ReadAll(context.Request.Body)
	require.Contains(t, string(rewritten), `"prompt":"recolor this"`)
	require.Contains(t, string(rewritten), `"image":"https://example.com/ref.png"`)
}

func TestRewriteChatRequestForMediaBridgesImageOnlyRequest(t *testing.T) {
	// 用户只发图片不发文字（纯图生图）时，prompt 为空但参考图必须保留，
	// 否则会静默退化成文生图，出图与用户预期完全不符。
	gin.SetMode(gin.TestMode)
	body := `{"model":"qwen-image-3.0","messages":[{"role":"user","content":[
		{"type":"image_url","image_url":{"url":"https://example.com/only.png"}}
	]}]}`
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	context.Request.Header.Set("Content-Type", "application/json")

	require.True(t, rewriteChatRequestForMedia(context))

	rewritten, _ := io.ReadAll(context.Request.Body)
	require.Contains(t, string(rewritten), `"image":"https://example.com/only.png"`)
}

func TestRewriteChatRequestForMediaKeepsExplicitImageField(t *testing.T) {
	// 用户显式传的 image 字段优先：不能用 messages 里的图覆盖它，
	// 否则用户的显式意图被静默改写。
	gin.SetMode(gin.TestMode)
	body := `{"model":"qwen-image-3.0","image":"https://example.com/explicit.png","messages":[{"role":"user","content":[
		{"type":"text","text":"edit"},
		{"type":"image_url","image_url":{"url":"https://example.com/from-msg.png"}}
	]}]}`
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	context.Request.Header.Set("Content-Type", "application/json")

	require.True(t, rewriteChatRequestForMedia(context))

	rewritten, _ := io.ReadAll(context.Request.Body)
	require.Contains(t, string(rewritten), `"image":"https://example.com/explicit.png"`)
	require.NotContains(t, string(rewritten), `"image":"https://example.com/from-msg.png"`)
}

func TestRewriteChatRequestForMediaRejectsStreaming(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{"model":"qwen-image-3.0","stream":true,"prompt":"a cat"}`
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	context.Request.Header.Set("Content-Type", "application/json")

	require.False(t, rewriteChatRequestForMedia(context))
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "streaming is not supported")
}

func TestChatCompatWriterErrorMessageExtractsBothShapes(t *testing.T) {
	objectShape := &chatCompatWriter{buf: *bytes.NewBufferString(`{"error":{"type":"upstream_error","message":"quota exhausted"}}`)}
	require.Equal(t, "quota exhausted", objectShape.ErrorMessage())

	stringShape := &chatCompatWriter{buf: *bytes.NewBufferString(`{"error":"plain failure"}`)}
	require.Equal(t, "plain failure", stringShape.ErrorMessage())

	empty := &chatCompatWriter{buf: *bytes.NewBufferString(`{"foo":"bar"}`)}
	require.Equal(t, "", empty.ErrorMessage())
}

func TestWriteChatMediaCompletionWrapsOpenAIImageResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	capture := &chatCompatWriter{
		ResponseWriter: context.Writer,
		buf:            *bytes.NewBufferString(`{"created":1,"data":[{"url":"https://example.com/a.png"},{"url":"https://example.com/b.png"}]}`),
	}

	writeChatMediaCompletion(context, capture, "qwen-image-3.0")

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "![result-1](https://example.com/a.png)")
	require.Contains(t, recorder.Body.String(), "![result-2](https://example.com/b.png)")
	require.Contains(t, recorder.Body.String(), `"model":"qwen-image-3.0"`)
}

func TestWriteChatMediaCompletionWrapsBase64ImageResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	capture := &chatCompatWriter{
		ResponseWriter: context.Writer,
		buf:            *bytes.NewBufferString(`{"created":1,"data":[{"b64_json":"cG5n"}]}`),
	}

	writeChatMediaCompletion(context, capture, "qwen-image-3.0")

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "data:image/png;base64,cG5n")
}
