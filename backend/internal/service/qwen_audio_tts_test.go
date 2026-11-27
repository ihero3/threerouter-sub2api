package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQwenAudioTTSAdapterSupportsOfficialModels(t *testing.T) {
	qwen := NewQwenAudioTTSAdapter()
	for _, models := range [][]string{qwenAudioTTSModels, qwenAudioTTSNextModels, qwenCosyVoiceModels, qwen3TTSModels} {
		for _, model := range models {
			require.True(t, qwen.Supports("qwen", model), model)
			require.True(t, qwen.Supports("openai", model), "%s 在承载平台上同样命中", model)
		}
	}
	require.False(t, qwen.Supports("qwen", "qwen-audio-9.9-tts-flash"))
	require.False(t, qwen.Supports("qwen", "speech-2.8-hd"))
	require.False(t, qwen.Supports("qwen", "qwen3-tts-flash-realtime"), "实时模型不在同步 adapter 范围")

	// 回归：MiniMax 的宽泛 "tts" 匹配曾抢走 qwen 模型，且它注册在前面。
	minimax := NewMiniMaxTTSAdapter()
	require.False(t, minimax.Supports("minimax", "qwen-audio-3.1-tts-flash"))
	require.False(t, minimax.Supports("minimax", "qwen-tts"), "qwen-tts 应交回阿里 adapter")
	require.True(t, minimax.Supports("minimax", "tts"), "旧别名仍保留")
	require.True(t, NewAliyunTTSAdapter().Supports("aliyun", "qwen-tts"))
	require.True(t, NewAliyunTTSAdapter().Supports("aliyun", "dashscope-tts"))
	require.False(t, NewAliyunTTSAdapter().Supports("aliyun", "cosyvoice-v3-flash"), "CosyVoice 走千问 SpeechSynthesizer")
}

