package handler

// media_gateway_handler.go — 用户侧统一媒体生成网关（图片 / 视频 / 音频）。
// 端点：
//   POST /v1/media/generations   创建媒体生成任务
//   GET  /v1/media/:id           查询任务状态
//   GET  /v1/media/:id/content   获取产物内容（302）
//
// 低耦合：仅依赖 service.MediaTaskService，不直接访问 repository 或 adapter。

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// MediaGatewayHandler 用户侧统一媒体生成网关 handler。
type MediaGatewayHandler struct {
	mediaTaskService    *service.MediaTaskService
	billingCacheService *service.BillingCacheService
	logger              *zap.Logger
}

// NewMediaGatewayHandler 创建 handler 实例。
func NewMediaGatewayHandler(mediaTaskService *service.MediaTaskService, billingCacheService *service.BillingCacheService) *MediaGatewayHandler {
	return &MediaGatewayHandler{
		mediaTaskService:    mediaTaskService,
		billingCacheService: billingCacheService,
		logger:              logger.L(),
	}
}

// AudioSpeech POST /v1/audio/speech
// OpenAI 兼容同步端点。返回原始音频字节（audio/mpeg），或 302 到上游音频 URL。
func (h *MediaGatewayHandler) AudioSpeech(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		mediaErrorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		mediaErrorResponse(c, http.StatusUnauthorized, "authentication_error", "User context not found")
		return
	}
	subscription, _ := middleware2.GetSubscriptionFromContext(c)

	bodyBytes, err := readMediaRequestBody(c)
	if err != nil {
		mediaErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	if len(bodyBytes) == 0 {
		mediaErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}
	var body map[string]any
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		mediaErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Invalid JSON body")
		return
	}
	publicModel, _ := body["model"].(string)
	if strings.TrimSpace(publicModel) == "" {
		mediaErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	// 调用前额度检查
	if h.billingCacheService != nil {
		if err := h.billingCacheService.CheckBillingEligibility(
			c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription,
			service.QuotaPlatform(c.Request.Context(), apiKey),
		); err != nil {
			status, code, message, retryAfter := billingErrorDetails(err)
			if retryAfter > 0 {
				c.Header("Retry-After", strconv.Itoa(retryAfter))
			}
			mediaErrorResponse(c, status, code, message)
			return
		}
	}

	bytesOut, urlOut, err := h.mediaTaskService.ResolveAudioSpeechBytes(
		c, c.Request.Context(), apiKey.GroupID, publicModel, body,
	)
	if err != nil {
		h.logger.Warn("media_gateway.audio_speech_failed",
			zap.Int64("user_id", subject.UserID),
			zap.String("model", publicModel),
			zap.Error(err),
		)
		mediaErrorResponse(c, http.StatusBadGateway, "api_error", "Audio speech request failed")
		return
	}
	if len(bytesOut) > 0 {
		c.Data(http.StatusOK, "audio/mpeg", bytesOut)
		return
	}
	if urlOut != "" {
		c.Redirect(http.StatusFound, urlOut)
		return
	}
	// 既无字节也无 URL（异步任务），回退到统一任务流程
	h.Create(c)
}

// audioFileEndpoint 处理 OpenAI 兼容 /audio/transcriptions 与 /audio/translations。
// 读取 multipart file，转发到上游同名端点，返回上游 JSON。
func (h *MediaGatewayHandler) audioFileEndpoint(c *gin.Context, endpoint string) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		mediaErrorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	// 读 multipart
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		mediaErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "file field is required")
		return
	}
	defer func() { _ = file.Close() }()
	fileBytes, err := io.ReadAll(io.LimitReader(file, 200<<20))
	if err != nil {
		mediaErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "read file failed")
		return
	}
	model := strings.TrimSpace(c.PostForm("model"))
	if model == "" {
		mediaErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}

	extraForm := map[string]string{}
	for k := range c.Request.MultipartForm.Value {
		switch k {
		case "model", "file":
			continue
		default:
			extraForm[k] = c.PostForm(k)
		}
	}

	respBody, err := h.mediaTaskService.ResolveAudioTranscription(
		c, c.Request.Context(), apiKey.GroupID, model, endpoint, fileBytes, header.Filename, header.Header.Get("Content-Type"), extraForm,
	)
	if err != nil {
		mediaErrorResponse(c, http.StatusBadGateway, "api_error", "Audio file request failed")
		return
	}
	// 直接原样返回上游 JSON（OpenAI 格式 {"text":"..."}）。
	c.Data(http.StatusOK, "application/json", respBody)
}

