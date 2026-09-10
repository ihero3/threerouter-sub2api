package service

// video_task_service.go — 视频任务业务逻辑。
// 负责：调度选 channel → model_mapping 翻译 → 调 adapter → 写表 → 返回。
// 复用现有 GatewayService 调度器和 RateLimitService 故障转移逻辑。

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// VideoTaskRepo 视频任务仓储接口（service 层定义，repository 层实现）。
// 避免直接 import repository 包导致 import cycle。
type VideoTaskRepo interface {
	Create(ctx context.Context, task *VideoTaskRecord) (*VideoTaskRecord, error)
	GetByLocalID(ctx context.Context, localID string) (*VideoTaskRecord, error)
	GetByID(ctx context.Context, id int64) (*VideoTaskRecord, error)
	UpdateStatus(ctx context.Context, id int64, status, errorMsg string) error
	UpdateResult(ctx context.Context, id int64, status, videoURL, thumbnailURL string, durationSec int, costUSD float64) (bool, error)
	UpdateUpstreamTaskID(ctx context.Context, id int64, upstreamTaskID string) error
	ListByUserID(ctx context.Context, userID int64, limit, offset int) ([]*VideoTaskRecord, int, error)
	ListProcessingTasks(ctx context.Context, before time.Time, limit int) ([]*VideoTaskRecord, error)
	ListAdmin(ctx context.Context, userID int64, status string, limit, offset int) ([]*VideoTaskRecord, int, error)
}

// VideoTaskRecord 是 video_tasks 表的 Go 映射。
type VideoTaskRecord struct {
	ID             int64
	LocalID        string
	UserID         int64
	APIKeyID       int64
	PublicModel    string
	UpstreamModel  string
	AccountID      int64
	UpstreamTaskID string
	Status         string
	Resolution     string
	DurationSec    int
	VideoURL       string
	ThumbnailURL   string
	RequestBody    map[string]any
	ErrorMessage   string
	CostUSD        float64
	ReservedCost   *float64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	FinishedAt     *time.Time
}

// VideoTaskService 视频任务业务服务。
// maxVideoTaskDurationBeforeFail 是视频任务轮询超时阈值：任务创建后超过该时长
// 仍处于 processing（上游一直不回结果），直接标记 failed，避免 Worker 无限轮询。
const maxVideoTaskDurationBeforeFail = 30 * time.Minute

type VideoTaskService struct {
	videoTaskRepo        VideoTaskRepo
	accountService       *AccountService
	gatewayService       *GatewayService
	rateLimitService     *RateLimitService
	billingService       *BillingService
	apiKeyService        *APIKeyService
	openAIGatewayService *OpenAIGatewayService
	adapter              *VideoAdapterRegistry
	logger               *zap.Logger
}

// NewVideoTaskService 创建视频任务服务实例。
func NewVideoTaskService(
	videoTaskRepo VideoTaskRepo,
	accountService *AccountService,
	gatewayService *GatewayService,
	rateLimitService *RateLimitService,
	billingService *BillingService,
	apiKeyService *APIKeyService,
	openAIGatewayService *OpenAIGatewayService,
	adapter *VideoAdapterRegistry,
) *VideoTaskService {
	return &VideoTaskService{
		videoTaskRepo:        videoTaskRepo,
		accountService:       accountService,
		gatewayService:       gatewayService,
		rateLimitService:     rateLimitService,
		billingService:       billingService,
		apiKeyService:        apiKeyService,
		openAIGatewayService: openAIGatewayService,
		adapter:              adapter,
		logger:               logger.L(),
	}
}

// billingDeps 提取视频结算管线依赖（见 video_task_billing.go）。
func (s *VideoTaskService) billingDeps() *videoTaskBillingDeps {
	return &videoTaskBillingDeps{
		billingService:       s.billingService,
		apiKeyService:        s.apiKeyService,
		openAIGatewayService: s.openAIGatewayService,
	}
}

