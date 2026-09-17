package service

import "context"

// media_adapter_bridge.go — 把现有 VideoAdapter 桥接成 MediaAdapter。
// 现有视频厂商 adapter（Seedance/MiniMax/Wan/OpenAI）全部实现 VideoAdapter，
// 这里包一层，让它们也能注册进 MediaAdapterRegistry，供统一 /v1/media 入口使用。
// 桥接只做类型映射，不改变任何上游行为。

// VideoAsMediaAdapter 将 VideoAdapter 桥接为 MediaAdapter。
type VideoAsMediaAdapter struct {
	video VideoAdapter
	kind  MediaKind
}

// NewVideoAsMediaAdapter 创建桥接适配器。默认视频 kind。
func NewVideoAsMediaAdapter(video VideoAdapter) *VideoAsMediaAdapter {
	return &VideoAsMediaAdapter{video: video, kind: MediaKindVideo}
}

// Kind 实现 MediaAdapter.Kind。
func (a *VideoAsMediaAdapter) Kind() MediaKind { return a.kind }

// Create 桥接 Create，做类型转换。
func (a *VideoAsMediaAdapter) Create(ctx context.Context, account *Account, req MediaCreateRequest) (*MediaCreateResult, error) {
	vreq := VideoCreateRequest{
		PublicModel:    req.PublicModel,
		UpstreamModel:  req.UpstreamModel,
		Prompt:         req.Prompt,
		NegativePrompt: req.NegativePrompt,
		ImageRefURLs:   req.ImageRefURLs,
		VideoRefURLs:   req.VideoRefURLs,
		AudioRefURLs:   req.AudioRefURLs,
		Media:          req.Media,
		Resolution:     req.Resolution,
		Ratio:          req.Ratio,
		DurationSec:    req.DurationSec,
		Seed:           req.Seed,
		Extra:          req.Extra,
	}
	vres, err := a.video.Create(ctx, account, vreq)
	if err != nil {
		return nil, err
	}
	return &MediaCreateResult{
		TaskID:             vres.TaskID,
		InlineURL:          vres.InlineVideoURL,
		Status:             vres.Status,
		Mode:               videoToMediaCompletionMode(vres.Mode),
		UpstreamStatusCode: vres.UpstreamStatusCode,
		UpstreamRaw:        vres.UpstreamRaw,
		ErrorMessage:       vres.ErrorMessage,
		UpstreamEndpoint:   vres.UpstreamEndpoint,
	}, nil
}

// GetResult 桥接 GetResult，做类型转换。
func (a *VideoAsMediaAdapter) GetResult(ctx context.Context, account *Account, upstreamTaskID string) (*MediaTaskResult, error) {
	vres, err := a.video.GetResult(ctx, account, upstreamTaskID)
	if err != nil {
		return nil, err
	}
	return &MediaTaskResult{
		Status:       vres.Status,
		URL:          vres.VideoURL,
		ThumbnailURL: vres.ThumbnailURL,
		DurationSec:  vres.DurationSec,
		ErrorMessage: vres.ErrorMessage,
		StatusCode:   vres.StatusCode,
		UpstreamRaw:  vres.UpstreamRaw,
	}, nil
}

// Cancel 透传到视频厂商 adapter（若其支持 Cancel）。供 MediaTaskService 尽力取消上游任务。
func (a *VideoAsMediaAdapter) Cancel(ctx context.Context, account *Account, upstreamTaskID string) error {
	if a.video == nil {
		return nil
	}
	if canceller, ok := a.video.(interface {
		Cancel(ctx context.Context, account *Account, upstreamTaskID string) error
	}); ok {
		return canceller.Cancel(ctx, account, upstreamTaskID)
	}
	return nil
}

// Supports 实现 MediaAdapterMatcher，转发到底层视频 adapter 的 Supports（若有）。
func (a *VideoAsMediaAdapter) Supports(platform, model string) bool {
	if matcher, ok := a.video.(VideoAdapterMatcher); ok {
		return matcher.Supports(platform, model)
	}
	return false
}

func videoToMediaCompletionMode(mode VideoCompletionMode) MediaCompletionMode {
	switch mode {
	case VideoCompletionSync:
		return MediaCompletionSync
	case VideoCompletionFailed:
		return MediaCompletionFailed
	default:
		return MediaCompletionAsync
	}
}