func TestQwenAudioTTSAdapterRegistryRoutesQwenFamilies(t *testing.T) {
	reg := NewMediaAdapterRegistry(
		NewMiniMaxTTSAdapter(),
		NewQwenAudioTTSAdapter(),
		NewAliyunTTSAdapter(),
		NewAudioMediaAdapter(),
	)
	for _, model := range []string{
		"qwen-audio-3.1-tts-flash", "qwen-audio-3.1-tts-next",
		"cosyvoice-v3.5-plus", "qwen3-tts-flash",
	} {
		adapter, err := reg.Resolve(MediaKindAudio, "openai", model)
		require.NoError(t, err, model)
		_, ok := adapter.(*QwenAudioTTSAdapter)
		require.True(t, ok, "%s 应命中 Qwen adapter，实际命中 %T", model, adapter)
	}
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

func TestBuildQwenTSBodyMapsOpenAISpeedToRate(t *testing.T) {
	var out map[string]any
	require.NoError(t, json.Unmarshal(buildQwenTTSBody(MediaCreateRequest{
		UpstreamModel: "qwen-audio-3.1-tts-plus",
		Prompt:        "慢一点",
		Extra:         map[string]any{"speed": 0.8},
	}), &out))
	input := out["input"].(map[string]any)
	require.Equal(t, 0.8, input["rate"], "OpenAI speed 应映射为千问 rate")
	require.NotContains(t, input, "speed")
}

func TestBuildQwenCosyVoiceBodyUsesCosyVoiceDefaultVoice(t *testing.T) {
	var out map[string]any
	require.NoError(t, json.Unmarshal(buildQwenTTSBody(MediaCreateRequest{
		UpstreamModel: "cosyvoice-v3-flash",
		Prompt:        "你好",
	}), &out))
	input := out["input"].(map[string]any)
	require.Equal(t, "你好", input["text"])
	require.Equal(t, qwenCosyVoiceDefaultVoice, input["voice"])
	require.Equal(t, qwenAudioTTSDefaultFormat, input["format"])
}

func TestBuildQwenAudioTTSNextBody(t *testing.T) {
	var out map[string]any
	require.NoError(t, json.Unmarshal(buildQwenAudioTTSNextBody(MediaCreateRequest{
		UpstreamModel: "qwen-audio-3.1-tts-next",
		Prompt:        "@voice1 说：“你好。”@voice2 回答：“你好呀。”",
		AudioRefURLs:  []string{"https://cdn.test/v1.wav", "https://cdn.test/v2.wav"},
		Extra: map[string]any{
			"voice":           "should-not-pass",
			"response_format": "mp3",
			"speed":           1.2,
			"channels":        1,
			"request_id":      "secret-idempotency-key",
		},
	}), &out))
	require.Equal(t, "qwen-audio-3.1-tts-next", out["model"])
	input := out["input"].(map[string]any)
	require.Equal(t, "@voice1 说：“你好。”@voice2 回答：“你好呀。”", input["text_prompt"])
	require.Equal(t, "mp3", input["format"])
	require.Equal(t, float64(48000), input["sample_rate"])
	require.Equal(t, float64(1), input["channels"])
	require.Equal(t, 1.2, input["rate"])
	require.NotContains(t, input, "voice", "tts-next 不接受系统音色，参考音频走 references")
	require.NotContains(t, input, "request_id")

	refs, ok := input["references"].([]any)
	require.True(t, ok)
	require.Len(t, refs, 2)
	require.Equal(t, "https://cdn.test/v1.wav", refs[0].(map[string]any)["audio_url"])
	require.Equal(t, "https://cdn.test/v2.wav", refs[1].(map[string]any)["audio_url"])
}

func TestBuildQwenAudioTTSNextBodyNativeReferences(t *testing.T) {
	var out map[string]any
	require.NoError(t, json.Unmarshal(buildQwenAudioTTSNextBody(MediaCreateRequest{
		UpstreamModel: "qwen-audio-3.1-tts-next",
		Prompt:        "@voice1 说：你好。",
		Extra: map[string]any{
			"references": []any{
				map[string]any{"audio_data": "data:audio/wav;base64,AAAA"},
				map[string]any{"audio_url": "https://cdn.test/v2.mp3", "audio_data": "data:audio/wav;base64,BBBB"},
			},
		},
	}), &out))
	input := out["input"].(map[string]any)
	refs := input["references"].([]any)
	require.Len(t, refs, 2)
	require.Equal(t, "data:audio/wav;base64,AAAA", refs[0].(map[string]any)["audio_data"])
	second := refs[1].(map[string]any)
	require.Equal(t, "https://cdn.test/v2.mp3", second["audio_url"])
	require.NotContains(t, second, "audio_data", "URL 与 Base64 二选一，URL 优先")
}

func TestBuildQwen3TTSBody(t *testing.T) {
	var out map[string]any
	require.NoError(t, json.Unmarshal(buildQwen3TTSBody(MediaCreateRequest{
		UpstreamModel: "qwen3-tts-flash",
		Prompt:        "你好",
		Extra: map[string]any{
			"language_type":   "Chinese",
			"instructions":    "温柔一点",
			"response_format": "mp3",
			"speed":           1.1,
			"request_id":      "secret-idempotency-key",
		},
	}), &out))
	input := out["input"].(map[string]any)
	require.Equal(t, "你好", input["text"])
	require.Equal(t, qwen3TTSDefaultVoice, input["voice"])
	require.Equal(t, "Chinese", input["language_type"])
	require.Equal(t, "温柔一点", input["instructions"])
	require.NotContains(t, input, "format", "该端点不接受 format")
	require.NotContains(t, input, "speed")
	require.NotContains(t, input, "request_id")
}

func TestQwenAudioContentType(t *testing.T) {
	require.Equal(t, "audio/wav", qwenAudioContentType("wav", "https://cdn.test/a.wav?x=1"))
	require.Equal(t, "audio/mpeg", qwenAudioContentType("mp3", ""))
	require.Equal(t, "audio/pcm", qwenAudioContentType("pcm", ""))
	require.Equal(t, "audio/ogg", qwenAudioContentType("opus", ""))
	require.Equal(t, "audio/wav", qwenAudioContentType("wav", "https://cdn.test/no-ext"))
	require.Equal(t, "mp3", qwenAudioOutputFormat(&MediaCreateRequest{
		UpstreamModel: "qwen-audio-3.1-tts-flash",
		Extra:         map[string]any{"response_format": "mp3"},
	}))
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
	require.True(t, qwenAudioDownloadURLAllowed("http://dashscope-result-bj.oss.test/a.wav"), "文档示例中存在 http CDN")
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

func TestQwenAudioTTSCreateRoutesFamilyAndSetsContentType(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte("RIFFfake-wav"))
			return
		}
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"request_id":"x","output":{"audio":{"url":"` + srvURL(r) + `/audio.wav","duration":1.2}}}`))
	}))
	defer srv.Close()

	a := NewQwenAudioTTSAdapter()
	a.httpClient = srv.Client()
	account := &Account{ID: 1, Platform: "openai", Credentials: map[string]any{
		"base_url": srv.URL,
		"api_key":  "test-key",
	}}

	// Qwen-Audio-TTS：SpeechSynthesizer 端点。
	result, err := a.Create(context.Background(), account, MediaCreateRequest{
		UpstreamModel: "qwen-audio-3.1-tts-flash",
		Prompt:        "你好",
	})
	require.NoError(t, err)
	require.Equal(t, "succeeded", result.Status)
	require.Equal(t, "/api/v1/services/audio/tts/SpeechSynthesizer", gotPath)
	require.Equal(t, "你好", gotBody["input"].(map[string]any)["text"])
	require.Equal(t, []byte("RIFFfake-wav"), result.InlineBytes)
	require.Equal(t, "audio/wav", result.InlineContentType)
	require.Empty(t, result.InlineURL, "临时 URL 不能透传给客户端")

	// Qwen3-TTS：multimodal-generation 端点，字段名仍是 input.text。
	result, err = a.Create(context.Background(), account, MediaCreateRequest{
		UpstreamModel: "qwen3-tts-flash",
		Prompt:        "你好",
	})
	require.NoError(t, err)
	require.Equal(t, "succeeded", result.Status)
	require.Equal(t, "/api/v1/services/aigc/multimodal-generation/generation", gotPath)
	require.Equal(t, "你好", gotBody["input"].(map[string]any)["text"])
}

// srvURL 从请求反推测试服务器地址，避免在 handler 里引用尚未赋值的 srv 变量。
func srvURL(r *http.Request) string {
	return "https://" + r.Host
}

func TestParseMediaCreateRequestAcceptsQwenNativeAudioFields(t *testing.T) {
	// 千问非实时 TTS 的原生形状：input.text。
	req, err := parseMediaCreateRequest(MediaKindAudio, "qwen-audio-3.1-tts-flash", map[string]any{
		"model": "qwen-audio-3.1-tts-flash",
		"input": map[string]any{"text": "你好", "voice": "longanhuan_v3.1"},
	})
	require.NoError(t, err)
	require.Equal(t, "你好", req.Prompt)

	// tts-next 的原生形状：input.text_prompt。
	req, err = parseMediaCreateRequest(MediaKindAudio, "qwen-audio-3.1-tts-next", map[string]any{
		"model": "qwen-audio-3.1-tts-next",
		"input": map[string]any{
			"text_prompt": "@voice1 说：你好。",
			"references":  []any{map[string]any{"audio_url": "https://cdn.test/v1.wav"}},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "@voice1 说：你好。", req.Prompt)
	require.Len(t, req.Extra["references"].([]any), 1)
}