// AudioTranscription POST /v1/audio/transcriptions
func (h *MediaGatewayHandler) AudioTranscription(c *gin.Context) {
	h.audioFileEndpoint(c, "/v1/audio/transcriptions")
}

// AudioTranslation POST /v1/audio/translations
func (h *MediaGatewayHandler) AudioTranslation(c *gin.Context) {
	h.audioFileEndpoint(c, "/v1/audio/translations")
}

// mediaCreateIdempotent 创建媒体任务；当请求带 Idempotency-Key 头时用幂等协调器包住，
// 防止客户端重试重复创建付费任务。坐标器不可用时退化为直接执行。
func (h *MediaGatewayHandler) mediaCreateIdempotent(
	c *gin.Context,
	kind service.MediaKind,
	apiKey *service.APIKey,
	subject middleware2.AuthSubject,
	publicModel string,
	body map[string]any,
) (*service.MediaTaskRecord, bool, error) {
	coordinator := service.DefaultIdempotencyCoordinator()
	if coordinator == nil {
		record, err := h.mediaTaskService.CreateTask(c, kind, apiKey.GroupID, subject.UserID, apiKey.ID, publicModel, body)
		return record, false, err
	}

	actorScope := "user:" + strconv.FormatInt(subject.UserID, 10)
	var record *service.MediaTaskRecord
	result, err := coordinator.Execute(c.Request.Context(), service.IdempotencyExecuteOptions{
		Scope:          "media_create",
		ActorScope:     actorScope,
		Method:         c.Request.Method,
		Route:          c.FullPath(),
		IdempotencyKey: c.GetHeader("Idempotency-Key"),
		Payload:        body,
		RequireKey:     false,
	}, func(ctx context.Context) (any, error) {
		rec, err := h.mediaTaskService.CreateTask(c, kind, apiKey.GroupID, subject.UserID, apiKey.ID, publicModel, body)
		if err != nil {
			return nil, err
		}
		record = rec
		return rec, nil
	})
	if err != nil {
		return nil, false, err
	}
	if result != nil {
		if rec, ok := result.Data.(*service.MediaTaskRecord); ok {
			record = rec
		} else if rec := decodeMediaTaskRecord(result.Data); rec != nil {
			record = rec
		}
	}
	return record, result != nil && result.Replayed, nil
}

// decodeMediaTaskRecord 在幂等重放时把存储的 JSON 数据（map)还原为 MediaTaskRecord。
func decodeMediaTaskRecord(data any) *service.MediaTaskRecord {
	if data == nil {
		return nil
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return nil
	}
	var rec service.MediaTaskRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return nil
	}
	return &rec
}

// Create POST /v1/media/generations
// 请求体：{ "model": "wan3.0-video", "prompt": "...", ... }
// 响应体：{ "id": "vid_xxx", "status": "processing", "model": "wan3.0-video" }
func (h *MediaGatewayHandler) Create(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		mediaErrorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		mediaErrorResponse(c, http.StatusUnauthorized, "authentication_error", "User context not found")
		return
	}
	subscription, _ := middleware2.GetSubscriptionFromContext(c)

	bodyBytes, err := readMediaRequestBody(c)
	if err != nil {
		mediaErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	if len(bodyBytes) == 0 {
		mediaErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}

	var body map[string]any
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		mediaErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Invalid JSON body")
		return
	}

	publicModel, _ := body["model"].(string)
	if strings.TrimSpace(publicModel) == "" {
		mediaErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	kind := service.MediaKindFromModel(publicModel, body)

	// 调用前额度/余额检查：余额或平台配额不足时直接拒绝，避免白白调用上游。
	if h.billingCacheService != nil {
		if err := h.billingCacheService.CheckBillingEligibility(
			c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription,
			service.QuotaPlatform(c.Request.Context(), apiKey),
		); err != nil {
			reqLog := h.logger.With(
				zap.Int64("user_id", subject.UserID),
				zap.Int64("api_key_id", apiKey.ID),
				zap.String("model", publicModel),
				zap.String("kind", string(kind)),
			)
			reqLog.Info("media_gateway.billing_eligibility_check_failed", zap.Error(err))
			status, code, message, retryAfter := billingErrorDetails(err)
			if retryAfter > 0 {
				c.Header("Retry-After", strconv.Itoa(retryAfter))
			}
			mediaErrorResponse(c, status, code, message)
			return
		}
	}

	reqLog := h.logger.With(
		zap.String("component", "handler.media_gateway"),
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
		zap.String("model", publicModel),
		zap.String("kind", string(kind)),
	)

	record, replayed, err := h.mediaCreateIdempotent(c, kind, apiKey, subject, publicModel, body)
	if err != nil {
		reqLog.Error("media_gateway.create_task_failed", zap.Error(err))
		// 参数契约类错误（如 resolution 档位非法）应返回 400 而非上游故障。
		var invalidReq *service.MediaInvalidRequestError
		if errors.As(err, &invalidReq) {
			mediaErrorResponse(c, http.StatusBadRequest, "invalid_request_error", invalidReq.Reason)
			return
		}
		if strings.Contains(err.Error(), "no available account") {
			mediaErrorResponse(c, http.StatusServiceUnavailable, "capacity_error", "No available media generation channels")
			return
		}
		mediaErrorResponse(c, http.StatusBadGateway, "api_error", "Media generation request failed")
		return
	}
	if replayed {
		c.Header("X-Idempotency-Replayed", "true")
	}

	reqLog.Info("media_gateway.create_task_succeeded",
		zap.String("local_id", record.LocalID),
		zap.String("status", record.Status),
	)

	response := mediaTaskResponse{
		ID:        record.LocalID,
		Status:    record.Status,
		Model:     record.PublicModel,
		CreatedAt: record.CreatedAt.UTC().Format(time.RFC3339),
	}
	if record.MediaURL != "" {
		response.URL = record.MediaURL
	}
	if record.ErrorMessage != "" {
		response.Error = record.ErrorMessage
	}

	c.JSON(http.StatusAccepted, response)
}

