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

// TestSanitizeMediaUpstreamErrorMessage 锁定 fix C：从上游错误体抽取结构化
// code/message，而非把整段原始 JSON 直接甩给调用方。
func TestSanitizeMediaUpstreamErrorMessage(t *testing.T) {
	// OpenAI 风格 error 对象
	openaiBody := []byte(`{"error":{"message":"Invalid prompt","code":"invalid_request_error","type":"invalid_request_error"}}`)
	require.Equal(t, "invalid_request_error: Invalid prompt", sanitizeMediaUpstreamErrorMessage(400, openaiBody))

	// DashScope / MiniMax 顶层 code + message
	dashBody := []byte(`{"code":"InvalidParameter","message":"Field required: input.prompt"}`)
	require.Equal(t, "InvalidParameter: Field required: input.prompt", sanitizeMediaUpstreamErrorMessage(400, dashBody))

	// 非 JSON 空体：回退到状态码文案
	require.Contains(t, sanitizeMediaUpstreamErrorMessage(400, []byte("")), "400")

	// 非 JSON 长体：截断回退
	long := []byte("some raw text longer than two hundred fifty six characters xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")
	require.Contains(t, sanitizeMediaUpstreamErrorMessage(500, long), "upstream 500")
}
