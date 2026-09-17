package service

// media_task_service.go — 媒体任务统一业务逻辑（图片 / 视频 / 音频）。
// 负责：解析统一请求 → 选上游账号（同类型多账号自动轮转）→ 调 adapter →
// 写表 → 异步轮询 → 幂等计费。复用现有 GatewayService 调度器 + BillingService。

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// mediaUpstreamCreateTimeout 上游 create 请求的独立超时。
//
// 为什么必须独立于客户端连接：create 用的是 gin 的 request context，客户端
// （或其反向代理，常见 60 秒）一断开，gin 立刻取消该 context，已发出的上游
// 请求被连带掐断，表现为 502 + "context canceled"。更糟的是上游可能已经建好
// 任务，我们却记为失败——任务成为孤儿，预扣费用也无处结算。
//
// 因此上游 create 走脱离取消链的 context，只受本超时约束：客户端断开后仍要
// 把这次创建跑完并落库，用户后续轮询即可拿到结果。
const mediaUpstreamCreateTimeout = 300 * time.Second

// mediaPersistTimeout 落库与结算的超时。
//
// 上游任务一旦建好，落库和结算就是"必须完成"的动作：若跟着客户端取消链一起
// 失败，上游任务就成了无人认领的孤儿，预扣费用也永远无法结算（用户会白扣额度
// 却查不到使用记录）。因此这些写操作同样脱离客户端取消链，只受本超时约束。
const mediaPersistTimeout = 30 * time.Second

