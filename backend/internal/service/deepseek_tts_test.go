package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// deepseek_tts_test.go — DeepSeek 官方 TTS（/v1/tools/tts）接入回归。
//
// 该厂商与其它音频渠道的差异全部落在这些用例上：无 model 字段、text 而非 input、
// voice_id 而非 voice、只有 wav/pcm、响应是音频裸字节、按字符计费。

func TestDeepSeekTTSAdapterSupportsOnlyOwnModels(t *testing.T) {
	adapter := NewDeepSeekTTSAdapter()
	require.True(t, adapter.Supports("deepseek", "deepseek-tts"))
	require.True(t, adapter.Supports("openai", "deepseek-tts"), "承载平台不影响协议归属")
	require.True(t, adapter.Supports("deepseek", "ds-tts"))
	require.True(t, adapter.Supports("deepseek", "deepseek-speech-1"))

	// 绝不抢其它厂商的 tts 模型：这些模型名同样会挂在 deepseek 承载平台上。
	for _, model := range []string{"minimax-tts", "volc-tts", "qwen-tts", "cosyvoice-v2", "speech-2.8-hd", "tts-1"} {
		require.False(t, adapter.Supports("deepseek", model), model)
	}
	require.False(t, adapter.Supports("deepseek", "deepseek-chat"), "文本模型不能进音频适配器")
	require.False(t, adapter.Supports("deepseek", "deepseek-stt"), "stt 不属于本 adapter")
}

// TestMediaAdapterRegistryRoutesDeepSeekTTS 守住注册顺序：通用兜底 adapter 在列表
// 最后，DeepSeek 必须命中的是它自己的 adapter，否则会打错端点（/v1/audio/speech）。
func TestMediaAdapterRegistryRoutesDeepSeekTTS(t *testing.T) {
	reg := NewMediaAdapterRegistry(
		NewMiniMaxTTSAdapter(),
		NewQwenAudioTTSAdapter(),
		NewDeepSeekTTSAdapter(),
		NewAudioMediaAdapter(),
	)
	adapter, err := reg.Resolve(MediaKindAudio, PlatformDeepseek, "deepseek-tts")
	require.NoError(t, err)
	_, isDeepSeek := adapter.(*DeepSeekTTSAdapter)
	require.True(t, isDeepSeek, "deepseek-tts 必须命中 DeepSeek adapter，实际命中 %T", adapter)

	// 其它厂商模型仍归各自 adapter（不能被前缀相近的规则抢走）。
	other, err := reg.Resolve(MediaKindAudio, PlatformDeepseek, "minimax-tts")
	require.NoError(t, err)
	_, isMiniMax := other.(*MiniMaxTTSAdapter)
	require.True(t, isMiniMax, "minimax-tts 应归 MiniMax adapter，实际命中 %T", other)
}

func TestBuildDeepSeekTTSBodyUsesOfficialFieldNames(t *testing.T) {
	var out map[string]any
	require.NoError(t, json.Unmarshal(buildDeepSeekTTSBody(MediaCreateRequest{
		UpstreamModel: "deepseek-tts",
		Prompt:        "你好世界",
		Extra: map[string]any{
			"voice":           "tide",
			"response_format": "pcm",
			"filter_markdown": false,
			"model":           "deepseek-tts",
			"request_id":      "secret-idempotency-key",
			"input":           "你好世界",
		},
	}, "你好世界"), &out))

	require.Equal(t, "你好世界", out["text"], "文本字段名必须是 text")
	require.Equal(t, "tide", out["voice_id"], "voice -> voice_id")
	require.Equal(t, "pcm", out["format"], "response_format -> format")
	require.Equal(t, false, out["filter_markdown"])
	require.NotContains(t, out, "model", "官方契约没有 model 字段")
	require.NotContains(t, out, "input", "不能把 OpenAI 的 input 透传给上游")
	require.NotContains(t, out, "voice", "不能透传 OpenAI 的 voice")
	require.NotContains(t, out, "response_format")
	require.NotContains(t, out, "request_id", "幂等键绝不下发上游")
}

func TestBuildDeepSeekTTSBodyDefaultsAndFallbacks(t *testing.T) {
	// 无任何可选参数 → 官方默认值。
	var out map[string]any
	require.NoError(t, json.Unmarshal(buildDeepSeekTTSBody(MediaCreateRequest{Prompt: "hi"}, "hi"), &out))
	require.Equal(t, deepSeekTTSDefaultVoice, out["voice_id"])
	require.Equal(t, deepSeekTTSDefaultFormat, out["format"])
	require.NotContains(t, out, "filter_markdown")

	// OpenAI 客户端默认传 mp3 / 未知音色：回退 wav + 默认音色，不让整条请求 400。
	out = nil
	require.NoError(t, json.Unmarshal(buildDeepSeekTTSBody(MediaCreateRequest{
		Prompt: "hi",
		Extra:  map[string]any{"response_format": "mp3", "voice": "alloy"},
	}, "hi"), &out))
	require.Equal(t, deepSeekTTSDefaultFormat, out["format"])
	require.Equal(t, deepSeekTTSDefaultVoice, out["voice_id"])
}

