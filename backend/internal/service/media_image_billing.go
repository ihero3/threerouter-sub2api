package service

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// media_image_billing.go — 图片任务（同步/异步）统一结算。
//
// 背景：图片此前只做 API Key 配额预扣（UpdateQuotaUsed），既不写 usage_logs 也不扣
// 余额/订阅，导致「使用记录」看不到生图调用、余额侧漏收。这里按视频任务
// （video_task_billing.go）的同款口径补齐：
//  1. 幂等键：media-image:<local_id>，同一任务只记一条日志、只扣一次费
//  2. 写 usage_logs（BillingMode=image，ImageCount/ImageSize 字段齐备）
//  3. applyUsageBilling 原子扣费（余额/订阅/Key 配额/账号配额）
//  4. 扣费成功后退还创建时的预扣，净效果 = 实际费用
//
// 同步出图（绝大多数厂商）在创建成功时立即结算；异步出图（如千问异步模式）在
// refreshTaskStatus 命中 succeeded 时结算，两处由 claimed / 状态互斥，不会重复。

// mediaImageBillingRequestIDPrefix 图片任务计费幂等键前缀。
const mediaImageBillingRequestIDPrefix = "media-image:"

// mediaImageBillingInput 单个图片任务终态结算所需的最小字段集。
type mediaImageBillingInput struct {
	LocalID       string
	UserID        int64
	APIKeyID      int64
	AccountID     int64
	Account       *Account // 成功结算必需；失败日志可为 nil
	Model         string
	UpstreamModel string
	Resolution    string
	ImageCount    int
	ReservedCost  *float64
}

// MediaImageBillingRequestID 返回图片任务的稳定幂等键：
// 同一任务无论创建即结算还是轮询结算，只扣一次费、只记一条日志。
func MediaImageBillingRequestID(localID string) string {
	localID = strings.TrimSpace(localID)
	if localID == "" {
		return ""
	}
	if strings.HasPrefix(localID, mediaImageBillingRequestIDPrefix) {
		return localID
	}
	return mediaImageBillingRequestIDPrefix + localID
}

// calculateImageTaskCostBreakdown 计算图片任务费用明细。
// 价格来源与同步网关图片链路保持一致：分组图片定价卡 → 分组媒体图片价 → 兜底默认价。
// 与 MediaTaskService.calculateMediaCost 的图片分支共用同一实现，避免预扣与实际两处口径漂移。
func calculateImageTaskCostBreakdown(
	ctx context.Context,
	billingService *BillingService,
	gw *OpenAIGatewayService,
	apiKey *APIKey,
	model, resolution string,
	imageCount int,
) (*CostBreakdown, float64) {
	if billingService == nil || apiKey == nil {
		return nil, 0
	}
	baseMultiplier := apiKey.Group.RateMultiplier
	if gw != nil && apiKey.GroupID != nil {
		baseMultiplier = gw.ResolveUserGroupRateMultiplier(ctx, apiKey.UserID, *apiKey.GroupID, apiKey.Group.RateMultiplier)
	}
	multiplier := resolveVideoRateMultiplier(apiKey, baseMultiplier)
	size := strings.TrimSpace(resolution)
	if size == "" {
		size = ImageBillingSize2K
	}
	cost := billingService.CalculateImageCost(model, size, clampImageCount(imageCount), imagePriceConfigFromAPIKey(apiKey), multiplier)
	if cost != nil && cost.BillingMode == "" {
		cost.BillingMode = string(BillingModeImage)
	}
	return cost, multiplier
}

// mediaImageBillingResolution 计费/日志用的图片尺寸档，缺失时落到 2K 默认档。
func mediaImageBillingResolution(resolution string) string {
	size := strings.TrimSpace(resolution)
	if size == "" {
		return ImageBillingSize2K
	}
	return size
}

