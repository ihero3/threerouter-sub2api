package service

import (
	"testing"
	"time"
)

// TestMediaImageBillingRequestID 图片任务幂等键必须稳定且幂等：
// 同一 local_id 重复构造返回同一值，usage_logs 才能靠 (request_id, api_key_id) 去重。
func TestMediaImageBillingRequestID(t *testing.T) {
	if got := MediaImageBillingRequestID(""); got != "" {
		t.Fatalf("empty local id should yield empty request id, got %q", got)
	}
	first := MediaImageBillingRequestID("img-abc")
	if first != "media-image:img-abc" {
		t.Fatalf("unexpected request id: %q", first)
	}
	if second := MediaImageBillingRequestID(first); second != first {
		t.Fatalf("request id must be idempotent, got %q want %q", second, first)
	}
}

// TestBuildMediaImageUsageLogSetsImageFields 图片日志必须带齐 image 模式字段，
// 否则「使用记录」里无法按图片口径展示与统计。
func TestBuildMediaImageUsageLogSetsImageFields(t *testing.T) {
	in := &mediaImageBillingInput{
		LocalID:       "img-1",
		UserID:        7,
		APIKeyID:      9,
		AccountID:     3,
		Model:         "qwen-image-plus",
		UpstreamModel: "qwen-image-plus",
		Resolution:    "1024x1024",
		ImageCount:    2,
	}
	groupID := int64(5)
	apiKey := &APIKey{ID: 9, GroupID: &groupID}
	cost := &CostBreakdown{TotalCost: 2, ActualCost: 2, BillingMode: string(BillingModeImage)}

	log := buildMediaImageUsageLog(in, apiKey, nil, cost, 1.5, BillingTypeBalance, time.Now())

	if log.ImageCount != 2 {
		t.Fatalf("ImageCount = %d, want 2", log.ImageCount)
	}
	if log.ImageSize == nil || *log.ImageSize != "1024x1024" {
		t.Fatalf("ImageSize = %v, want 1024x1024", log.ImageSize)
	}
	if log.BillingMode == nil || *log.BillingMode != string(BillingModeImage) {
		t.Fatalf("BillingMode = %v, want image", log.BillingMode)
	}
	if log.RequestID != "media-image:img-1" {
		t.Fatalf("RequestID = %q, want media-image:img-1", log.RequestID)
	}
	if log.ActualCost != 2 {
		t.Fatalf("ActualCost = %v, want 2", log.ActualCost)
	}
	if log.GroupID == nil || *log.GroupID != 5 {
		t.Fatalf("GroupID = %v, want 5", log.GroupID)
	}
}

// TestBuildMediaImageUsageLogDefaultsSize 未指定尺寸时落到 2K 默认档，
// 与预扣算价口径（ImageBillingSize2K）保持一致，避免日志与扣费档位不一致。
func TestBuildMediaImageUsageLogDefaultsSize(t *testing.T) {
	in := &mediaImageBillingInput{LocalID: "img-2", Model: "minimax-image-01", ImageCount: 1}
	log := buildMediaImageUsageLog(in, nil, nil, nil, 1, BillingTypeBalance, time.Now())
	if log.ImageSize == nil || *log.ImageSize != ImageBillingSize2K {
		t.Fatalf("ImageSize = %v, want %s", log.ImageSize, ImageBillingSize2K)
	}
	if log.ImageCount != 1 {
		t.Fatalf("ImageCount = %d, want 1", log.ImageCount)
	}
}

// TestMediaImageBillingInputFromRecord 结算入参必须完整携带预扣与模型信息，
// 缺一会导致预扣无法退还或日志模型名错位。
func TestMediaImageBillingInputFromRecord(t *testing.T) {
	reserved := 3.5
	record := &MediaTaskRecord{
		LocalID:       "img-3",
		UserID:        1,
		APIKeyID:      2,
		AccountID:     3,
		PublicModel:   "qwen-image-plus",
		UpstreamModel: "qwen-image-plus",
		Resolution:    "1536x1024",
		ReservedCost:  &reserved,
	}
	in := mediaImageBillingInputFromRecord(record, 4)
	if in == nil {
		t.Fatal("input should not be nil")
	}
	if in.LocalID != "img-3" || in.APIKeyID != 2 || in.ImageCount != 4 {
		t.Fatalf("unexpected input: %+v", in)
	}
	if in.ReservedCost == nil || *in.ReservedCost != 3.5 {
		t.Fatalf("ReservedCost not carried: %v", in.ReservedCost)
	}
	if got := reservedCostOf(in.ReservedCost); got != 3.5 {
		t.Fatalf("reservedCostOf = %v, want 3.5", got)
	}
	if nilRecord := mediaImageBillingInputFromRecord(nil, 1); nilRecord != nil {
		t.Fatal("nil record should yield nil input")
	}
}
