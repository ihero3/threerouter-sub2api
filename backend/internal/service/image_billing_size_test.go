package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClassifyImageBillingTier(t *testing.T) {
	tests := []struct {
		name     string
		size     string
		wantTier string
		wantOK   bool
	}{
		{name: "explicit 2k square", size: "2048x2048", wantTier: "2K", wantOK: true},
		{name: "explicit 2k landscape", size: "2048x1152", wantTier: "2K", wantOK: true},
		{name: "explicit 4k landscape", size: "3840x2160", wantTier: "4K", wantOK: true},
		{name: "explicit 4k portrait", size: "2160x3840", wantTier: "4K", wantOK: true},
		{name: "long edge 1k", size: "1024X768", wantTier: "1K", wantOK: true},
		{name: "long edge 2k", size: "1280x768", wantTier: "2K", wantOK: true},
		{name: "long edge 4k", size: "2560x1600", wantTier: "4K", wantOK: true},
		{name: "tier string 1k", size: "1k", wantTier: "1K", wantOK: true},
		{name: "empty", size: "", wantOK: false},
		{name: "auto", size: "auto", wantOK: false},
		{name: "invalid", size: "not-a-size", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTier, gotOK := ClassifyImageBillingTier(tt.size)
			require.Equal(t, tt.wantOK, gotOK)
			require.Equal(t, tt.wantTier, gotTier)
		})
	}
}

func TestResolveImageBillingSize(t *testing.T) {
	tests := []struct {
		name          string
		inputSize     string
		outputSizes   []string
		wantBilling   string
		wantOutput    string
		wantSource    string
		wantBreakdown map[string]int
	}{
		{
			name:          "output wins over input",
			inputSize:     "1024x1024",
			outputSizes:   []string{"3840x2160"},
			wantBilling:   "4K",
			wantOutput:    "3840x2160",
			wantSource:    ImageSizeSourceOutput,
			wantBreakdown: map[string]int{"4K": 1},
		},
		{
			name:        "input fallback",
			inputSize:   "1024x1024",
			wantBilling: "1K",
			wantSource:  ImageSizeSourceInput,
		},
		{
			name:        "auto defaults",
			inputSize:   "auto",
			wantBilling: "2K",
			wantSource:  ImageSizeSourceDefault,
		},
		{
			name:        "empty defaults",
			inputSize:   "",
			wantBilling: "2K",
			wantSource:  ImageSizeSourceDefault,
		},
		{
			name:        "invalid defaults",
			inputSize:   "largest",
			wantBilling: "2K",
			wantSource:  ImageSizeSourceDefault,
		},
		{
			name:          "mixed output chooses highest tier",
			inputSize:     "1024x1024",
			outputSizes:   []string{"1024x1024", "3840x2160", "1280x720"},
			wantBilling:   "4K",
			wantOutput:    "1024x1024",
			wantSource:    ImageSizeSourceOutput,
			wantBreakdown: map[string]int{"1K": 1, "2K": 1, "4K": 1},
		},
		{
			name:        "unparseable output falls back to parseable input",
			inputSize:   "2048x1152",
			outputSizes: []string{"auto"},
			wantBilling: "2K",
			wantOutput:  "auto",
			wantSource:  ImageSizeSourceInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveImageBillingSize(tt.inputSize, tt.outputSizes)
			require.Equal(t, tt.wantBilling, got.BillingSize)
			require.Equal(t, tt.inputSize, got.InputSize)
			require.Equal(t, tt.wantOutput, got.OutputSize)
			require.Equal(t, tt.wantSource, got.Source)
			require.Equal(t, tt.wantBreakdown, got.Breakdown)
		})
	}
}

func TestImageBillingSizeFallbacksDescends(t *testing.T) {
	require.Equal(t, []string{ImageBillingSize4K, ImageBillingSize2K, ImageBillingSize1K}, ImageBillingSizeFallbacks("4K"))
	require.Equal(t, []string{ImageBillingSize2K, ImageBillingSize1K}, ImageBillingSizeFallbacks("2K"))
	require.Equal(t, []string{ImageBillingSize1K}, ImageBillingSizeFallbacks("1K"))
	// WxH 输入先归一化（3840x2160 → 4K）再降档
	require.Equal(t, []string{ImageBillingSize4K, ImageBillingSize2K, ImageBillingSize1K}, ImageBillingSizeFallbacks("3840x2160"))
	// 未知尺寸归一到 2K 再降档
	require.Equal(t, []string{ImageBillingSize2K, ImageBillingSize1K}, ImageBillingSizeFallbacks("not-a-size"))
}

// TestCalculateImageCost_FallsBackToConfiguredLowerTier 分组只配了低档价格时，
// 请求高档位应沿降档链向已配档位回退（4K→2K→1K）并告警，而不是静默掉到默认价。
func TestCalculateImageCost_FallsBackToConfiguredLowerTier(t *testing.T) {
	svc := &BillingService{}
	price1K := 0.10
	price2K := 0.15

	// 只配了 1K：请求 4K 逐级降到 1K（0.10），而不是掉到默认价 0.134*2=0.268
	cost := svc.CalculateImageCost("gemini-3-pro-image", "4K", 1, &ImagePriceConfig{Price1K: &price1K}, 1.0)
	require.InDelta(t, 0.10, cost.TotalCost, 1e-9)

	// 配了 1K+2K：请求 4K 降到 2K
	cfg := &ImagePriceConfig{Price1K: &price1K, Price2K: &price2K}
	cost = svc.CalculateImageCost("gemini-3-pro-image", "4K", 1, cfg, 1.0)
	require.InDelta(t, 0.15, cost.TotalCost, 1e-9)

	// 精确命中档位不降档
	cost = svc.CalculateImageCost("gemini-3-pro-image", "1K", 1, cfg, 1.0)
	require.InDelta(t, 0.10, cost.TotalCost, 1e-9)
	cost = svc.CalculateImageCost("gemini-3-pro-image", "2K", 1, cfg, 1.0)
	require.InDelta(t, 0.15, cost.TotalCost, 1e-9)

	// 分组一张价都没配时仍然走默认价，不受降档影响
	cost = svc.CalculateImageCost("gemini-3-pro-image", "4K", 1, &ImagePriceConfig{}, 1.0)
	require.InDelta(t, 0.268, cost.TotalCost, 0.0001)
}