// settleMediaImageTaskSuccess 图片任务成功终态结算：
//  1. 算实际费用 → usage_logs（image 计费模式）→ applyUsageBilling 原子扣费
//  2. 扣费成功后退还创建时的预扣（Key 配额净效果 = ActualCost）
//  3. 上下文缺失/扣费失败时降级：保留预扣近似入账 + 0 费用日志，绝不阻断任务状态
//
// 返回实际入账费用（0 表示未扣费，交由 0 费用日志对账）。
func settleMediaImageTaskSuccess(ctx context.Context, deps *videoTaskBillingDeps, in *mediaImageBillingInput) float64 {
	if deps == nil || in == nil {
		return 0
	}
	gw := deps.openAIGatewayService
	now := time.Now()

	apiKey, keyErr := videoTaskLoadAPIKey(ctx, deps, in.APIKeyID)
	var user *User
	if keyErr == nil && gw != nil && gw.userRepo != nil {
		if u, uErr := gw.userRepo.GetByID(ctx, apiKey.UserID); uErr == nil && u != nil {
			user = u
		}
	}
	// 成功结算必须有完整上下文；缺失时退预扣（不让用户为无法计费的请求买单）
	if keyErr != nil || user == nil || deps.billingService == nil || in.Account == nil || gw == nil {
		logger.L().Warn("image billing: settlement context unavailable, refund reserved quota",
			zap.String("local_id", in.LocalID),
			zap.Int64("api_key_id", in.APIKeyID),
			zap.Error(keyErr),
		)
		releaseMediaReservedQuota(deps.apiKeyService, ctx, in.APIKeyID, reservedCostOf(in.ReservedCost))
		writeMediaImageZeroCostUsageLog(ctx, deps, in, nil, now)
		return 0
	}

	cost, multiplier := calculateImageTaskCostBreakdown(ctx, deps.billingService, gw, apiKey, in.Model, in.Resolution, in.ImageCount)
	if cost == nil {
		// 算不出费用不能让这次调用从「使用记录」里凭空消失：和其他降级分支口径一致，
		// 退还预扣 + 写 0 费用日志，保证出图成功一定有行可对账。
		logger.L().Warn("image billing: cost calculation returned nil, refund reserved quota",
			zap.String("local_id", in.LocalID),
			zap.String("model", in.Model),
		)
		releaseMediaReservedQuota(deps.apiKeyService, ctx, in.APIKeyID, reservedCostOf(in.ReservedCost))
		writeMediaImageZeroCostUsageLog(ctx, deps, in, apiKey, now)
		return 0
	}

	// 订阅分组且存在有效订阅 → 走订阅计费，否则走余额
	var subscription *UserSubscription
	isSubscriptionBill := false
	if apiKey.Group.IsSubscriptionType() && gw.userSubRepo != nil && apiKey.GroupID != nil {
		if sub, subErr := gw.userSubRepo.GetActiveByUserIDAndGroupID(ctx, apiKey.UserID, *apiKey.GroupID); subErr == nil && sub != nil {
			subscription = sub
			isSubscriptionBill = true
		}
	}
	billingType := BillingTypeBalance
	if isSubscriptionBill {
		billingType = BillingTypeSubscription
	}

	usageLog := buildMediaImageUsageLog(in, apiKey, subscription, cost, multiplier, billingType, now)

	// Simple 运行模式：只记日志不扣费（与视频/普通模型行为一致）
	if gw.cfg != nil && gw.cfg.RunMode == config.RunModeSimple {
		writeUsageLogBestEffort(ctx, gw.usageLogRepo, usageLog, "service.media_image")
		releaseMediaReservedQuota(deps.apiKeyService, ctx, in.APIKeyID, reservedCostOf(in.ReservedCost))
		return 0
	}

	// 原子扣费（幂等键 media-image:<local_id>，重试不会重复扣费）
	if _, billingErr := applyUsageBilling(ctx, usageLog.RequestID, usageLog, &postUsageBillingParams{
		Cost:                  cost,
		User:                  user,
		APIKey:                apiKey,
		Account:               in.Account,
		Subscription:          subscription,
		RequestPayloadHash:    usageLog.RequestID,
		IsSubscriptionBill:    isSubscriptionBill,
		AccountRateMultiplier: in.Account.BillingRateMultiplier(),
		APIKeyService:         deps.apiKeyService,
		Platform:              PlatformFromAPIKey(apiKey),
	}, gw.billingDeps(), gw.usageBillingRepo); billingErr != nil {
		// 扣费失败：保留预扣（Key 配额维度近似入账，供对账），日志按 0 费用落库
		usageLog.TotalCost = 0
		usageLog.ActualCost = 0
		writeUsageLogBestEffort(ctx, gw.usageLogRepo, usageLog, "service.media_image")
		logger.L().Warn("image billing: apply billing failed, reserved quota kept for reconciliation",
			zap.String("request_id", usageLog.RequestID),
			zap.Int64("api_key_id", in.APIKeyID),
			zap.Error(billingErr),
		)
		return 0
	}

	// 扣费成功：退还创建时的预扣（Key 配额净效果 = ActualCost）
	releaseMediaReservedQuota(deps.apiKeyService, ctx, in.APIKeyID, reservedCostOf(in.ReservedCost))
	writeUsageLogBestEffort(ctx, gw.usageLogRepo, usageLog, "service.media_image")
	return cost.ActualCost
}