// CreateTask 处理用户视频生成请求。
// 流程：解析统一请求 → 选上游账号 → model_mapping 翻译 → 调 adapter → 写表。
// 当上游创建返回可重试错误（401/403/429/5xx 等）时，会排除该账号继续选下一个，
// 最多切换 3 次；耗尽后返回最后一个可识别的错误。
func (s *VideoTaskService) CreateTask(c *gin.Context, groupID *int64, userID int64, apiKeyID int64, publicModel string, requestBody map[string]any) (*VideoTaskRecord, error) {
	req, err := parseVideoCreateRequest(publicModel, requestBody)
	if err != nil {
		return nil, fmt.Errorf("video_task_service: parse request: %w", err)
	}

	ctx := c.Request.Context()
	excluded := make(map[int64]struct{})
	var lastUpstreamErr error
	const maxAttempts = 100

	for attempt := 0; attempt < maxAttempts; attempt++ {
		account, selectErr := s.gatewayService.SelectAccountForModelWithExclusions(ctx, groupID, "", publicModel, excluded)
		if selectErr != nil || account == nil {
			if lastUpstreamErr != nil {
				return nil, lastUpstreamErr
			}
			if selectErr == nil {
				selectErr = fmt.Errorf("video_task_service: no available account for model %s", publicModel)
			}
			if errors.Is(selectErr, ErrNoAvailableAccounts) {
				return nil, fmt.Errorf("video_task_service: no available account for model %s", publicModel)
			}
			return nil, fmt.Errorf("video_task_service: select account: %w", selectErr)
		}

		mapping := account.GetModelMapping()
		upstreamModel := publicModel
		if mapped, ok := mapping[publicModel]; ok && mapped != "" {
			upstreamModel = mapped
		}
		req.PublicModel = publicModel
		req.UpstreamModel = upstreamModel

		adapter, resolveErr := s.adapter.Resolve(account.Platform, upstreamModel)
		if resolveErr != nil {
			return nil, fmt.Errorf("video_task_service: resolve adapter: %w", resolveErr)
		}

		createResult, createErr := adapter.Create(ctx, account, *req)
		if createErr != nil {
			s.recordMediaTransportFailure(c, ctx, account, createErr)
			s.logger.Warn("video_task_service: upstream create failed",
				zap.Int64("account_id", account.ID),
				zap.String("model", upstreamModel),
				zap.Error(createErr),
			)
			excluded[account.ID] = struct{}{}
			lastUpstreamErr = fmt.Errorf("video_task_service: upstream create: %w", createErr)
			continue
		}

		if createResult.Status == "failed" && createResult.Mode == VideoCompletionFailed {
			s.logger.Warn("video_task_service: upstream returned failure",
				zap.Int64("account_id", account.ID),
				zap.String("model", upstreamModel),
				zap.Int("upstream_status", createResult.UpstreamStatusCode),
				zap.String("error", createResult.ErrorMessage),
			)
			failureBody := []byte(createResult.ErrorMessage)
			decision := classifyMediaUpstreamFailure(createResult.UpstreamStatusCode, failureBody)
			s.recordMediaUpstreamFailure(c, ctx, account, createResult.UpstreamStatusCode, failureBody, publicModel)
			if decision.ShouldFailover {
				excluded[account.ID] = struct{}{}
				lastUpstreamErr = fmt.Errorf("video_task_service: upstream failed with status %d: %s",
					createResult.UpstreamStatusCode, createResult.ErrorMessage)
				continue
			}
		}

		localID := generateVideoLocalID()
		record := &VideoTaskRecord{
			LocalID:        localID,
			UserID:         userID,
			APIKeyID:       apiKeyID,
			PublicModel:    publicModel,
			UpstreamModel:  upstreamModel,
			AccountID:      account.ID,
			UpstreamTaskID: createResult.TaskID,
			Status:         createResult.Status,
			Resolution:     req.Resolution,
			DurationSec:    req.DurationSec,
			RequestBody:    requestBody,
			ErrorMessage:   createResult.ErrorMessage,
		}
		if createResult.InlineVideoURL != "" {
			record.VideoURL = createResult.InlineVideoURL
		}
		if createResult.Status == "failed" {
			// 不触发 failover 的上游失败：无预扣可退，仍写 0 费用日志保持审计完整
			settleVideoTaskFailure(ctx, s.billingDeps(), &videoTaskBillingInput{
				LocalID:       record.LocalID,
				UserID:        record.UserID,
				APIKeyID:      record.APIKeyID,
				AccountID:     record.AccountID,
				Account:       account,
				Model:         record.PublicModel,
				UpstreamModel: record.UpstreamModel,
				Resolution:    record.Resolution,
				DurationSec:   record.DurationSec,
			})
		} else if cost, costErr := estimateVideoTaskCost(ctx, s.billingDeps(), apiKeyID, publicModel, req.Resolution, req.DurationSec); costErr == nil && cost > 0 {
			record.ReservedCost = &cost
			if s.apiKeyService != nil {
				_ = s.apiKeyService.UpdateQuotaUsed(ctx, apiKeyID, cost)
			}
		}

		saved, saveErr := s.videoTaskRepo.Create(ctx, record)
		if saveErr != nil {
			return nil, fmt.Errorf("video_task_service: save task: %w", saveErr)
		}
		return saved, nil
	}

	if lastUpstreamErr == nil {
		lastUpstreamErr = fmt.Errorf("video_task_service: upstream account switches exhausted")
	}
	return nil, lastUpstreamErr
}

