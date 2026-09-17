package service

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// media_audio_billing.go — 音频任务统一结算。
//
// 与图片同源的问题：音频此前只做 API Key 配额预扣，既不写 usage_logs 也不扣余额/订阅，
// 「使用记录」里看不到音频调用。这里按 media_image_billing.go 的同款口径补齐：
//  1. 幂等键：media-audio:<local_id>，同一任务只记一条日志、只扣一次费
//  2. 写 usage_logs → applyUsageBilling 原子扣费（余额/订阅/Key 配额/账号配额）
//  3. 扣费成功后退还创建时的预扣，净效果 = 实际费用
//
// 计费模式：UsageLog 没有 audio 专属字段与 BillingMode 枚举，这里用 per_request
// （按次/按时长混合口径，与分组音频价 audio_price_config 一致）。音频时长暂不落库
// ——待 usage_logs 增加 audio_duration_seconds 列后再补。
//
// 音频 adapter 目前都是同步返回（media_vendor_audio_adapter.go），创建成功即结算；
// 若将来出现异步音频，refreshTaskStatus 命中 succeeded 时同样会结算，两处互斥。

// mediaAudioBillingRequestIDPrefix 音频任务计费幂等键前缀。
const mediaAudioBillingRequestIDPrefix = "media-audio:"

// mediaAudioBillingInput 单个音频任务终态结算所需的最小字段集。
type mediaAudioBillingInput struct {
	LocalID       string
	UserID        int64
	APIKeyID      int64
	AccountID     int64
	Account       *Account // 成功结算必需；失败日志可为 nil
	Model         string
	UpstreamModel string
	DurationSec   int
	// RequestedDurationSec 用户在创建请求里指定的时长。上游不回传真实时长时用它兜底。
	RequestedDurationSec int
	ReservedCost         *float64
	// Meta 创建阶段采集的明细元数据，语义同 mediaImageBillingInput.Meta。
	Meta *mediaUsageMeta
}

// MediaAudioBillingRequestID 返回音频任务的稳定幂等键。
func MediaAudioBillingRequestID(localID string) string {
	localID = strings.TrimSpace(localID)
	if localID == "" {
		return ""
	}
	if strings.HasPrefix(localID, mediaAudioBillingRequestIDPrefix) {
		return localID
	}
	return mediaAudioBillingRequestIDPrefix + localID
}

// calculateAudioTaskCostBreakdown 计算音频任务费用明细。
// 价格来源与 MediaTaskService.calculateMediaCost 的音频分支完全一致：
// 配了每秒价 → 按秒（media_audio）；否则按分钟（realtime）。
// 两处共用同一实现，避免预扣与实际口径漂移。
func calculateAudioTaskCostBreakdown(
	ctx context.Context,
	billingService *BillingService,
	gw *OpenAIGatewayService,
	apiKey *APIKey,
	actualSec, requestedSec int,
) (*CostBreakdown, float64) {
	if billingService == nil || apiKey == nil {
		return nil, 0
	}
	baseMultiplier := apiKey.Group.RateMultiplier
	if gw != nil && apiKey.GroupID != nil {
		baseMultiplier = gw.ResolveUserGroupRateMultiplier(ctx, apiKey.UserID, *apiKey.GroupID, apiKey.Group.RateMultiplier)
	}
	audioCfg := groupAudioPriceConfigFromAPIKey(apiKey)
	// 音频同样优先用上游回传真实时长，缺失时回退用户请求时长。
	audioSecs := NormalizeVideoBillingDurationSeconds(actualSec, requestedSec)
	var cost *CostBreakdown
	if audioCfg != nil && audioCfg.PerSec != nil {
		secs := audioSecs
		if secs <= 0 {
			secs = 1
		}
		cost = billingService.CalculateAudioCost("media_audio", float64(secs), audioCfg, baseMultiplier)
	} else {
		mins := float64(audioSecs) / 60.0
		if mins <= 0 {
			mins = 1.0 / 60.0
		}
		cost = billingService.CalculateAudioCost("realtime", mins, audioCfg, baseMultiplier)
	}
	if cost == nil {
		return nil, baseMultiplier
	}
	if cost.BillingMode == "" {
		cost.BillingMode = string(BillingModePerRequest)
	}
	return cost, baseMultiplier
}

