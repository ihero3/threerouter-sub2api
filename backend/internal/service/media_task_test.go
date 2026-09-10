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
