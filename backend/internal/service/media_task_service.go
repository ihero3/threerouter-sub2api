package service

// media_task_service.go — 媒体任务统一业务逻辑（图片 / 视频 / 音频）。
// 负责：解析统一请求 → 选上游账号（同类型多账号自动轮转）→ 调 adapter →
// 写表 → 异步轮询 → 幂等计费。复用现有 GatewayService 调度器 + BillingService。

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// MediaTaskRepo 媒体任务仓储接口（service 层定义，repository 层实现）。
type MediaTaskRepo interface {
	Create(ctx context.Context, task *MediaTaskRecord) (*MediaTaskRecord, error)
	GetByLocalID(ctx context.Context, localID string) (*MediaTaskRecord, error)
	GetByID(ctx context.Context, id int64) (*MediaTaskRecord, error)
	UpdateStatus(ctx context.Context, id int64, status, errorMsg string) error
	UpdateResult(ctx context.Context, id int64, status, mediaURL, thumbnailURL string, durationSec int, costUSD float64) (bool, error)
	UpdateUpstreamTaskID(ctx context.Context, id int64, upstreamTaskID string) error
	ListByUserID(ctx context.Context, userID int64, limit, offset int) ([]*MediaTaskRecord, int, error)
	ListProcessingTasks(ctx context.Context, before time.Time, limit int) ([]*MediaTaskRecord, error)
	ListAdmin(ctx context.Context, userID int64, status, mediaKind string, limit, offset int) ([]*MediaTaskRecord, int, error)
}

// MediaTaskRecord 是 media_tasks 表的 Go 映射。
type MediaTaskRecord struct {
	ID             int64
	LocalID        string
	MediaKind      MediaKind
	UserID         int64
	APIKeyID       int64
	PublicModel    string
	UpstreamModel  string
	AccountID      int64
	UpstreamTaskID string
	Status         string
	Resolution     string
	DurationSec    int
	MediaURL       string
	ThumbnailURL   string
	RequestBody    map[string]any
	ErrorMessage   string
	CostUSD        float64
	ReservedCost   *float64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	FinishedAt     *time.Time
}

// MediaTaskService 媒体任务统一业务服务。
// maxMediaTaskDurationBeforeFail 是媒体任务轮询超时阈值：任务创建后超过该时长
// 仍处于 processing（上游一直不回结果），直接标记 failed，避免 Worker 无限轮询。
const maxMediaTaskDurationBeforeFail = 30 * time.Minute

type MediaTaskService struct {
	mediaTaskRepo        MediaTaskRepo
	accountService       *AccountService
	gatewayService       *GatewayService
	rateLimitService     *RateLimitService
	billingService       *BillingService
	apiKeyService        *APIKeyService
	openAIGatewayService *OpenAIGatewayService
	adapter              *MediaAdapterRegistry
	imageStorageSetting  *ImageStorageSettingService
	logger               *zap.Logger
}

// NewMediaTaskService 创建媒体任务服务实例。
func NewMediaTaskService(
	mediaTaskRepo MediaTaskRepo,
	accountService *AccountService,
	gatewayService *GatewayService,
	rateLimitService *RateLimitService,
	billingService *BillingService,
	apiKeyService *APIKeyService,
	openAIGatewayService *OpenAIGatewayService,
	adapter *MediaAdapterRegistry,
	imageStorageSetting *ImageStorageSettingService,
) *MediaTaskService {
	return &MediaTaskService{
		mediaTaskRepo:        mediaTaskRepo,
		accountService:       accountService,
		gatewayService:       gatewayService,
		rateLimitService:     rateLimitService,
		billingService:       billingService,
		apiKeyService:        apiKeyService,
		openAIGatewayService: openAIGatewayService,
		adapter:              adapter,
		imageStorageSetting:  imageStorageSetting,
		logger:               logger.L(),
	}
}

// billingDeps 提取视频结算管线依赖（见 video_task_billing.go）。
func (s *MediaTaskService) billingDeps() *videoTaskBillingDeps {
	return &videoTaskBillingDeps{
		billingService:       s.billingService,
		apiKeyService:        s.apiKeyService,
		openAIGatewayService: s.openAIGatewayService,
	}
}

