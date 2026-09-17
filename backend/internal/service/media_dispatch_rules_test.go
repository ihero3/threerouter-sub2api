package service

import "testing"

// media_dispatch_rules_test.go — 模型名 → 模态的分派规则回归测试。
//
// 这些规则是"用户只配 base_url + api_key + model"这一承诺的地基：判定错了，
// 请求会被送进错误链路，客户端拿到的是莫名其妙的 400/503 而不是想要的图。
// 规则是子串/前缀匹配，改动很容易误伤，所以把每个系列的归属固化下来。
//
// 两个判定函数用途不同，但对**已收录**的模型必须给出一致的模态：
//   - DispatchModelCapability：统一入口分派，未收录模型默认 text
//   - MediaKindFromModel：媒体端点分派，未收录模型默认 video（交给账号池决定）

func TestDispatchModelCapability_VendorFamilies(t *testing.T) {
	cases := []struct {
		model string
		want  ModelCapability
	}{
		// 视频：阿里 wan2/wan3 系列（t2v / i2v / *-video）
		{"wan3.0-video", ModelCapabilityVideo},
		{"wan2.2-t2v-plus", ModelCapabilityVideo},
		{"wan2.1-i2v", ModelCapabilityVideo},
		{"seedance-1.0-pro", ModelCapabilityVideo},
		{"minimax-hailuo-02", ModelCapabilityVideo},
		{"minimax-h3", ModelCapabilityVideo},
		{"minimax-video-01", ModelCapabilityVideo},
		{"grok-imagine-video", ModelCapabilityVideo},
		{"jimeng-video-3.0", ModelCapabilityVideo},
		// 图片：含 qwen-image / minimax-image / wanx / seedream / t2i 等
		{"qwen-image-3.0", ModelCapabilityImage},
		{"qwen-image-3.0-pro", ModelCapabilityImage},
		{"qwen-image-4.0", ModelCapabilityImage},
		{"minimax-image-01", ModelCapabilityImage},
		{"minimax-image-02", ModelCapabilityImage},
		{"image-01", ModelCapabilityImage},
		{"doubao-seedream-4.0", ModelCapabilityImage},
		{"gpt-image-1", ModelCapabilityImage},
		{"dall-e-3", ModelCapabilityImage},
		{"grok-imagine-image", ModelCapabilityImage},
		{"kolors-t2i", ModelCapabilityImage},
		// 音频
		{"speech-01", ModelCapabilityAudio},
		{"whisper-1", ModelCapabilityAudio},
		{"cosyvoice-v2", ModelCapabilityAudio},
		{"tts-1", ModelCapabilityAudio},
		{"qwen-tts", ModelCapabilityAudio},
		// 文本：未收录模型
		{"gpt-4o", ModelCapabilityText},
		{"claude-3-5-sonnet", ModelCapabilityText},
		{"deepseek-chat", ModelCapabilityText},
		{"qwen-plus", ModelCapabilityText},
	}
	for _, tc := range cases {
		t.Run(tc.model, func(t *testing.T) {
			if got := DispatchModelCapability(tc.model); got != tc.want {
				t.Fatalf("DispatchModelCapability(%q) = %v, want %v", tc.model, got, tc.want)
			}
		})
	}
}

// TestWanxIsImageNotVideo 守住一个真实踩过的坑：
// wanx 是阿里通义万相的**图像**系列，曾因与 wan2/wan3 视频系列共用 "wan" 前缀
// 被一起收进视频规则，导致 wanx2.1 的生图请求被送进视频链路。
func TestWanxIsImageNotVideo(t *testing.T) {
	for _, model := range []string{"wanx2.1", "wanx2.0-t2i-turbo", "wanx-poster"} {
		if IsKnownVideoVendorModel(model) {
			t.Errorf("%q 不应属于视频系列", model)
		}
		if !IsKnownImageVendorModel(model) {
			t.Errorf("%q 必须属于图片系列", model)
		}
		if got := DispatchModelCapability(model); got != ModelCapabilityImage {
			t.Errorf("DispatchModelCapability(%q) = %v, want image", model, got)
		}
	}
	// 视频系列不受影响
	for _, model := range []string{"wan3.0-video", "wan2.2-t2v-plus", "wan2.1-i2v"} {
		if !IsKnownVideoVendorModel(model) {
			t.Errorf("%q 必须属于视频系列", model)
		}
	}
}

// TestNewModelVersionsCoveredByWildcard 验证规则是通配而非枚举：
// 厂商发布新型号（pro / 02 / 4.0）时不应需要改代码。
func TestNewModelVersionsCoveredByWildcard(t *testing.T) {
	imageLike := []string{"qwen-image-3.0-pro", "qwen-image-9.9-max", "minimax-image-07", "wanx9.9"}
	for _, m := range imageLike {
		if DispatchModelCapability(m) != ModelCapabilityImage {
			t.Errorf("%q 应被图片通配规则覆盖，无需逐个收录", m)
		}
	}
	videoLike := []string{"wan3.5-video", "wan9.9-t2v", "minimax-h9"}
	for _, m := range videoLike {
		if DispatchModelCapability(m) != ModelCapabilityVideo {
			t.Errorf("%q 应被视频通配规则覆盖，无需逐个收录", m)
		}
	}
}

