package service

// video_task_billing.go — 视频任务终态统一结算。
//
// 背景：Seedance / Wan / MiniMax 等视频任务此前只在创建时预扣 API Key 配额，
// 完成后按实际费用多退少补，但不扣余额/订阅、也不写 usage_logs，
// 导致用户与管理员在用量记录里看不到视频调用明细。
//
// 本文件把两条视频任务链路（VideoTaskService /v1/video 与
// MediaTaskService /v1/media kind=video）的计费对齐到普通模型：
//   - 成功：CalculateVideoCost 按秒实际计费 → UsageBillingCommand 原子扣费
//     （余额/订阅/Key配额/账号配额 + 缓存同步 + 低余额通知）→ 写 usage_logs
//     （BillingMode=video，video_count/resolution/duration 字段齐备）
//   - 失败/取消：退还预扣 + 写 0 费用 usage_logs（审计可见"调过视频链路"）
//   - 幂等：requestID 固定为 "video-task:<local_id>"，轮询重试不会重复扣费/重复记录
//
// 计费依赖复用 OpenAIGatewayService 已装配字段（同包访问），无需改构造器装配。

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

// videoTaskBillingRequestIDPrefix 视频任务计费幂等键前缀。
const videoTaskBillingRequestIDPrefix = "video-task:"

// videoTaskBillingDeps 两条任务链路共有的计费依赖集合。
type videoTaskBillingDeps struct {
	billingService       *BillingService
	apiKeyService        *APIKeyService
	openAIGatewayService *OpenAIGatewayService
}

// videoTaskBillingInput 单个视频任务终态结算所需的最小字段集。
type videoTaskBillingInput struct {
	LocalID       string
	UserID        int64
	APIKeyID      int64
	AccountID     int64
	Account       *Account // 成功结算必需（账号配额/last_used 调度）；失败日志可为 nil
	Model         string
	UpstreamModel string
	Resolution    string
	DurationSec   int
	// RequestedDurationSec 用户在创建请求里指定的时长。上游不回传真实时长时用它兜底，
	// 避免一律按默认 8 秒计价。
	RequestedDurationSec int
	ReservedCost         *float64
}

// VideoTaskBillingRequestID 返回视频任务的稳定幂等键：
// 同一任务无论轮询/重试多少次，只扣一次费、只记一条日志。
func VideoTaskBillingRequestID(localID string) string {
	localID = strings.TrimSpace(localID)
	if localID == "" {
		return ""
	}
	if strings.HasPrefix(localID, videoTaskBillingRequestIDPrefix) {
		return localID
	}
	return videoTaskBillingRequestIDPrefix + localID
}

// videoTaskLoadAPIKey 加载参与结算的 API Key（分组倍率与视频价格配置随行）。
func videoTaskLoadAPIKey(ctx context.Context, deps *videoTaskBillingDeps, apiKeyID int64) (*APIKey, error) {
	if deps == nil || deps.apiKeyService == nil {
		return nil, fmt.Errorf("video billing: apiKeyService is not wired")
	}
	apiKey, err := deps.apiKeyService.GetByID(ctx, apiKeyID)
	if err != nil {
		return nil, fmt.Errorf("video billing: load api key %d: %w", apiKeyID, err)
	}
	if apiKey == nil || apiKey.GroupID == nil || apiKey.Group == nil {
		return nil, fmt.Errorf("video billing: api key %d has no group", apiKeyID)
	}
	return apiKey, nil
}

// videoTaskBillingDuration 确定计费时长：上游回传真实时长 > 用户请求时长 > 默认 8 秒。
func videoTaskBillingDuration(actualSeconds, requestedSeconds int) int {
	return NormalizeVideoBillingDurationSeconds(actualSeconds, requestedSeconds)
}

// resolveVideoTaskPricing 解析渠道/分组定价（与同步网关视频链路同一解析器）。
// 解析器未装配或模型无显式定价时返回 nil，由调用方回退到分组媒体视频价。
func resolveVideoTaskPricing(ctx context.Context, deps *videoTaskBillingDeps, apiKey *APIKey, model string) *ResolvedPricing {
	gw := deps.openAIGatewayService
	if gw == nil || gw.resolver == nil || apiKey == nil || apiKey.GroupID == nil || apiKey.Group == nil {
		return nil
	}
	gid := *apiKey.GroupID
	resolved := gw.resolver.Resolve(ctx, PricingInput{Model: model, GroupID: &gid, Group: apiKey.Group})
	if resolved == nil {
		return nil
	}
	if resolved.Source != PricingSourceGroup && resolved.Source != PricingSourceChannel {
		return nil
	}
	return resolved
}