// CreateTask 处理用户媒体生成请求。
// 流程：解析统一请求 → 选上游账号（同类型自动轮转）→ 调 adapter → 写 media_tasks。
func (s *MediaTaskService) CreateTask(c *gin.Context, kind MediaKind, groupID *int64, userID int64, apiKeyID int64, publicModel string, requestBody map[string]any) (*MediaTaskRecord, error) {
	req, err := parseMediaCreateRequest(kind, publicModel, requestBody)
	if err != nil {
		return nil, fmt.Errorf("media_task_service: parse request: %w", err)
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
				selectErr = fmt.Errorf("media_task_service: no available account for model %s", publicModel)
			}
			if errors.Is(selectErr, ErrNoAvailableAccounts) {
				return nil, fmt.Errorf("media_task_service: no available account for model %s", publicModel)
			}
			return nil, fmt.Errorf("media_task_service: select account: %w", selectErr)
		}

		mapping := account.GetModelMapping()
		upstreamModel := publicModel
		if mapped, ok := mapping[publicModel]; ok && mapped != "" {
			upstreamModel = mapped
		}
		req.PublicModel = publicModel
		req.UpstreamModel = upstreamModel

		adapter, resolveErr := s.adapter.Resolve(kind, account.Platform, upstreamModel)
		if resolveErr != nil {
			return nil, fmt.Errorf("media_task_service: resolve adapter: %w", resolveErr)
		}

		createResult, createErr := adapter.Create(ctx, account, *req)
		if createErr != nil {
			s.recordMediaTransportFailure(c, ctx, account, createErr)
			s.logger.Warn("media_task_service: upstream create failed",
				zap.Int64("account_id", account.ID),
				zap.String("kind", string(kind)),
				zap.String("model", upstreamModel),
				zap.Error(createErr),
			)
			excluded[account.ID] = struct{}{}
			lastUpstreamErr = fmt.Errorf("media_task_service: upstream create: %w", createErr)
			continue
		}

		if createResult.Status == "failed" && createResult.Mode == MediaCompletionFailed {
			s.logger.Warn("media_task_service: upstream returned failure",
				zap.Int64("account_id", account.ID),
				zap.String("kind", string(kind)),
				zap.String("model", upstreamModel),
				zap.Int("upstream_status", createResult.UpstreamStatusCode),
				zap.String("error", createResult.ErrorMessage),
			)
			failureBody := []byte(createResult.ErrorMessage)
			decision := classifyMediaUpstreamFailure(createResult.UpstreamStatusCode, failureBody)
			s.recordMediaUpstreamFailure(c, ctx, account, createResult.UpstreamStatusCode, failureBody, publicModel)
			if decision.ShouldFailover {
				excluded[account.ID] = struct{}{}
				lastUpstreamErr = fmt.Errorf("media_task_service: upstream failed with status %d: %s",
					createResult.UpstreamStatusCode, createResult.ErrorMessage)
				continue
			}
		}

		localID := generateMediaLocalID(kind)
		record := &MediaTaskRecord{
			LocalID:        localID,
			MediaKind:      kind,
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
		if createResult.InlineURL != "" {
			record.MediaURL = createResult.InlineURL
			if stored, ok := s.maybeStoreMedia(ctx, record, createResult.InlineURL); ok {
				record.MediaURL = stored
			}
		}
		if createResult.Status == "failed" && kind == MediaKindVideo {
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
		} else if createResult.Status != "failed" {
			if kind == MediaKindVideo {
				if cost, costErr := estimateVideoTaskCost(ctx, s.billingDeps(), apiKeyID, publicModel, req.Resolution, req.DurationSec); costErr == nil && cost > 0 {
					record.ReservedCost = &cost
					if s.apiKeyService != nil {
						_ = s.apiKeyService.UpdateQuotaUsed(ctx, apiKeyID, cost)
					}
				}
			} else if cost, costErr := s.calculateMediaCost(ctx, kind, apiKeyID, publicModel, req.Resolution, req.DurationSec); costErr == nil && cost > 0 {
				record.ReservedCost = &cost
				if s.apiKeyService != nil {
					_ = s.apiKeyService.UpdateQuotaUsed(ctx, apiKeyID, cost)
				}
			}
		}

		saved, saveErr := s.mediaTaskRepo.Create(ctx, record)
		if saveErr != nil {
			return nil, fmt.Errorf("media_task_service: save task: %w", saveErr)
		}
		return saved, nil
	}

	if lastUpstreamErr == nil {
		lastUpstreamErr = fmt.Errorf("media_task_service: upstream account switches exhausted")
	}
	return nil, lastUpstreamErr
}