// MediaTaskRepo 媒体任务仓储接口（service 层定义，repository 层实现）。
type MediaTaskRepo interface {
	Create(ctx context.Context, task *MediaTaskRecord) (*MediaTaskRecord, error)
	GetByLocalID(ctx context.Context, localID string) (*MediaTaskRecord, error)
	GetByID(ctx context.Context, id int64) (*MediaTaskRecord, error)
	UpdateStatusIfProcessing(ctx context.Context, id int64, status, errorMsg string) (bool, error)
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
	// MediaURLs 是 n>1 时的全部产物 URL，随 media_tasks.media_urls（JSONB）落库：
	// 创建同步响应与轮询 GET /v1/media/:id 都能返回完整列表。
	MediaURLs    []string
	ThumbnailURL string
	RequestBody  map[string]any
	ErrorMessage string
	CostUSD      float64
	ReservedCost *float64
	CreatedAt    time.Time
	UpdatedAt    time.Time
	FinishedAt   *time.Time
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
	// 视频模型 resolution 档位校验：在选号与调上游之前拦截，直接 400。
	if kind == MediaKindVideo {
		if vErr := validateVideoResolution(publicModel, req.Resolution); vErr != nil {
			return nil, fmt.Errorf("media_task_service: validate resolution: %w", vErr)
		}
	}

	ctx := c.Request.Context()
	// 上游 create 脱离客户端取消链（见 mediaUpstreamCreateTimeout 注释）。
	// 只有"选号"这类前置查询仍用原 ctx —— 客户端都走了，没必要再挑账号。
	upstreamBase := context.WithoutCancel(ctx)
	excluded := make(map[int64]struct{})
	var lastUpstreamErr error
	const maxAttempts = 100

	for attempt := 0; attempt < maxAttempts; attempt++ {
		account, selectErr := s.gatewayService.SelectAccountForModelWithExclusions(ctx, groupID, "", publicModel, excluded)
		if selectErr != nil || account == nil {
			// 整单失败（选号耗尽 / 上游全部失败）也要在使用记录里留一条 0 费用行，
			// 否则用户与运营在「用量明细」里完全看不到这次调用。
			if lastUpstreamErr != nil {
				s.writeMediaTaskFailureUsageLog(ctx, kind, userID, apiKeyID, publicModel, req)
				return nil, lastUpstreamErr
			}
			if selectErr == nil {
				selectErr = fmt.Errorf("media_task_service: no available account for model %s", publicModel)
			}
			if errors.Is(selectErr, ErrNoAvailableAccounts) {
				s.writeMediaTaskFailureUsageLog(ctx, kind, userID, apiKeyID, publicModel, req)
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

		createCtx, cancelCreate := context.WithTimeout(upstreamBase, mediaUpstreamCreateTimeout)
		createResult, createErr := adapter.Create(createCtx, account, *req)
		cancelCreate()
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

		// 上游任务已建好，从此刻起落库与结算必须跑完：脱离客户端取消链，
		// 且必须在这里才起算超时 —— 若在函数入口创建，上游 create 耗时长时
		// persistCtx 会先过期，反而把落库全部拖垮。
		persistCtx, cancelPersist := context.WithTimeout(upstreamBase, mediaPersistTimeout)
		defer cancelPersist()

		localID := generateMediaLocalID(kind)
		// 明细元数据（入站/上游端点、UA、IP、起算时间、请求与真实输出尺寸）：
		// 同步终态（创建即成功/失败）直接用；异步终态由结算器按 localID 回源缓存，
		// 否则轮询时拿不到 gin 上下文与 adapter 响应，明细会缺一半。
		usageMeta := newMediaUsageMeta(c, createResult.UpstreamEndpoint, req.Resolution, createResult.UpstreamSize)
		storeMediaEndpointMeta(localID, usageMeta)
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
		// 上游真实输出尺寸优先于请求值：不传 size 时模型会自行推荐分辨率，
		// 按请求值计费会错档（千问默认 1024x1024 属 1K，请求值缺失时会被按 2K 收）。
		if upstreamSize := strings.TrimSpace(createResult.UpstreamSize); upstreamSize != "" {
			record.Resolution = upstreamSize
		}
		// 状态兜底：adapter 万一没回 status，空串会让任务永远卡在非终态——
		// UpdateStatusIfProcessing 只认 "processing"，超时与取消都改不动它，
		// 客户端只能一直轮询到一个永不结束的任务。
		if strings.TrimSpace(record.Status) == "" {
			record.Status = "processing"
		}
		if createResult.InlineURL != "" {
			record.MediaURL = createResult.InlineURL
			if stored, ok := s.maybeStoreMedia(persistCtx, record, createResult.InlineURL); ok {
				record.MediaURL = stored
			}
		}
		// 多张结果（n>1）落库到 media_urls：创建响应直接带全量，
		// 轮询接口从库读取后同样返回完整 urls。
		if len(createResult.InlineURLs) > 1 {
			urls := make([]string, 0, len(createResult.InlineURLs))
			for i, u := range createResult.InlineURLs {
				if u == createResult.InlineURL {
					urls = append(urls, record.MediaURL)
					continue
				}
				// 多图时按序号加 salt：同一任务的每张图落在不同存储 key，
				// 否则 mediaStorageKey 只由 localID 决定，后写会覆盖先写，
				// n 张图最后全指向同一张（最后一张）。
				if stored, ok := s.maybeStoreMediaIndexed(persistCtx, record, u, i); ok {
					urls = append(urls, stored)
					continue
				}
				urls = append(urls, u)
			}
			record.MediaURLs = urls
		}
		if createResult.Status == "failed" && kind == MediaKindImage {
			// 图片失败同样退还预扣并写 0 费用日志：此前图片失败既不退预扣也不可见。
			imgInput := mediaImageBillingInputFromRecord(record, settledImageCount(req, createResult))
			imgInput.Account = account
			imgInput.RequestedSize = req.Resolution
			settleMediaImageTaskFailure(persistCtx, s.billingDeps(), imgInput)
		} else if createResult.Status == "failed" && kind == MediaKindAudio {
			audioInput := mediaAudioBillingInputFromRecord(record)
			audioInput.Account = account
			settleMediaAudioTaskFailure(persistCtx, s.billingDeps(), audioInput)
		} else if createResult.Status == "failed" && kind == MediaKindVideo {
			// 不触发 failover 的上游失败：无预扣可退，仍写 0 费用日志保持审计完整
			settleVideoTaskFailure(persistCtx, s.billingDeps(), &videoTaskBillingInput{
				LocalID:              record.LocalID,
				UserID:               record.UserID,
				APIKeyID:             record.APIKeyID,
				AccountID:            record.AccountID,
				Account:              account,
				Model:                record.PublicModel,
				UpstreamModel:        record.UpstreamModel,
				Resolution:           record.Resolution,
				DurationSec:          record.DurationSec,
				RequestedDurationSec: record.DurationSec,
			})
		} else if createResult.Status != "failed" {
			if kind == MediaKindVideo {
				// 创建时上游真实时长未知（0），用用户请求时长预估。
				if cost, costErr := estimateVideoTaskCost(persistCtx, s.billingDeps(), apiKeyID, publicModel, req.Resolution, 0, req.DurationSec); costErr == nil && cost > 0 {
					record.ReservedCost = &cost
					if s.apiKeyService != nil {
						_ = s.apiKeyService.UpdateQuotaUsed(persistCtx, apiKeyID, cost)
					}
				}
			} else if cost, costErr := s.calculateMediaCost(persistCtx, kind, apiKeyID, publicModel, record.Resolution, 0, req.DurationSec, settledImageCount(req, createResult)); costErr == nil && cost > 0 {
				record.ReservedCost = &cost
				if s.apiKeyService != nil {
					_ = s.apiKeyService.UpdateQuotaUsed(persistCtx, apiKeyID, cost)
				}
			}
		}

		saved, saveErr := s.mediaTaskRepo.Create(persistCtx, record)
		if saveErr != nil {
			return nil, fmt.Errorf("media_task_service: save task: %w", saveErr)
		}

		// 同步出图：创建即终态，立刻结算（写 usage_logs + 扣余额/订阅 + 预扣转实扣）。
		// 异步出图（status=processing）留到 refreshTaskStatus 命中 succeeded 时结算，
		// 两处由任务状态互斥，不会重复扣费。
		if kind == MediaKindImage && createResult.Status == "succeeded" {
			imgInput := mediaImageBillingInputFromRecord(saved, settledImageCount(req, createResult))
			imgInput.Account = account
			// 请求尺寸与上游真实输出尺寸分开带：计费档位与 usage_logs 明细
			// 由 resolveMediaImageBillingSize 统一推导，跨厂商口径一致。
			imgInput.RequestedSize = req.Resolution
			imgInput.OutputSize = createResult.UpstreamSize
			imgInput.Meta = usageMeta
			settleMediaImageTaskSuccess(persistCtx, s.billingDeps(), imgInput)
		}
		// 音频 adapter 目前均同步返回，创建成功即终态，同样立刻结算。
		if kind == MediaKindAudio && createResult.Status == "succeeded" {
			audioInput := mediaAudioBillingInputFromRecord(saved)
			audioInput.Account = account
			audioInput.Meta = usageMeta
			settleMediaAudioTaskSuccess(persistCtx, s.billingDeps(), audioInput)
		}
		return saved, nil
	}

	if lastUpstreamErr == nil {
		lastUpstreamErr = fmt.Errorf("media_task_service: upstream account switches exhausted")
	}
	// 轮换上限用尽：同样补 0 费用使用记录，保持「调用过就有一行」的口径。
	s.writeMediaTaskFailureUsageLog(ctx, kind, userID, apiKeyID, publicModel, req)
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
		claimed, err := s.mediaTaskRepo.UpdateStatusIfProcessing(ctx, record.ID, "failed", "upstream task timed out")
		if err != nil {
			return fmt.Errorf("media_task_service: timeout update status: %w", err)
		}
		if !claimed {
			return nil
		}
		if record.MediaKind == MediaKindVideo {
			settleVideoTaskFailure(ctx, s.billingDeps(), &videoTaskBillingInput{
				LocalID:              record.LocalID,
				UserID:               record.UserID,
				APIKeyID:             record.APIKeyID,
				AccountID:            record.AccountID,
				Model:                record.PublicModel,
				UpstreamModel:        record.UpstreamModel,
				Resolution:           record.Resolution,
				DurationSec:          record.DurationSec,
				RequestedDurationSec: record.DurationSec,
				ReservedCost:         record.ReservedCost,
			})
		} else if record.MediaKind == MediaKindImage {
			// 异步出图超时：退预扣 + 0 费用日志，让超时调用在用量记录里可见。
			imgInput := mediaImageBillingInputFromRecord(record, parseMediaImageCount(record.RequestBody))
			settleMediaImageTaskFailure(ctx, s.billingDeps(), imgInput)
		} else if record.MediaKind == MediaKindAudio {
			settleMediaAudioTaskFailure(ctx, s.billingDeps(), mediaAudioBillingInputFromRecord(record))
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
			if est, estErr := estimateVideoTaskCost(ctx, s.billingDeps(), record.APIKeyID, record.PublicModel, record.Resolution, result.DurationSec, record.DurationSec); estErr == nil {
				actual = est
			} else {
				s.logger.Warn("media_task_service: estimate video cost failed",
					zap.Int64("task_id", record.ID),
					zap.Error(estErr),
				)
			}
		} else if cost, costErr := s.calculateMediaCost(ctx, record.MediaKind, record.APIKeyID, record.PublicModel, record.Resolution, result.DurationSec, record.DurationSec, parseMediaImageCount(record.RequestBody)); costErr == nil {
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
					LocalID:              record.LocalID,
					UserID:               record.UserID,
					APIKeyID:             record.APIKeyID,
					AccountID:            record.AccountID,
					Account:              account,
					Model:                record.PublicModel,
					UpstreamModel:        record.UpstreamModel,
					Resolution:           record.Resolution,
					DurationSec:          result.DurationSec,
					RequestedDurationSec: record.DurationSec,
					ReservedCost:         record.ReservedCost,
				})
			} else if record.MediaKind == MediaKindImage {
				// 异步出图：claimed 守卫保证同一任务只结算一次。
				imgInput := mediaImageBillingInputFromRecord(record, parseMediaImageCount(record.RequestBody))
				imgInput.Account = account
				settleMediaImageTaskSuccess(ctx, s.billingDeps(), imgInput)
			} else if record.MediaKind == MediaKindAudio {
				// 异步音频（若将来出现）：claimed 守卫保证同一任务只结算一次。
				audioInput := mediaAudioBillingInputFromRecord(record)
				audioInput.Account = account
				// 上游真实时长优先；为 0 时算价内部回退到请求时长（RequestedDurationSec）。
				audioInput.DurationSec = result.DurationSec
				settleMediaAudioTaskSuccess(ctx, s.billingDeps(), audioInput)
			} else {
				// 其他未接入结算的类型：仅按预扣差额找平。
				var reserved float64
				if record.ReservedCost != nil {
					reserved = *record.ReservedCost
				}
				settleMediaReservedQuota(s.apiKeyService, ctx, record.APIKeyID, reserved, actual)
			}
		}
	case "failed", "cancelled":
		claimed, err := s.mediaTaskRepo.UpdateStatusIfProcessing(ctx, record.ID, result.Status, result.ErrorMessage)
		if err != nil {
			return fmt.Errorf("media_task_service: update status: %w", err)
		}
		if !claimed {
			return nil
		}
		if record.MediaKind == MediaKindVideo {
			settleVideoTaskFailure(ctx, s.billingDeps(), &videoTaskBillingInput{
				LocalID:              record.LocalID,
				UserID:               record.UserID,
				APIKeyID:             record.APIKeyID,
				AccountID:            record.AccountID,
				Account:              account,
				Model:                record.PublicModel,
				UpstreamModel:        record.UpstreamModel,
				Resolution:           record.Resolution,
				DurationSec:          result.DurationSec,
				RequestedDurationSec: record.DurationSec,
				ReservedCost:         record.ReservedCost,
			})
		} else if record.MediaKind == MediaKindImage {
			imgInput := mediaImageBillingInputFromRecord(record, parseMediaImageCount(record.RequestBody))
			imgInput.Account = account
			settleMediaImageTaskFailure(ctx, s.billingDeps(), imgInput)
		} else if record.MediaKind == MediaKindAudio {
			audioInput := mediaAudioBillingInputFromRecord(record)
			audioInput.Account = account
			settleMediaAudioTaskFailure(ctx, s.billingDeps(), audioInput)
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
	return s.maybeStoreMediaIndexed(ctx, record, rawURL, -1)
}

