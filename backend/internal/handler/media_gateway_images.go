package handler

// media_gateway_images.go — OpenAI 兼容的生图端点适配层。
//
// 关键点：/v1/images/generations 与 /v1/images/edits 在 OpenAI 语义下是
// **标准端点**，客户端（官方 SDK、Dify、LangChain、Chatbox…）会用 pydantic
// 强校验响应体必须含 created + data[].{url|b64_json}。统一媒体链路返回的是
// 自研的 {id, status, url, created_at} 任务结构，直接透传会让 SDK 抛
// ValidationError。本文件负责把统一链路的结果改写成 OpenAI ImagesResponse，
// 让"base_url + api_key + model"这一套配置在任何分组下表现一致。

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	// DefaultImageModel 是生图端点未传 model 时使用的默认模型。
	// 设计目标：客户端只配 base_url + api_key 就能出图，不必先查模型清单。
	DefaultImageModel = "qwen-image-3.0"

	// defaultImageSyncWait 是生图端点等待任务终态的默认时长。
	// OpenAI 的 images 端点是同步语义，绝大多数图片 adapter 创建即返回 URL；
	// 这个等待只为兜住少数返回 task_id 的异步图片上游。
	defaultImageSyncWait = 120 * time.Second

	// maxUploadImageBytes 是 /images/edits 单张上传图的体积上限（原始字节）。
	maxUploadImageBytes = 20 << 20

	// maxInlineUploadBytes 是未配置对象存储时，允许内联进请求体的
	// **base64 编码后**的总字节数（原始字节 × 4/3）。
	//
	// 上限来自下游 readMediaRequestBody 的 10MB 读取限制：编码后的图片要连同
	// model / prompt / 其他字段一起塞进同一个 JSON，必须给它们留出余量，
	// 否则客户端会收到一句与图片完全无关的 "Failed to read request body"。
	maxInlineUploadBytes = 6 << 20

	// maxUploadImageFiles 是 /images/edits multipart 单次可上传的图片文件数上限。
	// 千问 n 上限 6、MiniMax 上限 9，取保守值 4 已覆盖绝大多数图生图场景；
	// 不设上限时，开启对象存储的部署可被恶意客户端一次塞入海量文件，
	// 直接打穿存储容量与出口带宽。
	maxUploadImageFiles = 4

	// maxImageBase64Bytes 是 b64_json 转码时允许下载的最大字节数。
	maxImageBase64Bytes = 20 << 20

	// imageBase64Timeout 是单张图片下载转码的超时。
	imageBase64Timeout = 60 * time.Second
)

