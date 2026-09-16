package service

import (
	"encoding/json"
	"testing"
)

// TestDetectModelPlatformImageVendors 图片厂商模型必须能解析出承载平台。
//
// composite 分组靠 DetectModelPlatform 决定去哪个平台选号；解析不出来会在
// SelectAccountForModel 阶段直接报 composite target platform unknown，
// 表现为「统一入口按 model 调用却 503」。视频厂商早就这么处理了，图片补齐。
func TestDetectModelPlatformImageVendors(t *testing.T) {
	cases := map[string]string{
		"qwen-image-3.0-pro": PlatformDeepseek,
		"qwen-image-3.0":     PlatformDeepseek,
		"minimax-image-01":   PlatformDeepseek,
		"image-01":           PlatformDeepseek,
		"doubao-seedream-3":  PlatformDeepseek,
		"seedream-3.0":       PlatformDeepseek,
	}
	for model, want := range cases {
		got, ok := DetectModelPlatform(model)
		if !ok {
			t.Errorf("DetectModelPlatform(%q) 未解析出平台，want %q", model, want)
			continue
		}
		if got != want {
			t.Errorf("DetectModelPlatform(%q) = %q, want %q", model, got, want)
		}
	}
}

// TestDetectModelPlatformImageVendorsKeepNativeOwners 原生平台模型不能被
// 图片厂商分支抢走，否则 gpt-image / grok 出图会被送到国产承载平台。
func TestDetectModelPlatformImageVendorsKeepNativeOwners(t *testing.T) {
	cases := map[string]string{
		"gpt-image-2":        PlatformOpenAI,
		"dall-e-3":           PlatformOpenAI,
		"grok-imagine-image": PlatformGrok,
	}
	for model, want := range cases {
		got, ok := DetectModelPlatform(model)
		if !ok || got != want {
			t.Errorf("DetectModelPlatform(%q) = (%q,%v), want %q", model, got, ok, want)
		}
	}
}

// TestDetectModelPlatformVideoVendors 视频厂商模型（含 wan 全能系列与
// minimax-h 短型号）在 composite 分组下必须解析出 deepseek 承载平台。
func TestDetectModelPlatformVideoVendors(t *testing.T) {
	cases := []string{
		"wan3.0-video",
		"wan2.2-i2v",
		"wan-i2v",
		"minimax-hailuo-02",
		"minimax-h3",
		"minimax-h1",
		"seedance-1.0-pro",
	}
	for _, model := range cases {
		got, ok := DetectModelPlatform(model)
		if !ok || got != PlatformDeepseek {
			t.Errorf("DetectModelPlatform(%q) = (%q,%v), want %q", model, got, ok, PlatformDeepseek)
		}
	}
}

// TestMiniMaxImageAdapterSupportsMiniMaxImage01 MiniMax 图片 adapter 必须接住
// minimax-image-01 与官方裸名 image-01，否则模型会被判给通用兜底 adapter。
func TestMiniMaxImageAdapterSupportsMiniMaxImage01(t *testing.T) {
	adapter := NewMiniMaxImageAdapter()
	if adapter.Kind() != MediaKindImage {
		t.Fatalf("adapter 类型应为 image，实际 %q", adapter.Kind())
	}
	for _, model := range []string{"minimax-image-01", "image-01", "MiniMax-Image-01"} {
		if !adapter.Supports(PlatformDeepseek, model) {
			t.Errorf("MiniMax 图片 adapter 应支持 %q", model)
		}
	}
	if adapter.Supports(PlatformDeepseek, "qwen-image-3.0-pro") {
		t.Error("MiniMax 图片 adapter 不应抢走 qwen-image 模型")
	}
}

// TestBuildMiniMaxImageCreateBodyNormalizesModel 发给上游的 model 必须是 MiniMax
// 官方枚举值（image-01 / image-01-live）。客户端按本仓统一命名传 minimax-image-01 时，
// 若原样转发会被上游判非法参数，图片永远出不来、用量记录也就永远是空的。
func TestBuildMiniMaxImageCreateBodyNormalizesModel(t *testing.T) {
	cases := map[string]string{
		"minimax-image-01":      "image-01",
		"MiniMax-Image-01":      "image-01",
		"minimax-image":         "image-01",
		"hailuo-image-01":       "image-01",
		"image-01":              "image-01",
		"image-01-live":         "image-01-live",
		"minimax-image-01-live": "image-01-live",
		"image-01-preview":      "image-01-preview", // 渠道私有命名原样透传
	}
	for in, want := range cases {
		body := buildMiniMaxImageCreateBody(MediaCreateRequest{UpstreamModel: in, Prompt: "a cat"})
		var got map[string]any
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("model %q 请求体非法 JSON: %v", in, err)
		}
		if got["model"] != want {
			t.Errorf("model %q → 上游 model = %v, want %q", in, got["model"], want)
		}
	}
}
