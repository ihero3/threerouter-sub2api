package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMediaKindFromModel_Routing(t *testing.T) {
	cases := map[string]MediaKind{
		"wan3.0-video":    MediaKindVideo,
		"wan2.6-t2i":      MediaKindImage,
		"seedance-2.5":    MediaKindVideo,
		"doubao-seedream": MediaKindImage,
		"minimax-h3":      MediaKindVideo,
		"image-01":        MediaKindImage,
		"cosyvoice":       MediaKindAudio,
		"mini-tts":        MediaKindAudio,
	}
	for model, want := range cases {
		got := MediaKindFromModel(model, nil)
		require.Equalf(t, want, got, "model=%q", model)
	}
}

func TestBuildSeedanceImageCreateBody(t *testing.T) {
	req := MediaCreateRequest{UpstreamModel: "doubao-seedream-4-0", Prompt: "a cat", Resolution: "2048x1024"}
	body := buildSeedanceImageCreateBody(req)
	var m map[string]any
	require.NoError(t, json.Unmarshal(body, &m))
	require.Equal(t, "doubao-seedream-4-0", m["model"])
	require.Equal(t, "a cat", m["prompt"])
	require.Equal(t, "2048x1024", m["size"])
}

func TestBuildWanImageCreateBody(t *testing.T) {
	seed := int64(7)
	req := MediaCreateRequest{UpstreamModel: "wan2.6-t2i", Prompt: "a dog", Resolution: "1280*1280", Seed: &seed}
	body := buildWanImageCreateBody(req)
	var m map[string]any
	require.NoError(t, json.Unmarshal(body, &m))
	require.Equal(t, "wan2.6-t2i", m["model"])
	input := m["input"].(map[string]any)
	messages := input["messages"].([]any)
	require.Equal(t, "user", messages[0].(map[string]any)["role"])
	params := m["parameters"].(map[string]any)
	require.Equal(t, "1280*1280", params["size"])
	require.Equal(t, float64(7), params["seed"])
}

func TestBuildMiniMaxImageCreateBody_AspectRatio(t *testing.T) {
	req := MediaCreateRequest{UpstreamModel: "image-01", Prompt: "a house", Resolution: "16:9"}
	body := buildMiniMaxImageCreateBody(req)
	var m map[string]any
	require.NoError(t, json.Unmarshal(body, &m))
	require.Equal(t, "image-01", m["model"])
	require.Equal(t, "16:9", m["aspect_ratio"])
}

func TestBuildSeedanceVideoCreateBody_Content(t *testing.T) {
	req := VideoCreateRequest{UpstreamModel: "doubao-seedance-1-5-pro", Prompt: "sunset"}
	body := buildSeedanceVideoCreateBody(req)
	var m map[string]any
	require.NoError(t, json.Unmarshal(body, &m))
	require.Equal(t, "doubao-seedance-1-5-pro", m["model"])
	content := m["content"].([]any)
	require.Equal(t, "text", content[0].(map[string]any)["type"])
	require.Equal(t, "sunset", content[0].(map[string]any)["text"])
}

func TestBuildVideoCreateBodyDoesNotLetOriginalModelOverrideMapping(t *testing.T) {
	body := buildVideoCreateBody(VideoCreateRequest{
		UpstreamModel: "doubao-seedance-1-5-pro",
		Prompt:        "sunset",
		Resolution:    "1080p",
		DurationSec:   8,
		Extra: map[string]any{
			"model":      "seedance-1.0-pro",
			"prompt":     "original prompt",
			"resolution": "480p",
			"duration":   2,
			"ratio":      "16:9",
		},
	})

	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	require.Equal(t, "doubao-seedance-1-5-pro", payload["model"])
	require.Equal(t, "sunset", payload["prompt"])
	require.Equal(t, "1080p", payload["resolution"])
	require.Equal(t, float64(8), payload["duration"])
	require.Equal(t, "16:9", payload["ratio"])
}

