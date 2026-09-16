package service

// media_failure_usage.go — 媒体任务「整单失败」时也要在使用记录里留痕。
//
// 背景：选号阶段起就被判为可切换（failover）的上游失败——403 额度/鉴权、429、5xx——
// CreateTask 会把该账号排除后换号重试；所有账号都失败时直接返回错误。这条路径上
// **既没有 media_tasks 记录，也没有 usage_logs 行**，于是用户与运营在「用量明细」里
// 完全看不到这次调用（只有「错误请求」里能查到），排查时极易被误读成「没落库/没计费」。
//
// 这里补一条 0 费用 usage_logs 行：请求 ID 沿用媒体任务计费键（media-image:/media-audio:/
// video-task: + local_id），与成功结算共用幂等键，不会造成重复扣费，唯一目的是让
// 「这次调用确实发生过、且没有产生费用」这件事可被检索。

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// writeMediaTaskFailureUsageLog 为「未生成任务记录的整单失败」补一条 0 费用使用记录。
// 只在选号/上游全部失败时调用；参数校验类错误（400）在此之前就已返回，不写行。
func (s *MediaTaskService) writeMediaTaskFailureUsageLog(
	ctx context.Context,
	kind MediaKind,
	userID int64,
	apiKeyID int64,
	publicModel string,
	req *MediaCreateRequest,
) {
	if s == nil {
		return
	}
	deps := s.billingDeps()
	if deps == nil {
		return
	}
	gw := deps.openAIGatewayService
	if gw == nil || gw.usageLogRepo == nil {
		return
	}

	// 没有任务记录可用，用同格式的 local_id 生成唯一请求 ID。
	localID := generateMediaLocalID(kind)
	apiKey, keyErr := videoTaskLoadAPIKey(ctx, deps, apiKeyID)
	if keyErr != nil {
		apiKey = nil
	}

	resolution := ""
	durationSec := 0
	imageCount := 1
	if req != nil {
		resolution = req.Resolution
		durationSec = req.DurationSec
		if req.ImageCount > 0 {
			imageCount = req.ImageCount
		}
	}

	now := time.Now()
	switch kind {
	case MediaKindImage:
		writeMediaImageZeroCostUsageLog(ctx, deps, &mediaImageBillingInput{
			LocalID:       localID,
			UserID:        userID,
			APIKeyID:      apiKeyID,
			Model:         publicModel,
			UpstreamModel: publicModel,
			Resolution:    resolution,
			ImageCount:    imageCount,
		}, apiKey, now)
	case MediaKindAudio:
		writeMediaAudioZeroCostUsageLog(ctx, deps, &mediaAudioBillingInput{
			LocalID:              localID,
			UserID:               userID,
			APIKeyID:             apiKeyID,
			Model:                publicModel,
			UpstreamModel:        publicModel,
			DurationSec:          durationSec,
			RequestedDurationSec: durationSec,
		}, apiKey, now)
	default:
		writeVideoTaskZeroCostUsageLog(ctx, deps, &videoTaskBillingInput{
			LocalID:              localID,
			UserID:               userID,
			APIKeyID:             apiKeyID,
			Model:                publicModel,
			UpstreamModel:        publicModel,
			Resolution:           resolution,
			DurationSec:          durationSec,
			RequestedDurationSec: durationSec,
		}, apiKey, now)
	}

	if s.logger != nil {
		s.logger.Debug("media_task_service: failure usage log written",
			zap.String("kind", string(kind)),
			zap.String("local_id", localID),
			zap.String("model", publicModel),
			zap.Int64("api_key_id", apiKeyID),
		)
	}
}