// CreateImages 是 /v1/images/generations 与 /v1/images/edits 的统一实现。
//
// 与 Create（/v1/media/generations）共享鉴权、权限与额度校验，差别只有最后一步：
//   - 请求侧：接受 multipart（edits）、补默认 model、解析 response_format
//   - 响应侧：{created, data:[{url|b64_json}]}，成功回 200，失败回 OpenAI 错误结构
func (h *MediaGatewayHandler) CreateImages(c *gin.Context) {
	bodyBytes, responseFormat, status, errMessage := h.normalizeOpenAIImageBody(c)
	if bodyBytes == nil {
		mediaErrorResponse(c, status, "invalid_request_error", errMessage)
		return
	}
	// 写回归一化后的 JSON：后续 createMediaTask 与 MediaTaskService 都按 JSON 解析，
	// multipart 的上传图已转成 data URI 放进 image 字段。
	resetMediaRequestBody(c, bodyBytes)
	// multipart 已被转成 JSON，Content-Type 必须跟着改：留下 multipart 会让
	// body 与声明类型不一致，任何按 Content-Type 分派的下游/中间件都会读错。
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(c.GetHeader("Content-Type"))), "multipart/") {
		c.Request.Header.Set("Content-Type", "application/json")
	}

	outcome := h.createMediaTask(c)
	if outcome.record == nil {
		mediaErrorResponse(c, outcome.status, outcome.errType, outcome.message)
		return
	}
	if outcome.replayed {
		c.Header("X-Idempotency-Replayed", "true")
	}
	record := outcome.record
	if record.MediaKind != "" && record.MediaKind != service.MediaKindImage {
		mediaErrorResponse(c, http.StatusBadRequest, "invalid_request_error",
			"model "+record.PublicModel+" is not an image model; use /v1/videos/generations for video")
		return
	}

	// OpenAI 的 images 端点是同步语义：未到终态时等待，超时再交给客户端轮询。
	if !service.IsMediaTaskTerminal(record.Status) {
		record = h.awaitMediaTask(c, record, defaultImageSyncWait)
	}
	if record.Status == "failed" {
		h.logger.Warn("media_gateway.image_task_failed",
			zap.String("local_id", record.LocalID),
			zap.String("model", record.PublicModel),
			zap.String("error", record.ErrorMessage),
		)
		message := strings.TrimSpace(record.ErrorMessage)
		if message == "" {
			message = "Image generation failed"
		}
		mediaErrorResponse(c, http.StatusBadGateway, "upstream_error", message)
		return
	}
	if record.Status == "cancelled" {
		// 取消是"这次调用被终止"，不是上游故障：回 502 会让客户端以为是上游挂了
		// 从而盲目重试，其实重新提交一次即可。
		mediaErrorResponse(c, http.StatusConflict, "task_cancelled",
			"Image generation was cancelled: task "+record.LocalID+" (resubmit to retry)")
		return
	}
	if record.Status == "processing" {
		// 等待窗口内仍未出图：明确告诉客户端任务在哪，而不是回一个空 data。
		mediaErrorResponse(c, http.StatusGatewayTimeout, "timeout_error",
			"Image generation is still processing: task "+record.LocalID+" (poll GET /v1/media/"+record.LocalID+")")
		return
	}

	urls := collectImageURLs(record)
	if len(urls) == 0 {
		h.logger.Warn("media_gateway.image_task_without_url",
			zap.String("local_id", record.LocalID),
			zap.String("model", record.PublicModel),
		)
		mediaErrorResponse(c, http.StatusBadGateway, "upstream_error",
			"Image generation succeeded but no image url was returned by upstream")
		return
	}

	wantBase64 := responseFormat == "b64_json"
	data := make([]gin.H, 0, len(urls))
	for _, u := range urls {
		item, err := buildOpenAIImageData(c.Request.Context(), u, wantBase64)
		if err != nil {
			h.logger.Warn("media_gateway.image_payload_failed",
				zap.String("local_id", record.LocalID),
				zap.Bool("b64_json", wantBase64),
				zap.Error(err),
			)
			mediaErrorResponse(c, http.StatusBadGateway, "upstream_error",
				"Failed to produce "+responseFormat+" payload: "+err.Error())
			return
		}
		data = append(data, item)
	}

	h.logger.Info("media_gateway.image_served",
		zap.String("local_id", record.LocalID),
		zap.String("model", record.PublicModel),
		zap.String("response_format", responseFormat),
		zap.Int("images", len(data)),
	)

	created := record.CreatedAt
	if created.IsZero() {
		created = time.Now()
	}
	c.JSON(http.StatusOK, gin.H{
		"created": created.Unix(),
		"data":    data,
	})
}

