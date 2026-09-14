package service

import (
	"context"
	"testing"
	"time"
)

// TestMediaAudioBillingRequestID 音频任务幂等键必须稳定且幂等。
func TestMediaAudioBillingRequestID(t *testing.T) {
	if got := MediaAudioBillingRequestID(""); got != "" {
		t.Fatalf("empty local id should yield empty request id, got %q", got)
	}
	first := MediaAudioBillingRequestID("aud-1")
	if first != "media-audio:aud-1" {
		t.Fatalf("unexpected request id: %q", first)
	}
	if second := MediaAudioBillingRequestID(first); second != first {
		t.Fatalf("request id must be idempotent, got %q want %q", second, first)
	}
}

// TestBuildMediaAudioUsageLogUsesPerRequestMode 音频没有专属 BillingMode 枚举，
// 用 per_request 标识；关键是必须有 request_id / group_id / 费用，否则对不上账。
func TestBuildMediaAudioUsageLogUsesPerRequestMode(t *testing.T) {
	groupID := int64(11)
	apiKey := &APIKey{ID: 4, GroupID: &groupID}
	reserved := 2.0
	in := &mediaAudioBillingInput{
		LocalID:              "aud-1",
		UserID:               1,
		APIKeyID:             4,
		AccountID:            6,
		Model:                "tts-1",
		UpstreamModel:        "tts-1",
		DurationSec:          12,
		RequestedDurationSec: 12,
		ReservedCost:         &reserved,
	}
	cost := &CostBreakdown{TotalCost: 1.5, ActualCost: 1.5}

	log := buildMediaAudioUsageLog(in, apiKey, nil, cost, 1, BillingTypeBalance, time.Now())

	if log.RequestID != "media-audio:aud-1" {
		t.Fatalf("RequestID = %q, want media-audio:aud-1", log.RequestID)
	}
	if log.BillingMode == nil || *log.BillingMode != string(BillingModePerRequest) {
		t.Fatalf("BillingMode = %v, want per_request", log.BillingMode)
	}
	if log.ActualCost != 1.5 {
		t.Fatalf("ActualCost = %v, want 1.5", log.ActualCost)
	}
	if log.GroupID == nil || *log.GroupID != 11 {
		t.Fatalf("GroupID = %v, want 11", log.GroupID)
	}
	if log.Model != "tts-1" || log.UpstreamModel == nil || *log.UpstreamModel != "tts-1" {
		t.Fatalf("model fields mismatch: %+v", log)
	}
}

// TestMediaAudioBillingInputFromRecord 异步/轮询路径的入参必须带上请求时长与预扣。
func TestMediaAudioBillingInputFromRecord(t *testing.T) {
	reserved := 4.25
	record := &MediaTaskRecord{
		LocalID:       "aud-2",
		UserID:        2,
		APIKeyID:      3,
		AccountID:     4,
		PublicModel:   "tts-1-hd",
		UpstreamModel: "tts-1-hd",
		DurationSec:   30,
		ReservedCost:  &reserved,
	}
	in := mediaAudioBillingInputFromRecord(record)
	if in == nil {
		t.Fatal("input should not be nil")
	}
	if in.DurationSec != 30 || in.RequestedDurationSec != 30 {
		t.Fatalf("duration not carried: actual=%d requested=%d", in.DurationSec, in.RequestedDurationSec)
	}
	if in.ReservedCost == nil || *in.ReservedCost != 4.25 {
		t.Fatalf("ReservedCost not carried: %v", in.ReservedCost)
	}
	if nilRecord := mediaAudioBillingInputFromRecord(nil); nilRecord != nil {
		t.Fatal("nil record should yield nil input")
	}
}

// TestCalculateAudioTaskCostBreakdownPrefersPerSecondPrice 配了每秒价时按秒计费，
// 未配时按分钟（realtime）——与分组音频价配置语义一致。
func TestCalculateAudioTaskCostBreakdownPrefersPerSecondPrice(t *testing.T) {
	perSec := 0.02
	perMin := 1.2
	group := &Group{ID: 1, RateMultiplier: 1}
	group.AudioPricePerSec = &perSec
	group.AudioRealtimePricePerMin = &perMin
	groupID := int64(1)
	apiKey := &APIKey{ID: 1, GroupID: &groupID, Group: group}
	billing := NewBillingService(nil, nil)
	ctx := context.Background()

	perSecCost, _ := calculateAudioTaskCostBreakdown(ctx, billing, nil, apiKey, 30, 30)
	if perSecCost == nil {
		t.Fatal("per-second cost should not be nil")
	}
	if perSecCost.ActualCost <= 0 {
		t.Fatalf("per-second cost = %v, want > 0", perSecCost.ActualCost)
	}

	// 去掉每秒价 → 走 realtime 按分钟
	group.AudioPricePerSec = nil
	realtimeCost, _ := calculateAudioTaskCostBreakdown(ctx, billing, nil, apiKey, 60, 60)
	if realtimeCost == nil || realtimeCost.ActualCost <= 0 {
		t.Fatalf("realtime cost = %v, want > 0", realtimeCost)
	}

	// 时长更长应当更贵（同一计费口径下的单调性）
	short, _ := calculateAudioTaskCostBreakdown(ctx, billing, nil, apiKey, 30, 30)
	long, _ := calculateAudioTaskCostBreakdown(ctx, billing, nil, apiKey, 120, 120)
	if long.ActualCost <= short.ActualCost {
		t.Fatalf("longer audio should cost more: 30s=%v 120s=%v", short.ActualCost, long.ActualCost)
	}
}