// settleMediaImageTaskFailure 图片任务失败/取消终态：退还预扣 + 写 0 费用 usage_logs。
// 0 费用行让用户与管理员在用量记录里也能看到这次失败的生图调用。
func settleMediaImageTaskFailure(ctx context.Context, deps *videoTaskBillingDeps, in *mediaImageBillingInput) {
	if deps == nil || in == nil {
		return
	}
	releaseMediaReservedQuota(deps.apiKeyService, ctx, in.APIKeyID, reservedCostOf(in.ReservedCost))
	apiKey, keyErr := videoTaskLoadAPIKey(ctx, deps, in.APIKeyID)
	if keyErr != nil {
		apiKey = nil
	}
	writeMediaImageZeroCostUsageLog(ctx, deps, in, apiKey, time.Now())
}

// writeMediaImageZeroCostUsageLog 写 0 费用图片任务日志（失败/取消/上下文缺失兜底）。
// requestID 与成功结算同键：usage_logs 的 ON CONFLICT (request_id, api_key_id)
// 保证同一任务的成败日志互斥、先到先得。
func writeMediaImageZeroCostUsageLog(ctx context.Context, deps *videoTaskBillingDeps, in *mediaImageBillingInput, apiKey *APIKey, now time.Time) {
	if deps == nil || in == nil {
		return
	}
	gw := deps.openAIGatewayService
	if gw == nil || gw.usageLogRepo == nil {
		return
	}
	usageLog := buildMediaImageUsageLog(in, apiKey, nil, nil, 0, BillingTypeBalance, now)
	writeUsageLogBestEffort(ctx, gw.usageLogRepo, usageLog, "service.media_image")
}

// buildMediaImageUsageLog 构造图片任务的 usage_logs 行（字段对齐同步网关图片链路）。
func buildMediaImageUsageLog(in *mediaImageBillingInput, apiKey *APIKey, subscription *UserSubscription, cost *CostBreakdown, multiplier float64, billingType int8, now time.Time) *UsageLog {
	size := mediaImageBillingResolution(in.Resolution)
	imageCount := clampImageCount(in.ImageCount)
	usageLog := &UsageLog{
		UserID:         in.UserID,
		APIKeyID:       in.APIKeyID,
		AccountID:      in.AccountID,
		RequestID:      MediaImageBillingRequestID(in.LocalID),
		Model:          in.Model,
		RequestedModel: in.Model,
		UpstreamModel:  optionalTrimmedStringPtr(in.UpstreamModel),
		ImageCount:     imageCount,
		ImageSize:      &size,
		RateMultiplier: multiplier,
		BillingType:    billingType,
		RequestType:    RequestTypeSync,
		CreatedAt:      now,
	}
	if in.Account != nil {
		m := in.Account.BillingRateMultiplier()
		usageLog.AccountRateMultiplier = &m
	}
	if cost != nil {
		usageLog.TotalCost = cost.TotalCost
		usageLog.ActualCost = cost.ActualCost
		usageLog.ImageInputCost = cost.ImageInputCost
		usageLog.ImageOutputCost = cost.ImageOutputCost
	}
	billingMode := string(BillingModeImage)
	usageLog.BillingMode = &billingMode
	if apiKey != nil {
		usageLog.GroupID = apiKey.GroupID
	}
	if subscription != nil {
		usageLog.SubscriptionID = &subscription.ID
	}
	return usageLog
}

// mediaImageBillingInputFromRecord 从媒体任务记录构造图片结算入参。
// imageCount 由调用方给定（创建时用实际出图张数，轮询时用请求张数）。
func mediaImageBillingInputFromRecord(record *MediaTaskRecord, imageCount int) *mediaImageBillingInput {
	if record == nil {
		return nil
	}
	return &mediaImageBillingInput{
		LocalID:       record.LocalID,
		UserID:        record.UserID,
		APIKeyID:      record.APIKeyID,
		AccountID:     record.AccountID,
		Model:         record.PublicModel,
		UpstreamModel: record.UpstreamModel,
		Resolution:    record.Resolution,
		ImageCount:    imageCount,
		ReservedCost:  record.ReservedCost,
	}
}
