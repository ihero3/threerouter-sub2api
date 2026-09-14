package service

// media_adapter.go — 统一媒体生成抽象（图片 / 视频 / 音频）。
//
// 设计目标：用户只需一个生成入口 + 一个模型名，平台自动识别媒体类型，
// 从该类型对应的多个上游账号中选一个调用，创建失败自动切换到下一个。
// 该文件只定义契约，不包含具体厂商实现。视频厂商 adapter 通过适配器
// 接入，图片 / 音频厂商 adapter 直接实现本接口。
//
// 现有 VideoAdapter 链路保持不动，避免破坏已上线的视频功能；
// 本抽象先作为新增「总入口 + 统一调度」的承载层，后续逐步迁移。

import (
	"context"
	"strings"
)

// MediaKind 标识媒体任务的类型。
type MediaKind string

const (
	MediaKindImage MediaKind = "image"
	MediaKindVideo MediaKind = "video"
	MediaKindAudio MediaKind = "audio"
)

// MediaCompletionMode 描述上游创建任务后的完成方式。
type MediaCompletionMode string

const (
	// MediaCompletionSync 表示创建响应已含最终产物 URL，无需后台轮询。
	MediaCompletionSync MediaCompletionMode = "sync"
	// MediaCompletionAsync 表示上游已受理、后台完成，本地任务保持 processing。
	MediaCompletionAsync MediaCompletionMode = "async"
	// MediaCompletionFailed 表示上游拒绝，任务直接标记失败。
	MediaCompletionFailed MediaCompletionMode = "failed"
)

// MediaCreateRequest 是客户端媒体生成请求的统一内部表示。
// 字段与 VideoCreateRequest 对齐以便视频 adapter 复用；图片/音频 adapter
// 只消费各自关心的字段，其余经 Extra 透传。
type MediaCreateRequest struct {
	PublicModel    string            // 用户暴露的统一模型名
	UpstreamModel  string            // 上游实际模型名
	Prompt         string            // 文本提示词
	NegativePrompt string            // 负面提示词
	ImageRefURLs   []string          // 参考图 URL（图生图 / 图生视频）
	VideoRefURLs   []string          // 参考视频 URL
	AudioRefURLs   []string          // 参考音频 URL
	Media          []VideoMediaInput // 官方多模态素材列表（优先于扁平字段）
	Resolution     string            // 分辨率档位
	Ratio          string            // 宽高比
	DurationSec    int               // 时长（秒）
	ImageCount     int               // 单次生成的图片张数（图片按张计费，视频忽略）
	Seed           *int64            // 随机种子
	Extra          map[string]any    // 其他上游专有参数
}

// MediaCreateResult 是创建任务后上游的即时响应。
type MediaCreateResult struct {
	TaskID             string              // 上游异步任务 ID
	InlineURL          string              // 同步返回的产物 URL
	InlineURLs         []string            // 同步返回的多张产物 URL（n>1 时按张计费与返回）
	// UpstreamSize 是上游回传的真实输出尺寸（形如 "1024x1024"）。
	// 不传 size 时模型会自行推荐分辨率，按请求值计费会错档，故优先用真实尺寸。
	UpstreamSize string
	Status             string              // processing / succeeded / failed
	Mode               MediaCompletionMode // 规范化完成模式
	UpstreamStatusCode int                 // 上游 HTTP 状态码，用于 failover 判定
	UpstreamRaw        []byte              // 上游原始响应
	InlineBytes        []byte              // 同步返回的原始字节（如音频二进制），与 InlineURL 二选一
	ErrorMessage       string
}

// MediaTaskResult 是查询任务状态/结果的响应。
type MediaTaskResult struct {
	Status       string // processing / succeeded / failed / cancelled
	URL          string // 产物 URL（视频 / 图片 / 音频均可）
	ThumbnailURL string
	DurationSec  int
	ErrorMessage string
	StatusCode   int    // 上游 HTTP 状态码
	UpstreamRaw  []byte // 上游原始查询响应
}

