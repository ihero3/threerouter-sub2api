package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestCanonicalGrokImagineVideoPriceFamily(t *testing.T) {
	t.Parallel()
	require.Equal(t, VideoPriceFamilyGrokImagineVideo, CanonicalGrokImagineVideoPriceFamily("grok-imagine-video"))
	require.Equal(t, VideoPriceFamilyGrokImagineVideo15, CanonicalGrokImagineVideoPriceFamily("grok-imagine-video-1.5"))
	require.Equal(t, VideoPriceFamilyGrokImagineVideo15, CanonicalGrokImagineVideoPriceFamily("grok-imagine-video-1.5-preview"))
	require.Equal(t, VideoPriceFamilyGrokImagineVideo15, CanonicalGrokImagineVideoPriceFamily("xai/grok-video-1.5"))
	require.Equal(t, "grok-imagine-video-2", CanonicalGrokImagineVideoPriceFamily("grok-imagine-video-2"))
	require.Equal(t, "grok-imagine-video-2", CanonicalGrokImagineVideoPriceFamily("xai/grok-imagine-video-2"))
}

func TestNormalizeAndLookupVideoModelPrices(t *testing.T) {
	t.Parallel()
	raw := map[string]map[string]float64{
		"grok-imagine-video-1.5-preview": {"480p": 0.08, "720p": 0.14},
		"grok-imagine-video":             {"480p": 0.05},
		"grok-imagine-video-2":           {"1080p": 0.4},
	}
	norm := NormalizeVideoModelPrices(raw)
	require.NotNil(t, norm)
	require.Contains(t, norm, VideoPriceFamilyGrokImagineVideo15)
	require.Contains(t, norm, VideoPriceFamilyGrokImagineVideo)
	require.Contains(t, norm, "grok-imagine-video-2")

	p15 := LookupVideoModelPrice(norm, "grok-imagine-video-1.5", "480p")
	require.NotNil(t, p15)
	require.InDelta(t, 0.08, *p15, 1e-9)

	pBase := LookupVideoModelPrice(norm, "grok-imagine-video", "480p")
	require.NotNil(t, pBase)
	require.InDelta(t, 0.05, *pBase, 1e-9)
	// A missing model-specific tier must fall back to the flat tier price,
	// rather than borrowing another model-specific resolution.
	require.Nil(t, LookupVideoModelPrice(norm, "grok-imagine-video", "720p"))

	p2 := LookupVideoModelPrice(norm, "grok-imagine-video-2", "1080p")
	require.NotNil(t, p2)
	require.InDelta(t, 0.4, *p2, 1e-9)

	// Unmatched model → nil (caller falls back to flat columns / defaults).
	require.Nil(t, LookupVideoModelPrice(norm, "unknown-model", "480p"))
}

func TestNormalizeVideoModelPricesDropsUnknownResolutions(t *testing.T) {
	t.Parallel()
	// "8k" and "1080i" are not billable tiers. Collapsing them into 480p would
	// charge a 480p request at the operator's high-resolution price.
	norm := NormalizeVideoModelPrices(map[string]map[string]float64{
		"grok-imagine-video": {"480p": 0.05, "8k": 0.50, "1080i": 0.30},
	})
	require.NotNil(t, norm)
	require.Equal(t, map[string]float64{VideoBillingResolution480P: 0.05}, norm[VideoPriceFamilyGrokImagineVideo])

	// A model whose tiers are all unrecognized contributes no family at all.
	require.Nil(t, NormalizeVideoModelPrices(map[string]map[string]float64{
		"grok-imagine-video": {"8k": 0.50},
	}))
}

func TestNormalizeVideoModelPricesKeepsVendorSpecificTiers(t *testing.T) {
	t.Parallel()
	// 768p / 2k / 4k 是厂商专有档位（MiniMax H3 等），必须原样保留而不是被丢弃。
	// 注意：必须以价格族名（minimax-video）而非厂商别名（minimax-hailuo）作 key，
	// 否则保存期归一化与查询期归一化的 key 对不上（见审计报告 P0-3）。
	norm := NormalizeVideoModelPrices(map[string]map[string]float64{
		VideoPriceFamilyMiniMaxVideo: {"768p": 0.20, "2k": 0.45},
	})
	require.NotNil(t, norm)
	require.Equal(t, map[string]float64{
		VideoBillingResolution768P: 0.20,
		VideoBillingResolution2K:   0.45,
	}, norm[VideoPriceFamilyMiniMaxVideo])
}

