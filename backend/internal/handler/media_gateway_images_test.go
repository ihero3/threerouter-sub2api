package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newImagesTestContext(method, target string, body []byte, contentType string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(method, target, bytes.NewReader(body))
	c.Request.ContentLength = int64(len(body))
	if contentType != "" {
		c.Request.Header.Set("Content-Type", contentType)
	}
	return c
}

func TestNormalizeOpenAIImageBody_DefaultsModelAndExtractsFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := newImagesTestContext(http.MethodPost, "/v1/images/generations",
		[]byte(`{"prompt":"a cat","response_format":"b64_json"}`), "application/json")

	body, format, status, errMsg := (&MediaGatewayHandler{}).normalizeOpenAIImageBody(c)
	require.NotEmpty(t, body, "err=%s", errMsg)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "b64_json", format)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(body, &decoded))
	require.Equal(t, DefaultImageModel, decoded["model"], "未传 model 时必须补默认图片模型")
	_, exists := decoded["response_format"]
	require.False(t, exists, "response_format 由本层消费，不能透传给上游")
}

func TestNormalizeOpenAIImageBody_KeepsExplicitModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := newImagesTestContext(http.MethodPost, "/v1/images/generations",
		[]byte(`{"model":"minimax-image-01","prompt":"a dog"}`), "application/json")

	body, format, _, errMsg := (&MediaGatewayHandler{}).normalizeOpenAIImageBody(c)
	require.NotEmpty(t, body, "err=%s", errMsg)
	require.Equal(t, "url", format, "未指定时按 OpenAI 默认值 url 处理")

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(body, &decoded))
	require.Equal(t, "minimax-image-01", decoded["model"])
}

func TestNormalizeOpenAIImageBody_RejectsUnknownResponseFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := newImagesTestContext(http.MethodPost, "/v1/images/generations",
		[]byte(`{"model":"qwen-image-3.0","prompt":"a dog","response_format":"xml"}`), "application/json")

	body, format, status, errMsg := (&MediaGatewayHandler{}).normalizeOpenAIImageBody(c)
	require.Nil(t, body)
	require.Empty(t, format)
	require.Equal(t, http.StatusBadRequest, status)
	require.Contains(t, errMsg, "response_format must be url or b64_json")
}

func TestNormalizeOpenAIImageBody_MultipartEditsBecomesDataURI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	require.NoError(t, writer.WriteField("prompt", "换成水墨风格"))
	require.NoError(t, writer.WriteField("model", "qwen-image-3.0"))
	require.NoError(t, writer.WriteField("n", "1"))
	part, err := writer.CreateFormFile("image", "cat.png")
	require.NoError(t, err)
	// 用真实 PNG magic bytes：octet-stream 的 part 必须靠内容嗅探还原出 image/png。
	_, _ = part.Write([]byte("\x89PNG\r\n\x1a\nfake-png-bytes"))
	require.NoError(t, writer.Close())

	c := newImagesTestContext(http.MethodPost, "/v1/images/edits", buf.Bytes(), writer.FormDataContentType())

	body, _, status, errMsg := (&MediaGatewayHandler{}).normalizeOpenAIImageBody(c)
	require.NotEmpty(t, body, "err=%s", errMsg)
	require.Equal(t, http.StatusOK, status)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(body, &decoded))
	require.Equal(t, "qwen-image-3.0", decoded["model"])
	imageRef, _ := decoded["image"].(string)
	require.True(t, strings.HasPrefix(imageRef, "data:image/"), "multipart 上传图必须转成 data URI，got=%q", imageRef)
	require.Contains(t, imageRef, base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\nfake-png-bytes")))
}

func TestNormalizeOpenAIImageBody_RejectsNonImageModelBeforeBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := newImagesTestContext(http.MethodPost, "/v1/images/generations",
		[]byte(`{"model":"seedance-1.0-pro","prompt":"海浪"}`), "application/json")

	body, _, status, errMsg := (&MediaGatewayHandler{}).normalizeOpenAIImageBody(c)
	require.Nil(t, body)
	require.Equal(t, http.StatusBadRequest, status)
	require.Contains(t, errMsg, "video model", "必须在建任务/预扣之前拦掉视频模型")
	require.Contains(t, errMsg, "/v1/videos/generations", "要指明该用哪个端点，而不是笼统说平台不支持")
}

func TestNormalizeOpenAIImageBody_RejectsUnknownModelBeforeBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// 文本模型打到生图端点：MediaKindFromModel 对未知模型默认返回 video，
	// 这里必须拦在计费之前，且不能谎称"它是视频模型"。
	c := newImagesTestContext(http.MethodPost, "/v1/images/generations",
		[]byte(`{"model":"gpt-4o-mini","prompt":"hello"}`), "application/json")

	body, _, status, errMsg := (&MediaGatewayHandler{}).normalizeOpenAIImageBody(c)
	require.Nil(t, body)
	require.Equal(t, http.StatusBadRequest, status)
	require.Contains(t, errMsg, "not an image model")
	require.Contains(t, errMsg, DefaultImageModel, "要提示可以省略 model 用默认模型")
	require.NotContains(t, errMsg, "video model", "文本模型不是视频模型，不能指错端点")
}