// maybeStoreMediaIndexed 与 maybeStoreMedia 相同，但可带序号 salt：
// idx >= 0 时会把序号计入存储 key，用于同一任务的多张产物落到不同 key。
func (s *MediaTaskService) maybeStoreMediaIndexed(ctx context.Context, record *MediaTaskRecord, rawURL string, idx int) (string, bool) {
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
	var key string
	if idx >= 0 {
		key = mediaStorageKey(record, contentType, strconv.Itoa(idx))
	} else {
		key = mediaStorageKey(record, contentType)
	}
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

// MediaStorageEnabled 报告对象存储是否已配置可用。
// /v1/images/edits 的上传图需要它才能转成稳定 URL；没有存储时只能内联 base64，
// 因此必须让调用方提前知道能不能走"存起来再引用"这条路。
func (s *MediaTaskService) MediaStorageEnabled() bool {
	if s == nil || s.imageStorageSetting == nil {
		return false
	}
	storage, ok := s.imageStorageSetting.Storage()
	return ok && storage != nil
}

// StoreMediaBytes 把任意字节写入对象存储并返回可访问 URL。
// 未配置存储或写入失败时返回 ("", false)，由调用方决定是否回退到内联 base64。
func (s *MediaTaskService) StoreMediaBytes(ctx context.Context, kind MediaKind, contentType string, data []byte) (string, bool) {
	if s == nil || s.imageStorageSetting == nil {
		return "", false
	}
	storage, ok := s.imageStorageSetting.Storage()
	if !ok || storage == nil || len(data) == 0 {
		return "", false
	}
	record := &MediaTaskRecord{LocalID: generateMediaLocalID(kind), MediaKind: kind}
	storedURL, err := storage.Save(ctx, mediaStorageKey(record, contentType), contentType, data)
	if err != nil {
		s.logger.Warn("media_task_service: store uploaded bytes failed",
			zap.String("kind", string(kind)),
			zap.Error(err),
		)
		return "", false
	}
	return storedURL, true
}

// mediaStorageKey 生成对象存储 key。salt 用于区分同一任务的多张产物：
// 不传 salt（空串）时 key 只由 localID 决定，重复轮询同 URL 不会重复覆盖；
// 传 salt（如多图序号）时同一任务的不同产物落在不同 key，避免后写覆盖先写。
func mediaStorageKey(record *MediaTaskRecord, contentType string, salt ...string) string {
	raw := record.LocalID
	if record.UpstreamTaskID != "" {
		raw = record.LocalID + "-" + record.UpstreamTaskID
	}
	if len(salt) > 0 && salt[0] != "" {
		raw = raw + "-" + salt[0]
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

// DownloadMediaBytes 下载媒体产物字节（供 b64_json 转码等场景复用）。
// 返回 (字节, content-type, error)。只接受 http/https，限制 200MB。
func DownloadMediaBytes(ctx context.Context, rawURL string) ([]byte, string, error) {
	return downloadMediaBytes(ctx, rawURL)
}

// DownloadMediaBytesLimit 与 DownloadMediaBytes 相同，但把体积上限收紧到
// maxBytes：先用 Content-Length 预判，再在读取时硬性截断并校验。
func DownloadMediaBytesLimit(ctx context.Context, rawURL string, maxBytes int64) ([]byte, string, error) {
	if maxBytes <= 0 {
		return nil, "", fmt.Errorf("download media: invalid size limit")
	}
	return downloadMediaBytes(ctx, rawURL, maxBytes)
}

func downloadMediaBytes(ctx context.Context, rawURL string, maxBytes ...int64) ([]byte, string, error) {
	limit := int64(200 << 20)
	if len(maxBytes) > 0 && maxBytes[0] > 0 {
		limit = maxBytes[0]
	}
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
	// 先按声明长度拦一次，能省掉一次注定要失败的大流量下载。
	if resp.ContentLength > limit {
		return nil, "", fmt.Errorf("download media: content length %d exceeds limit %d", resp.ContentLength, limit)
	}
	// 多读 1 字节：读满 limit 并不代表刚好等于 limit，只有多读一字节才能区分
	// "正好到上限" 与 "超过上限"，否则超限文件会被静默截断。
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > limit {
		return nil, "", fmt.Errorf("download media: content exceeds limit %d", limit)
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
	claimed, err := s.mediaTaskRepo.UpdateStatusIfProcessing(ctx, id, "cancelled", "cancelled by admin")
	if err != nil {
		return fmt.Errorf("media_task_service: cancel task: %w", err)
	}
	if !claimed {
		return nil
	}
	if record.MediaKind == MediaKindVideo {
		settleVideoTaskFailure(ctx, s.billingDeps(), &videoTaskBillingInput{
			LocalID:              record.LocalID,
			UserID:               record.UserID,
			APIKeyID:             record.APIKeyID,
			AccountID:            record.AccountID,
			Model:                record.PublicModel,
			UpstreamModel:        record.UpstreamModel,
			Resolution:           record.Resolution,
			DurationSec:          record.DurationSec,
			RequestedDurationSec: record.DurationSec,
			ReservedCost:         record.ReservedCost,
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

// settledImageCount 决定图片按多少张计费：上游实际返回张数优先于请求值。
// 上游可能因安全策略少出图，一律按请求值收会多收。
func settledImageCount(req *MediaCreateRequest, result *MediaCreateResult) int {
	if result != nil {
		if n := len(result.InlineURLs); n > 0 {
			return n
		}
	}
	return req.ImageCount
}

// calculateMediaCost 按媒体类型计算费用。视频/图片/音频分别走对应计费器。
// imageCount 只对图片生效：上游按张出图也按张收费，计费张数必须与实际出图数一致。
func (s *MediaTaskService) calculateMediaCost(ctx context.Context, kind MediaKind, apiKeyID int64, model, resolution string, actualSec, requestedSec, imageCount int) (float64, error) {
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

	switch kind {
	case MediaKindVideo:
		cost, _ := videoTaskCostBreakdown(ctx, s.billingDeps(), apiKey, model, resolution, actualSec, requestedSec)
		return cost.ActualCost, nil
	case MediaKindImage:
		// 与图片结算（media_image_billing.go）共用同一算价实现：预扣与实际必须同口径。
		cost, _ := calculateImageTaskCostBreakdown(ctx, s.billingService, s.openAIGatewayService, apiKey, model, resolution, imageCount)
		if cost == nil {
			return 0, fmt.Errorf("media_task_service: image cost calculation failed")
		}
		return cost.ActualCost, nil
	case MediaKindAudio:
		// 与音频结算（media_audio_billing.go）共用同一算价实现：预扣与实际必须同口径。
		cost, _ := calculateAudioTaskCostBreakdown(ctx, s.billingService, s.openAIGatewayService, apiKey, actualSec, requestedSec)
		if cost == nil {
			return 0, fmt.Errorf("media_task_service: audio cost calculation failed")
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
	// resolution 与 OpenAI 风格的 size 等价：客户端照搬 /v1/images/generations 的写法
	// 传 size 时不能静默丢弃，否则上游按默认比例出图、计费也落到默认档。
	req.Resolution = normalizeMediaResolution(firstMediaStringValue(body, "resolution", "size"))
	if v, ok := body["ratio"].(string); ok {
		req.Ratio = v
	}
	req.DurationSec = parseVideoDurationSecondsParam(body)
	// n：图片按张计费。上游按 n 张出图也按 n 张收费，漏掉会系统性少收。
	req.ImageCount = parseMediaImageCount(body)
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
	// 图生视频契约：存在参考图（image/image_urls/media 素材中的图片项）时按图生视频
	// 路由并忽略 ratio —— 各上游对 i2v 的 ratio 支持不一致，插件侧约定不下发。
	// Extra 里也一并删除，MiniMax 官方协议的 ratio 取自 Extra。
	if len(req.ImageRefURLs) > 0 || mediaHasImageReference(req.Media) {
		req.Ratio = ""
		delete(req.Extra, "ratio")
	}
	return req, nil
}

// firstMediaStringValue 返回第一个非空字符串字段值。
func firstMediaStringValue(body map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := body[key].(string); ok {
			if trimmed := strings.TrimSpace(v); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

// normalizeMediaResolution 归一化图片尺寸参数。
// OpenAI 的 size 允许 "auto"（由模型自选），媒体链路没有等价语义，
// 直接透传会被当成 aspect_ratio 下发给上游导致报错，这里统一视为未指定。
func normalizeMediaResolution(resolution string) string {
	trimmed := strings.TrimSpace(resolution)
	if trimmed == "" || strings.EqualFold(trimmed, "auto") {
		return ""
	}
	return trimmed
}

// parseMediaImageCount 解析单次生成的图片张数 n。
// 图片按张计费，n 缺失或非法时按 1 张（与上游默认一致），不做静默放大。
func parseMediaImageCount(body map[string]any) int {
	if body == nil {
		return 1
	}
	var n int
	switch v := body["n"].(type) {
	case float64:
		n = int(v)
	case int:
		n = v
	case int64:
		n = int(v)
	case json.Number:
		parsed, err := v.Int64()
		if err != nil {
			return 1
		}
		n = int(parsed)
	default:
		return 1
	}
	return clampImageCount(n)
}

// MediaInvalidRequestError 表示请求参数不满足媒体生成契约（如 resolution 档位
// 非法）。handler 应将其映射为 400 invalid_request_error，而非上游故障。
type MediaInvalidRequestError struct {
	Reason string
}

func (e *MediaInvalidRequestError) Error() string { return e.Reason }

// videoModelResolutionAllowlist 按模型前缀列出 resolution 白名单。
// MiniMax-H3 系列（含 UCloud ModelVerse 转售）仅支持 480P / 768P / 2K 三档；
// 未列出的模型不校验，档位透传上游。
var videoModelResolutionAllowlist = []struct {
	prefix  string
	allowed []string
}{
	// H3 Max 与 fal.ai 联合出品，最高只到 768P；必须排在 minimax-h3 之前，
	// 否则会被前缀规则放过 2K（上游会拒绝）。
	{prefix: "minimax-h3-max", allowed: []string{"480p", "768p"}},
	{prefix: "minimax-h3", allowed: []string{"480p", "768p", "2k"}},
}

// validateVideoResolution 按模型校验视频 resolution 档位。大小写不敏感，兼容
// 480/480p/sd、768/768p、2k/1440p 等常见写法（复用计费侧归一化）。resolution
// 为空表示使用上游默认档位，不校验。
func validateVideoResolution(model, resolution string) error {
	trimmed := strings.TrimSpace(resolution)
	if trimmed == "" {
		return nil
	}
	m := strings.ToLower(strings.TrimSpace(model))
	for _, rule := range videoModelResolutionAllowlist {
		if !strings.HasPrefix(m, rule.prefix) {
			continue
		}
		normalized, _ := LookupVideoBillingResolutionAny(trimmed)
		for _, allowed := range rule.allowed {
			if normalized == allowed {
				return nil
			}
		}
		return &MediaInvalidRequestError{
			Reason: fmt.Sprintf("resolution %q is not supported by model %s: allowed values are 480P, 768P, 2K", trimmed, model),
		}
	}
	return nil
}

// mediaHasImageReference 判断官方 media 素材列表里是否包含图片类素材
// （首帧 / 尾帧 / 参考图），用于识别图生视频请求。
func mediaHasImageReference(media []VideoMediaInput) bool {
	for _, m := range media {
		switch m.Type {
		case "first_frame", "last_frame", "reference_image":
			return true
		}
	}
	return false
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
func (s *MediaTaskService) ResolveAudioSpeechBytes(c *gin.Context, ctx context.Context, groupID *int64, userID, apiKeyID int64, publicModel string, requestBody map[string]any) ([]byte, string, error) {
	req, err := parseMediaCreateRequest(MediaKindAudio, publicModel, requestBody)
	if err != nil {
		return nil, "", fmt.Errorf("media_task_service: parse request: %w", err)
	}
	excluded := make(map[int64]struct{})
	const maxAttempts = 100
	// 与 CreateTask 同口径：上游 create 不跟随客户端断连（见该常量注释）。
	upstreamBase := context.WithoutCancel(ctx)

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
		createCtx, cancelCreate := context.WithTimeout(upstreamBase, mediaUpstreamCreateTimeout)
		createResult, createErr := adapter.Create(createCtx, account, *req)
		cancelCreate()
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
		// 同步音频此前完全不计费（不写 media_tasks，也就没有预扣/结算）。
		// 这里在返回前补一次结算：写 usage_logs + 扣余额/订阅，与异步音频任务同口径。
		// 无 media_tasks 记录，用一次性 localID 作幂等键（本函数内只在成功后结算一次）。
		hasAudioOutput := len(createResult.InlineBytes) > 0 || createResult.InlineURL != ""
		if hasAudioOutput {
			// 音频已生成，结算必须跑完（客户端断开就漏计费 = 白送）。
			// 同样在此刻才起算超时，避免上游 create 耗时把结算窗口耗光。
			persistCtx, cancelPersist := context.WithTimeout(upstreamBase, mediaPersistTimeout)
			defer cancelPersist()
			audioInput := &mediaAudioBillingInput{
				LocalID:       generateMediaLocalID(MediaKindAudio),
				UserID:        userID,
				APIKeyID:      apiKeyID,
				AccountID:     account.ID,
				Account:       account,
				Model:         publicModel,
				UpstreamModel: upstreamModel,
				// MediaCreateResult 不回传时长，只能取请求时长；
				// 缺失时由音频算价内部兜底（与异步音频任务同默认时长口径）。
				DurationSec:          req.DurationSec,
				RequestedDurationSec: req.DurationSec,
			}
			settleMediaAudioTaskSuccess(persistCtx, s.billingDeps(), audioInput)
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
