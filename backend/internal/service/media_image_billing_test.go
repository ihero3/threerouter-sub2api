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
	// image_size 存的是计费档位，原始请求尺寸落在 image_input_size。
	if log.ImageSize == nil || *log.ImageSize != ImageBillingSize1K {
		t.Fatalf("ImageSize = %v, want %s", log.ImageSize, ImageBillingSize1K)
	}
	if log.ImageInputSize == nil || *log.ImageInputSize != "1024x1024" {
		t.Fatalf("ImageInputSize = %v, want 1024x1024", log.ImageInputSize)
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

// TestBuildMediaImageUsageLogSizeConsistentAcrossVendors 两个图片厂商的明细口径必须一致。
//
// 千问会回传真实输出像素（usage.output_width/height），MiniMax 什么都不回传，
// 客户端还可能只写 "16:9"。若各按各的原始值落库，同一档的图在明细里会显示成
// 不同字符串、甚至落到不同计费档。这里锁定：无论哪个厂商，image_size 都是
// 统一档位（1K/2K/4K），原始值分别落在 image_input_size / image_output_size。
func TestBuildMediaImageUsageLogSizeConsistentAcrossVendors(t *testing.T) {
	build := func(requested, output string) *UsageLog {
		in := &mediaImageBillingInput{
			LocalID:       "img-size",
			Model:         "image",
			Resolution:    requested,
			RequestedSize: requested,
			OutputSize:    output,
			ImageCount:    1,
		}
		return buildMediaImageUsageLog(in, nil, nil, nil, 1, BillingTypeBalance, time.Now())
	}

	// 千问：请求 1024x1024，真实输出 1664x928（长边 1664 → 2K）
	qwen := build("1024x1024", "1664x928")
	if qwen.ImageSize == nil || *qwen.ImageSize != ImageBillingSize2K {
		t.Fatalf("千问 ImageSize = %v, want %s", qwen.ImageSize, ImageBillingSize2K)
	}
	if qwen.ImageSizeSource == nil || *qwen.ImageSizeSource != ImageSizeSourceOutput {
		t.Fatalf("千问 ImageSizeSource = %v, want %s", qwen.ImageSizeSource, ImageSizeSourceOutput)
	}
	if qwen.ImageOutputSize == nil || *qwen.ImageOutputSize != "1664x928" {
		t.Fatalf("千问 ImageOutputSize = %v, want 1664x928", qwen.ImageOutputSize)
	}

	// MiniMax：只给宽高比 16:9（折算 1280x720，长边 1280 → 2K），无真实输出
	minimax := build("16:9", "")
	if minimax.ImageSize == nil || *minimax.ImageSize != ImageBillingSize2K {
		t.Fatalf("MiniMax ImageSize = %v, want %s", minimax.ImageSize, ImageBillingSize2K)
	}
	if minimax.ImageSizeSource == nil || *minimax.ImageSizeSource != ImageSizeSourceInput {
		t.Fatalf("MiniMax ImageSizeSource = %v, want %s", minimax.ImageSizeSource, ImageSizeSourceInput)
	}

	// 同档 → 明细里的档位串必须一致（用户对账时看的是这一列）
	if *qwen.ImageSize != *minimax.ImageSize {
		t.Fatalf("两个厂商同档却显示不同：%q vs %q", *qwen.ImageSize, *minimax.ImageSize)
	}

	// 1K 档：真实输出 1024x1024
	small := build("1024x1024", "1024x1024")
	if small.ImageSize == nil || *small.ImageSize != ImageBillingSize1K {
		t.Fatalf("1K 图 ImageSize = %v, want %s", small.ImageSize, ImageBillingSize1K)
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