// GetTask 查询任务状态。仍在 processing 则尝试向上游刷新。
func (s *MediaTaskService) GetTask(ctx context.Context, localID string, userID int64) (*MediaTaskRecord, error) {
	record, err := s.mediaTaskRepo.GetByLocalID(ctx, localID)
	if err != nil {
		return nil, fmt.Errorf("media_task_service: get task: %w", err)
	}
	if record.UserID != userID {
		return nil, fmt.Errorf("media_task_service: task not found for user")
	}
	if record.Status == "processing" && record.UpstreamTaskID != "" {
		_ = s.refreshTaskStatus(ctx, record)
	}
	return record, nil
}

// PollTask 供 Worker 调用：轮询单个 processing 任务的上游状态。
func (s *MediaTaskService) PollTask(ctx context.Context, record *MediaTaskRecord) error {
	if record.Status != "processing" || record.UpstreamTaskID == "" {
		return nil
	}
	if !record.CreatedAt.IsZero() && time.Since(record.CreatedAt) > maxMediaTaskDurationBeforeFail {
		s.logger.Warn("media_task_service: task timed out, marking failed",
			zap.Int64("task_id", record.ID),
			zap.String("local_id", record.LocalID),
			zap.Time("created_at", record.CreatedAt),
		)
		if err := s.mediaTaskRepo.UpdateStatus(ctx, record.ID, "failed", "upstream task timed out"); err != nil {
			return fmt.Errorf("media_task_service: timeout update status: %w", err)
		}
		if record.MediaKind == MediaKindVideo {
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
		}
		return nil
	}
	return s.refreshTaskStatus(ctx, record)
}

func (s *MediaTaskService) refreshTaskStatus(ctx context.Context, record *MediaTaskRecord) error {
	account, err := s.accountService.GetByID(ctx, record.AccountID)
	if err != nil || account == nil {
		return fmt.Errorf("media_task_service: get account %d: %w", record.AccountID, err)
	}

	adapter, err := s.adapter.Resolve(record.MediaKind, account.Platform, record.UpstreamModel)
	if err != nil {
		return fmt.Errorf("media_task_service: resolve adapter: %w", err)
	}
	result, err := adapter.GetResult(ctx, account, record.UpstreamTaskID)
	if err != nil {
		return fmt.Errorf("media_task_service: get upstream result: %w", err)
	}

	switch result.Status {
	case "succeeded":
		actual := 0.0
		if record.MediaKind == MediaKindVideo {
			if est, estErr := estimateVideoTaskCost(ctx, s.billingDeps(), record.APIKeyID, record.PublicModel, record.Resolution, result.DurationSec); estErr == nil {
				actual = est
			} else {
				s.logger.Warn("media_task_service: estimate video cost failed",
					zap.Int64("task_id", record.ID),
					zap.Error(estErr),
				)
			}
		} else if cost, costErr := s.calculateMediaCost(ctx, record.MediaKind, record.APIKeyID, record.PublicModel, record.Resolution, result.DurationSec); costErr == nil {
			actual = cost
		} else {
			s.logger.Warn("media_task_service: calculate cost failed",
				zap.Int64("task_id", record.ID),
				zap.Error(costErr),
			)
		}
		mediaURL := result.URL
		if storedURL, ok := s.maybeStoreMedia(ctx, record, mediaURL); ok {
			mediaURL = storedURL
		}
		claimed, err := s.mediaTaskRepo.UpdateResult(ctx, record.ID, "succeeded", mediaURL, result.ThumbnailURL, result.DurationSec, actual)
		if err != nil {
			return fmt.Errorf("media_task_service: update result: %w", err)
		}
		if claimed {
			if record.MediaKind == MediaKindVideo {
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
			} else {
				var reserved float64
				if record.ReservedCost != nil {
					reserved = *record.ReservedCost
				}
				settleMediaReservedQuota(s.apiKeyService, ctx, record.APIKeyID, reserved, actual)
			}
		}
	case "failed", "cancelled":
		if err := s.mediaTaskRepo.UpdateStatus(ctx, record.ID, result.Status, result.ErrorMessage); err != nil {
			return fmt.Errorf("media_task_service: update status: %w", err)
		}
		if record.MediaKind == MediaKindVideo {
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
		} else {
			var reserved float64
			if record.ReservedCost != nil {
				reserved = *record.ReservedCost
			}
			releaseMediaReservedQuota(s.apiKeyService, ctx, record.APIKeyID, reserved)
		}
	default:
		// still processing
	}
	return nil
}

