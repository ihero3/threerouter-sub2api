package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// 回归：MiniMax 对 ratio 有硬性要求 —— 纯文生视频（t2va）必填且不能为 adaptive，
// 有素材输入时恒为 adaptive。此前 adapter 只读 Extra 里的 ratio，统一解析出的
// req.Ratio 被整体丢弃，客户端传值静默失效，纯文生视频还会因缺字段被上游拒绝。
func TestBuildMiniMaxVideoCreateBody_RatioDefaults(t *testing.T) {
	t.Parallel()

	t.Run("text-to-video falls back to 16:9 when ratio missing", func(t *testing.T) {
		body := decodeMiniMaxBody(t, buildMiniMaxVideoCreateBody(VideoCreateRequest{
			UpstreamModel: "MiniMax-H3",
			Prompt:        "a woman sitting in a cafe",
			DurationSec:   5,
		}))
		require.Equal(t, "16:9", body["ratio"])
	})

	t.Run("text-to-video rejects adaptive and falls back", func(t *testing.T) {
		body := decodeMiniMaxBody(t, buildMiniMaxVideoCreateBody(VideoCreateRequest{
			UpstreamModel: "MiniMax-H3",
			Prompt:        "a woman sitting in a cafe",
			Ratio:         "adaptive",
		}))
		require.Equal(t, "16:9", body["ratio"], "t2va must not send adaptive")
	})

	t.Run("image-to-video uses adaptive", func(t *testing.T) {
		body := decodeMiniMaxBody(t, buildMiniMaxVideoCreateBody(VideoCreateRequest{
			UpstreamModel: "MiniMax-H3",
			Prompt:        "the person starts dancing",
			ImageRefURLs:  []string{"https://cdn.example.com/first.png"},
		}))
		require.Equal(t, "adaptive", body["ratio"])
	})

	t.Run("explicit client ratio is preserved", func(t *testing.T) {
		body := decodeMiniMaxBody(t, buildMiniMaxVideoCreateBody(VideoCreateRequest{
			UpstreamModel: "MiniMax-H3",
			Prompt:        "a woman sitting in a cafe",
			Ratio:         "9:16",
		}))
		require.Equal(t, "9:16", body["ratio"])
	})
}

// 回归：content 必须是多模态数组，首帧图要带 role=first_frame。
func TestBuildMiniMaxVideoCreateBody_ContentShape(t *testing.T) {
	t.Parallel()
	body := decodeMiniMaxBody(t, buildMiniMaxVideoCreateBody(VideoCreateRequest{
		UpstreamModel: "MiniMax-H3",
		Prompt:        "the person starts dancing",
		ImageRefURLs:  []string{"https://cdn.example.com/first.png"},
		Resolution:    "768P",
		DurationSec:   5,
	}))
	require.Equal(t, "MiniMax-H3", body["model"])
	require.Equal(t, "768P", body["resolution"])
	require.Equal(t, float64(5), body["duration"])

	content, ok := body["content"].([]any)
	require.True(t, ok, "content must be an array: %v", body["content"])
	require.Len(t, content, 2)

	text, ok := content[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "text", text["type"])
	require.Equal(t, "the person starts dancing", text["text"])

	frame, ok := content[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "image_url", frame["type"])
	require.Equal(t, "first_frame", frame["role"])
	imageURL, ok := frame["image_url"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "https://cdn.example.com/first.png", imageURL["url"])
}

func decodeMiniMaxBody(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(raw, &parsed))
	return parsed
}