// recordMediaUpstreamFailure 复用媒体错误分类，让视频旧链路同样冷却坏账号。
func (s *VideoTaskService) recordMediaUpstreamFailure(c *gin.Context, ctx context.Context, account *Account, statusCode int, responseBody []byte, requestedModel string) {
	decision := classifyMediaUpstreamFailure(statusCode, responseBody)
	if account != nil {
		message := strings.TrimSpace(sanitizeUpstreamErrorMessage(string(responseBody)))
		if message == "" && statusCode != 0 {
			message = strings.TrimSpace(sanitizeUpstreamErrorMessage(string(decision.Class)))
		}
		safeBody := string(truncateForLog([]byte(message), 1024))
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:             account.Platform,
			AccountID:            account.ID,
			AccountName:          account.Name,
			UpstreamStatusCode:   statusCode,
			UpstreamResponseBody: safeBody,
			Kind:                 "http_error",
			Stage:                "media_upstream",
			Reason:               string(decision.Class),
			Message:              message,
		})
	}
	if !decision.ShouldCooldown || account == nil {
		return
	}
	if s.rateLimitService != nil && (decision.Class == MediaFailureAuth || decision.Class == MediaFailureBillingQuota) {
		_ = s.rateLimitService.HandleUpstreamError(ctx, account, statusCode, nil, responseBody, requestedModel)
		return
	}
	if s.rateLimitService != nil {
		until := time.Now().Add(decision.Cooldown)
		if s.rateLimitService.SetMediaTempUnschedulable(ctx, account.ID, until, "media upstream failure: "+string(decision.Class)) {
			return
		}
	}
	if s.accountService == nil {
		return
	}
	until := time.Now().Add(decision.Cooldown)
	_ = s.accountService.SetTempUnschedulable(ctx, account.ID, until, "media upstream failure: "+string(decision.Class))
}

// recordMediaTransportFailure 持久网络故障停调账号；瞬时网络故障只切换账号。
func (s *VideoTaskService) recordMediaTransportFailure(c *gin.Context, ctx context.Context, account *Account, createErr error) {
	if account == nil || createErr == nil {
		return
	}
	safeErr := strings.TrimSpace(sanitizeUpstreamErrorMessage(createErr.Error()))
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Platform:           account.Platform,
		AccountID:          account.ID,
		AccountName:        account.Name,
		UpstreamStatusCode: 0,
		Kind:               "request_error",
		Stage:              "media_upstream",
		Reason:             "transport_error",
		Message:            safeErr,
	})
	if !classifyUpstreamTransportError(createErr).Persistent {
		return
	}
	until := time.Now().Add(10 * time.Minute)
	if s.rateLimitService == nil || !s.rateLimitService.SetMediaTempUnschedulable(ctx, account.ID, until, "media upstream transport error: "+createErr.Error()) {
		if s.accountService != nil {
			_ = s.accountService.SetTempUnschedulable(ctx, account.ID, until, "media upstream transport error: "+createErr.Error())
		}
	}
}

