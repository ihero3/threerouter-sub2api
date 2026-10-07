package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestValidateMediaPromptRequired 锁定 fix A：空 prompt 在选号 / 打上游之前被拦成
// 400（*MediaInvalidRequestError），且音频带参考素材时可省略 prompt。
func TestValidateMediaPromptRequired(t *testing.T) {
	cases := []struct {
		name    string
		kind    MediaKind
		req     *MediaCreateRequest
		wantErr bool
	}{
		{"image empty", MediaKindImage, &MediaCreateRequest{}, true},
		{"image with prompt", MediaKindImage, &MediaCreateRequest{Prompt: "a cat"}, false},
		{"video empty", MediaKindVideo, &MediaCreateRequest{}, true},
		{"video with prompt", MediaKindVideo, &MediaCreateRequest{Prompt: "explosion"}, false},
		{"audio empty no ref", MediaKindAudio, &MediaCreateRequest{}, true},
		{"audio empty with media ref", MediaKindAudio, &MediaCreateRequest{Media: []VideoMediaInput{{Type: "audio", URL: "http://x/a.mp3"}}}, false},
		{"audio empty with audio ref url", MediaKindAudio, &MediaCreateRequest{AudioRefURLs: []string{"http://x/a.mp3"}}, false},
		{"audio with prompt", MediaKindAudio, &MediaCreateRequest{Prompt: "hello"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateMediaPromptRequired(c.kind, c.req)
			if c.wantErr {
				require.Error(t, err)
				var inv *MediaInvalidRequestError
				require.ErrorAs(t, err, &inv)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestParseMediaCreateRequest_NestedInputPrompt 锁定 fix B：阿里 DashScope 原生
// 异步协议把 prompt 放在 input.prompt，顶层无 prompt 时必须回落到这里，
// 否则上游会回 "Field required: input.prompt"。
func TestParseMediaCreateRequest_NestedInputPrompt(t *testing.T) {
	body := map[string]any{
		"model": "qwen-image-3.0",
		"input": map[string]any{
			"prompt": "a red apple",
		},
	}
	req, err := parseMediaCreateRequest(MediaKindImage, "qwen-image-3.0", body)
	require.NoError(t, err)
	require.Equal(t, "a red apple", req.Prompt)
}

// TestSanitizeMediaUpstreamErrorMessage 锁定 fix C：媒体链路对调用方的错误文案只按
// 状态码给出平台统一话术，绝不回显上游原文（code/message/原始体都可能泄露上游实现
// 细节与请求回显）。上游原文仍保留在 UpstreamRaw 供分类与运营排查。
func TestSanitizeMediaUpstreamErrorMessage(t *testing.T) {
	// OpenAI 风格 error 对象：不得回显上游 message / code
	openaiBody := []byte(`{"error":{"message":"Invalid prompt","code":"invalid_request_error","type":"invalid_request_error"}}`)
	got := sanitizeMediaUpstreamErrorMessage(400, openaiBody)
	require.Equal(t, "Upstream request failed", got)
	require.NotContains(t, got, "Invalid prompt")
	require.NotContains(t, got, "invalid_request_error")

	// DashScope / MiniMax 顶层 code + message：同样不得回显
	dashBody := []byte(`{"code":"InvalidParameter","message":"Field required: input.prompt"}`)
	got = sanitizeMediaUpstreamErrorMessage(400, dashBody)
	require.Equal(t, "Upstream request failed", got)
	require.NotContains(t, got, "Field required")
	require.NotContains(t, got, "InvalidParameter")

	// 空体：仍按状态码给话术
	require.Equal(t, "Upstream request failed", sanitizeMediaUpstreamErrorMessage(400, []byte("")))

	// 非 JSON 长体：不得回显原文
	long := []byte("some raw text longer than two hundred fifty six characters xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")
	got = sanitizeMediaUpstreamErrorMessage(500, long)
	require.Equal(t, "Upstream service temporarily unavailable", got)
	require.NotContains(t, got, "some raw text")

	// 其他状态码映射
	require.Equal(t, "Upstream authentication failed", sanitizeMediaUpstreamErrorMessage(401, openaiBody))
	require.Equal(t, "Upstream access denied", sanitizeMediaUpstreamErrorMessage(403, openaiBody))
	require.Equal(t, "Upstream resource not found", sanitizeMediaUpstreamErrorMessage(404, openaiBody))
	require.Equal(t, "Upstream rate limit exceeded", sanitizeMediaUpstreamErrorMessage(429, openaiBody))
}