// ListAdmin 管理后台全量列表（条件在 SQL 层过滤）。
func (s *MediaTaskService) ListAdmin(ctx context.Context, userID int64, status, mediaKind string, limit, offset int) ([]*MediaTaskRecord, int, error) {
	return s.mediaTaskRepo.ListAdmin(ctx, userID, status, mediaKind, limit, offset)
}

// maybeStoreMedia 在对象存储可用时把上游媒体 URL 下载并转存为稳定 URL。
// 返回 (稳定URL, true) 表示转存成功；否则返回 (原URL, false)。
func (s *MediaTaskService) maybeStoreMedia(ctx context.Context, record *MediaTaskRecord, rawURL string) (string, bool) {
	if s == nil || s.imageStorageSetting == nil {
		return rawURL, false
	}
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" || !strings.HasPrefix(rawURL, "http") {
		return rawURL, false
	}
	storage, ok := s.imageStorageSetting.Storage()
	if !ok || storage == nil {
		return rawURL, false
	}
	data, contentType, err := downloadMediaBytes(ctx, rawURL)
	if err != nil {
		s.logger.Warn("media_task_service: download for storage failed, keep original URL",
			zap.Int64("task_id", record.ID),
			zap.String("kind", string(record.MediaKind)),
			zap.Error(err),
		)
		return rawURL, false
	}
	key := mediaStorageKey(record, contentType)
	storedURL, err := storage.Save(ctx, key, contentType, data)
	if err != nil {
		s.logger.Warn("media_task_service: storage save failed, keep original URL",
			zap.Int64("task_id", record.ID),
			zap.String("kind", string(record.MediaKind)),
			zap.Error(err),
		)
		return rawURL, false
	}
	return storedURL, true
}

// mediaStorageKey 生成对象存储 key，保证同一任务多次轮询不重复覆盖。
func mediaStorageKey(record *MediaTaskRecord, contentType string) string {
	raw := record.LocalID
	if record.UpstreamTaskID != "" {
		raw = record.LocalID + "-" + record.UpstreamTaskID
	}
	h := sha1.Sum([]byte(raw))
	sum := hex.EncodeToString(h[:6])
	ext := mediaExtensionForContentType(contentType)
	return "media/" + string(record.MediaKind) + "/" + sum + ext
}

func mediaExtensionForContentType(contentType string) string {
	ct := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	switch {
	case strings.Contains(ct, "mp4"):
		return ".mp4"
	case strings.Contains(ct, "webm"):
		return ".webm"
	case strings.Contains(ct, "quicktime"), strings.Contains(ct, "mov"):
		return ".mov"
	case strings.Contains(ct, "mpeg"), strings.Contains(ct, "mp3"):
		return ".mp3"
	case strings.Contains(ct, "wav"):
		return ".wav"
	case strings.Contains(ct, "ogg"):
		return ".ogg"
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "jpeg"):
		return ".jpg"
	case strings.Contains(ct, "webp"):
		return ".webp"
	case strings.Contains(ct, "gif"):
		return ".gif"
	default:
		// 根据 media kind 兜底
		return ".bin"
	}
}

func downloadMediaBytes(ctx context.Context, rawURL string) ([]byte, string, error) {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return nil, "", fmt.Errorf("unsupported url scheme")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "sub2api-media-store")
	// 部分签名 URL 需保留 Range；此处仅 GET 全量。
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, "", fmt.Errorf("download media: unexpected status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 200<<20)) // 200MB 上限
	if err != nil {
		return nil, "", err
	}
	contentType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	return data, contentType, nil
}

// ListTasksByUserID 查询指定用户媒体任务列表（分页）。
func (s *MediaTaskService) ListTasksByUserID(ctx context.Context, userID int64, limit, offset int) ([]*MediaTaskRecord, int, error) {
	return s.mediaTaskRepo.ListByUserID(ctx, userID, limit, offset)
}

// GetTaskByLocalID 按 local_id 查询（管理后台用，不校验 user_id）。
func (s *MediaTaskService) GetTaskByLocalID(ctx context.Context, localID string) (*MediaTaskRecord, error) {
	record, err := s.mediaTaskRepo.GetByLocalID(ctx, localID)
	if err != nil {
		return nil, fmt.Errorf("media_task_service: get task by local_id: %w", err)
	}
	return record, nil
}