// GetTask 查询任务状态。如果任务仍在 processing，尝试向上游刷新状态。
func (s *VideoTaskService) GetTask(ctx context.Context, localID string, userID int64) (*VideoTaskRecord, error) {
	record, err := s.videoTaskRepo.GetByLocalID(ctx, localID)
	if err != nil {
		return nil, fmt.Errorf("video_task_service: get task: %w", err)
	}
	if record.UserID != userID {
		return nil, fmt.Errorf("video_task_service: task not found for user")
	}

	// 如果任务还在 processing，尝试刷新
	if record.Status == "processing" && record.UpstreamTaskID != "" {
		s.refreshTaskStatus(ctx, record)
	}

	return record, nil
}

// PollTask 供 Worker 调用：轮询单个 processing 任务的上游状态。
func (s *VideoTaskService) PollTask(ctx context.Context, record *VideoTaskRecord) error {
	if record.Status != "processing" || record.UpstreamTaskID == "" {
		return nil
	}
	if !record.CreatedAt.IsZero() && time.Since(record.CreatedAt) > maxVideoTaskDurationBeforeFail {
		s.logger.Warn("video_task_service: task timed out, marking failed",
			zap.Int64("task_id", record.ID),
			zap.String("local_id", record.LocalID),
			zap.Time("created_at", record.CreatedAt),
		)
		if err := s.videoTaskRepo.UpdateStatus(ctx, record.ID, "failed", "upstream task timed out"); err != nil {
			return fmt.Errorf("video_task_service: timeout update status: %w", err)
		}
		settleVideoTaskFailure(ctx, s.billingDeps(), &videoTaskBillingInput{
			LocalID:       record.LocalID,
			UserID:        record.UserID,
			APIKeyID:      record.APIKeyID,
			AccountID:     record.AccountID,
			Model:         record.PublicModel,
			UpstreamModel: record.UpstreamModel,
			Resolution:    record.Resolution,
			DurationSec:   record.DurationSec,
			ReservedCost:  record.ReservedCost,
		})
		return nil
	}
	return s.refreshTaskStatus(ctx, record)
}

// refreshTaskStatus 向上游查询并更新本地任务状态。
func (s *VideoTaskService) refreshTaskStatus(ctx context.Context, record *VideoTaskRecord) error {
	// 获取 account
	account, err := s.accountService.GetByID(ctx, record.AccountID)
	if err != nil || account == nil {
		return fmt.Errorf("video_task_service: get account %d: %w", record.AccountID, err)
	}

	adapter, err := s.adapter.Resolve(account.Platform, record.UpstreamModel)
	if err != nil {
		return fmt.Errorf("video_task_service: resolve adapter: %w", err)
	}
	result, err := adapter.GetResult(ctx, account, record.UpstreamTaskID)
	if err != nil {
		return fmt.Errorf("video_task_service: get upstream result: %w", err)
	}

	switch result.Status {
	case "succeeded":
		actual := 0.0
		if est, estErr := estimateVideoTaskCost(ctx, s.billingDeps(), record.APIKeyID, record.PublicModel, record.Resolution, result.DurationSec); estErr == nil {
			actual = est
		} else {
			s.logger.Warn("video_task_service: estimate cost failed",
				zap.Int64("task_id", record.ID),
				zap.Error(estErr),
			)
		}
		claimed, err := s.videoTaskRepo.UpdateResult(ctx, record.ID, "succeeded", result.VideoURL, result.ThumbnailURL, result.DurationSec, actual)
		if err != nil {
			return fmt.Errorf("video_task_service: update result: %w", err)
		}
		if claimed {
			// claimed 守卫保证同一任务只结算一次：实际按秒计费（余额/订阅/Key配额）
			// + usage_logs 幂等落库 + 退还预扣，见 video_task_billing.go。
			settleVideoTaskSuccess(ctx, s.billingDeps(), &videoTaskBillingInput{
				LocalID:       record.LocalID,
				UserID:        record.UserID,
				APIKeyID:      record.APIKeyID,
				AccountID:     record.AccountID,
				Account:       account,
				Model:         record.PublicModel,
				UpstreamModel: record.UpstreamModel,
				Resolution:    record.Resolution,
				DurationSec:   result.DurationSec,
				ReservedCost:  record.ReservedCost,
			})
		}
	case "failed", "cancelled":
		if err := s.videoTaskRepo.UpdateStatus(ctx, record.ID, result.Status, result.ErrorMessage); err != nil {
			return fmt.Errorf("video_task_service: update status: %w", err)
		}
		settleVideoTaskFailure(ctx, s.billingDeps(), &videoTaskBillingInput{
			LocalID:       record.LocalID,
			UserID:        record.UserID,
			APIKeyID:      record.APIKeyID,
			AccountID:     record.AccountID,
			Account:       account,
			Model:         record.PublicModel,
			UpstreamModel: record.UpstreamModel,
			Resolution:    record.Resolution,
			DurationSec:   result.DurationSec,
			ReservedCost:  record.ReservedCost,
		})
	default:
		// still processing, no update needed
	}

	return nil
}