// resolveVideoTaskBillingTier 在渠道/分组定价卡上选出实际计价档位：
// 优先请求档位，未配价时沿降档链向低档找第一个配了价的档位（与分组媒体价口径一致）。
// 都没配价时返回请求档位，交由 DefaultPerRequestPrice 兜底。
func resolveVideoTaskBillingTier(resolver *ModelPricingResolver, resolved *ResolvedPricing, requested string) string {
	if resolver == nil || resolved == nil {
		return requested
	}
	for _, tier := range VideoBillingResolutionFallbacks(requested) {
		// GetRequestTierPrice 按 tier_label 精确匹配，返回 0 表示该档位未配价。
		if resolver.GetRequestTierPrice(resolved, tier) > 0 {
			if tier != requested {
				logger.L().Warn("video billing: resolution tier fallback",
					zap.String("requested_resolution", requested),
					zap.String("billed_resolution", tier),
					zap.String("pricing_source", resolved.Source),
				)
			}
			return tier
		}
	}
	return requested
}

// calculateVideoTaskUnifiedCost 用渠道/分组定价卡按「每秒单价 × 时长 × 分辨率档」计价。
// units 为计费单位数：video 模式传时长（每秒价），per_request / image 模式传视频数（按次）。
func calculateVideoTaskUnifiedCost(
	ctx context.Context,
	deps *videoTaskBillingDeps,
	apiKey *APIKey,
	model, resolution string,
	units float64,
	videoMultiplier float64,
	resolved *ResolvedPricing,
) *CostBreakdown {
	gw := deps.openAIGatewayService
	if gw == nil || gw.resolver == nil || apiKey.GroupID == nil || resolved == nil {
		return nil
	}
	gid := *apiKey.GroupID
	cost, err := deps.billingService.CalculateCostUnified(CostInput{
		Ctx:            ctx,
		Model:          model,
		GroupID:        &gid,
		Group:          apiKey.Group,
		RequestCount:   1,
		UsageUnits:     units,
		SizeTier:       resolution,
		RateMultiplier: videoMultiplier,
		Resolver:       gw.resolver,
		Resolved:       resolved,
	})
	if err != nil || cost == nil {
		if err != nil {
			logger.L().Warn("video billing: unified cost calculation failed",
				zap.String("model", model),
				zap.String("resolution", resolution),
				zap.Error(err),
			)
		}
		return nil
	}
	cost.BillingMode = string(BillingModeVideo)
	return cost
}

// videoTaskCostBreakdown 计算视频任务费用。
//
// 价格来源顺序与同步网关视频链路（calculateOpenAIVideoCost）保持一致：
//  1. 分组定价卡（groups.model_pricing，Mode=video）→ 按秒 × 分辨率档
//  2. 分组媒体视频价（video_price_* / video_model_prices）→ CalculateVideoCost
//  3. 渠道定价（Mode ∈ per_request / image / video）→ video 模式按秒，其余按次
//  4. 兜底：CalculateVideoCost 走代码默认价
//
// 这样管理员既可以用分组媒体价，也可以在渠道上单独配视频价格。
func videoTaskCostBreakdown(ctx context.Context, deps *videoTaskBillingDeps, apiKey *APIKey, model, resolution string, actualSec, requestedSec int) (*CostBreakdown, float64) {
	baseMultiplier := apiKey.Group.RateMultiplier
	if gw := deps.openAIGatewayService; gw != nil {
		baseMultiplier = gw.ResolveUserGroupRateMultiplier(ctx, apiKey.UserID, *apiKey.GroupID, apiKey.Group.RateMultiplier)
	}
	videoMultiplier := resolveVideoRateMultiplier(apiKey, baseMultiplier)
	groupConfig := videoPriceConfigFromAPIKey(apiKey)
	durationSec := videoTaskBillingDuration(actualSec, requestedSec)
	billingResolution := NormalizeVideoBillingResolutionAnyOrDefault(resolution)

	// 1) 分组定价卡
	if resolved := resolveVideoTaskPricing(ctx, deps, apiKey, model); resolved != nil &&
		resolved.Source == PricingSourceGroup && resolved.Mode == BillingModeVideo {
		tier := resolveVideoTaskBillingTier(deps.openAIGatewayService.resolver, resolved, billingResolution)
		if cost := calculateVideoTaskUnifiedCost(ctx, deps, apiKey, model, tier, float64(durationSec), videoMultiplier, resolved); cost != nil {
			return cost, videoMultiplier
		}
	}

	// 2) 分组媒体视频价
	if apiKeyHasConfiguredVideoPrice(apiKey, model, billingResolution) {
		return deps.billingService.CalculateVideoCost(model, resolution, 1, durationSec, groupConfig, videoMultiplier), videoMultiplier
	}

	// 3) 渠道定价
	if resolved := resolveVideoTaskPricing(ctx, deps, apiKey, model); resolved != nil &&
		resolved.Source == PricingSourceChannel &&
		(resolved.Mode == BillingModePerRequest || resolved.Mode == BillingModeImage || resolved.Mode == BillingModeVideo) {
		// 渠道 per_request / image 保持"按次"口径（价格由管理员按次配置），不乘时长。
		units := 1.0
		if resolved.Mode == BillingModeVideo {
			units = float64(durationSec)
		}
		tier := resolveVideoTaskBillingTier(deps.openAIGatewayService.resolver, resolved, billingResolution)
		if cost := calculateVideoTaskUnifiedCost(ctx, deps, apiKey, model, tier, units, videoMultiplier, resolved); cost != nil {
			return cost, videoMultiplier
		}
	}

	// 4) 兜底
	return deps.billingService.CalculateVideoCost(model, resolution, 1, durationSec, groupConfig, videoMultiplier), videoMultiplier
}