func TestParseVideoBillingResolutionFromDimensions(t *testing.T) {
	t.Parallel()
	// 用户直接传长宽尺寸时按**短边**映射，横屏竖屏都落在同一档。
	cases := map[string]string{
		"1920x1080":   VideoBillingResolution1080P,
		"1920*1080":   VideoBillingResolution1080P,
		"1920×1080":   VideoBillingResolution1080P,
		"1920:1080":   VideoBillingResolution1080P,
		"1920 X 1080": VideoBillingResolution1080P,
		"1080x1920":   VideoBillingResolution1080P, // 竖屏：短边 1080
		"1280x720":    VideoBillingResolution720P,
		"720x1280":    VideoBillingResolution720P,
		"854x480":     VideoBillingResolution480P,
		"2560x1440":   VideoBillingResolution2K,
		"3840x2160":   VideoBillingResolution4K,
		"1080":        VideoBillingResolution1080P, // 纯数字按纵向像素
	}
	for input, want := range cases {
		got, ok := ParseVideoBillingResolution(input)
		require.True(t, ok, "expected %q to parse", input)
		require.Equal(t, want, got, "resolution %q", input)
	}

	for _, input := range []string{"", "abc", "1080i", "8k", "not-a-size"} {
		_, ok := ParseVideoBillingResolution(input)
		require.False(t, ok, "expected %q to be rejected", input)
	}
}

func TestLookupVideoBillingResolutionAnyAcceptsNamesAndDimensions(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"720p":      VideoBillingResolution720P,
		"768p":      VideoBillingResolution768P,
		"2K":        VideoBillingResolution2K,
		"4k":        VideoBillingResolution4K,
		"HD":        VideoBillingResolution720P,
		"full_hd":   VideoBillingResolution1080P,
		"1920x1080": VideoBillingResolution1080P,
		"1080x1920": VideoBillingResolution1080P,
	}
	for input, want := range cases {
		got, ok := LookupVideoBillingResolutionAny(input)
		require.True(t, ok, "expected %q to resolve", input)
		require.Equal(t, want, got, "resolution %q", input)
	}
}

func TestVideoBillingResolutionFallbacksDescends(t *testing.T) {
	t.Parallel()
	require.Equal(t, []string{
		VideoBillingResolution4K,
		VideoBillingResolution2K,
		VideoBillingResolution1080P,
		VideoBillingResolution768P,
		VideoBillingResolution720P,
		VideoBillingResolution480P,
	}, VideoBillingResolutionFallbacks("4k"))

	require.Equal(t, []string{
		VideoBillingResolution1080P,
		VideoBillingResolution768P,
		VideoBillingResolution720P,
		VideoBillingResolution480P,
	}, VideoBillingResolutionFallbacks("1920x1080"))

	// 最低档没有更低可降
	require.Equal(t, []string{VideoBillingResolution480P}, VideoBillingResolutionFallbacks("480p"))
	// 空输入退化为只查最低档
	require.Equal(t, []string{VideoBillingResolution480P}, VideoBillingResolutionFallbacks(""))
}

func TestNormalizeVideoBillingDurationSecondsPrefersActual(t *testing.T) {
	t.Parallel()
	// 上游回传真实时长优先：防止"传 1 秒实际生成更长视频"的套利。
	require.Equal(t, 12, NormalizeVideoBillingDurationSeconds(12, 5))
	// 上游不回传时用用户请求时长，避免一律按默认 8 秒多收。
	require.Equal(t, 5, NormalizeVideoBillingDurationSeconds(0, 5))
	// 两者都缺失才用默认 8 秒。
	require.Equal(t, VideoBillingDefaultDurationSeconds, NormalizeVideoBillingDurationSeconds(0, 0))
}