// ListAdmin 管理后台全量列表（status 在 SQL 层过滤）。
func (s *VideoTaskService) ListAdmin(ctx context.Context, userID int64, status string, limit, offset int) ([]*VideoTaskRecord, int, error) {
	return s.videoTaskRepo.ListAdmin(ctx, userID, status, limit, offset)
}

// ListTasksByUserID 查询指定用户的视频任务列表（分页）。
func (s *VideoTaskService) ListTasksByUserID(ctx context.Context, userID int64, limit, offset int) ([]*VideoTaskRecord, int, error) {
	return s.videoTaskRepo.ListByUserID(ctx, userID, limit, offset)
}

// GetTaskByLocalID 按 local_id 查询任务（管理后台用，不校验 user_id）。
func (s *VideoTaskService) GetTaskByLocalID(ctx context.Context, localID string) (*VideoTaskRecord, error) {
	record, err := s.videoTaskRepo.GetByLocalID(ctx, localID)
	if err != nil {
		return nil, fmt.Errorf("video_task_service: get task by local_id: %w", err)
	}
	return record, nil
}

// GetTaskByID 按主键 ID 查询任务（管理后台用，不校验 user_id）。
func (s *VideoTaskService) GetTaskByID(ctx context.Context, id int64) (*VideoTaskRecord, error) {
	record, err := s.videoTaskRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("video_task_service: get task by id: %w", err)
	}
	return record, nil
}

// CancelTask 管理员手动取消任务：仅更新本地状态为 cancelled。
// 不调上游 cancel API（上游 cancel 支持不一致），上游任务自然超时或被忽略。
func (s *VideoTaskService) CancelTask(ctx context.Context, id int64) error {
	record, err := s.videoTaskRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("video_task_service: get task before cancel: %w", err)
	}
	// 尽力同步上游取消（视频厂商若支持 Cancel 接口则调用，失败忽略）。
	if record.UpstreamTaskID != "" {
		account, accErr := s.accountService.GetByID(ctx, record.AccountID)
		if accErr == nil && account != nil {
			if adapter, rErr := s.adapter.Resolve(account.Platform, record.UpstreamModel); rErr == nil {
				if canceller, ok := adapter.(interface {
					Cancel(ctx context.Context, account *Account, upstreamTaskID string) error
				}); ok {
					if cErr := canceller.Cancel(ctx, account, record.UpstreamTaskID); cErr != nil {
						s.logger.Warn("video_task_service: upstream cancel failed (ignored)",
							zap.Int64("task_id", record.ID),
							zap.Int64("account_id", record.AccountID),
							zap.Error(cErr),
						)
					}
				}
			}
		}
	}
	if err := s.videoTaskRepo.UpdateStatus(ctx, id, "cancelled", "cancelled by admin"); err != nil {
		return fmt.Errorf("video_task_service: cancel task: %w", err)
	}
	settleVideoTaskFailure(ctx, s.billingDeps(), &videoTaskBillingInput{
		LocalID:       record.LocalID,
		UserID:        record.UserID,
		APIKeyID:      record.APIKeyID,
		AccountID:     record.AccountID,
		Model:         record.PublicModel,
		UpstreamModel: record.UpstreamModel,
		Resolution:    record.Resolution,
		DurationSec:   record.DurationSec,
		ReservedCost:  record.ReservedCost,
	})
	return nil
}