// MediaAdapter 是媒体生成上游适配器接口。
// 不同厂商契约差异巨大，adapter 负责把统一请求转成上游 wire 格式，
// 并归一化上游状态。adapter 不负责选号 / 计费 / 持久化。
type MediaAdapter interface {
	// Kind 返回该 adapter 服务的媒体类型。
	Kind() MediaKind
	// Create 向上游提交媒体生成任务。
	Create(ctx context.Context, account *Account, req MediaCreateRequest) (*MediaCreateResult, error)
	// GetResult 查询上游任务状态和结果。
	GetResult(ctx context.Context, account *Account, upstreamTaskID string) (*MediaTaskResult, error)
}

// MediaAdapterMatcher 让 adapter 声明服务哪些平台 / 模型前缀。
// 与 MediaAdapter.Kind 一起构成精确路由：先按类型，再按平台/模型。
type MediaAdapterMatcher interface {
	Supports(platform, model string) bool
}

// MediaAdapterRegistry 按媒体类型 + 平台 + 模型解析 adapter。
//
// 解析顺序：显式登记的 adapter（实现了 MediaAdapterMatcher）按注册顺序匹配，
// 若都未命中则回退到 fallback（可为 nil）。这样新增厂商只需新增 adapter 并
// 在 Provider 里登记，不改调度主流程。
type MediaAdapterRegistry struct {
	adapters []registryMediaAdapter
	fallback MediaAdapter
}

type registryMediaAdapter struct {
	adapter MediaAdapter
	matcher MediaAdapterMatcher
}

// NewMediaAdapterRegistry 组装 adapter 链。最后一个参数作为 fallback（可为 nil）。
func NewMediaAdapterRegistry(adapters ...MediaAdapter) *MediaAdapterRegistry {
	reg := &MediaAdapterRegistry{}
	if len(adapters) > 0 {
		reg.fallback = adapters[len(adapters)-1]
		adapters = adapters[:len(adapters)-1]
	}
	for _, a := range adapters {
		if a == nil {
			continue
		}
		entry := registryMediaAdapter{adapter: a}
		if matcher, ok := a.(MediaAdapterMatcher); ok {
			entry.matcher = matcher
		}
		reg.adapters = append(reg.adapters, entry)
	}
	return reg
}

// Resolve 返回指定媒体类型 + 平台 + 模型对应的 adapter。
func (r *MediaAdapterRegistry) Resolve(kind MediaKind, platform, model string) (MediaAdapter, error) {
	if r == nil {
		return nil, ErrVideoAdapterRegistryNil
	}
	for _, entry := range r.adapters {
		if entry.adapter == nil || entry.adapter.Kind() != kind {
			continue
		}
		if entry.matcher != nil && entry.matcher.Supports(platform, model) {
			return entry.adapter, nil
		}
	}
	if r.fallback != nil && r.fallback.Kind() == kind {
		return r.fallback, nil
	}
	return nil, ErrVideoAdapterNotFound
}

// HasExplicitAdapter 报告是否为非 fallback 的显式匹配。
func (r *MediaAdapterRegistry) HasExplicitAdapter(kind MediaKind, platform, model string) bool {
	if r == nil {
		return false
	}
	for _, entry := range r.adapters {
		if entry.adapter == nil || entry.adapter.Kind() != kind {
			continue
		}
		if entry.matcher != nil && entry.matcher.Supports(platform, model) {
			return true
		}
	}
	return false
}

