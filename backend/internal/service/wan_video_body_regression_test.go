package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// 回归：wan3.0-video 图生视频必须把参考图放进 input.media，
// 并且请求体里不能再出现泄漏的顶层 image 字段（此前会导致上游 502）。
func TestBuildWanVideoCreateBody_ImageGoesIntoInputMedia(t *testing.T) {
	t.Parallel()
	body := buildWanVideoCreateBody(VideoCreateRequest{
		UpstreamModel: "wan3.0-video",
		Prompt:        "a dog running",
		ImageRefURLs:  []string{"https://cdn.example.com/a.png"},
		Resolution:    "720p",
		DurationSec:   5,
	})
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotContains(t, parsed, "image", "top-level image must not leak to upstream")

	in, ok := parsed["input"].(map[string]any)
	require.True(t, ok)
	media, ok := in["media"].([]any)
	require.True(t, ok, "input.media missing: %v", in)
	require.Len(t, media, 1)
	first, ok := media[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "first_frame", first["type"])
	require.Equal(t, "https://cdn.example.com/a.png", first["url"])
}