// normalizeOpenAIImageBody 把 images 端点的请求归一成统一媒体链路的 JSON body，
// 并返回客户端要求的 response_format。
// 返回值：(body, responseFormat, httpStatus, errorMessage)；body 为 nil 表示失败。
func (h *MediaGatewayHandler) normalizeOpenAIImageBody(c *gin.Context) ([]byte, string, int, string) {
	contentType := strings.TrimSpace(c.GetHeader("Content-Type"))
	var body map[string]any

	if mediaType, _, err := mime.ParseMediaType(contentType); err == nil && strings.EqualFold(mediaType, "multipart/form-data") {
		parsed, err := h.openAIImageBodyFromMultipart(c)
		if err != nil {
			return nil, "", http.StatusBadRequest, err.Error()
		}
		body = parsed
	} else {
		raw, err := readMediaRequestBody(c)
		if err != nil {
			return nil, "", http.StatusBadRequest, "Failed to read request body"
		}
		if len(raw) == 0 {
			return nil, "", http.StatusBadRequest, "Request body is empty"
		}
		body = map[string]any{}
		if err := json.Unmarshal(raw, &body); err != nil {
			return nil, "", http.StatusBadRequest, "Invalid JSON body"
		}
	}

	// 默认模型：客户端只配 base_url + api_key 也能直接出图。
	if model, _ := body["model"].(string); strings.TrimSpace(model) == "" {
		body["model"] = DefaultImageModel
	}
	// 在计费之前拦掉非图片模型：否则会在视频/音频链路上真的建一次任务并预扣费用。
	if modelStr := fmt.Sprint(body["model"]); service.MediaKindFromModel(modelStr, body) != service.MediaKindImage {
		if service.IsKnownVideoVendorModel(modelStr) {
			return nil, "", http.StatusBadRequest, fmt.Sprintf(
				"model %s is a video model; submit it to POST /v1/videos/generations instead", modelStr)
		}
		return nil, "", http.StatusBadRequest, fmt.Sprintf(
			"model %s is not an image model; check the model name or omit it to use the default %s",
			modelStr, DefaultImageModel)
	}
	// response_format 由本层消费（决定返回 url 还是 b64_json），不能透传给上游：
	// 各家图片 API 的这个字段语义不一，原样下发会被当成未知参数拒绝。
	responseFormat := ""
	if v, ok := body["response_format"].(string); ok {
		responseFormat = strings.ToLower(strings.TrimSpace(v))
	}
	delete(body, "response_format")

	if responseFormat == "" {
		responseFormat = "url"
	}
	if responseFormat != "url" && responseFormat != "b64_json" {
		return nil, "", http.StatusBadRequest, fmt.Sprintf(
			"response_format must be url or b64_json, got %q", responseFormat)
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, "", http.StatusBadRequest, "Invalid request body"
	}
	return encoded, responseFormat, http.StatusOK, ""
}

// openAIImageBodyFromMultipart 把 OpenAI SDK 的 multipart 请求（/images/edits）
// 转成统一媒体链路的 JSON。上传的图片文件转成 data URI 放进 image 字段：
// 统一链路各 adapter 只接受"可引用的图片"，而 multipart 里的文件没有 URL。
func (h *MediaGatewayHandler) openAIImageBodyFromMultipart(c *gin.Context) (map[string]any, error) {
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		return nil, errBadRequest("Failed to parse multipart body")
	}
	body := map[string]any{}
	if form := c.Request.MultipartForm; form != nil {
		for key, values := range form.Value {
			if len(values) == 0 {
				continue
			}
			body[key] = values[0]
		}
		// 内联预算：对象存储不可用时只能把图片 base64 塞进请求体，
		// 请求体再大就会撞上媒体链路 10MB 的读取上限，必须在这一层就拦住。
		inlineBudget := maxInlineUploadBytes
		refs := make([]string, 0, 1)
		for _, field := range []string{"image[]", "image"} {
			for _, header := range form.File[field] {
				if len(refs) >= maxUploadImageFiles {
					return nil, errBadRequest(fmt.Sprintf(
						"最多支持一次上传 %d 张图片", maxUploadImageFiles))
				}
				ref, size, err := h.uploadToImageReference(c, header)
				if err != nil {
					return nil, err
				}
				refs = append(refs, ref)
				if strings.HasPrefix(ref, "data:") {
					// 预算扣的是"编码后"的体积：base64 会把原始字节放大约 4/3，
					// 按原始字节扣会低估占用，最终撞上请求体读取上限。
					inlineBudget -= base64EncodedSize(size) + len("data:image/png;base64,")
					if inlineBudget < 0 {
						return nil, errBadRequest(
							"上传图片过大：请配置对象存储后重试，或改用图片 URL 传入 image 字段")
					}
				}
			}
		}
		if len(refs) == 1 {
			body["image"] = refs[0]
		} else if len(refs) > 1 {
			body["image"] = refs
		}
	}
	return body, nil
}