// GetTaskByID 按主键 ID 查询（管理后台用）。
func (s *MediaTaskService) GetTaskByID(ctx context.Context, id int64) (*MediaTaskRecord, error) {
	record, err := s.mediaTaskRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("media_task_service: get task by id: %w", err)
	}
	return record, nil
}

// CancelTask 管理员手动取消任务：仅更新本地状态。
func (s *MediaTaskService) CancelTask(ctx context.Context, id int64) error {
	record, err := s.mediaTaskRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("media_task_service: get task before cancel: %w", err)
	}
	// 尽力同步上游取消：若 adapter 支持 Cancel 则调用，忽略不支持/失败的厂商。
	if record.UpstreamTaskID != "" {
		account, accErr := s.accountService.GetByID(ctx, record.AccountID)
		if accErr == nil && account != nil {
			if adapter, rErr := s.adapter.Resolve(record.MediaKind, account.Platform, record.UpstreamModel); rErr == nil {
				if canceller, ok := adapter.(interface {
					Cancel(ctx context.Context, account *Account, upstreamTaskID string) error
				}); ok {
					if cErr := canceller.Cancel(ctx, account, record.UpstreamTaskID); cErr != nil {
						s.logger.Warn("media_task_service: upstream cancel failed (ignored)",
							zap.Int64("task_id", record.ID),
							zap.Int64("account_id", record.AccountID),
							zap.Error(cErr),
						)
					}
				}
			}
		}
	}
	if err := s.mediaTaskRepo.UpdateStatus(ctx, id, "cancelled", "cancelled by admin"); err != nil {
		return fmt.Errorf("media_task_service: cancel task: %w", err)
	}
	if record.MediaKind == MediaKindVideo {
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
	var reserved float64
	if record.ReservedCost != nil {
		reserved = *record.ReservedCost
	}
	releaseMediaReservedQuota(s.apiKeyService, ctx, record.APIKeyID, reserved)
	return nil
}

// calculateMediaCost 按媒体类型计算费用。视频/图片/音频分别走对应计费器。
func (s *MediaTaskService) calculateMediaCost(ctx context.Context, kind MediaKind, apiKeyID int64, model, resolution string, durationSec int) (float64, error) {
	if s == nil || s.billingService == nil || s.apiKeyService == nil {
		return 0, fmt.Errorf("media_task_service: billing dependencies are not wired")
	}
	apiKey, err := s.apiKeyService.GetByID(ctx, apiKeyID)
	if err != nil || apiKey == nil {
		return 0, fmt.Errorf("media_task_service: load api key %d: %w", apiKeyID, err)
	}
	if apiKey.GroupID == nil || apiKey.Group == nil {
		return 0, fmt.Errorf("media_task_service: api key %d has no group", apiKeyID)
	}

	baseMultiplier := apiKey.Group.RateMultiplier
	if s.openAIGatewayService != nil {
		baseMultiplier = s.openAIGatewayService.ResolveUserGroupRateMultiplier(
			ctx, apiKey.UserID, *apiKey.GroupID, apiKey.Group.RateMultiplier,
		)
	}
	multiplier := resolveVideoRateMultiplier(apiKey, baseMultiplier)

	switch kind {
	case MediaKindVideo:
		cost, _ := videoTaskCostBreakdown(ctx, s.billingDeps(), apiKey, model, resolution, durationSec)
		return cost.ActualCost, nil
	case MediaKindImage:
		size := resolution
		if size == "" {
			size = ImageBillingSize2K
		}
		cost := s.billingService.CalculateImageCost(model, size, 1, imagePriceConfigFromAPIKey(apiKey), multiplier)
		return cost.ActualCost, nil
	case MediaKindAudio:
		// 音频采用通行口径：优先按秒（media_audio），其次按分钟（realtime）。
		// 价格独立于视频（分组音频价），不并行用视频秒价。
		audioCfg := groupAudioPriceConfigFromAPIKey(apiKey)
		var cost *CostBreakdown
		if audioCfg != nil && audioCfg.PerSec != nil {
			secs := durationSec
			if secs <= 0 {
				secs = 1
			}
			cost = s.billingService.CalculateAudioCost("media_audio", float64(secs), audioCfg, baseMultiplier)
		} else {
			mins := float64(durationSec) / 60.0
			if mins <= 0 {
				mins = 1.0 / 60.0
			}
			cost = s.billingService.CalculateAudioCost("realtime", mins, audioCfg, baseMultiplier)
		}
		return cost.ActualCost, nil
	default:
		return 0, fmt.Errorf("media_task_service: unsupported media kind %q", kind)
	}
}