func TestBase64EncodedSize(t *testing.T) {
	require.Zero(t, base64EncodedSize(0))
	require.Equal(t, 4, base64EncodedSize(3))
	require.Equal(t, 8, base64EncodedSize(4), "不足 3 字节的部分也会补到 4 的倍数")
	// 预算扣减必须按编码后体积算：6MB 原始字节编码后是 8MB。
	require.Equal(t, 8<<20, base64EncodedSize(6<<20))
}

func TestOpenAIImageBodyFromMultipart_RejectsOversizedInline(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// 未配置对象存储时图片只能内联成 data URI，编码后体积约为原始的 4/3。
	// 按原始字节算预算会低估占用，最终撞上请求体读取上限报出无关错误，
	// 所以这里必须用"编码后超过 6MB"的图来验证预算真的按编码体积扣。
	oversized := make([]byte, 5<<20) // 5MB 原始 -> 6.7MB 编码，超出 6MB 预算
	for i := range oversized {
		oversized[i] = byte(i % 251)
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	require.NoError(t, writer.WriteField("model", "qwen-image-3.0"))
	part, err := writer.CreateFormFile("image", "big.png")
	require.NoError(t, err)
	_, _ = part.Write(oversized)
	require.NoError(t, writer.Close())

	c := newImagesTestContext(http.MethodPost, "/v1/images/edits", buf.Bytes(), writer.FormDataContentType())
	_, _, status, errMsg := (&MediaGatewayHandler{}).normalizeOpenAIImageBody(c)
	require.Equal(t, http.StatusBadRequest, status)
	require.Contains(t, errMsg, "过大", "超预算必须给出与图片体积相关的明确错误，got=%q", errMsg)
}

func TestOpenAIImageBodyFromMultipart_RejectsTooManyFiles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	require.NoError(t, writer.WriteField("model", "qwen-image-3.0"))
	// maxUploadImageFiles = 4，传 5 张应被明确拒绝。
	for i := 0; i < 5; i++ {
		part, err := writer.CreateFormFile("image[]", fmt.Sprintf("img%d.png", i))
		require.NoError(t, err)
		_, _ = part.Write([]byte("tiny"))
	}
	require.NoError(t, writer.Close())

	c := newImagesTestContext(http.MethodPost, "/v1/images/edits", buf.Bytes(), writer.FormDataContentType())
	_, _, status, errMsg := (&MediaGatewayHandler{}).normalizeOpenAIImageBody(c)
	require.Equal(t, http.StatusBadRequest, status)
	require.Contains(t, errMsg, "最多支持一次上传")
}

func TestBuildOpenAIImageData_URLAndBase64(t *testing.T) {
	urlItem, err := buildOpenAIImageData(context.Background(), "https://example.com/a.png", false)
	require.NoError(t, err)
	require.Equal(t, "https://example.com/a.png", urlItem["url"])
	require.NotContains(t, urlItem, "b64_json")

	// data URI（上游直接回 base64 的情况）应剥掉前缀，只留载荷。
	b64Item, err := buildOpenAIImageData(context.Background(),
		"data:image/png;base64,"+base64.StdEncoding.EncodeToString([]byte("png-bytes")), true)
	require.NoError(t, err)
	require.Equal(t, base64.StdEncoding.EncodeToString([]byte("png-bytes")), b64Item["b64_json"])
	require.NotContains(t, b64Item, "url")
}

func TestBuildOpenAIImageData_Base64RejectsNonHTTP(t *testing.T) {
	_, err := buildOpenAIImageData(context.Background(), "file:///tmp/a.png", true)
	require.Error(t, err)
}

func TestCollectImageURLs_PrefersFullList(t *testing.T) {
	single := &service.MediaTaskRecord{MediaURL: "https://example.com/1.png"}
	require.Equal(t, []string{"https://example.com/1.png"}, collectImageURLs(single))

	multi := &service.MediaTaskRecord{
		MediaURL:  "https://example.com/1.png",
		MediaURLs: []string{"https://example.com/1.png", "https://example.com/2.png"},
	}
	require.Len(t, collectImageURLs(multi), 2)

	require.Empty(t, collectImageURLs(&service.MediaTaskRecord{}))
}

func TestParseMediaWaitParam(t *testing.T) {
	require.Zero(t, parseMediaWaitParam("", 0))
	require.Equal(t, 90*time.Second, parseMediaWaitParam("", 90*time.Second))
	require.Equal(t, 30*time.Second, parseMediaWaitParam("30", 0))
	require.Equal(t, service.MaxMediaTaskWait, parseMediaWaitParam("9999", 0), "超长等待必须被截断到上限")
	require.Equal(t, 45*time.Second, parseMediaWaitParam("oops", 45*time.Second))
	require.Zero(t, parseMediaWaitParam("0", 90*time.Second), "wait=0 是显式的不等待")
}

func TestMediaTaskIDParam_AcceptsVideoRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := newImagesTestContext(http.MethodGet, "/v1/videos/vid_abc", nil, "")
	c.Params = gin.Params{{Key: "request_id", Value: "vid_abc"}}
	require.Equal(t, "vid_abc", mediaTaskIDParam(c),
		"视频端点的路径参数是 request_id，媒体 handler 必须认，否则收敛后查不到任务")
}

func TestMediaNoAvailableAccountMessage(t *testing.T) {
	msg := mediaNoAvailableAccountMessage("qwen-image-3.0")
	require.Contains(t, msg, "qwen-image-3.0", "错误信息必须带模型名，否则无法排障")
	require.Contains(t, msg, "composite")
}