// uploadToImageReference 把上传文件转成可引用的图片：优先存进对象存储拿稳定 URL，
// 没有对象存储时才退化成 data URI（返回值为 (引用, 原始字节数, error)）。
func (h *MediaGatewayHandler) uploadToImageReference(c *gin.Context, header *multipart.FileHeader) (string, int, error) {
	file, err := header.Open()
	if err != nil {
		return "", 0, errBadRequest("Failed to read image file")
	}
	defer func() { _ = file.Close() }()
	// 多读 1 字节用于识别"超过上限"：只读到上限会让大图被静默截断成损坏文件，
	// 上游随后报出与体积无关的解析错误，排障时会完全跑偏。
	data, err := io.ReadAll(io.LimitReader(file, maxUploadImageBytes+1))
	if err != nil {
		return "", 0, errBadRequest("Failed to read image file")
	}
	if len(data) == 0 {
		return "", 0, errBadRequest("Image file is empty")
	}
	if len(data) > maxUploadImageBytes {
		return "", 0, errBadRequest(fmt.Sprintf(
			"Image file exceeds the %d MB upload limit", maxUploadImageBytes>>20))
	}
	contentType := strings.ToLower(strings.TrimSpace(header.Header.Get("Content-Type")))
	// application/octet-stream 是 multipart 的默认值，不含任何图片信息，
	// 必须按内容嗅探，否则会把 png 标成 octet-stream 传给上游。
	if !strings.HasPrefix(contentType, "image/") {
		contentType = http.DetectContentType(data)
	}
	// 走对象存储时不用受内联预算限制：请求体里只留一个短 URL。
	if h.mediaTaskService != nil && h.mediaTaskService.MediaStorageEnabled() {
		if url, ok := h.mediaTaskService.StoreMediaBytes(c.Request.Context(), service.MediaKindImage, contentType, data); ok && url != "" {
			return url, len(data), nil
		}
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), len(data), nil
}

// buildOpenAIImageData 生成 OpenAI ImagesResponse 的单个 data 项。
// response_format=b64_json 时必须真的返回 base64 —— 客户端显式要什么就给什么，
// 给不出来就明确报错，不能静默换成 url（客户端拿 url 当 base64 解码会直接炸）。
func buildOpenAIImageData(ctx context.Context, imageURL string, wantBase64 bool) (gin.H, error) {
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return nil, errBadRequest("empty image url")
	}
	if !wantBase64 {
		return gin.H{"url": imageURL}, nil
	}
	payload, err := imageBase64Payload(ctx, imageURL)
	if err != nil {
		return nil, err
	}
	return gin.H{"b64_json": payload}, nil
}

// imageBase64Payload 把图片产物转成裸 base64（不含 data URI 前缀）：
//   - 已经是 data URI 的（上游直接回 base64，如 MiniMax）直接截取载荷
//   - 其余按 URL 下载后编码
func imageBase64Payload(ctx context.Context, imageURL string) (string, error) {
	if idx := strings.Index(imageURL, ","); strings.HasPrefix(strings.ToLower(imageURL), "data:") && idx >= 0 {
		return strings.TrimSpace(imageURL[idx+1:]), nil
	}
	if !strings.HasPrefix(imageURL, "http://") && !strings.HasPrefix(imageURL, "https://") {
		return "", errBadRequest("unsupported image url: " + imageURL)
	}
	downloadCtx, cancel := context.WithTimeout(ctx, imageBase64Timeout)
	defer cancel()
	// 带上限下载：先按 Content-Length 拦一次，避免把远超限额的图完整拉下来
	// 才拒绝（既浪费出口带宽，也让客户端多等几十秒才拿到错误）。
	data, _, err := service.DownloadMediaBytesLimit(downloadCtx, imageURL, maxImageBase64Bytes)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// collectImageURLs 汇总任务产物 URL：多图取 media_urls 全量，单图回退 media_url。
func collectImageURLs(record *service.MediaTaskRecord) []string {
	if record == nil {
		return nil
	}
	urls := make([]string, 0, 1)
	for _, u := range record.MediaURLs {
		if trimmed := strings.TrimSpace(u); trimmed != "" {
			urls = append(urls, trimmed)
		}
	}
	if len(urls) == 0 {
		if trimmed := strings.TrimSpace(record.MediaURL); trimmed != "" {
			urls = append(urls, trimmed)
		}
	}
	return urls
}

// resetMediaRequestBody 把改写后的 body 放回请求，供下游再次读取。
func resetMediaRequestBody(c *gin.Context, body []byte) {
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	c.Request.ContentLength = int64(len(body))
	c.Request.Header.Set("Content-Length", strconv.Itoa(len(body)))
}

// errBadRequest 构造一个只带文本的入参错误，统一映射为 400。
func errBadRequest(message string) error { return errors.New(message) }

// base64EncodedSize 返回 size 字节经标准 base64 编码后的长度（不含换行）。
func base64EncodedSize(size int) int {
	if size <= 0 {
		return 0
	}
	return (size + 2) / 3 * 4
}