// parseMediaCreateRequest 从请求 body 解析统一参数。
func parseMediaCreateRequest(kind MediaKind, model string, body map[string]any) (*MediaCreateRequest, error) {
	req := &MediaCreateRequest{
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
	if v, ok := body["seed"]; ok {
		switch seed := v.(type) {
		case float64:
			s := int64(seed)
			req.Seed = &s
		case int64:
			req.Seed = &seed
		}
	}
	if len(body) > 0 {
		req.Extra = make(map[string]any, len(body))
		for k, v := range body {
			req.Extra[k] = v
		}
	}
	return req, nil
}

// generateMediaLocalID 生成唯一本地任务 ID，按媒体类型区分前缀。
func generateMediaLocalID(kind MediaKind) string {
	prefix := "med"
	switch kind {
	case MediaKindImage:
		prefix = "img"
	case MediaKindVideo:
		prefix = "vid"
	case MediaKindAudio:
		prefix = "aud"
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}

// ResolveAudioSpeechBytes 同步处理 OpenAI 兼容 /v1/audio/speech。
// 它选号 + 调 adapter，若上游返回原始音频字节则直接返回字节；
// 若返回 URL 则返回 URL 让 handler 302。不写 media_tasks（同步端点）。
func (s *MediaTaskService) ResolveAudioSpeechBytes(c *gin.Context, ctx context.Context, groupID *int64, publicModel string, requestBody map[string]any) ([]byte, string, error) {
	req, err := parseMediaCreateRequest(MediaKindAudio, publicModel, requestBody)
	if err != nil {
		return nil, "", fmt.Errorf("media_task_service: parse request: %w", err)
	}
	excluded := make(map[int64]struct{})
	const maxAttempts = 100

	for attempt := 0; attempt < maxAttempts; attempt++ {
		account, selectErr := s.gatewayService.SelectAccountForModelWithExclusions(ctx, groupID, "", publicModel, excluded)
		if selectErr != nil || account == nil {
			if selectErr == nil {
				selectErr = fmt.Errorf("media_task_service: no available account for model %s", publicModel)
			}
			return nil, "", fmt.Errorf("media_task_service: select account: %w", selectErr)
		}
		upstreamModel := publicModel
		if mapped := account.GetMappedModel(publicModel); mapped != "" {
			upstreamModel = mapped
		}
		req.PublicModel = publicModel
		req.UpstreamModel = upstreamModel
		adapter, resolveErr := s.adapter.Resolve(MediaKindAudio, account.Platform, upstreamModel)
		if resolveErr != nil {
			return nil, "", fmt.Errorf("media_task_service: resolve adapter: %w", resolveErr)
		}
		createResult, createErr := adapter.Create(ctx, account, *req)
		if createErr != nil {
			s.recordMediaTransportFailure(c, ctx, account, createErr)
			excluded[account.ID] = struct{}{}
			continue
		}
		if createResult.Status == "failed" && createResult.Mode == MediaCompletionFailed {
			failureBody := []byte(createResult.ErrorMessage)
			decision := classifyMediaUpstreamFailure(createResult.UpstreamStatusCode, failureBody)
			s.recordMediaUpstreamFailure(c, ctx, account, createResult.UpstreamStatusCode, failureBody, publicModel)
			if decision.ShouldFailover {
				excluded[account.ID] = struct{}{}
				continue
			}
			return nil, "", fmt.Errorf("media_task_service: upstream failed with status %d: %s", createResult.UpstreamStatusCode, createResult.ErrorMessage)
		}
		// 同步字节直接返回；有 URL 返回 URL。
		if len(createResult.InlineBytes) > 0 {
			return createResult.InlineBytes, "audio/mpeg", nil
		}
		if createResult.InlineURL != "" {
			return nil, createResult.InlineURL, nil
		}
		// 既无字节也无 URL：视为异步任务，返回空让 handler 走任务 JSON。
		return nil, "", nil
	}
	return nil, "", fmt.Errorf("media_task_service: upstream audio speech exhausted")
}