func TestDeepSeekTTSBilledChars(t *testing.T) {
	// 官方口径：汉字 2，其它字符（字母/数字/空格/标点/假名）1。
	require.Equal(t, 0, DeepSeekTTSBilledChars(""))
	require.Equal(t, 4, DeepSeekTTSBilledChars("你好"))
	require.Equal(t, 5, DeepSeekTTSBilledChars("hello"))
	require.Equal(t, 8, DeepSeekTTSBilledChars("你好 hi!"))
	require.Equal(t, 2, DeepSeekTTSBilledChars("あい")) // 假名属「其它字符」
}

func TestParseDeepSeekTTSResultWavBytes(t *testing.T) {
	wav := append([]byte("RIFF\x00\x00\x00\x00WAVEfmt "), make([]byte, 32)...)
	res, err := parseDeepSeekTTSResult(wav, http.StatusOK, "audio/wav", "你好")
	require.NoError(t, err)
	require.Equal(t, "succeeded", res.Status)
	require.Equal(t, MediaCompletionSync, res.Mode)
	require.Equal(t, wav, res.InlineBytes)
	require.Equal(t, "audio/wav", res.InlineContentType)
	require.Equal(t, 4, res.BilledChars)
	require.Empty(t, res.UpstreamRaw, "音频字节不再重复留档")
}

func TestParseDeepSeekTTSResultSniffsContentType(t *testing.T) {
	// 上游没给 header：靠 WAV 魔数识别。
	wav := append([]byte("RIFF\x00\x00\x00\x00WAVEfmt "), make([]byte, 8)...)
	res, err := parseDeepSeekTTSResult(wav, http.StatusOK, "", "hi")
	require.NoError(t, err)
	require.Equal(t, "audio/wav", res.InlineContentType)

	// 裸 PCM（无容器）→ audio/pcm。
	res, err = parseDeepSeekTTSResult([]byte{0x01, 0x02, 0x03, 0x04}, http.StatusOK, "", "hi")
	require.NoError(t, err)
	require.Equal(t, "audio/pcm", res.InlineContentType)
}

func TestParseDeepSeekTTSResultFailures(t *testing.T) {
	// 空响应 = 上游给了空壳，绝不能按 succeeded 结算。
	res, err := parseDeepSeekTTSResult(nil, http.StatusOK, "audio/wav", "hi")
	require.NoError(t, err)
	require.Equal(t, "failed", res.Status)

	// 4xx/5xx 错误体：只回统一文案，不回显上游原文。
	raw := []byte(`{"error":{"message":"secret upstream detail"}}`)
	res, err = parseDeepSeekTTSResult(raw, http.StatusBadRequest, "application/json", "hi")
	require.NoError(t, err)
	require.Equal(t, "failed", res.Status)
	require.Equal(t, http.StatusBadRequest, res.UpstreamStatusCode)
	require.NotContains(t, res.ErrorMessage, "secret")
}

// TestDeepSeekTTSAdapterCreatePostsOfficialWire 端到端校验 wire 格式：
// 路径、鉴权头、请求体字段，以及音频字节 + MIME 的返回。
func TestDeepSeekTTSAdapterCreatePostsOfficialWire(t *testing.T) {
	wav := append([]byte("RIFF\x00\x00\x00\x00WAVEfmt "), make([]byte, 16)...)
	var gotPath, gotAuth, gotCT string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "audio/wav")
		_, _ = w.Write(wav)
	}))
	defer server.Close()

	account := &Account{ID: 7, Platform: PlatformDeepseek, Credentials: map[string]any{
		"base_url": server.URL,
		"api_key":  "sk-test",
	}}
	adapter := NewDeepSeekTTSAdapter()
	res, err := adapter.Create(context.Background(), account, MediaCreateRequest{
		PublicModel:   "deepseek-tts",
		UpstreamModel: "deepseek-tts",
		Prompt:        "你好",
	})
	require.NoError(t, err)

	require.Equal(t, "/v1/tools/tts", gotPath, "必须打官方 /v1/tools/tts")
	require.Equal(t, "Bearer sk-test", gotAuth)
	require.Equal(t, "application/json", gotCT)
	require.Equal(t, "你好", gotBody["text"])
	require.NotContains(t, gotBody, "model")

	require.Equal(t, "succeeded", res.Status)
	require.Equal(t, wav, res.InlineBytes)
	require.Equal(t, "audio/wav", res.InlineContentType)
	require.Equal(t, 4, res.BilledChars)
}