func TestNormalizeVideoBillingDurationSecondsHasNoBusinessCap(t *testing.T) {
	t.Parallel()
	// 不再有 15 秒业务上限：支持 30 秒 / 60 秒的模型必须按实际时长计费，否则少收。
	require.Equal(t, 30, NormalizeVideoBillingDurationSeconds(30, 30))
	require.Equal(t, 60, NormalizeVideoBillingDurationSecondsOrDefault(60))
	require.Equal(t, 15, NormalizeVideoBillingDurationSecondsOrDefault(15))
	// 上游脏数据（典型是把毫秒当秒返回）视为无效，回退到下一优先级 / 默认值，
	// 而不是按 999999 秒计费。
	require.Equal(t, VideoBillingDefaultDurationSeconds, NormalizeVideoBillingDurationSeconds(999999, 0))
	require.Equal(t, 10, NormalizeVideoBillingDurationSeconds(999999, 10))
	require.Equal(t, VideoBillingDefaultDurationSeconds, NormalizeVideoBillingDurationSecondsOrDefault(999999))
	// 缺失 / 非正值仍然走默认值。
	require.Equal(t, VideoBillingDefaultDurationSeconds, NormalizeVideoBillingDurationSecondsOrDefault(0))
	require.Equal(t, VideoBillingDefaultDurationSeconds, NormalizeVideoBillingDurationSecondsOrDefault(-3))
}

func TestVideoTaskCostBreakdownFallsBackToGroupPriceWithoutResolver(t *testing.T) {
	t.Parallel()
	// 未装配定价解析器时，必须回退到分组媒体视频价，不能报错也不能返回 0。
	gid := int64(7)
	p480 := 0.05
	apiKey := &APIKey{
		GroupID: &gid,
		Group: &Group{
			ID:             gid,
			RateMultiplier: 1.0,
			VideoPrice480P: &p480,
		},
	}
	deps := &videoTaskBillingDeps{billingService: &BillingService{}}

	// 1920x1080 → 1080p，没配 1080p/768p/720p，逐级降档到 480p：0.05 × 10s
	cost, multiplier := videoTaskCostBreakdown(context.Background(), deps, apiKey, "grok-imagine-video", "1920x1080", 10, 0)
	require.InDelta(t, 1.0, multiplier, 1e-9)
	require.InDelta(t, 0.5, cost.TotalCost, 1e-9)
	require.Equal(t, string(BillingModeVideo), cost.BillingMode)
}

func TestVideoTaskCostBreakdownUsesChannelPricing(t *testing.T) {
	t.Parallel()
	// 渠道/分组定价卡配了 video 模式时，异步视频任务必须按「每秒单价 × 时长」计价。
	gid := int64(9)
	perSecond := 0.5
	apiKey := &APIKey{
		GroupID: &gid,
		Group: &Group{
			ID:             gid,
			RateMultiplier: 1.0,
			ModelPricing: []ChannelModelPricing{{
				Models:          []string{"grok-imagine-video"},
				BillingMode:     BillingModeVideo,
				PerRequestPrice: &perSecond,
			}},
		},
	}
	resolver := NewModelPricingResolver(nil, &BillingService{})
	deps := &videoTaskBillingDeps{
		billingService:       &BillingService{},
		openAIGatewayService: &OpenAIGatewayService{resolver: resolver},
	}

	// 每秒 0.5 × 10 秒
	cost, _ := videoTaskCostBreakdown(context.Background(), deps, apiKey, "grok-imagine-video", "720p", 10, 0)
	require.InDelta(t, 5.0, cost.TotalCost, 1e-9)
	require.Equal(t, string(BillingModeVideo), cost.BillingMode)
}

func TestCalculateVideoCostHonorsVendorTierAndFallsBack(t *testing.T) {
	t.Parallel()
	// 零值 BillingService 即可：下面每条用例都命中分组配置的价格，不依赖默认价目表。
	svc := &BillingService{}

	// 768p 档位此前会被折叠成 480p 而永远取不到，现在必须命中。
	p768 := 0.20
	cfg := &VideoPriceConfig{
		Price480P:   float64Ptr(0.05),
		ModelPrices: map[string]map[string]float64{"grok-imagine-video": {"768p": p768}},
	}
	got := svc.CalculateVideoCost("grok-imagine-video", "768p", 1, 1, cfg, 1.0)
	require.InDelta(t, p768, got.TotalCost, 1e-9)

	// 用户传长宽尺寸：1920x1080 → 1080p
	p1080 := 0.30
	cfg2 := &VideoPriceConfig{
		Price480P:   float64Ptr(0.05),
		ModelPrices: map[string]map[string]float64{"grok-imagine-video": {"1080p": p1080}},
	}
	got2 := svc.CalculateVideoCost("grok-imagine-video", "1920x1080", 1, 1, cfg2, 1.0)
	require.InDelta(t, p1080, got2.TotalCost, 1e-9)

	// 2k 没配价时逐级降档到 1080p，而不是掉到 480p。
	got3 := svc.CalculateVideoCost("grok-imagine-video", "2560x1440", 1, 1, cfg2, 1.0)
	require.InDelta(t, p1080, got3.TotalCost, 1e-9)
}

