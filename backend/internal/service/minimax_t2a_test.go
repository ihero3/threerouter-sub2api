package service

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// MiniMax T2A v2 官方模型名不含 "tts" / "speech-01"，旧口径会漏判并落到
// fallback 打 /v1/audio/speech。这里锁住所有官方模型与历史别名。
func TestMiniMaxTTSAdapterSupportsOfficialSpeechModels(t *testing.T) {
	a := NewMiniMaxTTSAdapter()
	for _, model := range []string{
		"speech-2.8-hd", "speech-2.8-turbo",
		"speech-2.6-hd", "speech-2.6-turbo",
		"speech-02-hd", "speech-02-turbo",
		"speech-01-hd", "speech-01-turbo",
		"minimax-tts", "speech-01", "tts",
	} {
		if !a.Supports("minimax", model) {
			t.Errorf("Supports(minimax, %q) = false, want true", model)
		}
	}
}

func decodeMiniMaxTTSBody(t *testing.T, req MediaCreateRequest) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal(buildMiniMaxTTSBody(req), &out))
	return out
}

func TestBuildMiniMaxTTSBodyMapsOpenAIParams(t *testing.T) {
	out := decodeMiniMaxTTSBody(t, MediaCreateRequest{
		UpstreamModel: "speech-2.8-hd",
		Prompt:        "你好，世界",
		Extra: map[string]any{
			"input":                   "你好，世界",
			"voice":                   "male-qn-qingse",
			"speed":                   1.2,
			"response_format":         "flac",
			"language_boost":          "auto",
			"text_normalization_mode": "quality",
			"request_id":              "secret-idempotency-key",
			"stream":                  true,
			"keep-me":                 "passthrough",
		},
	})
	require.Equal(t, "speech-2.8-hd", out["model"])
	require.Equal(t, "你好，世界", out["text"])
	require.Equal(t, false, out["stream"], "同步端点必须强制 stream=false")

	voice, ok := out["voice_setting"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "male-qn-qingse", voice["voice_id"])
	require.Equal(t, 1.2, voice["speed"])

	audio, ok := out["audio_setting"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "flac", audio["format"])

	// 原生字段与未识别字段都要继续透传，不能把排除逻辑收窄成白名单。
	require.Equal(t, "auto", out["language_boost"])
	require.Equal(t, "quality", out["text_normalization_mode"])
	require.Equal(t, "passthrough", out["keep-me"])

	for _, key := range []string{"input", "voice", "response_format", "request_id", "speed"} {
		require.NotContains(t, out, key, "OpenAI 风格字段不应原样下发: %s", key)
	}
}

func TestBuildMiniMaxTTSBodyNativeSettingsWin(t *testing.T) {
	out := decodeMiniMaxTTSBody(t, MediaCreateRequest{
		UpstreamModel: "speech-2.6-turbo",
		Prompt:        "test",
		Extra: map[string]any{
			"voice":           "ignored-if-native-set",
			"voice_setting":   map[string]any{"voice_id": "native-voice", "emotion": "happy"},
			"audio_setting":   map[string]any{"format": "mp3", "sample_rate": 32000},
			"response_format": "flac",
		},
	})
	voice := out["voice_setting"].(map[string]any)
	require.Equal(t, "native-voice", voice["voice_id"], "客户端显式给的 voice_setting 不应被 OpenAI voice 覆盖")
	require.Equal(t, "happy", voice["emotion"])
	audio := out["audio_setting"].(map[string]any)
	require.Equal(t, "mp3", audio["format"], "客户端显式给的 audio_setting 不应被 response_format 覆盖")
	require.NotContains(t, out, "voice")
}

// t2a_v2 默认 output_format=hex，data.audio 是 hex 编码音频字节，不是 URL。
func TestParseMiniMaxTTSResultHexAudio(t *testing.T) {
	raw := []byte(`{"data":{"audio":"` + hex.EncodeToString([]byte("ID3fake-mp3")) + `","status":2},"base_resp":{"status_code":0,"status_msg":"success"}}`)
	res, err := parseMiniMaxTTSResult(raw, http.StatusOK)
	require.NoError(t, err)
	require.Equal(t, "succeeded", res.Status)
	require.Equal(t, MediaCompletionSync, res.Mode)
	require.Equal(t, []byte("ID3fake-mp3"), res.InlineBytes)
	require.Empty(t, res.InlineURL)
	require.Equal(t, raw, res.UpstreamRaw)
}

func TestParseMiniMaxTTSResultURLCompat(t *testing.T) {
	res, err := parseMiniMaxTTSResult([]byte(`{"data":{"audio":"https://cdn.test/a.mp3"}}`), http.StatusOK)
	require.NoError(t, err)
	require.Equal(t, "succeeded", res.Status)
	require.Equal(t, "https://cdn.test/a.mp3", res.InlineURL)
	require.Empty(t, res.InlineBytes)
}

func TestParseMiniMaxTTSResultBusinessFailureDoesNotLeakUpstream(t *testing.T) {
	raw := []byte(`{"data":null,"base_resp":{"status_code":1002,"status_msg":"insufficient balance secret"}}`)
	res, err := parseMiniMaxTTSResult(raw, http.StatusOK)
	require.NoError(t, err)
	require.Equal(t, "failed", res.Status)
	require.Equal(t, MediaCompletionFailed, res.Mode)
	require.Empty(t, res.InlineBytes)
	require.NotContains(t, res.ErrorMessage, "insufficient", "上游原文绝不能进终端错误文案")
	require.Contains(t, string(res.UpstreamRaw), "insufficient", "完整原文只留在 UpstreamRaw 供 Ops 归档")
}

// 成功 = 状态成功 AND 产物非空：succeeded 却给不出音频会立刻真实扣费。
func TestParseMiniMaxTTSResultRejectsMissingOrInvalidAudio(t *testing.T) {
	for _, body := range []string{
		`{"data":{}}`,
		`{"data":{"audio":""}}`,
		`{"data":{"audio":"not-hex-and-not-url"}}`,
	} {
		res, err := parseMiniMaxTTSResult([]byte(body), http.StatusOK)
		require.NoError(t, err, body)
		require.Equal(t, "failed", res.Status, body)
		require.Empty(t, res.InlineBytes, body)
		require.Empty(t, res.InlineURL, body)
	}
}

func TestParseMediaCreateRequestAudioStringInput(t *testing.T) {
	audioReq, err := parseMediaCreateRequest(MediaKindAudio, "speech-2.8-hd", map[string]any{"input": "你好，世界"})
	require.NoError(t, err)
	require.Equal(t, "你好，世界", audioReq.Prompt)

	imageReq, err := parseMediaCreateRequest(MediaKindImage, "image-01", map[string]any{"input": "你好，世界"})
	require.NoError(t, err)
	require.Empty(t, imageReq.Prompt, "字符串 input 只在音频链路生效，不能改变图片/视频口径")
}