// parseVideoCreateRequest 从请求 body 解析统一参数。
func parseVideoCreateRequest(model string, body map[string]any) (*VideoCreateRequest, error) {
	req := &VideoCreateRequest{
		PublicModel: model,
	}

	if v, ok := body["prompt"].(string); ok {
		req.Prompt = v
	}
	if v, ok := body["negative_prompt"].(string); ok {
		req.NegativePrompt = v
	}
	if v, ok := body["resolution"].(string); ok {
		req.Resolution = v
	}
	if v, ok := body["ratio"].(string); ok {
		req.Ratio = v
	}
	if v, ok := body["duration"]; ok {
		switch d := v.(type) {
		case float64:
			req.DurationSec = int(d)
		case int:
			req.DurationSec = d
		}
	}
	if v, ok := body["duration_sec"]; ok {
		switch d := v.(type) {
		case float64:
			req.DurationSec = int(d)
		case int:
			req.DurationSec = d
		}
	}
	// 图片参考。除 OpenAI 风格的 image_url / image_urls 外，还要接受
	// 各家文档常用的 image 字段：字符串、字符串数组，以及 {"url": "..."} 对象。
	if items, ok := body["image"].([]any); ok {
		for _, raw := range items {
			if u := videoRefURLFromAny(raw); u != "" {
				req.ImageRefURLs = append(req.ImageRefURLs, u)
			}
		}
	} else if u := videoRefURLFromAny(body["image"]); u != "" {
		req.ImageRefURLs = append(req.ImageRefURLs, u)
	}
	if v, ok := body["image_url"].(string); ok && v != "" {
		req.ImageRefURLs = append(req.ImageRefURLs, v)
	}
	if urls, ok := body["image_urls"].([]any); ok {
		for _, u := range urls {
			if s, ok := u.(string); ok && s != "" {
				req.ImageRefURLs = append(req.ImageRefURLs, s)
			}
		}
	}
	// 视频参考
	if v, ok := body["video_url"].(string); ok && v != "" {
		req.VideoRefURLs = []string{v}
	}
	if urls, ok := body["video_urls"].([]any); ok {
		for _, u := range urls {
			if s, ok := u.(string); ok && s != "" {
				req.VideoRefURLs = append(req.VideoRefURLs, s)
			}
		}
	}
	// 音频参考
	if v, ok := body["audio_url"].(string); ok && v != "" {
		req.AudioRefURLs = []string{v}
	}
	if urls, ok := body["audio_urls"].([]any); ok {
		for _, u := range urls {
			if s, ok := u.(string); ok && s != "" {
				req.AudioRefURLs = append(req.AudioRefURLs, s)
			}
		}
	}
	if media, ok := body["media"].([]any); ok {
		for _, raw := range media {
			obj, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			mediaType, _ := obj["type"].(string)
			mediaURL, _ := obj["url"].(string)
			mediaType = strings.TrimSpace(mediaType)
			mediaURL = strings.TrimSpace(mediaURL)
			if mediaType == "" || mediaURL == "" {
				continue
			}
			req.Media = append(req.Media, VideoMediaInput{Type: mediaType, URL: mediaURL})
		}
	}
	// 种子
	if v, ok := body["seed"]; ok {
		switch seed := v.(type) {
		case float64:
			s := int64(seed)
			req.Seed = &s
		case int64:
			req.Seed = &seed
		}
	}

	// 保留未被上述统一字段消费的请求参数，交给具体 adapter 透传。
	// adapter 的 mergeVideoExtra 会再次过滤统一字段，避免原始值覆盖规范化结果。
	if len(body) > 0 {
		req.Extra = make(map[string]any, len(body))
		for k, v := range body {
			req.Extra[k] = v
		}
	}

	return req, nil
}

// generateVideoLocalID 生成唯一的本地任务 ID，格式：vid_<16hex>。
func generateVideoLocalID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "vid_" + hex.EncodeToString(b)
}