// MediaKindFromModel 根据模型名 + 请求体推断媒体类型。
// 规则：
//   - 明确的 video 模型（seedance/minimax-h3/minimax-video/wan*/doubao-seedance/grok-imagine-video）→ video
//   - 明确的 image 模型（grok-imagine-image*/gpt-image/dall-e/wan-image/seedance-image 等）→ image
//   - 明确的 audio 模型（tts/stt/whisper/mimax-tts/volc-tts 等）→ audio
//   - 未识别时回退到请求体 hint（type/media_kind）或默认 video。
func MediaKindFromModel(model string, body map[string]any) MediaKind {
	kindFromBody := func() MediaKind {
		if body == nil {
			return ""
		}
		for _, key := range []string{"media_kind", "kind", "type"} {
			if v, ok := body[key].(string); ok && strings.TrimSpace(v) != "" {
				switch strings.ToLower(strings.TrimSpace(v)) {
				case "image", "img":
					return MediaKindImage
				case "audio", "voice", "tts", "speech":
					return MediaKindAudio
				case "video":
					return MediaKindVideo
				}
			}
		}
		return ""
	}

	if b := kindFromBody(); b != "" {
		return b
	}

	m := strings.ToLower(strings.TrimSpace(model))
	if m == "" {
		return MediaKindVideo
	}
	// 图片：优先识别（seedream / t2i / qwen-image / image-01 等，避免被 video 前辍误判）
	if strings.Contains(m, "seedream") ||
		strings.Contains(m, "doubao-seedream") ||
		strings.Contains(m, "qwen-image") ||
		strings.Contains(m, "gpt-image") ||
		strings.Contains(m, "dall-e") ||
		strings.Contains(m, "image-01") ||
		strings.Contains(m, "minimax-image") ||
		strings.Contains(m, "t2i") ||
		strings.Contains(m, "-image") {
		return MediaKindImage
	}
	// 视频
	if strings.Contains(m, "seedance") ||
		strings.Contains(m, "doubao-seedance") ||
		strings.Contains(m, "minimax-h3") ||
		strings.Contains(m, "minimax-video") ||
		strings.Contains(m, "hailuo") ||
		strings.Contains(m, "jimeng-video") ||
		(strings.HasPrefix(m, "wan") && (strings.Contains(m, "video") || strings.Contains(m, "t2v"))) ||
		strings.Contains(m, "grok-imagine-video") {
		return MediaKindVideo
	}
	// 音频
	if strings.Contains(m, "tts") ||
		strings.Contains(m, "stt") ||
		strings.Contains(m, "whisper") ||
		strings.Contains(m, "speech") ||
		strings.Contains(m, "realtime") ||
		strings.Contains(m, "voice") ||
		strings.Contains(m, "cosyvoice") ||
		strings.Contains(m, "t2a") ||
		strings.Contains(m, "speech-01") {
		return MediaKindAudio
	}
	return MediaKindVideo
}

// IsKnownImageVendorModel reports whether the model should be routed to the
// unified media image pipeline rather than the legacy Grok/OpenAI image
// forwarders.
func IsKnownImageVendorModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "" {
		return false
	}
	return strings.Contains(m, "seedream") ||
		strings.Contains(m, "doubao-seedream") ||
		isGrokImagineImageModel(m) ||
		strings.Contains(m, "gpt-image") ||
		strings.Contains(m, "dall-e") ||
		strings.Contains(m, "qwen-image") ||
		strings.Contains(m, "image-01") ||
		strings.Contains(m, "t2i") ||
		strings.Contains(m, "-image")
}

// isGrokImagineImageModel 判定 grok-imagine 系列里的**图片**型号。
//
// 该前缀下 image / edit 出图、video 出视频。早先用 Contains("grok-imagine")
// 一把抓，会把 grok-imagine-video 误判成图片并送进图片链路——统一入口按
// model 自动分派后，这个误判会直接把视频请求路由错，故必须区分。
func isGrokImagineImageModel(m string) bool {
	if !strings.Contains(m, "grok-imagine") {
		return false
	}
	return !strings.Contains(m, "video")
}

// IsKnownAudioVendorModel reports whether the model should be routed to the
// unified media audio pipeline (TTS / STT / voice).
func IsKnownAudioVendorModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "" {
		return false
	}
	return strings.Contains(m, "tts") ||
		strings.Contains(m, "stt") ||
		strings.Contains(m, "whisper") ||
		strings.Contains(m, "speech") ||
		strings.Contains(m, "realtime") ||
		strings.Contains(m, "voice") ||
		strings.Contains(m, "cosyvoice") ||
		strings.Contains(m, "t2a") ||
		strings.Contains(m, "speech-01")
}