// TestDispatchAgreesWithMediaKindForKnownModels 对已收录模型，两个分派函数必须一致，
// 否则同一个模型在统一入口和媒体端点会走不同链路。
func TestDispatchAgreesWithMediaKindForKnownModels(t *testing.T) {
	known := []string{
		"wan3.0-video", "wan2.2-t2v-plus", "wan2.1-i2v", "seedance-1.0-pro",
		"minimax-hailuo-02", "minimax-h3", "minimax-video-01", "grok-imagine-video",
		"qwen-image-3.0", "qwen-image-3.0-pro", "minimax-image-01", "minimax-image-02",
		"wanx2.1", "doubao-seedream-4.0", "gpt-image-1", "dall-e-3",
		"grok-imagine-image", "kolors-t2i",
		"speech-01", "whisper-1", "cosyvoice-v2", "tts-1", "qwen-tts",
	}
	for _, model := range known {
		dispatch := DispatchModelCapability(model)
		kind := MediaKindFromModel(model, nil)
		var want MediaKind
		switch dispatch {
		case ModelCapabilityImage:
			want = MediaKindImage
		case ModelCapabilityVideo:
			want = MediaKindVideo
		case ModelCapabilityAudio:
			want = MediaKindAudio
		default:
			t.Fatalf("%q 应是已收录模型，却落到默认分支", model)
		}
		if kind != want {
			t.Errorf("%q: DispatchModelCapability=%v 但 MediaKindFromModel=%v，两处必须一致",
				model, dispatch, kind)
		}
	}
}

func TestMediaKindFromModelDoesNotLetBodyHintOverrideModel(t *testing.T) {
	if got := MediaKindFromModel("gpt-4o", map[string]any{"type": "image"}); got != MediaKindVideo {
		t.Fatalf("text model with image hint = %v, want video fallback for media endpoint", got)
	}
	if got := MediaKindFromModel("qwen-image-3.0", map[string]any{"type": "video"}); got != MediaKindImage {
		t.Fatalf("image model with video hint = %v, want image", got)
	}
	if got := MediaKindFromModel("", map[string]any{"type": "image"}); got != MediaKindImage {
		t.Fatalf("missing model with image hint = %v, want image compatibility hint", got)
	}
}

// TestGrokImagineSeriesSplit 同前缀下 image 与 video 必须分开。
func TestGrokImagineSeriesSplit(t *testing.T) {
	if DispatchModelCapability("grok-imagine-image") != ModelCapabilityImage {
		t.Error("grok-imagine-image 应为图片")
	}
	if DispatchModelCapability("grok-imagine-video") != ModelCapabilityVideo {
		t.Error("grok-imagine-video 应为视频")
	}
}

// TestTextModelsAreNotMisrouted 纯文本大模型必须落在文本链路。
//
// 媒体关键词多为宽泛子串（"-image"、"voice"、"speech"、"t2i"），文本模型名一旦
// 碰巧包含这些词就会被送进图片/视频/音频链路，客户端会收到与意图完全无关的
// 失败。这里把国内外主流文本模型钉死为 text，尤其覆盖「数字版本号 + 厂商前缀」
// 的新型号（mini­max-m3 / kimi-k3 / qwen3.8-max / glm-5.3 这类）。
func TestTextModelsAreNotMisrouted(t *testing.T) {
	textModels := []string{
		// 用户明确点名的四个
		"minimax-m3", "kimi-k3", "qwen3.8-max", "glm-5.3",
		// MiniMax / Moonshot（kimi）文本系列
		"minimax-m1", "minimax-m2", "minimax-m2-her", "minimax-text-01", "minimax-01",
		"kimi-k2", "kimi-latest", "kimi-k2-thinking", "moonshot-v1-8k", "moonshot-v1-128k",
		// 通义文本系列（注意 qwen-vl-max 是视觉语言模型，走 chat 而非生图）
		"qwen-plus", "qwen-max", "qwen3-max", "qwen-long",
		"qwen3-235b-a22b", "qwen2.5-72b-instruct", "qwen-vl-max",
		// 智谱文本系列（glm-4v 是视觉理解，不是生图）
		"glm-4.6", "glm-4.5-air", "glm-4-plus", "glm-4v", "glm-4v-plus", "glm-4-9b-chat",
		// 其他国产 + 海外文本模型
		"ernie-4.0-8k", "hunyuan-turbo", "doubao-pro-32k", "step-3", "baichuan4",
		"yi-large", "ling-1t",
		"gpt-4o", "gpt-4o-mini", "o3-mini", "claude-3-5-sonnet", "claude-opus-4-1",
		"deepseek-chat", "gemini-2.5-pro", "grok-4", "grok-4-fast",
	}
	for _, model := range textModels {
		if got := DispatchModelCapability(model); got != ModelCapabilityText {
			t.Errorf("%q 是文本模型，却被判为 %v", model, got)
		}
		if IsKnownImageVendorModel(model) {
			t.Errorf("%q 被图片规则误收", model)
		}
		if IsKnownVideoVendorModel(model) {
			t.Errorf("%q 被视频规则误收", model)
		}
		if IsKnownAudioVendorModel(model) {
			t.Errorf("%q 被音频规则误收", model)
		}
	}
}

// TestTextModelsStayTextAcrossFutureVersions 版本号演进不得改变模态归属。
//
// 厂商几乎必然会在同一前缀下继续发新版本（m3→m4、k3→k4、glm-5.3→glm-6.x）。
// 只要没带图片/视频/音频关键词，就永远是文本。
func TestTextModelsStayTextAcrossFutureVersions(t *testing.T) {
	for _, model := range []string{
		"minimax-m4", "minimax-m10", "kimi-k4", "kimi-k9",
		"qwen4.2-max", "qwen9.9-max", "glm-6.1", "glm-10.0",
	} {
		if got := DispatchModelCapability(model); got != ModelCapabilityText {
			t.Errorf("%q 新型号应仍是文本，实际 %v", model, got)
		}
	}
}
