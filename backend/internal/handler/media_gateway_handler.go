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
	"fmt"
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
		c, c.Request.Context(), apiKey.GroupID, apiKey.UserID, apiKey.ID, publicModel, body,
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
	key := resolveMediaIdempotencyKey(c.GetHeader("Idempotency-Key"), body)
	var record *service.MediaTaskRecord
	result, err := coordinator.Execute(c.Request.Context(), service.IdempotencyExecuteOptions{
		Scope:          "media_create",
		ActorScope:     actorScope,
		Method:         c.Request.Method,
		Route:          c.FullPath(),
		IdempotencyKey: key,
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

// resolveMediaIdempotencyKey 决定本次创建用哪个幂等键。
//
// 与 /v1/images/generations/async 保持同一口径：Idempotency-Key 头优先（跨语言
// 通用做法），其次回落到请求体 request_id。视频创建与同步生图共用
// mediaCreateIdempotent，因此两条链路一起获益。request_id 只是本网关的标识，
// adapter 侧已把它从上游请求体中剔除。
//
// 两者都为空时返回空串：调用方（含协调器）按"未提供幂等键"处理，保持历史行为。
func resolveMediaIdempotencyKey(headerKey string, body map[string]any) string {
	if key := strings.TrimSpace(headerKey); key != "" {
		return key
	}
	if body == nil {
		return ""
	}
	value, ok := body["request_id"].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
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

// mediaCreateOutcome 是创建媒体任务的公共结果：成功时带任务记录，
// 失败时带已经映射好的 HTTP 状态码 + OpenAI 风格错误字段。
// 抽出它的原因是 /v1/media/generations 与 OpenAI 兼容的 /v1/images/* 必须
// 共享同一套鉴权、权限与额度校验，只有最后一步响应结构不同。
type mediaCreateOutcome struct {
	record   *service.MediaTaskRecord
	replayed bool
	status   int
	errType  string
	message  string
}

// createMediaTask 执行创建媒体任务的公共流程：鉴权 → 读参 → 图片权限 → 额度 → 落库。
// 调用方负责按自己的响应契约渲染 record。
func (h *MediaGatewayHandler) createMediaTask(c *gin.Context) mediaCreateOutcome {
	fail := func(status int, errType, message string) mediaCreateOutcome {
		return mediaCreateOutcome{status: status, errType: errType, message: message}
	}

	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		return fail(http.StatusUnauthorized, "authentication_error", "Invalid API key")
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		return fail(http.StatusUnauthorized, "authentication_error", "User context not found")
	}
	subscription, _ := middleware2.GetSubscriptionFromContext(c)

	bodyBytes, err := readMediaRequestBody(c)
	if err != nil {
		return fail(http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
	}
	if len(bodyBytes) == 0 {
		return fail(http.StatusBadRequest, "invalid_request_error", "Request body is empty")
	}

	var body map[string]any
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return fail(http.StatusBadRequest, "invalid_request_error", "Invalid JSON body")
	}

	publicModel, _ := body["model"].(string)
	if strings.TrimSpace(publicModel) == "" {
		return fail(http.StatusBadRequest, "invalid_request_error", "model is required")
	}
	kind := service.MediaKindFromModel(publicModel, body)

	// 生图权限：与 /v1/images/* 保持同一口径。统一媒体链路是本平台的推荐入口，
	// 若这里不校验，后台的「允许生成图片」开关形同虚设——客户端绕道
	// /v1/media/generations 就能出图。只拦图片类任务，视频 / 音频不受影响，
	// 避免把既有视频调用一并挡在门外。
	if kind == service.MediaKindImage && !service.GroupAllowsImageGeneration(apiKey.Group) {
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
		return fail(http.StatusForbidden, "permission_error", service.ImageGenerationPermissionMessage())
	}

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
			return fail(status, code, message)
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
			return fail(http.StatusBadRequest, "invalid_request_error", invalidReq.Reason)
		}
		if strings.Contains(err.Error(), "no available account") {
			// 选号失败发生在计费之前：任务表与用量明细都不会有记录，
			// 这条日志是运营侧唯一的排障线索（哪个分组缺哪个模型）。
			reqLog.Warn("media_gateway.no_available_account",
				zap.Any("group_id", apiKey.GroupID),
				zap.String("group_platform", mediaGroupPlatform(apiKey)),
				zap.String("model", publicModel),
				zap.String("kind", string(kind)),
			)
			return fail(http.StatusServiceUnavailable, "capacity_error", mediaNoAvailableAccountMessage(publicModel))
		}
		return fail(http.StatusBadGateway, "api_error", "Media generation request failed")
	}

	reqLog.Info("media_gateway.create_task_succeeded",
		zap.String("local_id", record.LocalID),
		zap.String("status", record.Status),
	)

	return mediaCreateOutcome{record: record, replayed: replayed, status: http.StatusAccepted}
}

// mediaNoAvailableAccountMessage 生成"选不到号"的可读错误。
//
// 此前这里是一句无上下文的 "No available media generation channels"，用户看不出
// 是模型没挂、分组没账号，还是账号被限流。选号阶段在计费之前就失败，任务表与
// 用量明细都不会有记录，所以错误信息是唯一的排障线索，必须说清该做什么。
func mediaNoAvailableAccountMessage(model string) string {
	trimmed := strings.TrimSpace(model)
	if trimmed == "" {
		return "No available media generation channel for the requested model"
	}
	return fmt.Sprintf(
		"当前分组没有可服务模型 %s 的可用账号：请在该分组内添加支持此模型的账号并挂上该模型，或改用 composite 分组由平台按模型自动选择通道",
		trimmed,
	)
}

