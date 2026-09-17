package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// media_usage_endpoint_test.go — 媒体链路 usage_logs 明细字段的回归保护。
//
// 此前媒体链路（生图/生视频/音频）只写金额与模型，入站端点、上游端点、耗时、
// UA、IP 全为空，与原生文本链路的明细对不上。这里锁定补齐后的口径。

func testMediaGinContext(t *testing.T, path string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, path, nil)
	c.Request.Header.Set("User-Agent", "threerouter-test/1.0")
	return c
}

// TestNewMediaUsageMetaCollectsInboundAndUpstream 创建阶段必须同时采到入站与上游端点。
func TestNewMediaUsageMetaCollectsInboundAndUpstream(t *testing.T) {
	c := testMediaGinContext(t, "/v1/images/generations")
	meta := newMediaUsageMeta(c, "/api/v1/services/aigc/multimodal-generation/generation", "1024x1024", "1664x928")

	require.Equal(t, "/v1/images/generations", meta.InboundEndpoint, "入站端点应来自网关入口路径")
	require.Equal(t, "/api/v1/services/aigc/multimodal-generation/generation", meta.UpstreamEndpoint)
	require.Equal(t, "1024x1024", meta.RequestedSize)
	require.Equal(t, "1664x928", meta.OutputSize)
	require.Equal(t, "threerouter-test/1.0", meta.UserAgent)
	require.NotEmpty(t, meta.IPAddress, "IP 应能取到（测试环境至少是回环地址）")
	require.False(t, meta.StartedAt.IsZero(), "起算时间必须有值，否则 duration_ms 恒为 0")
}

// TestUpstreamEndpointPathStripsHostAndQuery 明细里只保留上游 path，不落域名与 query。
func TestUpstreamEndpointPathStripsHostAndQuery(t *testing.T) {
	cases := map[string]string{
		"https://dashscope.aliyuncs.com/api/v1/services/aigc/x?a=1": "/api/v1/services/aigc/x",
		"https://api.minimaxi.com/v1/image_generation":              "/v1/image_generation",
		"/v1/videos/generations":                                    "/v1/videos/generations",
		"v1/images/generations":                                     "/v1/images/generations",
		"":                                                          "",
	}
	for raw, want := range cases {
		require.Equal(t, want, upstreamEndpointPath(raw), "raw=%q", raw)
	}
}

// TestApplyMediaUsageMetaWritesDetailColumns 明细字段必须真正写进 usage_logs 行。
func TestApplyMediaUsageMetaWritesDetailColumns(t *testing.T) {
	meta := &mediaUsageMeta{
		InboundEndpoint:  "/v1/images/generations",
		UpstreamEndpoint: "/v1/image_generation",
		UserAgent:        "threerouter-test/1.0",
		IPAddress:        "127.0.0.1",
		StartedAt:        time.Now().Add(-0),
	}
	log := &UsageLog{}
	applyMediaUsageMeta(log, meta)

	require.NotNil(t, log.InboundEndpoint)
	require.Equal(t, "/v1/images/generations", *log.InboundEndpoint)
	require.NotNil(t, log.UpstreamEndpoint)
	require.Equal(t, "/v1/image_generation", *log.UpstreamEndpoint)
	require.NotNil(t, log.UserAgent)
	require.Equal(t, "threerouter-test/1.0", *log.UserAgent)
	require.NotNil(t, log.IPAddress)
	require.Equal(t, "127.0.0.1", *log.IPAddress)
}

// TestMediaImageUsageLogCarriesEndpointDetail 图片行必须带上端点明细（此前全空）。
func TestMediaImageUsageLogCarriesEndpointDetail(t *testing.T) {
	localID := "img-endpoint-1"
	in := &mediaImageBillingInput{
		LocalID:       localID,
		Model:         "qwen-image-3.0",
		RequestedSize: "1024x1024",
		OutputSize:    "1664x928",
		ImageCount:    1,
		Meta: &mediaUsageMeta{
			InboundEndpoint:  "/v1/images/generations",
			UpstreamEndpoint: "/api/v1/services/aigc/multimodal-generation/generation",
			UserAgent:        "threerouter-test/1.0",
			IPAddress:        "127.0.0.1",
		},
	}
	log := buildMediaImageUsageLog(in, nil, nil, nil, 1, BillingTypeBalance, time.Now())

	require.NotNil(t, log.InboundEndpoint, "入站端点必须写入，否则明细是半截的")
	require.Equal(t, "/v1/images/generations", *log.InboundEndpoint)
	require.NotNil(t, log.UpstreamEndpoint)
	require.Equal(t, "/api/v1/services/aigc/multimodal-generation/generation", *log.UpstreamEndpoint)
	require.NotNil(t, log.UserAgent)
	require.NotNil(t, log.IPAddress)
}

// TestMediaImageAsyncSettlementReusesCreationMeta 异步出图在轮询时才结算，
// 此时拿不到 adapter 响应：RequestSize/OutputSize 与端点必须能从创建阶段缓存回源，
// 否则同步与异步两条路径的 image_size_source 会分叉（output vs input）。
func TestMediaImageAsyncSettlementReusesCreationMeta(t *testing.T) {
	localID := "img-async-1"
	storeMediaEndpointMeta(localID, &mediaUsageMeta{
		InboundEndpoint:  "/v1/images/generations",
		UpstreamEndpoint: "/api/v1/services/aigc/multimodal-generation/generation",
		UserAgent:        "threerouter-test/1.0",
		IPAddress:        "127.0.0.1",
		RequestedSize:    "1024x1024",
		OutputSize:       "1664x928",
	})

	// 模拟异步轮询结算：只带任务记录里的字段，Meta 为空
	in := &mediaImageBillingInput{
		LocalID:    localID,
		Model:      "qwen-image-3.0",
		Resolution: "1664x928",
		ImageCount: 1,
	}
	log := buildMediaImageUsageLog(in, nil, nil, nil, 1, BillingTypeBalance, time.Now())

	require.NotNil(t, log.ImageSize)
	require.Equal(t, ImageBillingSize2K, *log.ImageSize, "异步档位必须与同步一致")
	require.NotNil(t, log.ImageSizeSource)
	require.Equal(t, ImageSizeSourceOutput, *log.ImageSizeSource, "回源后应仍按真实输出分档")
	require.NotNil(t, log.ImageOutputSize)
	require.Equal(t, "1664x928", *log.ImageOutputSize)
	require.NotNil(t, log.InboundEndpoint, "异步结算也要带端点明细")
	require.Equal(t, "/v1/images/generations", *log.InboundEndpoint)
	require.NotNil(t, log.UpstreamEndpoint)

	// 缓存一次性消费：二次结算不应再读到同一份元数据
	require.Nil(t, takeMediaEndpointMeta(localID), "元数据应只消费一次")
}

// TestMediaEndpointMetaCacheSweepsOrphans 缓存不能因失败任务无限增长。
func TestMediaEndpointMetaCacheSweepsOrphans(t *testing.T) {
	localID := "img-orphan-1"
	storeMediaEndpointMeta(localID, &mediaUsageMeta{InboundEndpoint: "/v1/images/generations"})
	require.NotNil(t, takeMediaEndpointMeta(localID))

	// 空 localID 不应写入（否则会污染缓存）
	storeMediaEndpointMeta("", &mediaUsageMeta{InboundEndpoint: "/x"})
	require.Nil(t, takeMediaEndpointMeta(""))
}
