package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQwenAudioTTSAdapterSupportsOnlyOfficialModels(t *testing.T) {
	qwen := NewQwenAudioTTSAdapter()
	for _, model := range qwenAudioTTSModels {
		require.True(t, qwen.Supports("qwen", model), model)
	}
	require.False(t, qwen.Supports("qwen", "qwen-audio-9.9-tts-flash"))
	require.False(t, qwen.Supports("qwen", "speech-2.8-hd"))

	// 回归：MiniMax 的宽泛 "tts" 匹配曾抢走 qwen 模型，且它注册在前面。
	minimax := NewMiniMaxTTSAdapter()
	require.False(t, minimax.Supports("minimax", "qwen-audio-3.1-tts-flash"))
	require.False(t, minimax.Supports("minimax", "qwen-tts"), "qwen-tts 应交回阿里 adapter")
	require.True(t, minimax.Supports("minimax", "tts"), "旧别名仍保留")
	require.True(t, NewAliyunTTSAdapter().Supports("aliyun", "qwen-tts"))
}

func TestBuildQwenAudioTTSBody(t *testing.T) {
	var out map[string]any
	require.NoError(t, json.Unmarshal(buildQwenAudioTTSBody(MediaCreateRequest{
		UpstreamModel: "qwen-audio-3.1-tts-flash",
		Prompt:        "你好",
		Extra: map[string]any{
			"voice":           "longwan_v3.1",
			"response_format": "mp3",
			"sample_rate":     32000,
			"instruction":     "请用上海话表达。",
			"request_id":      "secret-idempotency-key",
			"input":           "你好",
		},
	}), &out))
	require.Equal(t, "qwen-audio-3.1-tts-flash", out["model"])
	input, ok := out["input"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "你好", input["text"])
	require.Equal(t, "longwan_v3.1", input["voice"])
	require.Equal(t, "mp3", input["format"])
	require.Equal(t, float64(32000), input["sample_rate"])
	require.Equal(t, "请用上海话表达。", input["instruction"])
	require.NotContains(t, input, "request_id", "幂等键绝不下发上游")
	require.NotContains(t, input, "input")
}

func TestBuildQwenAudioTTSBodyDefaults(t *testing.T) {
	var out map[string]any
	require.NoError(t, json.Unmarshal(buildQwenAudioTTSBody(MediaCreateRequest{
		UpstreamModel: "qwen-audio-3.0-tts-plus",
		Prompt:        "hi",
	}), &out))
	input := out["input"].(map[string]any)
	require.Equal(t, qwenAudioTTSDefaultVoice, input["voice"])
	require.Equal(t, qwenAudioTTSDefaultFormat, input["format"])
	require.Equal(t, float64(24000), input["sample_rate"])
}

func TestParseQwenAudioTTSResultReturnsAudioURL(t *testing.T) {
	res, err := parseQwenAudioTTSResult([]byte(`{"output":{"audio":{"url":"https://cdn.test/a.wav"}},"request_id":"x"}`), http.StatusOK)
	require.NoError(t, err)
	require.Equal(t, "succeeded", res.Status)
	require.Equal(t, MediaCompletionSync, res.Mode)
	require.Equal(t, "https://cdn.test/a.wav", res.InlineURL)
}

func TestParseQwenAudioTTSResultBusinessFailureNoLeak(t *testing.T) {
	raw := []byte(`{"code":"InvalidParameter","message":"secret upstream detail","request_id":"x"}`)
	res, err := parseQwenAudioTTSResult(raw, http.StatusOK)
	require.NoError(t, err)
	require.Equal(t, "failed", res.Status)
	require.NotContains(t, res.ErrorMessage, "secret", "上游原文绝不能进终端错误文案")
	require.Contains(t, string(res.UpstreamRaw), "secret", "完整原文只留 UpstreamRaw 供 Ops 归档")
}

func TestParseQwenAudioTTSResultNoAudio(t *testing.T) {
	for _, body := range []string{`{"output":{}}`, `{"output":{"audio":{}}}`} {
		res, err := parseQwenAudioTTSResult([]byte(body), http.StatusOK)
		require.NoError(t, err, body)
		require.Equal(t, "failed", res.Status, body)
		require.Empty(t, res.InlineURL, body)
	}
}

func TestQwenAudioDownloadURLAllowed(t *testing.T) {
	require.True(t, qwenAudioDownloadURLAllowed("https://cdn.test/a.wav"))
	require.False(t, qwenAudioDownloadURLAllowed("http://cdn.test/a.wav"))
	require.False(t, qwenAudioDownloadURLAllowed("file:///etc/passwd"))
	require.False(t, qwenAudioDownloadURLAllowed(""))
}

func TestQwenAudioDownloadAudio(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fail" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("RIFFfake-wav"))
	}))
	defer srv.Close()

	a := NewQwenAudioTTSAdapter()
	a.httpClient = srv.Client()

	got, err := a.downloadAudio(context.Background(), srv.URL+"/a.wav")
	require.NoError(t, err)
	require.Equal(t, []byte("RIFFfake-wav"), got)

	_, err = a.downloadAudio(context.Background(), srv.URL+"/fail")
	require.Error(t, err, "下载失败必须报错，调用方据此判 failed 且不计费")
}
