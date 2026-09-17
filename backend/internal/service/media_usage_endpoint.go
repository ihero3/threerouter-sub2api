package service

// media_usage_endpoint.go — 媒体链路（图片 / 音频 / 视频）usage_logs 明细字段补齐。
//
// 背景：原生文本链路与 OpenAI 图片链路都会写入站端点、上游端点、耗时、UA、IP，
// 但媒体链路此前只写金额与模型，导致「使用记录」里生图/生视频/音频的行是半截的：
// 金额有、端点与耗时空。这里把缺失字段按同一口径补齐，**不新增数据库列**。
//
// 难点与取舍：
//   - 同步出图（千问 / MiniMax）在 CreateTask 里就结算，能直接拿到 gin 上下文；
//   - 异步视频在轮询（GetTask → refreshTaskStatus）时才结算，而 handler 传进来的是
//     c.Request.Context()，拿不到 gin 上下文，且此时入站端点是"查询端点"而非"创建端点"。
//   因此创建时把端点信息按 localID 暂存到内存，终态结算时取回（取不到就留空降级）。
//   这是纯内存缓存，进程重启即失效，仅影响明细展示，不影响计费。

import (
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ginCtxKeyInboundEndpoint 与 handler.InboundEndpointMiddleware 写入的 key 保持一致。
// 中间件已在网关入口规范化并入站端点（去掉 /openai 等前缀），优先复用它的结果。
const ginCtxKeyInboundEndpoint = "_gateway_inbound_endpoint"

// mediaUsageMeta 创建阶段采集、终态结算时消费的明细元数据。
// 全部字段均为展示用途，不参与计费，也不落 media_tasks 表。
type mediaUsageMeta struct {
	InboundEndpoint  string
	UpstreamEndpoint string
	UserAgent        string
	IPAddress        string
	StartedAt        time.Time
	// RequestedSize / OutputSize 是创建时的请求尺寸与上游真实输出尺寸。
	// 异步出图在轮询时才结算，此时 adapter 的即时响应已不可得，靠这里回源，
	// 保证同步与异步两条路径的计费档位、image_size_source 完全一致。
	RequestedSize string
	OutputSize    string
}

// mediaEndpointMetaTTL 缓存保留时长。超过后视为孤儿（任务失败/未终态）自动丢弃。
const mediaEndpointMetaTTL = 24 * time.Hour

var (
	mediaEndpointMetaCache sync.Map // localID(string) -> *mediaUsageMeta
	mediaEndpointWrites    counter
)

type counter struct {
	mu sync.Mutex
	n  int
}

// storeMediaEndpointMeta 缓存创建阶段的明细元数据，供异步终态结算取回。
func storeMediaEndpointMeta(localID string, meta *mediaUsageMeta) {
	if localID == "" || meta == nil {
		return
	}
	mediaEndpointMetaCache.Store(localID, meta)

	// 轻量清理：每积累 256 次写入扫一遍过期项，避免失败任务留下永久条目。
	mediaEndpointWrites.mu.Lock()
	mediaEndpointWrites.n++
	shouldSweep := mediaEndpointWrites.n >= 256
	if shouldSweep {
		mediaEndpointWrites.n = 0
	}
	mediaEndpointWrites.mu.Unlock()
	if shouldSweep {
		sweepMediaEndpointMeta()
	}
}

// takeMediaEndpointMeta 取出并删除（终态结算只消费一次）。
func takeMediaEndpointMeta(localID string) *mediaUsageMeta {
	if localID == "" {
		return nil
	}
	value, ok := mediaEndpointMetaCache.LoadAndDelete(localID)
	if !ok {
		return nil
	}
	meta, _ := value.(*mediaUsageMeta)
	return meta
}

func sweepMediaEndpointMeta() {
	now := time.Now()
	mediaEndpointMetaCache.Range(func(key, value any) bool {
		meta, ok := value.(*mediaUsageMeta)
		if !ok {
			mediaEndpointMetaCache.Delete(key)
			return true
		}
		if now.Sub(meta.StartedAt) > mediaEndpointMetaTTL {
			mediaEndpointMetaCache.Delete(key)
		}
		return true
	})
}

// newMediaUsageMeta 在创建阶段采集明细元数据：
//   - 入站端点 / UA / IP 来自网关入口的 gin 上下文；
//   - 上游端点来自 adapter 实际打出去的 URL（仅取 path，不落域名）。
//
// c 为 nil（测试或内部调用）时退化为只带上游端点与时间，调用方按降级处理。
func newMediaUsageMeta(c *gin.Context, upstreamEndpoint, requestedSize, outputSize string) *mediaUsageMeta {
	meta := &mediaUsageMeta{
		UpstreamEndpoint: strings.TrimSpace(upstreamEndpoint),
		RequestedSize:    strings.TrimSpace(requestedSize),
		OutputSize:       strings.TrimSpace(outputSize),
		StartedAt:        time.Now(),
	}
	if c == nil {
		return meta
	}
	meta.InboundEndpoint = inboundEndpointFromGinContext(c)
	if c.Request != nil {
		meta.UserAgent = c.Request.UserAgent()
		meta.IPAddress = c.ClientIP()
	}
	return meta
}

// loadMediaUsageMeta 终态结算取回明细元数据。
// inline 非空（同步结算：创建时直接带下来）优先直接用；
// 否则（异步轮询结算）回源到创建阶段按 localID 存的缓存。
func loadMediaUsageMeta(localID string, inline *mediaUsageMeta) *mediaUsageMeta {
	if inline != nil {
		return inline
	}
	return takeMediaEndpointMeta(localID)
}

// applyMediaUsageMeta 把明细元数据写进 usage_logs 行，与原生链路同口径。
// meta 为 nil 时保持字段为空，不影响既有行为。
func applyMediaUsageMeta(log *UsageLog, meta *mediaUsageMeta) {
	if log == nil || meta == nil {
		return
	}
	log.InboundEndpoint = optionalTrimmedStringPtr(meta.InboundEndpoint)
	log.UpstreamEndpoint = optionalTrimmedStringPtr(meta.UpstreamEndpoint)
	log.UserAgent = optionalTrimmedStringPtr(meta.UserAgent)
	log.IPAddress = optionalTrimmedStringPtr(meta.IPAddress)
	if ms := mediaDurationMs(meta.StartedAt); ms > 0 {
		log.DurationMs = &ms
	}
}

// inboundEndpointFromGinContext 优先用中间件规范化后的入站端点，
// 缺失时按请求路径做轻量规范化（去掉网关前缀），保证明细里有值而不是空。
func inboundEndpointFromGinContext(c *gin.Context) string {
	if v, ok := c.Get(ginCtxKeyInboundEndpoint); ok {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	path := ""
	if c.Request != nil && c.Request.URL != nil {
		path = c.Request.URL.Path
	}
	if path == "" {
		path = c.FullPath()
	}
	return normalizeMediaInboundEndpoint(path)
}

// normalizeMediaInboundEndpoint 去掉网关路由前缀，保留 /v1/... 形态。
// 媒体端点只有固定的几个，不需要像文本链路那样处理通配折叠。
func normalizeMediaInboundEndpoint(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	for _, prefix := range []string{"/antigravity", "/openai", "/gemini", "/backend-api"} {
		if strings.HasPrefix(trimmed, prefix+"/") {
			trimmed = strings.TrimPrefix(trimmed, prefix)
			break
		}
	}
	return trimmed
}

// upstreamEndpointPath 从完整上游 URL 中取出端点路径，供明细展示。
// 只保留 path（去掉 scheme / host / query），避免明细里暴露上游域名与密钥相关信息。
func upstreamEndpointPath(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ""
	}
	// 相对路径（无 scheme）直接返回。
	if !strings.Contains(trimmed, "://") {
		if idx := strings.IndexAny(trimmed, "?#"); idx >= 0 {
			trimmed = trimmed[:idx]
		}
		if strings.HasPrefix(trimmed, "/") {
			return trimmed
		}
		return "/" + trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Path == "" {
		return ""
	}
	return parsed.Path
}

// mediaDurationMs 计算从创建到现在的耗时（毫秒），用于 duration_ms 明细。
func mediaDurationMs(startedAt time.Time) int {
	if startedAt.IsZero() {
		return 0
	}
	ms := time.Since(startedAt).Milliseconds()
	if ms < 0 {
		return 0
	}
	if ms > 1<<31-1 {
		return 1<<31 - 1
	}
	return int(ms)
}
