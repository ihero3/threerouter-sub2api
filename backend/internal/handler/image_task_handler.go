package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AsyncImageHandler struct {
	tasks   *service.ImageTaskService
	openAI  *OpenAIGatewayHandler
	execute func(platform string, c *gin.Context)
}

func NewAsyncImageHandler(tasks *service.ImageTaskService, openAI *OpenAIGatewayHandler) *AsyncImageHandler {
	h := &AsyncImageHandler{tasks: tasks, openAI: openAI}
	h.execute = h.executeWithGateway
	return h
}

// enabled reports whether the async image task feature is available. Object
// storage is the enablement gate: without it the endpoints are fully disabled
// so that large base64 results never land in Redis.
func (h *AsyncImageHandler) enabled() bool {
	return h != nil && h.tasks != nil && h.tasks.Enabled()
}

// pollable reports whether task lookups can be served. It is deliberately weaker
// than enabled(): results already written to Redis stay readable after the
// feature is switched off, so an in-flight task is never stranded.
func (h *AsyncImageHandler) pollable() bool {
	return h != nil && h.tasks != nil && h.tasks.Pollable()
}

// Submit accepts the same payload as the synchronous Images endpoint and
// returns before the upstream image generation begins.
func (h *AsyncImageHandler) Submit(c *gin.Context) {
	if !h.enabled() {
		imageTaskJSONError(c, http.StatusNotFound, "not_found_error", "async image tasks are not enabled")
		return
	}
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.UserID <= 0 || apiKey.ID <= 0 {
		imageTaskError(c, service.ErrImageTaskForbidden)
		return
	}
	platform := ""
	if apiKey.Group != nil {
		platform = apiKey.Group.Platform
	}
	if platform != service.PlatformOpenAI && platform != service.PlatformGrok {
		imageTaskJSONError(c, http.StatusNotFound, "not_found_error", "Images API is not supported for this platform")
		return
	}
	if !service.GroupAllowsImageGeneration(apiKey.Group) {
		imageTaskJSONError(c, http.StatusForbidden, "permission_error", service.ImageGenerationPermissionMessage())
		return
	}
	if h == nil || h.tasks == nil || h.execute == nil {
		imageTaskError(c, service.ErrImageTaskUnavailable)
		return
	}

	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			imageTaskJSONError(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return
		}
		imageTaskJSONError(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	if len(body) == 0 {
		imageTaskJSONError(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}
	if asyncImageRequestStreams(c.GetHeader("Content-Type"), body) {
		imageTaskJSONError(c, http.StatusBadRequest, "invalid_request_error", "streaming image requests cannot be submitted as asynchronous tasks")
		return
	}
	if err := h.validateRequest(c, platform, body); err != nil {
		imageTaskJSONError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	if !h.checkSecurityAuditBeforeSubmit(c, apiKey, platform, body) {
		return
	}

	// request_id 是本网关的幂等标识，上游厂商并不认这个字段；OpenAI 官方 API 对
	// 未知参数是严格拒绝的（Unrecognized request argument），因此必须在下发前摘掉。
	body, bodyRequestID := stripImageTaskRequestID(c.GetHeader("Content-Type"), body)
	// 幂等键优先级：Idempotency-Key 头（跨语言通用做法）> 请求体 request_id。
	requestID := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if requestID == "" {
		requestID = bodyRequestID
	}

	h.submitTask(c, platform, body, requestID)
}

// submitTask 创建异步图片任务；带幂等键时经幂等协调器包裹，保证同一键的重复提交
// 只会创建一次任务。
//
// 为什么异步端点必须有幂等：本服务对上游的调用刻意脱离客户端取消链
// （见 MediaTaskService 的 upstreamBase），客户端超时/断连后服务端仍会继续
// 生成。此时若调用方重试，就会真的重复生成、重复扣费。
func (h *AsyncImageHandler) submitTask(c *gin.Context, platform string, body []byte, requestID string) {
	coordinator := service.DefaultIdempotencyCoordinator()
	if coordinator == nil || requestID == "" {
		// 未装配幂等基建、或调用方未提供幂等键：保持原有行为，不做强制，
		// 避免打断已经上线、尚未携带幂等键的调用方。
		data, err := h.createImageTask(c, platform, body, requestID)
		if err != nil {
			imageTaskError(c, err)
			return
		}
		h.writeImageTaskAccepted(c, data, false)
		return
	}

	// scope 里带上 owner：否则 A 能用猜到的 request_id 读到 B 的提交响应，
	// 且不同用户写同一个 request_id 时会互相撞 409。
	actorScope := imageTaskActorScope(c)
	result, err := coordinator.Execute(c.Request.Context(), service.IdempotencyExecuteOptions{
		Scope:          service.ImageTaskIdempotencyScope(actorScope),
		ActorScope:     actorScope,
		Method:         c.Request.Method,
		Route:          c.FullPath(),
		IdempotencyKey: requestID,
		Payload:        body,
		TTL:            service.DefaultWriteIdempotencyTTL(),
	}, func(context.Context) (any, error) {
		return h.createImageTask(c, platform, body, requestID)
	})
	if err != nil {
		if retryAfter := service.RetryAfterSecondsFromError(err); retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		imageTaskError(c, err)
		return
	}
	h.writeImageTaskAccepted(c, result.Data, result.Replayed)
}

// createImageTask 落库任务并异步启动生成。返回值即对外响应体（也是幂等回放时
// 缓存的内容），因此只放轻量字段——结果图 URL 由轮询端点给出。
func (h *AsyncImageHandler) createImageTask(c *gin.Context, platform string, body []byte, requestID string) (any, error) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil {
		return nil, service.ErrImageTaskForbidden
	}
	// 落库同样要脱离客户端取消链。异步端点的承诺是"提交后可以立刻断开"，
	// 若沿用请求 context，客户端在提交瞬间掉线（超时/502 场景）会让任务创建
	// 直接失败：既没留下可找回的任务，幂等键又被占住，调用方反而更难恢复。
	// 与下方执行链路（newAsyncImageContext 内的 WithoutCancel）保持一致。
	task, err := h.tasks.Create(context.WithoutCancel(c.Request.Context()),
		service.ImageTaskOwner{UserID: apiKey.UserID, APIKeyID: apiKey.ID})
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"id":         task.ID,
		"task_id":    task.TaskID,
		"object":     task.Object,
		"status":     task.Status,
		"created_at": task.CreatedAt,
		"expires_at": task.ExpiresAt,
		"poll_url":   imageTaskPollURL(c.Request.URL.Path, task.ID),
	}
	if requestID != "" {
		payload["request_id"] = requestID
	}
	// 任务已落库才启动执行协程：Create 失败时不应产生无人回收的 context。
	taskCtx, recorder, cancel := newAsyncImageContext(c, body, h.tasks.ExecutionTimeout())
	go h.run(task.ID, platform, taskCtx, recorder, cancel)
	return payload, nil
}

func (h *AsyncImageHandler) writeImageTaskAccepted(c *gin.Context, data any, replayed bool) {
	taskID := imageTaskSubmitField(data, "task_id")
	pollURL := imageTaskSubmitField(data, "poll_url")
	if pollURL == "" && taskID != "" {
		pollURL = imageTaskPollURL(c.Request.URL.Path, taskID)
	}
	c.Header("Cache-Control", "no-store")
	if pollURL != "" {
		c.Header("Location", pollURL)
	}
	c.Header("Retry-After", "3")
	if replayed {
		// 复用原响应时显式告知调用方：这次没有真正创建新任务。
		c.Header("X-Idempotency-Replayed", "true")
	}
	c.JSON(http.StatusAccepted, data)
}

func imageTaskSubmitField(data any, key string) string {
	values, ok := data.(map[string]any)
	if !ok {
		return ""
	}
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

// stripImageTaskRequestID 从请求体里摘出 request_id 并返回摘除后的 body。
//
// 返回值 (body, requestID)：requestID 为空表示请求体里没有该字段。
// 非 JSON / 非 multipart body 原样返回——这类请求的 request_id 只能走
// Idempotency-Key 头，我们不去猜测其内部结构。
func stripImageTaskRequestID(contentType string, body []byte) ([]byte, string) {
	if isMultipartImagesContentType(contentType) {
		return stripImageTaskRequestIDMultipart(contentType, body)
	}
	if !json.Valid(body) {
		return body, ""
	}
	// 用 RawMessage 逐字段解析，避免重新序列化时改动其它字段的字面量
	// （例如大整数被转成浮点）。
	var envelope map[string]json.RawMessage
	if json.Unmarshal(body, &envelope) != nil {
		return body, ""
	}
	raw, ok := envelope["request_id"]
	if !ok {
		return body, ""
	}
	var requestID string
	_ = json.Unmarshal(raw, &requestID)
	requestID = strings.TrimSpace(requestID)
	delete(envelope, "request_id")
	stripped, err := json.Marshal(envelope)
	if err != nil {
		return body, requestID
	}
	return stripped, requestID
}

// stripImageTaskRequestIDMultipart 处理 multipart 请求（如 /v1/images/edits）。
//
// 之前这里直接放行，留下两个问题：请求体形式的 request_id 在 multipart 下不生效
// （客户端会以为幂等没起作用），且该字段会随原始 body 一路透传给上游厂商。
// multipart 无法原地删字段，只能逐 part 重建，所以先扫一遍确认存在与否：
// 不存在时返回原 body（绝大多数请求都不走重建，boundary 也不变）。
func stripImageTaskRequestIDMultipart(contentType string, body []byte) ([]byte, string) {
	if len(body) == 0 {
		return body, ""
	}
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return body, ""
	}
	boundary := strings.TrimSpace(params["boundary"])
	if boundary == "" {
		return body, ""
	}
	requestID, found := multipartFieldValue(body, boundary, "request_id")
	requestID = strings.TrimSpace(requestID)
	if !found {
		return body, ""
	}
	stripped, err := dropMultipartField(body, boundary, "request_id")
	if err != nil {
		// 重建失败也要保住幂等键：body 里多一个字段总好过静默失去幂等。
		return body, requestID
	}
	return stripped, requestID
}

// multipartFieldValue 扫描 multipart body 取第一个同名字段的字符串值。
func multipartFieldValue(body []byte, boundary, name string) (string, bool) {
	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	for {
		part, err := reader.NextPart()
		if err != nil {
			return "", false
		}
		if !strings.EqualFold(strings.TrimSpace(part.FormName()), name) || part.FileName() != "" {
			_ = part.Close()
			continue
		}
		raw, readErr := io.ReadAll(io.LimitReader(part, maxMultipartFieldBytes))
		_ = part.Close()
		if readErr != nil {
			return "", false
		}
		return string(raw), true
	}
}

// dropMultipartField 返回剔除了指定表单字段后的 multipart body。
//
// 刻意复用原 boundary：本函数只返回 body，调用方仍持有原来的 Content-Type 头，
// 换 boundary 会让下游按旧 boundary 解析失败（body 与声明类型不一致）。
func dropMultipartField(body []byte, boundary, name string) ([]byte, error) {
	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	if err := writer.SetBoundary(boundary); err != nil {
		return nil, err
	}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		formName := strings.TrimSpace(part.FormName())
		if strings.EqualFold(formName, name) && part.FileName() == "" {
			_ = part.Close()
			continue
		}
		target, err := writer.CreatePart(cloneTextprotoHeader(part.Header))
		if err != nil {
			_ = part.Close()
			return nil, err
		}
		if _, err := io.Copy(target, part); err != nil {
			_ = part.Close()
			return nil, err
		}
		_ = part.Close()
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// maxMultipartFieldBytes 是扫描普通表单字段时的读取上限。幂等键只有几十字节，
// 给 64KB 足够容纳任何正常写法，同时挡住"把字段当文件塞"的畸形请求。
const maxMultipartFieldBytes = 64 << 10

func cloneTextprotoHeader(src textproto.MIMEHeader) textproto.MIMEHeader {
	dst := make(textproto.MIMEHeader, len(src))
	for key, values := range src {
		copied := make([]string, len(values))
		copy(copied, values)
		dst[key] = copied
	}
	return dst
}

func imageTaskActorScope(c *gin.Context) string {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil {
		return ""
	}
	// 刻意与任务归属同口径（任务按 API Key 隔离，轮询必须用提交时那把 key）：
	// 若这里按 user 隔离，同一用户的另一把 key 会回放到自己轮询不到的 task_id。
	return "apikey:" + strconv.FormatInt(apiKey.ID, 10)
}

func (h *AsyncImageHandler) checkSecurityAuditBeforeSubmit(c *gin.Context, apiKey *service.APIKey, platform string, body []byte) bool {
	if h == nil || h.openAI == nil {
		return true
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		imageTaskJSONError(c, http.StatusInternalServerError, "api_error", "User context not found")
		return false
	}
	model := ""
	moderationBody := body
	if platform == service.PlatformGrok {
		parsed := service.ParseGrokMediaRequest(c.GetHeader("Content-Type"), body)
		model, moderationBody = parsed.Model, parsed.ModerationBody()
	} else if h.openAI.gatewayService != nil {
		parsed, err := h.openAI.gatewayService.ParseOpenAIImagesRequest(c, body)
		if err != nil {
			imageTaskJSONError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
			return false
		}
		model, moderationBody = parsed.Model, parsed.ModerationBody()
	}
	if len(moderationBody) == 0 {
		c.Set(securityAuditCompletedContextKey, true)
		return true
	}
	reqLog := requestLogger(c, "handler.async_image.security_audit",
		zap.Int64("user_id", subject.UserID), zap.Int64("api_key_id", apiKey.ID), zap.String("model", model))
	decision := h.openAI.checkSecurityAudit(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIImages, model, moderationBody)
	if decision != nil && !decision.AllowNextStage {
		h.openAI.openAISecurityAuditError(c, decision)
		return false
	}
	return true
}

func (h *AsyncImageHandler) Get(c *gin.Context) {
	// Polling deliberately does not require the feature to be enabled, only that
	// the task store is reachable. Turning the switch off in the admin UI must not
	// strand tasks that were already accepted — their results are still in Redis
	// and their submitters are still polling.
	if !h.pollable() {
		imageTaskJSONError(c, http.StatusNotFound, "not_found_error", "async image tasks are not enabled")
		return
	}
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.UserID <= 0 || apiKey.ID <= 0 {
		imageTaskError(c, service.ErrImageTaskForbidden)
		return
	}
	task, err := h.tasks.Get(c.Request.Context(), service.ImageTaskOwner{UserID: apiKey.UserID, APIKeyID: apiKey.ID}, c.Param("task_id"))
	if err != nil {
		imageTaskError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	if task.Status == service.ImageTaskStatusProcessing {
		c.Header("Retry-After", "3")
	}
	c.JSON(http.StatusOK, task)
}

// GetByRequest 凭幂等键（request_id）找回原任务。
//
// 解决的是最棘手的一类场景：提交请求因超时、连接中断或 502 而丢失响应时，
// 调用方手里只剩 request_id。有了这个端点就能找回原任务继续等待，而不是
// 重试——重试会真的再生成一次、再扣一次费。
func (h *AsyncImageHandler) GetByRequest(c *gin.Context) {
	if !h.pollable() {
		imageTaskJSONError(c, http.StatusNotFound, "not_found_error", "async image tasks are not enabled")
		return
	}
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.UserID <= 0 || apiKey.ID <= 0 {
		imageTaskError(c, service.ErrImageTaskForbidden)
		return
	}
	requestID := strings.TrimSpace(c.Param("request_id"))
	if requestID == "" {
		imageTaskJSONError(c, http.StatusBadRequest, "invalid_request_error", "request_id is required")
		return
	}
	coordinator := service.DefaultIdempotencyCoordinator()
	if coordinator == nil {
		imageTaskError(c, service.ErrImageTaskUnavailable)
		return
	}
	// scope 与提交时同源，owner 隔离由它承担：拿别人的 request_id 查不到记录。
	lookup, err := coordinator.Lookup(c.Request.Context(),
		service.ImageTaskIdempotencyScope(imageTaskActorScope(c)), requestID)
	if err != nil {
		imageTaskError(c, err)
		return
	}
	taskID := imageTaskTaskIDFromStoredResponse(lookup)
	if taskID == "" {
		// 未知 request_id，或该次提交本身失败、没有任务可找回。
		imageTaskError(c, service.ErrImageTaskNotFound)
		return
	}
	task, err := h.tasks.Get(c.Request.Context(), service.ImageTaskOwner{UserID: apiKey.UserID, APIKeyID: apiKey.ID}, taskID)
	if err != nil {
		imageTaskError(c, err)
		return
	}
	task.RequestID = requestID
	c.Header("Cache-Control", "no-store")
	if task.Status == service.ImageTaskStatusProcessing {
		c.Header("Retry-After", "3")
	}
	c.JSON(http.StatusOK, task)
}

// imageTaskTaskIDFromStoredResponse 从幂等记录缓存的提交响应里取出 task_id。
// 提交失败（记录处于 backoff/失败态）时没有响应体，返回空串。
func imageTaskTaskIDFromStoredResponse(lookup *service.IdempotencyLookupResult) string {
	if lookup == nil || !lookup.Found || lookup.ResponseBody == nil {
		return ""
	}
	stored := strings.TrimSpace(*lookup.ResponseBody)
	if stored == "" {
		return ""
	}
	var envelope struct {
		TaskID string `json:"task_id"`
	}
	if json.Unmarshal([]byte(stored), &envelope) != nil {
		return ""
	}
	return strings.TrimSpace(envelope.TaskID)
}

func (h *AsyncImageHandler) validateRequest(c *gin.Context, platform string, body []byte) error {
	if h.openAI == nil || h.openAI.gatewayService == nil {
		return nil
	}
	if platform == service.PlatformGrok {
		parsed := service.ParseGrokMediaRequest(c.GetHeader("Content-Type"), body)
		if strings.TrimSpace(parsed.Model) == "" {
			return errors.New("model is required")
		}
		return nil
	}
	parsed, err := h.openAI.gatewayService.ParseOpenAIImagesRequest(c, body)
	if err != nil {
		return err
	}
	if parsed.Stream {
		return errors.New("streaming image requests cannot be submitted as asynchronous tasks")
	}
	return nil
}

func (h *AsyncImageHandler) executeWithGateway(platform string, c *gin.Context) {
	if h.openAI == nil {
		imageTaskJSONError(c, http.StatusServiceUnavailable, "api_error", "image gateway is unavailable")
		return
	}
	if platform == service.PlatformGrok {
		h.openAI.GrokImages(c)
		return
	}
	h.openAI.Images(c)
}

func (h *AsyncImageHandler) run(taskID, platform string, taskCtx *gin.Context, recorder *httptest.ResponseRecorder, cancel context.CancelFunc) {
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.L().Error("image_task.execution_panicked", zap.String("task_id", taskID), zap.Any("panic", recovered))
			h.failTask(taskID, http.StatusInternalServerError, imageTaskErrorPayload("api_error", "image generation task panicked"))
		}
	}()

	h.execute(platform, taskCtx)
	body := bytes.TrimSpace(recorder.Body.Bytes())
	if err := taskCtx.Request.Context().Err(); err != nil && len(body) == 0 {
		h.failTask(taskID, http.StatusGatewayTimeout, imageTaskErrorPayload("timeout_error", "image generation task timed out"))
		return
	}
	statusCode := recorder.Code
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	if statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices {
		if len(body) == 0 || !json.Valid(body) {
			h.failTask(taskID, http.StatusBadGateway, imageTaskErrorPayload("api_error", "upstream returned an invalid image response"))
			return
		}
		if err := h.tasks.Complete(context.Background(), taskID, statusCode, json.RawMessage(body)); err != nil {
			logger.L().Error("image_task.complete_store_failed", zap.String("task_id", taskID), zap.Error(err))
		}
		return
	}
	h.failTask(taskID, statusCode, extractImageTaskError(body))
}

func (h *AsyncImageHandler) failTask(taskID string, statusCode int, taskErr json.RawMessage) {
	if err := h.tasks.Fail(context.Background(), taskID, statusCode, taskErr); err != nil {
		logger.L().Error("image_task.failure_store_failed", zap.String("task_id", taskID), zap.Error(err))
	}
}

func newAsyncImageContext(c *gin.Context, body []byte, timeoutDuration time.Duration) (*gin.Context, *httptest.ResponseRecorder, context.CancelFunc) {
	base := context.WithoutCancel(c.Request.Context())
	executionCtx, cancel := context.WithTimeout(base, timeoutDuration)
	request := c.Request.Clone(executionCtx)
	request.Body = io.NopCloser(bytes.NewReader(body))
	request.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	request.ContentLength = int64(len(body))
	request.URL.Path = strings.TrimSuffix(request.URL.Path, "/async")

	taskCtx := c.Copy()
	recorder := httptest.NewRecorder()
	recorderCtx, _ := gin.CreateTestContext(recorder)
	taskCtx.Writer = recorderCtx.Writer
	taskCtx.Request = request
	return taskCtx, recorder, cancel
}

func asyncImageRequestStreams(contentType string, body []byte) bool {
	if isMultipartImagesContentType(contentType) {
		return false
	}
	var envelope struct {
		Stream bool `json:"stream"`
	}
	return json.Unmarshal(body, &envelope) == nil && envelope.Stream
}

func imageTaskPollURL(submitPath, taskID string) string {
	if strings.HasPrefix(submitPath, "/v1/") {
		return "/v1/images/tasks/" + taskID
	}
	return "/images/tasks/" + taskID
}

func extractImageTaskError(body []byte) json.RawMessage {
	if json.Valid(body) {
		var envelope struct {
			Error json.RawMessage `json:"error"`
		}
		if json.Unmarshal(body, &envelope) == nil && len(envelope.Error) > 0 && json.Valid(envelope.Error) {
			return envelope.Error
		}
		return json.RawMessage(body)
	}
	return imageTaskErrorPayload("api_error", "image generation failed")
}

func imageTaskErrorPayload(errorType, message string) json.RawMessage {
	data, _ := json.Marshal(gin.H{"type": errorType, "message": message})
	return data
}

func imageTaskError(c *gin.Context, err error) {
	status := infraerrors.Code(err)
	code := infraerrors.Reason(err)
	message := infraerrors.Message(err)
	if status <= 0 {
		status = http.StatusInternalServerError
	}
	if strings.TrimSpace(code) == "" {
		code = "IMAGE_TASK_ERROR"
	}
	imageTaskJSONError(c, status, code, message)
}

func imageTaskJSONError(c *gin.Context, status int, code, message string) {
	c.Header("Cache-Control", "no-store")
	c.JSON(status, gin.H{"error": gin.H{"type": code, "code": code, "message": message}})
}