func TestParseMediaCreateRequest_IgnoresRatioForImageToVideo(t *testing.T) {
	// image 字段存在 → 图生视频：Ratio 清空且 Extra 里的 ratio 一并删除。
	req, err := parseMediaCreateRequest(MediaKindVideo, "minimax-h3", map[string]any{
		"prompt":     "让画面动起来",
		"image":      "https://example.com/first.png",
		"ratio":      "16:9",
		"resolution": "768P",
	})
	require.NoError(t, err)
	require.Empty(t, req.Ratio)
	require.NotContains(t, req.Extra, "ratio")
	require.Equal(t, []string{"https://example.com/first.png"}, req.ImageRefURLs)

	// 纯文生视频：ratio 原样保留（req.Ratio 与 Extra 均不动）。
	req, err = parseMediaCreateRequest(MediaKindVideo, "minimax-h3", map[string]any{
		"prompt": "海边日落",
		"ratio":  "16:9",
	})
	require.NoError(t, err)
	require.Equal(t, "16:9", req.Ratio)
	require.Equal(t, "16:9", req.Extra["ratio"])

	// media 素材中的图片项同样视为图生视频。
	req, err = parseMediaCreateRequest(MediaKindVideo, "minimax-h3", map[string]any{
		"prompt": "让画面动起来",
		"media":  []any{map[string]any{"type": "first_frame", "url": "https://example.com/f.png"}},
		"ratio":  "9:16",
	})
	require.NoError(t, err)
	require.Empty(t, req.Ratio)
}

func TestParseMediaCreateRequest_ImageFieldVariants(t *testing.T) {
	// 契约允许 http URL 或 base64 data URL；服务端原样透传，由上游决定是否接受。
	req, err := parseMediaCreateRequest(MediaKindVideo, "minimax-h3", map[string]any{
		"image": "data:image/png;base64,iVBORw0KGgo=",
	})
	require.NoError(t, err)
	require.Equal(t, []string{"data:image/png;base64,iVBORw0KGgo="}, req.ImageRefURLs)

	req, err = parseMediaCreateRequest(MediaKindVideo, "minimax-h3", map[string]any{
		"image": []any{"https://example.com/a.png", map[string]any{"url": "https://example.com/b.png"}},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"https://example.com/a.png", "https://example.com/b.png"}, req.ImageRefURLs)
}

func TestValidateVideoResolution_MinimaxH3(t *testing.T) {
	// 合法档位：480P / 768P / 2K（大小写不敏感，兼容常见写法）与空值（用上游默认）。
	for _, res := range []string{"480P", "768p", "2K", "768", "2kp", "1440p", ""} {
		require.NoErrorf(t, validateVideoResolution("MiniMax-H3", res), "resolution=%q", res)
	}
	// 非法档位：返回可被 handler 识别的 400 类型化错误。
	for _, res := range []string{"1080p", "720p", "4k", "1080"} {
		err := validateVideoResolution("MiniMax-H3", res)
		var invalidReq *MediaInvalidRequestError
		require.ErrorAsf(t, err, &invalidReq, "resolution=%q", res)
		require.NotEmpty(t, invalidReq.Reason)
	}
	// H3 系列前缀同样生效；其他模型不校验、透传上游。
	require.Error(t, validateVideoResolution("minimax-h3-max", "1080p"))
	require.NoError(t, validateVideoResolution("seedance-2.5", "1080p"))
	require.NoError(t, validateVideoResolution("wan2.6-t2v", "4k"))
}

// TestParseMiniMaxImageCreateResult_OfficialShape 用 MiniMax 官方文生图响应结构验证解析。
// 官方返回 data 是**对象**而非数组：url 模式为 {"image_urls":[...]}，
// base64 模式为 {"image_base64":[...]}。此前按 data[0].url 解析，URL 恒为空。
func TestParseMiniMaxImageCreateResult_OfficialShape(t *testing.T) {
	t.Run("url 模式", func(t *testing.T) {
		got, err := parseMiniMaxImageCreateResult([]byte(`{
			"id": "03ff3cd0820949eb8a410056b5f21d38",
			"data": {"image_urls": ["https://cdn.hailuoai.com/a.jpeg", "https://cdn.hailuoai.com/b.jpeg"]},
			"metadata": {"success_count": "2"},
			"base_resp": {"status_code": 0, "status_msg": "success"}
		}`), 200)
		require.NoError(t, err)
		require.Equal(t, "succeeded", got.Status)
		require.Equal(t, "https://cdn.hailuoai.com/a.jpeg", got.InlineURL)
		require.Equal(t, []string{"https://cdn.hailuoai.com/a.jpeg", "https://cdn.hailuoai.com/b.jpeg"}, got.InlineURLs,
			"n>1 时应保留全部 URL，计费与返回都按张数")
	})

	t.Run("base64 模式转 data URI", func(t *testing.T) {
		got, err := parseMiniMaxImageCreateResult([]byte(`{
			"data": {"image_base64": ["aGVsbG8="]},
			"base_resp": {"status_code": 0}
		}`), 200)
		require.NoError(t, err)
		require.Equal(t, "succeeded", got.Status)
		require.Equal(t, "data:image/jpeg;base64,aGVsbG8=", got.InlineURL)
	})

	t.Run("HTTP 200 但业务失败", func(t *testing.T) {
		got, err := parseMiniMaxImageCreateResult([]byte(`{
			"data": {},
			"base_resp": {"status_code": 1026, "status_msg": "input content violates content policy"}
		}`), 200)
		require.NoError(t, err)
		require.Equal(t, "failed", got.Status, "MiniMax 用 base_resp.status_code 表达业务失败，只看 HTTP 码会误判为成功")
		require.Contains(t, got.ErrorMessage, "1026")
	})

	t.Run("无图视为失败", func(t *testing.T) {
		got, err := parseMiniMaxImageCreateResult([]byte(`{"data": {}, "base_resp": {"status_code": 0}}`), 200)
		require.NoError(t, err)
		require.Equal(t, "failed", got.Status)
	})
}