// mediaGroupPlatform 取分组平台名，仅用于日志字段（分组可能未预加载）。
func mediaGroupPlatform(apiKey *service.APIKey) string {
	if apiKey == nil || apiKey.Group == nil {
		return ""
	}
	return apiKey.Group.Platform
}

// awaitMediaTask 按 ?wait=N 等待任务到达终态，返回最终记录（失败时回退原记录）。
// 不传 wait 时返回原记录且不额外等待 —— 现有异步客户端行为完全不变。
func (h *MediaGatewayHandler) awaitMediaTask(c *gin.Context, record *service.MediaTaskRecord, defaultWait time.Duration) *service.MediaTaskRecord {
	if record == nil || service.IsMediaTaskTerminal(record.Status) {
		return record
	}
	wait := parseMediaWaitParam(c.Query("wait"), defaultWait)
	if wait <= 0 {
		return record
	}
	userID := record.UserID
	refreshed, err := h.mediaTaskService.AwaitTerminal(c.Request.Context(), record.LocalID, userID, wait)
	if err != nil || refreshed == nil {
		return record
	}
	return refreshed
}

// parseMediaWaitParam 解析 ?wait=N（秒）。非法值回落默认值，上限 180 秒。
func parseMediaWaitParam(raw string, defaultWait time.Duration) time.Duration {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultWait
	}
	seconds, err := strconv.Atoi(trimmed)
	if err != nil || seconds < 0 {
		return defaultWait
	}
	// wait=0 是显式的"不要等待"，不能被默认值覆盖。
	if seconds == 0 {
		return 0
	}
	wait := time.Duration(seconds) * time.Second
	if wait > service.MaxMediaTaskWait {
		wait = service.MaxMediaTaskWait
	}
	return wait
}

// Create POST /v1/media/generations
// 请求体：{ "model": "wan3.0-video", "prompt": "...", ... }
// 响应体：{ "id": "vid_xxx", "status": "processing", "model": "wan3.0-video" }
//
// 兼容说明：状态码恒为 202（与历史行为一致），但带 ?wait=N 时响应里的
// status / url 会反映等待后的最新状态，客户端可省去轮询。
func (h *MediaGatewayHandler) Create(c *gin.Context) {
	outcome := h.createMediaTask(c)
	if outcome.record == nil {
		mediaErrorResponse(c, outcome.status, outcome.errType, outcome.message)
		return
	}
	if outcome.replayed {
		c.Header("X-Idempotency-Replayed", "true")
	}
	record := h.awaitMediaTask(c, outcome.record, 0)

	response := mediaTaskResponse{
		ID:        record.LocalID,
		Status:    record.Status,
		Model:     record.PublicModel,
		CreatedAt: record.CreatedAt.UTC().Format(time.RFC3339),
	}
	if record.MediaURL != "" {
		response.URL = record.MediaURL
	}
	if len(record.MediaURLs) > 1 {
		response.URLs = record.MediaURLs
	}
	if record.ErrorMessage != "" {
		response.Error = record.ErrorMessage
	}

	c.JSON(http.StatusAccepted, response)
}

// HasLocalTask 判断任务是否存在于统一媒体表且属于该用户。
//
// 用途：视频链路收敛后，/v1/videos/generations 产出的任务写进 media_tasks，
// 但历史上 /video-tasks 的任务仍在 video_tasks，两条链路的 local_id 都是
// "vid_ + 32 位十六进制"，无法从字面区分。status/content 查询必须先问媒体表，
// 查不到再回退到旧视频表，否则老任务会被判成 404。
func (h *MediaGatewayHandler) HasLocalTask(ctx context.Context, localID string, userID int64) bool {
	if h.mediaTaskService == nil || strings.TrimSpace(localID) == "" {
		return false
	}
	// 用不带刷新的查询：这里只是判归属，真正的状态刷新交给随后的 Get 做一次，
	// 否则每次轮询都会打两遍上游。
	record, err := h.mediaTaskService.GetTaskByLocalID(ctx, localID)
	if err != nil || record == nil {
		return false
	}
	return record.UserID == userID
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
	if len(record.MediaURLs) > 0 {
		response.URLs = record.MediaURLs
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
	ID     string `json:"id"`
	Status string `json:"status"`
	Model  string `json:"model"`
	URL    string `json:"url,omitempty"`
	// URLs 是 n>1 时的全部产物 URL（media_urls 列全量落库）。
	// 图片为同步返回，创建响应里直接带全量；轮询路径同样返回全量。
	URLs         []string `json:"urls,omitempty"`
	ThumbnailURL string   `json:"thumbnail_url,omitempty"`
	Resolution   string   `json:"resolution,omitempty"`
	DurationSec  int      `json:"duration_sec,omitempty"`
	Error        string   `json:"error,omitempty"`
	CreatedAt    string   `json:"created_at"`
	FinishedAt   string   `json:"finished_at,omitempty"`
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

// mediaTaskIDParam 取路径里的任务 ID。
//
// 必须同时认 `request_id`：视频端点收敛后 GET /v1/videos/:request_id 会路由到
// 媒体 handler，而那条路由的参数名是 request_id（不是 task_id / id），
// 只认后两者会让所有视频状态查询都变成 "task_id is required"。
func mediaTaskIDParam(c *gin.Context) string {
	for _, key := range []string{"task_id", "id", "request_id"} {
		if taskID := strings.TrimSpace(c.Param(key)); taskID != "" {
			return taskID
		}
	}
	return ""
}
