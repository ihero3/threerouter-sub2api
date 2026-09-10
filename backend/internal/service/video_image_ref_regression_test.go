package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// 回归：用户按各家文档传 "image"（而非 OpenAI 风格的 image_url）时，
// 图生视频的参考图不能被静默忽略。视频与媒体两条链路都必须识别。
func TestParseVideoCreateRequest_AcceptsImageField(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body map[string]any
		want string
	}{
		{
			name: "image as string",
			body: map[string]any{"model": "minimax-h3", "prompt": "run", "image": "https://cdn.example.com/a.png"},
			want: "https://cdn.example.com/a.png",
		},
		{
			name: "image as object with url",
			body: map[string]any{"model": "minimax-h3", "prompt": "run", "image": map[string]any{"url": "https://cdn.example.com/b.png"}},
			want: "https://cdn.example.com/b.png",
		},
		{
			name: "image as array of strings",
			body: map[string]any{"model": "minimax-h3", "prompt": "run", "image": []any{"https://cdn.example.com/c.png"}},
			want: "https://cdn.example.com/c.png",
		},
		{
			name: "image_url still works",
			body: map[string]any{"model": "minimax-h3", "prompt": "run", "image_url": "https://cdn.example.com/d.png"},
			want: "https://cdn.example.com/d.png",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req, err := parseVideoCreateRequest(tc.body["model"].(string), tc.body)
			require.NoError(t, err)
			require.Equal(t, []string{tc.want}, req.ImageRefURLs)
		})
	}
}

func TestParseMediaCreateRequest_AcceptsImageField(t *testing.T) {
	t.Parallel()
	req, err := parseMediaCreateRequest(MediaKindVideo, "minimax-h3", map[string]any{
		"model":  "minimax-h3",
		"prompt": "run",
		"image":  "https://cdn.example.com/a.png",
	})
	require.NoError(t, err)
	require.Equal(t, []string{"https://cdn.example.com/a.png"}, req.ImageRefURLs)
}

// 回归：MiniMax H3 的图生视频必须把参考图放进 content 数组并标 role=first_frame，
// 否则上游会当成纯文生视频，产出一条和参考图无关的结果。
func TestBuildMiniMaxVideoCreateBody_ImageBecomesFirstFrame(t *testing.T) {
	t.Parallel()
	body := buildMiniMaxVideoCreateBody(VideoCreateRequest{
		UpstreamModel: "MiniMax-H3",
		Prompt:        "push in slowly",
		ImageRefURLs:  []string{"https://cdn.example.com/a.png"},
		Resolution:    "2K",
		DurationSec:   5,
		Ratio:         "adaptive",
	})

	var parsed struct {
		Model   string           `json:"model"`
		Content []map[string]any `json:"content"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.Equal(t, "MiniMax-H3", parsed.Model)
	require.Len(t, parsed.Content, 2)
	require.Equal(t, "text", parsed.Content[0]["type"])

	img := parsed.Content[1]
	require.Equal(t, "image_url", img["type"])
	require.Equal(t, "first_frame", img["role"])
	urlObj, ok := img["image_url"].(map[string]any)
	require.True(t, ok, "image_url must be an object, got %T", img["image_url"])
	require.Equal(t, "https://cdn.example.com/a.png", urlObj["url"])
}

// MiniMax H3 使用 768P / 2K，计费档位不能被折算成 480p。
func TestLookupVideoBillingResolutionAnyKeepsVendorTiers(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"768P", "768p", "768"} {
		got, ok := LookupVideoBillingResolutionAny(in)
		require.True(t, ok, "input=%q", in)
		require.Equal(t, "768p", got)
	}
	for _, in := range []string{"2K", "2k"} {
		got, ok := LookupVideoBillingResolutionAny(in)
		require.True(t, ok, "input=%q", in)
		require.Equal(t, "2k", got)
	}
	// 标准档位仍走原有归一化。
	got, ok := LookupVideoBillingResolutionAny("full_hd")
	require.True(t, ok)
	require.Equal(t, VideoBillingResolution1080P, got)
}