// TestBuildMiniMaxImageCreateBody 校验请求体构造：张数下传与图生图参考图映射。
func TestBuildMiniMaxImageCreateBody(t *testing.T) {
	body := buildMiniMaxImageCreateBody(MediaCreateRequest{
		UpstreamModel: "image-01",
		Prompt:        "a cat",
		Resolution:    "16:9",
		ImageCount:    3,
	})
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(body, &decoded))
	require.Equal(t, "image-01", decoded["model"])
	require.Equal(t, "16:9", decoded["aspect_ratio"])
	require.Equal(t, float64(3), decoded["n"], "n 必须下传，否则上游只出 1 张但计费按多张")

	// WxH 走 width/height 分支；8 的倍数由上游校验，这里只确认不落到 aspect_ratio。
	wBody := buildMiniMaxImageCreateBody(MediaCreateRequest{UpstreamModel: "image-01", Resolution: "1024x1024"})
	var wDecoded map[string]any
	require.NoError(t, json.Unmarshal(wBody, &wDecoded))
	require.Equal(t, float64(1024), wDecoded["width"])
	require.Equal(t, float64(1024), wDecoded["height"])
	require.NotContains(t, wDecoded, "aspect_ratio")

	// 图生图：image 参考图映射为 subject_reference。
	i2i := buildMiniMaxImageCreateBody(MediaCreateRequest{
		UpstreamModel: "image-01",
		ImageRefURLs:  []string{"https://example.com/ref.jpg"},
	})
	var i2iDecoded map[string]any
	require.NoError(t, json.Unmarshal(i2i, &i2iDecoded))
	refs, ok := i2iDecoded["subject_reference"].([]any)
	require.True(t, ok, "应生成 subject_reference")
	require.Len(t, refs, 1, "MiniMax 每次仅支持一张参考图")
	require.Equal(t, "https://example.com/ref.jpg", refs[0].(map[string]any)["image_file"])
}

// TestParseMediaCreateRequest_ImageParams 图片参数解析：
// size 是 resolution 的别名（客户端照搬 OpenAI Images 写法时不能静默丢弃），
// n 决定计费张数，auto 视为未指定。
func TestParseMediaCreateRequest_ImageParams(t *testing.T) {
	req, err := parseMediaCreateRequest(MediaKindImage, "image-01", map[string]any{
		"prompt": "a cat",
		"size":   "1024x1024",
		"n":      float64(4),
	})
	require.NoError(t, err)
	require.Equal(t, "1024x1024", req.Resolution)
	require.Equal(t, 4, req.ImageCount)

	// resolution 优先于 size
	req, err = parseMediaCreateRequest(MediaKindImage, "image-01", map[string]any{
		"resolution": "16:9", "size": "1024x1024",
	})
	require.NoError(t, err)
	require.Equal(t, "16:9", req.Resolution)

	// auto 视为未指定：媒体链路没有等价语义，透传会被当成 aspect_ratio 下发导致上游报错
	req, err = parseMediaCreateRequest(MediaKindImage, "image-01", map[string]any{"size": "auto"})
	require.NoError(t, err)
	require.Equal(t, "", req.Resolution)

	// n 越界/非法时收敛到 1，与上游默认值一致
	require.Equal(t, 1, parseMediaImageCount(map[string]any{"n": float64(0)}))
	require.Equal(t, 1, parseMediaImageCount(map[string]any{"n": "3"}))
	require.Equal(t, 9, parseMediaImageCount(map[string]any{"n": float64(99)}), "超过 9 收敛到上限")
	require.Equal(t, 1, parseMediaImageCount(nil))
}