// settleMediaAudioTaskSuccess 音频任务成功终态结算：
//  1. 算实际费用 → usage_logs → applyUsageBilling 原子扣费
//  2. 扣费成功后退还创建时的预扣（Key 配额净效果 = ActualCost）
//  3. 上下文缺失/扣费失败时降级：保留预扣近似入账 + 0 费用日志，绝不阻断任务状态
//
// 返回实际入账费用（0 表示未扣费，交由 0 费用日志对账）。
func settleMediaAudioTaskSuccess(ctx context.Context, deps *videoTaskBillingDeps, in *mediaAudioBillingInput) float64 {
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
	if keyErr != nil || user == nil || deps.billingService == nil || in.Account == nil || gw == nil {
		logger.L().Warn("audio billing: settlement context unavailable, refund reserved quota",
			zap.String("local_id", in.LocalID),
			zap.Int64("api_key_id", in.APIKeyID),
			zap.Error(keyErr),
		)
		releaseMediaReservedQuota(deps.apiKeyService, ctx, in.APIKeyID, reservedCostOf(in.ReservedCost))
		writeMediaAudioZeroCostUsageLog(ctx, deps, in, nil, now)
		return 0
	}

	cost, multiplier := calculateAudioTaskCostBreakdown(ctx, deps.billingService, gw, apiKey, in.DurationSec, in.RequestedDurationSec)
	if cost == nil {
		// 同图片链路：算不出费用也必须留下可见的 0 费用行，
		// 不能让一次成功的音频合成在用量记录里无痕消失。
		logger.L().Warn("audio billing: cost calculation returned nil, refund reserved quota",
			zap.String("local_id", in.LocalID),
			zap.String("model", in.Model),
		)
		releaseMediaReservedQuota(deps.apiKeyService, ctx, in.APIKeyID, reservedCostOf(in.ReservedCost))
		writeMediaAudioZeroCostUsageLog(ctx, deps, in, apiKey, now)
		return 0
	}

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

	usageLog := buildMediaAudioUsageLog(in, apiKey, subscription, cost, multiplier, billingType, now)

	// Simple 运行模式：只记日志不扣费（与图片/视频/普通模型行为一致）
	if gw.cfg != nil && gw.cfg.RunMode == config.RunModeSimple {
		writeUsageLogBestEffort(ctx, gw.usageLogRepo, usageLog, "service.media_audio")
		releaseMediaReservedQuota(deps.apiKeyService, ctx, in.APIKeyID, reservedCostOf(in.ReservedCost))
		return 0
	}

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
		usageLog.TotalCost = 0
		usageLog.ActualCost = 0
		writeUsageLogBestEffort(ctx, gw.usageLogRepo, usageLog, "service.media_audio")
		logger.L().Warn("audio billing: apply billing failed, reserved quota kept for reconciliation",
			zap.String("request_id", usageLog.RequestID),
			zap.Int64("api_key_id", in.APIKeyID),
			zap.Error(billingErr),
		)
		return 0
	}

	releaseMediaReservedQuota(deps.apiKeyService, ctx, in.APIKeyID, reservedCostOf(in.ReservedCost))
	writeUsageLogBestEffort(ctx, gw.usageLogRepo, usageLog, "service.media_audio")
	return cost.ActualCost
}

// settleMediaAudioTaskFailure 音频任务失败/取消终态：退还预扣 + 写 0 费用 usage_logs。
func settleMediaAudioTaskFailure(ctx context.Context, deps *videoTaskBillingDeps, in *mediaAudioBillingInput) {
	if deps == nil || in == nil {
		return
	}
	releaseMediaReservedQuota(deps.apiKeyService, ctx, in.APIKeyID, reservedCostOf(in.ReservedCost))
	apiKey, keyErr := videoTaskLoadAPIKey(ctx, deps, in.APIKeyID)
	if keyErr != nil {
		apiKey = nil
	}
	writeMediaAudioZeroCostUsageLog(ctx, deps, in, apiKey, time.Now())
}

// writeMediaAudioZeroCostUsageLog 写 0 费用音频任务日志（失败/取消/上下文缺失兜底）。
// requestID 与成功结算同键：usage_logs 的 ON CONFLICT (request_id, api_key_id)
// 保证同一任务的成败日志互斥、先到先得。
func writeMediaAudioZeroCostUsageLog(ctx context.Context, deps *videoTaskBillingDeps, in *mediaAudioBillingInput, apiKey *APIKey, now time.Time) {
	if deps == nil || in == nil {
		return
	}
	gw := deps.openAIGatewayService
	if gw == nil || gw.usageLogRepo == nil {
		return
	}
	usageLog := buildMediaAudioUsageLog(in, apiKey, nil, nil, 0, BillingTypeBalance, now)
	writeUsageLogBestEffort(ctx, gw.usageLogRepo, usageLog, "service.media_audio")
}

// buildMediaAudioUsageLog 构造音频任务的 usage_logs 行。
// UsageLog 目前没有音频专属字段，用 per_request 计费模式标识；时长暂不落库。
func buildMediaAudioUsageLog(in *mediaAudioBillingInput, apiKey *APIKey, subscription *UserSubscription, cost *CostBreakdown, multiplier float64, billingType int8, now time.Time) *UsageLog {
	usageLog := &UsageLog{
		UserID:         in.UserID,
		APIKeyID:       in.APIKeyID,
		AccountID:      in.AccountID,
		RequestID:      MediaAudioBillingRequestID(in.LocalID),
		Model:          in.Model,
		RequestedModel: in.Model,
		UpstreamModel:  optionalTrimmedStringPtr(in.UpstreamModel),
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
	}
	billingMode := string(BillingModePerRequest)
	usageLog.BillingMode = &billingMode
	if apiKey != nil {
		usageLog.GroupID = apiKey.GroupID
	}
	if subscription != nil {
		usageLog.SubscriptionID = &subscription.ID
	}
	applyMediaUsageMeta(usageLog, loadMediaUsageMeta(in.LocalID, in.Meta))
	return usageLog
}

// mediaAudioBillingInputFromRecord 从媒体任务记录构造音频结算入参。
func mediaAudioBillingInputFromRecord(record *MediaTaskRecord) *mediaAudioBillingInput {
	if record == nil {
		return nil
	}
	return &mediaAudioBillingInput{
		LocalID:              record.LocalID,
		UserID:               record.UserID,
		APIKeyID:             record.APIKeyID,
		AccountID:            record.AccountID,
		Model:                record.PublicModel,
		UpstreamModel:        record.UpstreamModel,
		DurationSec:          record.DurationSec,
		RequestedDurationSec: record.DurationSec,
		ReservedCost:         record.ReservedCost,
	}
}