// estimateVideoTaskCost 预估实际费用：创建时预扣与结果落库算价共用，保证两处口径一致。
// actualSec 为上游回传的真实时长（创建时未知传 0），requestedSec 为用户请求时长。
func estimateVideoTaskCost(ctx context.Context, deps *videoTaskBillingDeps, apiKeyID int64, model, resolution string, actualSec, requestedSec int) (float64, error) {
	if deps == nil || deps.billingService == nil {
		return 0, fmt.Errorf("video billing: billing dependencies are not wired")
	}
	apiKey, err := videoTaskLoadAPIKey(ctx, deps, apiKeyID)
	if err != nil {
		return 0, err
	}
	cost, _ := videoTaskCostBreakdown(ctx, deps, apiKey, model, resolution, actualSec, requestedSec)
	return cost.ActualCost, nil
}

// settleVideoTaskSuccess 视频任务成功终态结算（调用方须用 UpdateResult 的 claimed
// 守卫防止同一任务重复触发）：
//  1. CalculateVideoCost 实际费用 → applyUsageBilling 原子扣费（余额/订阅/Key配额/账号配额）
//  2. 写 usage_logs（video 计费模式，与普通模型同表，用户/管理员可见）
//  3. 扣费成功后退还创建时的预扣（净效果 = 实际费用，与普通模型口径一致）
//  4. 上下文不可用/扣费失败时降级：退预扣（或保留预扣近似入账）+ 0 费用日志，绝不阻断任务状态
//
// 返回实际入账费用（0 表示未扣费，交由 0 费用日志对账）。
func settleVideoTaskSuccess(ctx context.Context, deps *videoTaskBillingDeps, in *videoTaskBillingInput) float64 {
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
		logger.L().Warn("video billing: settlement context unavailable, refund reserved quota",
			zap.String("local_id", in.LocalID),
			zap.Int64("api_key_id", in.APIKeyID),
			zap.Error(keyErr),
		)
		releaseMediaReservedQuota(deps.apiKeyService, ctx, in.APIKeyID, reservedCostOf(in.ReservedCost))
		writeVideoTaskZeroCostUsageLog(ctx, deps, in, nil, now)
		return 0
	}

	cost, videoMultiplier := videoTaskCostBreakdown(ctx, deps, apiKey, in.Model, in.Resolution, in.DurationSec, in.RequestedDurationSec)

	// 订阅分组且存在有效订阅 → 走订阅计费，否则走余额
	var subscription *UserSubscription
	isSubscriptionBill := false
	if apiKey.Group.IsSubscriptionType() && gw.userSubRepo != nil {
		if sub, subErr := gw.userSubRepo.GetActiveByUserIDAndGroupID(ctx, apiKey.UserID, *apiKey.GroupID); subErr == nil && sub != nil {
			subscription = sub
			isSubscriptionBill = true
		}
	}
	billingType := BillingTypeBalance
	if isSubscriptionBill {
		billingType = BillingTypeSubscription
	}

	usageLog := buildVideoTaskUsageLog(in, apiKey, subscription, cost, videoMultiplier, billingType, now)

	// Simple 运行模式：只记日志不扣费（与普通模型行为一致）
	if gw.cfg != nil && gw.cfg.RunMode == config.RunModeSimple {
		writeUsageLogBestEffort(ctx, gw.usageLogRepo, usageLog, "service.video_task")
		releaseMediaReservedQuota(deps.apiKeyService, ctx, in.APIKeyID, reservedCostOf(in.ReservedCost))
		return 0
	}

	// 原子扣费（幂等键 video-task:<local_id>，轮询重试不会重复扣费）
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
		writeUsageLogBestEffort(ctx, gw.usageLogRepo, usageLog, "service.video_task")
		logger.L().Warn("video billing: apply billing failed, reserved quota kept for reconciliation",
			zap.String("request_id", usageLog.RequestID),
			zap.Int64("api_key_id", in.APIKeyID),
			zap.Error(billingErr),
		)
		return 0
	}

	// 扣费成功：退还创建时的预扣（Key 配额净效果 = ActualCost）
	releaseMediaReservedQuota(deps.apiKeyService, ctx, in.APIKeyID, reservedCostOf(in.ReservedCost))
	writeUsageLogBestEffort(ctx, gw.usageLogRepo, usageLog, "service.video_task")
	return cost.ActualCost
}