// TestQwenImageDashScopeContract 用千问图像 3.0 的 DashScope 同步契约验证 Wan adapter。
// 官方要求：size 用星号分隔（宽*高）；I2I 通过 input.messages[0].content 里的
// {"image": ...} 传参考图；失败时返回 HTTP 200 + code/message。
func TestQwenImageDashScopeContract(t *testing.T) {
	// 1) size 分隔符：OpenAI 写法的 x 必须转成 DashScope 的 *
	raw := buildWanImageCreateBody(MediaCreateRequest{
		UpstreamModel: "qwen-image-3.0-pro",
		Prompt:        "a cat",
		Resolution:    "1024x1024",
	})
	var built struct {
		Parameters struct {
			Size string `json:"size"`
		} `json:"parameters"`
	}
	require.NoError(t, json.Unmarshal(raw, &built))
	if built.Parameters.Size != "1024*1024" {
		t.Errorf("size 分隔符错误: got %q, want %q", built.Parameters.Size, "1024*1024")
	}

	// 2) I2I：参考图必须进 messages content
	raw = buildWanImageCreateBody(MediaCreateRequest{
		UpstreamModel: "qwen-image-3.0-pro",
		Prompt:        "make it cyberpunk",
		ImageRefURLs:  []string{"https://cdn.example.com/a.png"},
	})
	var i2i struct {
		Input struct {
			Messages []struct {
				Content []map[string]any `json:"content"`
			} `json:"messages"`
		} `json:"input"`
	}
	require.NoError(t, json.Unmarshal(raw, &i2i))
	require.Len(t, i2i.Input.Messages, 1)
	hasImage := false
	for _, part := range i2i.Input.Messages[0].Content {
		if _, ok := part["image"]; ok {
			hasImage = true
		}
	}
	if !hasImage {
		t.Errorf("I2I 参考图未下发: content = %v", i2i.Input.Messages[0].Content)
	}

	// 3) HTTP 200 + code/message 是业务失败，不能当成成功
	got, err := parseWanImageCreateResult([]byte(`{"code":"InvalidApiKey","message":"Invalid API-key provided.","request_id":"xxx"}`), 200)
	require.NoError(t, err)
	if got.Status != "failed" {
		t.Errorf("HTTP 200 + code 未判失败: status = %q", got.Status)
	}
}

// TestQwenImageDashScopeMultiImage 覆盖千问的多张生成与真实尺寸回传。
func TestQwenImageDashScopeMultiImage(t *testing.T) {
	readParams := func(body []byte) map[string]any {
		var out struct {
			Parameters map[string]any `json:"parameters"`
			Input      struct {
				Messages []struct {
					Content []map[string]any `json:"content"`
				} `json:"messages"`
			} `json:"input"`
		}
		require.NoError(t, json.Unmarshal(body, &out))
		params := out.Parameters
		if len(out.Input.Messages) > 0 && params != nil {
			params["_content"] = out.Input.Messages[0].Content
		}
		return params
	}

	// n 上限：千问支持 1-6，超出必须收敛，否则上游直接拒单
	params := readParams(buildWanImageCreateBody(MediaCreateRequest{
		UpstreamModel: "qwen-image-3.0", Prompt: "a", ImageCount: 9,
	}))
	require.Equal(t, float64(6), params["n"], "超过 6 张应收敛到上限")

	params = readParams(buildWanImageCreateBody(MediaCreateRequest{
		UpstreamModel: "qwen-image-3.0", Prompt: "a",
	}))
	require.Equal(t, float64(1), params["n"], "未指定张数时按 1 张")

	// I2I：参考图上限 3 张，且必须排在文本之前
	params = readParams(buildWanImageCreateBody(MediaCreateRequest{
		UpstreamModel: "qwen-image-3.0", Prompt: "edit it",
		ImageRefURLs: []string{"1.png", "2.png", "3.png", "4.png"},
	}))
	content, ok := params["_content"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, content, 4, "3 张参考图 + 1 个文本")
	for i := 0; i < 3; i++ {
		require.Contains(t, content[i], "image", "第 %d 项应为参考图", i)
	}
	require.Contains(t, content[3], "text", "文本必须排在参考图之后")

	// 多图响应 + usage 真实尺寸
	resp := `{"output":{"choices":[
		{"finish_reason":"stop","message":{"role":"assistant","content":[{"image":"https://x/a.png"}]}},
		{"finish_reason":"stop","message":{"role":"assistant","content":[{"image":"https://x/b.png"}]}}
	]},"usage":{"output_width":1024,"output_height":1024,"output_image_count":2}}`
	got, err := parseWanImageCreateResult([]byte(resp), 200)
	require.NoError(t, err)
	require.Equal(t, "succeeded", got.Status)
	require.Equal(t, []string{"https://x/a.png", "https://x/b.png"}, got.InlineURLs)
	require.Equal(t, "https://x/a.png", got.InlineURL)
	require.Equal(t, "1024x1024", got.UpstreamSize, "真实输出尺寸必须回传用于计费")
}
