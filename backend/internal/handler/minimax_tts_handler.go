package handler

// minimax_tts_handler.go — MiniMax 音色复刻 / 音色查询 / 异步语音合成的原生兼容入口。
//
// 这些端点与 MiniMax 官方文档的路径、请求体保持一致，客户端只需把 base_url
// 换成本网关、Authorization 换成网关 API Key：
//
//   POST /v1/files/upload
//   POST /v1/voice_clone
//   POST /v1/get_voice
//   POST /v1/t2a_async_v2
//   GET  /v1/query/t2a_async_query_v2?task_id=...
//   GET  /v1/files/retrieve_content?file_id=...
//
// 账号选择 / 音色与文件归属 / 字符计费都在 service 层完成；handler 只负责鉴权、
// 参数读取、额度预检查与把内部错误收敛成 MiniMax 形状的 base_resp。

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// miniMaxUploadMaxBytes 与 service 层保持一致（异步文本 zip 可能比音色样本大）。
const miniMaxUploadMaxBytes = 100 << 20

// miniMaxResponseError 输出 MiniMax 原生风格的错误体。
func miniMaxResponseError(c *gin.Context, status, code int, message string) {
	c.JSON(status, gin.H{
		"base_resp": gin.H{
			"status_code": code,
			"status_msg":  message,
		},
	})
}

// miniMaxServiceError 把 service 错误收敛成 400 / 502，不向上游原文透传。
func (h *MediaGatewayHandler) miniMaxServiceError(c *gin.Context, operation string, err error) {
	h.logger.Warn("minimax_gateway."+operation+"_failed", zap.Error(err))
	switch {
	case errors.Is(err, service.ErrMiniMaxBadRequest),
		errors.Is(err, service.ErrMiniMaxInvalidToken):
		miniMaxResponseError(c, http.StatusBadRequest, 2013, "invalid or expired gateway id or parameter")
	case errors.Is(err, service.ErrMiniMaxStoreUnavailable):
		miniMaxResponseError(c, http.StatusServiceUnavailable, 1000, "MiniMax gateway storage is temporarily unavailable")
	default:
		miniMaxResponseError(c, http.StatusBadGateway, 1000, "MiniMax upstream request failed")
	}
}

// miniMaxRequestAPIKey 取当前 API Key；失败时已写出 401。
func (h *MediaGatewayHandler) miniMaxRequestAPIKey(c *gin.Context) (*service.APIKey, bool) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil {
		miniMaxResponseError(c, http.StatusUnauthorized, 1004, "Invalid API key")
		return nil, false
	}
	return apiKey, true
}

// miniMaxRequireBilling 做调用前额度检查，避免无余额用户白嫖上游试听/长文本。
func (h *MediaGatewayHandler) miniMaxRequireBilling(c *gin.Context, apiKey *service.APIKey) bool {
	if h.billingCacheService == nil || apiKey == nil {
		return true
	}
	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	if err := h.billingCacheService.CheckBillingEligibility(
		c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription,
		service.QuotaPlatform(c.Request.Context(), apiKey),
	); err != nil {
		status, _, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		miniMaxResponseError(c, status, 1008, message)
		return false
	}
	return true
}

// miniMaxReadJSON 读取 JSON 请求体（上限沿用媒体请求的 10MB）。
func miniMaxReadJSON(c *gin.Context) (map[string]any, bool) {
	bodyBytes, err := readMediaRequestBody(c)
	if err != nil {
		miniMaxResponseError(c, http.StatusBadRequest, 2013, "Failed to read request body")
		return nil, false
	}
	if len(bodyBytes) == 0 {
		miniMaxResponseError(c, http.StatusBadRequest, 2013, "Request body is empty")
		return nil, false
	}
	var body map[string]any
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		miniMaxResponseError(c, http.StatusBadRequest, 2013, "Invalid JSON body")
		return nil, false
	}
	return body, true
}