func TestDeepSeekTTSAdapterCreateRejectsEmptyText(t *testing.T) {
	adapter := NewDeepSeekTTSAdapter()
	account := &Account{ID: 1, Platform: PlatformDeepseek, Credentials: map[string]any{
		"base_url": "https://api.deepseek.com",
		"api_key":  "sk-test",
	}}
	res, err := adapter.Create(context.Background(), account, MediaCreateRequest{PublicModel: "deepseek-tts"})
	require.NoError(t, err)
	require.Equal(t, "failed", res.Status)
	require.Contains(t, strings.ToLower(res.ErrorMessage), "text")
	// 本地校验失败必须带 400，否则会被判成「可切换上游故障」把账号逐个排除。
	require.Equal(t, http.StatusBadRequest, res.UpstreamStatusCode)
	require.False(t, classifyMediaUpstreamFailure(res.UpstreamStatusCode, nil).ShouldFailover)
}

// TestDeepSeekTTSAdapterCreateUpstreamErrorNoLeak 上游 4xx 判失败且不泄露原文。
func TestDeepSeekTTSAdapterCreateUpstreamErrorNoLeak(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Content Exists Risk: secret"}}`))
	}))
	defer server.Close()

	account := &Account{ID: 2, Platform: PlatformDeepseek, Credentials: map[string]any{
		"base_url": server.URL,
		"api_key":  "sk-test",
	}}
	res, err := NewDeepSeekTTSAdapter().Create(context.Background(), account, MediaCreateRequest{
		PublicModel: "deepseek-tts", Prompt: "hi",
	})
	require.NoError(t, err)
	require.Equal(t, "failed", res.Status)
	require.Equal(t, http.StatusBadRequest, res.UpstreamStatusCode)
	require.NotContains(t, res.ErrorMessage, "secret")
}

// TestCalculateAudioTaskCostBreakdownPerChar 按字符计费走分组「每百万字符」价，
// 且与时长无关——按时长折算会把 DeepSeek 的计价单位算错几十倍。
func TestCalculateAudioTaskCostBreakdownPerChar(t *testing.T) {
	perMillionChars := 45.0
	perSec := 0.02
	group := &Group{ID: 1, RateMultiplier: 1}
	group.AudioTTSPricePerMillionChars = &perMillionChars
	group.AudioPricePerSec = &perSec
	groupID := int64(1)
	apiKey := &APIKey{ID: 1, GroupID: &groupID, Group: group}
	billing := NewBillingService(nil, nil)
	ctx := context.Background()

	// 500,000 计费字符 → 0.5 × 45 = 22.5
	cost, _ := calculateAudioTaskCostBreakdown(ctx, billing, nil, apiKey, "deepseek-tts", 600, 600, 500_000)
	require.NotNil(t, cost)
	require.InDelta(t, 22.5, cost.ActualCost, 1e-9)

	// 同一字符数下时长翻倍不应改变费用。
	longer, _ := calculateAudioTaskCostBreakdown(ctx, billing, nil, apiKey, "deepseek-tts", 1200, 1200, 500_000)
	require.InDelta(t, cost.ActualCost, longer.ActualCost, 1e-9)

	// 字符数 0 → 回到按秒口径，行为不变。
	perSecCost, _ := calculateAudioTaskCostBreakdown(ctx, billing, nil, apiKey, "deepseek-tts", 30, 30, 0)
	require.InDelta(t, 30*perSec, perSecCost.ActualCost, 1e-9)
}

// TestCalculateAudioTaskCostBreakdownDeepSeekVendorDefault 分组没配 TTS 字符价时，
// DeepSeek TTS 必须按官方价（45/百万字符）兜底，而不是通用默认 15：后者会让
// 每一笔调用只收到官方价的三分之一。显式配 0 仍表示免费。
func TestCalculateAudioTaskCostBreakdownDeepSeekVendorDefault(t *testing.T) {
	group := &Group{ID: 1, RateMultiplier: 1}
	groupID := int64(1)
	apiKey := &APIKey{ID: 1, GroupID: &groupID, Group: group}
	billing := NewBillingService(nil, nil)
	ctx := context.Background()

	// 100,000 字符 × 45 / 1e6 = 4.5
	cost, _ := calculateAudioTaskCostBreakdown(ctx, billing, nil, apiKey, "deepseek-tts", 0, 0, 100_000)
	require.NotNil(t, cost)
	require.InDelta(t, 4.5, cost.ActualCost, 1e-9)

	// 其它厂商（模型名不命中）仍走通用默认 15，不受影响。
	other, _ := calculateAudioTaskCostBreakdown(ctx, billing, nil, apiKey, "some-other-tts", 0, 0, 100_000)
	require.InDelta(t, 1.5, other.ActualCost, 1e-9)

	// 显式配 0 = 免费（运营主动把该渠道做成免费）。
	zero := 0.0
	group.AudioTTSPricePerMillionChars = &zero
	free, _ := calculateAudioTaskCostBreakdown(ctx, billing, nil, apiKey, "deepseek-tts", 0, 0, 100_000)
	require.InDelta(t, 0, free.ActualCost, 1e-12)
}