// settleVideoTaskFailure 视频任务失败/取消终态：退还预扣 + 写 0 费用 usage_logs。
// 0 费用行让用户与管理员在用量记录里也能看到这次失败的视频调用。
func settleVideoTaskFailure(ctx context.Context, deps *videoTaskBillingDeps, in *videoTaskBillingInput) {
	if deps == nil || in == nil {
		return
	}
	releaseMediaReservedQuota(deps.apiKeyService, ctx, in.APIKeyID, reservedCostOf(in.ReservedCost))
	apiKey, keyErr := videoTaskLoadAPIKey(ctx, deps, in.APIKeyID)
	if keyErr != nil {
		apiKey = nil
	}
	writeVideoTaskZeroCostUsageLog(ctx, deps, in, apiKey, time.Now())
}

// writeVideoTaskZeroCostUsageLog 写 0 费用视频任务日志（失败/取消/上下文缺失兜底）。
// requestID 与成功结算同键：usage_logs 的 ON CONFLICT (request_id, api_key_id)
// 保证同一任务的成败日志互斥、先到先得。
func writeVideoTaskZeroCostUsageLog(ctx context.Context, deps *videoTaskBillingDeps, in *videoTaskBillingInput, apiKey *APIKey, now time.Time) {
	if deps == nil || in == nil {
		return
	}
	gw := deps.openAIGatewayService
	if gw == nil || gw.usageLogRepo == nil {
		return
	}
	usageLog := buildVideoTaskUsageLog(in, apiKey, nil, nil, 0, BillingTypeBalance, now)
	writeUsageLogBestEffort(ctx, gw.usageLogRepo, usageLog, "service.video_task")
}

// buildVideoTaskUsageLog 构造视频任务的 usage_logs 行（字段对齐 Grok 视频模板）。
func buildVideoTaskUsageLog(in *videoTaskBillingInput, apiKey *APIKey, subscription *UserSubscription, cost *CostBreakdown, videoMultiplier float64, billingType int8, now time.Time) *UsageLog {
	resolution := NormalizeVideoBillingResolutionAnyOrDefault(in.Resolution)
	durationSec := videoTaskBillingDuration(in.DurationSec, in.RequestedDurationSec)
	usageLog := &UsageLog{
		UserID:               in.UserID,
		APIKeyID:             in.APIKeyID,
		AccountID:            in.AccountID,
		RequestID:            VideoTaskBillingRequestID(in.LocalID),
		Model:                in.Model,
		RequestedModel:       in.Model,
		UpstreamModel:        optionalTrimmedStringPtr(in.UpstreamModel),
		VideoCount:           1,
		VideoResolution:      &resolution,
		VideoDurationSeconds: &durationSec,
		RateMultiplier:       videoMultiplier,
		BillingType:          billingType,
		RequestType:          RequestTypeSync,
		CreatedAt:            now,
	}
	if in.Account != nil {
		m := in.Account.BillingRateMultiplier()
		usageLog.AccountRateMultiplier = &m
	}
	if cost != nil {
		usageLog.TotalCost = cost.TotalCost
		usageLog.ActualCost = cost.ActualCost
	}
	billingMode := string(BillingModeVideo)
	usageLog.BillingMode = &billingMode
	if apiKey != nil {
		usageLog.GroupID = apiKey.GroupID
	}
	if subscription != nil {
		usageLog.SubscriptionID = &subscription.ID
	}
	return usageLog
}

// reservedCostOf 安全读取预扣费用（nil/非正值归零）。
func reservedCostOf(reserved *float64) float64 {
	if reserved == nil || *reserved <= 0 {
		return 0
	}
	return *reserved
}