// UploadFile POST /v1/files/upload
// 兼容 MiniMax 原生 multipart：purpose + file；额外接受 model 用于选账号。
func (h *MediaGatewayHandler) UploadFile(c *gin.Context) {
	apiKey, ok := h.miniMaxRequestAPIKey(c)
	if !ok {
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		miniMaxResponseError(c, http.StatusBadRequest, 2013, "file field is required")
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		miniMaxResponseError(c, http.StatusBadRequest, 2013, "Failed to open uploaded file")
		return
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, miniMaxUploadMaxBytes+1))
	if err != nil {
		miniMaxResponseError(c, http.StatusBadRequest, 2013, "Failed to read uploaded file")
		return
	}
	if len(data) > miniMaxUploadMaxBytes {
		miniMaxResponseError(c, http.StatusRequestEntityTooLarge, 2013, "file is too large")
		return
	}
	model := strings.TrimSpace(c.PostForm("model"))
	purpose := strings.TrimSpace(c.PostForm("purpose"))
	result, err := h.mediaTaskService.MiniMaxUploadFile(
		c.Request.Context(), apiKey.GroupID, apiKey.UserID, apiKey.ID,
		model, purpose, fileHeader.Filename, data,
	)
	if err != nil {
		h.miniMaxServiceError(c, "upload_file", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// VoiceClone POST /v1/voice_clone
func (h *MediaGatewayHandler) VoiceClone(c *gin.Context) {
	apiKey, ok := h.miniMaxRequestAPIKey(c)
	if !ok {
		return
	}
	body, ok := miniMaxReadJSON(c)
	if !ok {
		return
	}
	// 只有带试听文本时上游才收 TTS 费；没有 text 的纯复刻不因余额被拦。
	if text, _ := body["text"].(string); strings.TrimSpace(text) != "" {
		if !h.miniMaxRequireBilling(c, apiKey) {
			return
		}
	}
	result, err := h.mediaTaskService.MiniMaxVoiceClone(
		c.Request.Context(), apiKey.UserID, apiKey.ID, body,
	)
	if err != nil {
		h.miniMaxServiceError(c, "voice_clone", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetVoices POST /v1/get_voice
// 额外接受 model 用于选择查询哪个上游账号（默认 speech-2.8-hd）。
func (h *MediaGatewayHandler) GetVoices(c *gin.Context) {
	apiKey, ok := h.miniMaxRequestAPIKey(c)
	if !ok {
		return
	}
	body, ok := miniMaxReadJSON(c)
	if !ok {
		return
	}
	model, _ := body["model"].(string)
	result, err := h.mediaTaskService.MiniMaxGetVoices(
		c.Request.Context(), apiKey.GroupID, apiKey.UserID, strings.TrimSpace(model), body,
	)
	if err != nil {
		h.miniMaxServiceError(c, "get_voice", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// CreateAsyncTTS POST /v1/t2a_async_v2
func (h *MediaGatewayHandler) CreateAsyncTTS(c *gin.Context) {
	apiKey, ok := h.miniMaxRequestAPIKey(c)
	if !ok {
		return
	}
	body, ok := miniMaxReadJSON(c)
	if !ok {
		return
	}
	if !h.miniMaxRequireBilling(c, apiKey) {
		return
	}
	result, err := h.mediaTaskService.MiniMaxCreateAsyncTTS(
		c.Request.Context(), apiKey.GroupID, apiKey.UserID, apiKey.ID, body,
	)
	if err != nil {
		h.miniMaxServiceError(c, "async_tts_create", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// QueryAsyncTTS GET /v1/query/t2a_async_query_v2?task_id=...
func (h *MediaGatewayHandler) QueryAsyncTTS(c *gin.Context) {
	apiKey, ok := h.miniMaxRequestAPIKey(c)
	if !ok {
		return
	}
	taskID := strings.TrimSpace(c.Query("task_id"))
	if taskID == "" {
		miniMaxResponseError(c, http.StatusBadRequest, 2013, "task_id is required")
		return
	}
	result, err := h.mediaTaskService.MiniMaxQueryAsyncTTS(
		c.Request.Context(), apiKey.UserID, taskID,
	)
	if err != nil {
		h.miniMaxServiceError(c, "async_tts_query", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// DownloadFile GET /v1/files/retrieve_content?file_id=...
func (h *MediaGatewayHandler) DownloadFile(c *gin.Context) {
	apiKey, ok := h.miniMaxRequestAPIKey(c)
	if !ok {
		return
	}
	fileID := strings.TrimSpace(c.Query("file_id"))
	if fileID == "" {
		miniMaxResponseError(c, http.StatusBadRequest, 2013, "file_id is required")
		return
	}
	download, err := h.mediaTaskService.MiniMaxDownloadFile(
		c.Request.Context(), apiKey.UserID, fileID,
	)
	if err != nil {
		h.miniMaxServiceError(c, "file_download", err)
		return
	}
	defer func() { _ = download.Body.Close() }()
	if download.ContentType != "" {
		c.Header("Content-Type", download.ContentType)
	}
	if download.ContentLength > 0 {
		c.Header("Content-Length", strconv.FormatInt(download.ContentLength, 10))
	}
	c.Status(http.StatusOK)
	if _, copyErr := io.Copy(c.Writer, download.Body); copyErr != nil {
		h.logger.Warn("minimax_gateway.file_download_copy_failed", zap.Error(copyErr))
	}
}