// Get GET /v1/media/:id
func (h *MediaGatewayHandler) Get(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		mediaErrorResponse(c, http.StatusUnauthorized, "authentication_error", "User context not found")
		return
	}

	taskID := mediaTaskIDParam(c)
	if taskID == "" {
		mediaErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "task_id is required")
		return
	}

	record, err := h.mediaTaskService.GetTask(c.Request.Context(), taskID, subject.UserID)
	if err != nil {
		h.logger.Warn("media_gateway.get_task_failed",
			zap.String("task_id", taskID),
			zap.Int64("user_id", subject.UserID),
			zap.Error(err),
		)
		mediaErrorResponse(c, http.StatusNotFound, "not_found_error", "Media task not found")
		return
	}

	response := mediaTaskResponse{
		ID:        record.LocalID,
		Status:    record.Status,
		Model:     record.PublicModel,
		CreatedAt: record.CreatedAt.UTC().Format(time.RFC3339),
	}
	if record.MediaURL != "" {
		response.URL = record.MediaURL
	}
	if record.ThumbnailURL != "" {
		response.ThumbnailURL = record.ThumbnailURL
	}
	if record.DurationSec > 0 {
		response.DurationSec = record.DurationSec
	}
	if record.Resolution != "" {
		response.Resolution = record.Resolution
	}
	if record.ErrorMessage != "" {
		response.Error = record.ErrorMessage
	}
	if record.FinishedAt != nil {
		response.FinishedAt = record.FinishedAt.UTC().Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, response)
}

// GetContent GET /v1/media/:id/content
func (h *MediaGatewayHandler) GetContent(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		mediaErrorResponse(c, http.StatusUnauthorized, "authentication_error", "User context not found")
		return
	}

	taskID := mediaTaskIDParam(c)
	if taskID == "" {
		mediaErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "task_id is required")
		return
	}

	record, err := h.mediaTaskService.GetTask(c.Request.Context(), taskID, subject.UserID)
	if err != nil {
		mediaErrorResponse(c, http.StatusNotFound, "not_found_error", "Media task not found")
		return
	}

	if record.Status != "succeeded" || record.MediaURL == "" {
		mediaErrorResponse(c, http.StatusNotFound, "not_found_error", "Media content not available")
		return
	}

	c.Redirect(http.StatusFound, record.MediaURL)
}

// --- 响应结构 ---

type mediaTaskResponse struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	Model        string `json:"model"`
	URL          string `json:"url,omitempty"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	Resolution   string `json:"resolution,omitempty"`
	DurationSec  int    `json:"duration_sec,omitempty"`
	Error        string `json:"error,omitempty"`
	CreatedAt    string `json:"created_at"`
	FinishedAt   string `json:"finished_at,omitempty"`
}

// --- 辅助函数 ---

func mediaErrorResponse(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}

func readMediaRequestBody(c *gin.Context) ([]byte, error) {
	const maxMediaBodySize = 10 * 1024 * 1024
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxMediaBodySize)
	return io.ReadAll(c.Request.Body)
}

func mediaTaskIDParam(c *gin.Context) string {
	if taskID := strings.TrimSpace(c.Param("task_id")); taskID != "" {
		return taskID
	}
	return strings.TrimSpace(c.Param("id"))
}