func TestNormalizeVideoModelPricesIsDeterministicAcrossAliasConflicts(t *testing.T) {
	t.Parallel()
	// Both keys canonicalize onto grok-imagine-video-1.5 and disagree on 480p.
	// Whichever price wins, it must be the same one on every run — a Go map walk
	// would let two processes bill the same request differently.
	raw := map[string]map[string]float64{
		"grok-imagine-video-1.5":         {"480p": 0.08},
		"grok-imagine-video-1.5-preview": {"480p": 0.11},
		"grok-video-1.5":                 {"480p": 0.09},
	}
	first := NormalizeVideoModelPrices(raw)
	require.NotNil(t, first)
	for i := 0; i < 50; i++ {
		require.Equal(t, first, NormalizeVideoModelPrices(raw), "run %d diverged", i)
	}

	// Aliases within one model key normalize to the same tier deterministically too.
	aliased := map[string]map[string]float64{
		"grok-imagine-video": {"720": 0.12, "720p": 0.13, "hd": 0.14},
	}
	firstAliased := NormalizeVideoModelPrices(aliased)
	require.NotNil(t, firstAliased)
	for i := 0; i < 50; i++ {
		require.Equal(t, firstAliased, NormalizeVideoModelPrices(aliased), "run %d diverged", i)
	}
}

func TestLookupVideoBillingResolutionReportsUnknownTiers(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"480", "480p", "SD", "720", "hd", "1080", "full-hd", " fhd "} {
		normalized, ok := LookupVideoBillingResolution(in)
		require.True(t, ok, "input=%q", in)
		require.NotEmpty(t, normalized)
	}
	for _, in := range []string{"", "4k", "1080i", "2160p", "potato"} {
		normalized, ok := LookupVideoBillingResolution(in)
		require.False(t, ok, "input=%q", in)
		require.Empty(t, normalized)
	}
	// Runtime billing still needs a tier for unrecognized upstream values.
	require.Equal(t, VideoBillingResolution480P, NormalizeVideoBillingResolutionOrDefault("4k"))
	require.Equal(t, VideoBillingResolution1080P, NormalizeVideoBillingResolutionOrDefault("full_hd"))
}

func TestVideoModelPriceMissingTierFallsBackToFlatTierPrice(t *testing.T) {
	t.Parallel()
	flat720P := 0.7
	service := &BillingService{}

	result := service.CalculateVideoCost("grok-imagine-video", "720p", 1, 1, &VideoPriceConfig{
		Price720P: &flat720P,
		ModelPrices: map[string]map[string]float64{
			VideoPriceFamilyGrokImagineVideo: {VideoBillingResolution480P: 0.05},
		},
	}, 1)

	require.InDelta(t, flat720P, result.TotalCost, 1e-9)
}

// TestCalculateOpenAIVideoCostKeepsVendorResolutionTier P0-4 回归：
// 同步网关链路（Grok Imagine）此前在入口就把分辨率折叠成三档，导致 video_model_prices
// 里配的 768p / 2k / 4k 永远命中不到。异步链路上一轮已修，这里补同步链路。
func TestCalculateOpenAIVideoCostKeepsVendorResolutionTier(t *testing.T) {
	t.Parallel()
	gid := int64(31)
	group := &Group{
		ID:             gid,
		RateMultiplier: 1.0,
		VideoModelPrices: NormalizeVideoModelPrices(map[string]map[string]float64{
			"grok-imagine-video": {VideoBillingResolution2K: 0.5},
		}),
	}
	svc := &OpenAIGatewayService{billingService: NewBillingService(&config.Config{}, nil)}

	cost := svc.calculateOpenAIVideoCost(
		context.Background(),
		"grok-imagine-video",
		&APIKey{GroupID: &gid, Group: group},
		&OpenAIForwardResult{VideoCount: 1, VideoResolution: "2k", VideoDurationSeconds: 10},
		1.0,
	)

	require.NotNil(t, cost)
	// 2k 档 0.5/秒 × 10 秒；若被折叠成 480p 则查不到 2k 价，会掉到兜底默认价。
	require.InDelta(t, 5.0, cost.TotalCost, 1e-9)
}
