package service

import "testing"

// TestDispatchModelCapability 统一入口的分派规则：客户端只给 model，
// 服务端据此决定走文本还是媒体链路，所以判定错了会直接导致调用失败。
func TestDispatchModelCapability(t *testing.T) {
	cases := map[string]ModelCapability{
		// 文本：未识别的一律按文本处理，让上游给出权威错误而不是本地拦截
		"gpt-4o":             ModelCapabilityText,
		"claude-3-5-sonnet":  ModelCapabilityText,
		"deepseek-chat":      ModelCapabilityText,
		"":                   ModelCapabilityText,
		"some-unknown-model": ModelCapabilityText,

		// 图片
		"qwen-image-3.0-pro": ModelCapabilityImage,
		"qwen-image-3.0":     ModelCapabilityImage,
		"image-01":           ModelCapabilityImage,
		"doubao-seedream-3":  ModelCapabilityImage,
		"gpt-image-2":        ModelCapabilityImage,
		"dall-e-3":           ModelCapabilityImage,
		"grok-imagine-image": ModelCapabilityImage,
		// 客户端反馈的 bug：minimax-image-01 曾被当成未注册模型，
		// 既进不了图片链路，也拿不到渠道。
		"minimax-image-01": ModelCapabilityImage,

		// 视频
		"seedance-1.0-pro":   ModelCapabilityVideo,
		"minimax-hailuo-02":  ModelCapabilityVideo,
		"wan2.2-t2v":         ModelCapabilityVideo,
		"grok-imagine-video": ModelCapabilityVideo,
		// wan3.0 全能系列与短型号（wan*-video / wan*-i2v / minimax-h*）
		"wan3.0-video": ModelCapabilityVideo,
		"wan2.2-i2v":   ModelCapabilityVideo,
		"wan-i2v":      ModelCapabilityVideo,
		"minimax-h3":   ModelCapabilityVideo,
		"minimax-h1":   ModelCapabilityVideo,

		// 音频
		"tts-1":     ModelCapabilityAudio,
		"speech-01": ModelCapabilityAudio,
	}
	for model, want := range cases {
		if got := DispatchModelCapability(model); got != want {
			t.Errorf("DispatchModelCapability(%q) = %q, want %q", model, got, want)
		}
	}
}

// TestDispatchModelCapabilityImageBeforeVideo 图片必须优先于视频判定。
// 部分模型名同时命中两类特征（如带 image 又带 video 前缀），
// 顺序反了会把图片请求送进视频链路。
func TestDispatchModelCapabilityImageBeforeVideo(t *testing.T) {
	// MediaKindFromModel 里明确注释过图片要优先，这里保持同序。
	if got := DispatchModelCapability("qwen-image-3.0-pro"); got != ModelCapabilityImage {
		t.Fatalf("qwen-image 应判为图片，实际 %q", got)
	}
}
