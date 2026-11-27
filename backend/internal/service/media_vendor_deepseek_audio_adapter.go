package service

// media_vendor_deepseek_audio_adapter.go — DeepSeek 官方 TTS（/v1/tools/tts）。
//
// 官方契约与 OpenAI /v1/audio/speech 差异较大，不能靠通用兜底 adapter 走通
// （兜底会 POST {base}/v1/audio/speech 并下发 model/input/voice/response_format，
// 对 DeepSeek 是 404 + 字段不认）：
//  1. 路径是 {base}/v1/tools/tts（base_url 给到 https://api.deepseek.com）
//  2. 请求体**没有任何 model 字段**，待合成文本的字段名是 text（不是 input）
//  3. 音色字段是 voice_id，只有 stella（女声，默认）/ tide（男声）
//  4. 输出格式只有 wav（默认）/ pcm，没有 mp3；语言由文本自动识别
//  5. 响应是**流式音频字节**（audio/wav 或 audio/pcm），没有 JSON 包装
//  6. 计费按字符：一个汉字算 2 个字符，其它字符（字母/数字/空格）算 1 个
//     （官方 10,000 字符 $0.45，限时 $0.225），与其它厂商按秒/按时长计费不同
//
// 另有限制（本 adapter 明确不支持，与官方说明一致）：不支持语速调节、不支持
// 断点续传；单请求最多 10,000 计费字符 / 64 KiB 请求体 / 30 分钟音频；
// WebSocket 流式（wss://.../v1/tools/tts）未接入，只支持同步 HTTP。

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"
)

const (
	// deepSeekTTSPath 是官方同步语音合成路径（相对 base_url，base_url 不带 /v1）。
	deepSeekTTSPath = "/v1/tools/tts"
	// deepSeekTTSDefaultVoice 是官方默认音色（女声）。
	deepSeekTTSDefaultVoice = "stella"
	// deepSeekTTSDefaultFormat 是官方默认容器格式。
	deepSeekTTSDefaultFormat = "wav"
	// deepSeekTTSMaxAudioBytes 是单次合成音频的读取上限：官方单请求最多 30 分钟，
	// 24kHz/16bit 单声道 PCM 约 86MB，留出余量后按 96MB 截断，防止异常响应打爆内存。
	deepSeekTTSMaxAudioBytes = 96 << 20
	// deepSeekTTSMaxTextBytes 是官方请求体硬上限（64 KiB），提前拦下可给出明确 400。
	deepSeekTTSMaxTextBytes = 64 << 10
	// deepSeekTTSTimeout 覆盖官方「单请求最多 30 分钟音频」的合成时长。
	deepSeekTTSTimeout = 20 * time.Minute
	// deepSeekTTSOfficialPricePerMillionChars 是官方目录价折算的「每百万计费字符」
	// 单价（USD）：10,000 字符 $0.45 → 45；限时 5 折为 22.5。
	// 分组未配置 audio_tts_price_per_million_chars 时用它兜底，避免落到通用默认 15
	// 造成系统性少收；分组显式配 0 仍表示免费。
	deepSeekTTSOfficialPricePerMillionChars = 45.0
)

// deepSeekTTSVoices 是官方音色白名单。
var deepSeekTTSVoices = []string{"stella", "tide"}

// deepSeekTTSFormats 是官方容器格式白名单。pcm 为 16bit 有符号单声道 24kHz 裸流。
var deepSeekTTSFormats = []string{"wav", "pcm"}

// DeepSeekTTSAdapter 接入 DeepSeek 官方 TTS。
// 复用 mediaVendorAudioAdapter 底座提供 Kind()/Supports()/GetResult()，
// 仅覆盖 Create() —— 官方响应是音频裸字节而非 JSON，且计费口径是按字符。
type DeepSeekTTSAdapter struct {
	mediaVendorAudioAdapter
	client *http.Client
}

// NewDeepSeekTTSAdapter 创建 DeepSeek 官方 TTS adapter。
func NewDeepSeekTTSAdapter() *DeepSeekTTSAdapter {
	a := &DeepSeekTTSAdapter{
		client: &http.Client{Timeout: deepSeekTTSTimeout},
	}
	a.mediaVendorAudioAdapter = newMediaVendorAudioAdapter("deepseek-tts")
	a.supports = func(platform, model string) bool {
		// 只按模型名归属，不校验平台：按仓库既有约定，媒体上游账号可能挂在
		// openai（最小配置）或 deepseek（国产承载）平台上，平台不影响协议。
		// 前缀匹配保证不会抢走 minimax-tts / volc-tts / qwen-tts 等其它厂商模型。
		return isDeepSeekTTSModel(model)
	}
	return a
}

// isDeepSeekTTSModel 报告模型名是否属于 DeepSeek 官方 TTS 系列。
// 官方该接口没有模型参数，这里的模型名只是中转站侧的「路由标签」，
// 用于选中本 adapter 与分组模型白名单，不会下发给上游。
func isDeepSeekTTSModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "" {
		return false
	}
	// 显式排除：deepseek 文本模型即便带 tts 子串也不应命中（防御未来命名）。
	if strings.Contains(m, "stt") || strings.Contains(m, "whisper") {
		return false
	}
	return strings.HasPrefix(m, "deepseek-tts") ||
		strings.HasPrefix(m, "deepseek-speech") ||
		strings.HasPrefix(m, "ds-tts")
}

// DeepSeekTTSBilledChars 按官方口径统计计费字符：汉字计 2，其它字符（字母、
// 数字、空格、标点、假名等）计 1。厂商的计费单价是「每 10,000 字符」。
//
// 只统计「汉字」而不是所有 CJK：官方原文为 "A Chinese character counts as 2
// characters; any other character ... counts as 1"，日文假名/韩文属于其它字符。
func DeepSeekTTSBilledChars(text string) int {
	chars := 0
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			chars += 2
			continue
		}
		chars++
	}
	return chars
}

// deepSeekTTSText 取出待合成文本：优先统一请求里的 prompt（/v1/audio/speech 的
// input 会解析到这里），其次兼容 DeepSeek 原生的 text 字段。
func deepSeekTTSText(req MediaCreateRequest) string {
	if text := strings.TrimSpace(req.Prompt); text != "" {
		return text
	}
	if req.Extra != nil {
		if text, ok := req.Extra["text"].(string); ok {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

// Create 覆盖底座实现：官方端点返回音频裸字节，需按字节读取并判定 MIME。
func (a *DeepSeekTTSAdapter) Create(ctx context.Context, account *Account, req MediaCreateRequest) (*MediaCreateResult, error) {
	if account == nil {
		return nil, fmt.Errorf("deepseek tts adapter: account is required")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(account.GetCredential("base_url")), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("deepseek tts adapter: account %d has no base_url", account.ID)
	}
	apiKey := strings.TrimSpace(account.GetCredential("api_key"))
	if apiKey == "" {
		return nil, fmt.Errorf("deepseek tts adapter: account %d has no api_key", account.ID)
	}

	text := deepSeekTTSText(req)
	if text == "" {
		// 音频链路允许「带参考音频」省略文本，但 DeepSeek 只有纯文本合成，
		// 空文本直接判失败，不打上游（官方会 400 无可读文本）。
		return &MediaCreateResult{
			Status: "failed", Mode: MediaCompletionFailed,
			// 本地校验失败必须带 400：状态码 0 会被上游故障分类判成「可切换」，
			// 一条空文本请求就会把分组内账号逐个排除一遍。
			UpstreamStatusCode: http.StatusBadRequest,
			ErrorMessage:       "deepseek tts requires non-empty text",
		}, nil
	}
	if len(text) > deepSeekTTSMaxTextBytes {
		return &MediaCreateResult{
			Status: "failed", Mode: MediaCompletionFailed,
			UpstreamStatusCode: http.StatusBadRequest,
			ErrorMessage:       "deepseek tts text exceeds 64KiB limit",
		}, nil
	}

	path := deepSeekTTSPath
	if override := strings.TrimSpace(account.GetCredential("video_create_path")); override != "" {
		path = normalizeVideoURLPath(override)
	}
	body := buildDeepSeekTTSBody(req, text)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("deepseek tts build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	// 响应是音频流，Accept 按音频声明；部分网关会据此决定是否走二进制分支。
	httpReq.Header.Set("Accept", "audio/*")
	account.ApplyHeaderOverrides(httpReq.Header)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("deepseek tts create request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// 上限 +1 用于判定「是否被截断」，截断一律判失败：官方明确「合成中途失败会
	// 直接截断响应」，把残缺音频当成功返回等于扣费给半段声音。
	audioBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, deepSeekTTSMaxAudioBytes+1))
	if readErr != nil {
		return nil, fmt.Errorf("deepseek tts read audio: %w", readErr)
	}
	if len(audioBytes) > deepSeekTTSMaxAudioBytes {
		return &MediaCreateResult{
			Status: "failed", Mode: MediaCompletionFailed, UpstreamStatusCode: resp.StatusCode,
			ErrorMessage: "deepseek tts response exceeded audio size limit",
		}, nil
	}
	return parseDeepSeekTTSResult(audioBytes, resp.StatusCode, resp.Header.Get("Content-Type"), text)
}

// buildDeepSeekTTSBody 把统一请求映射成官方字段，严格白名单下发：
//   - text            ← prompt / input / text
//   - voice_id        ← voice_id（原生优先）或 voice（OpenAI 风格）；仅 stella/tide
//   - format          ← format（原生）或 response_format（OpenAI 风格）；仅 wav/pcm
//   - filter_markdown ← 原样透传（缺省由上游按 true 处理）
//
// model / request_id / 其它 OpenAI 字段一律不下发：官方契约没有 model 参数，
// 多发的字段会成为 400 的来源。
func buildDeepSeekTTSBody(req MediaCreateRequest, text string) []byte {
	body := map[string]any{
		"text":     text,
		"voice_id": deepSeekTTSDefaultVoice,
		"format":   deepSeekTTSDefaultFormat,
	}
	if req.Extra != nil {
		if voice := deepSeekTTSVoice(req.Extra); voice != "" {
			body["voice_id"] = voice
		}
		if format := deepSeekTTSFormat(req.Extra); format != "" {
			body["format"] = format
		}
		if v, ok := req.Extra["filter_markdown"].(bool); ok {
			body["filter_markdown"] = v
		}
	}
	data, _ := json.Marshal(body)
	return data
}

// deepSeekTTSVoice 解析音色：非法值回退官方默认音色，避免整条请求被上游 400。
func deepSeekTTSVoice(extra map[string]any) string {
	candidates := []string{}
	for _, key := range []string{"voice_id", "voice"} {
		if v, ok := extra[key].(string); ok && strings.TrimSpace(v) != "" {
			candidates = append(candidates, strings.ToLower(strings.TrimSpace(v)))
		}
	}
	for _, candidate := range candidates {
		for _, allowed := range deepSeekTTSVoices {
			if candidate == allowed {
				return candidate
			}
		}
	}
	return ""
}

// deepSeekTTSFormat 解析容器格式：官方只支持 wav / pcm。
// OpenAI 客户端默认会传 mp3，这里回退 wav（响应头会如实声明 wav），
// 与千问 adapter「不在白名单就回退自身默认」的口径一致，避免直接 400。
func deepSeekTTSFormat(extra map[string]any) string {
	for _, key := range []string{"format", "response_format"} {
		v, ok := extra[key].(string)
		if !ok {
			continue
		}
		candidate := strings.ToLower(strings.TrimSpace(v))
		for _, allowed := range deepSeekTTSFormats {
			if candidate == allowed {
				return candidate
			}
		}
	}
	return ""
}

// parseDeepSeekTTSResult 归一化官方响应。
// 上游错误按状态码给统一文案，绝不回显原始报文；成功则以音频字节为唯一产物，
// 并按真实 MIME 声明 content type —— 上游没回 header 时用 WAV 魔数兜底。
func parseDeepSeekTTSResult(audioBytes []byte, statusCode int, contentType, text string) (*MediaCreateResult, error) {
	if statusCode >= 400 {
		return &MediaCreateResult{
			Status: "failed", Mode: MediaCompletionFailed, UpstreamStatusCode: statusCode,
			UpstreamRaw:  audioBytes,
			ErrorMessage: sanitizeMediaUpstreamErrorMessage(statusCode, audioBytes),
		}, nil
	}
	if len(audioBytes) == 0 {
		return &MediaCreateResult{
			Status: "failed", Mode: MediaCompletionFailed, UpstreamStatusCode: statusCode,
			ErrorMessage: "deepseek tts response contained no audio",
		}, nil
	}
	return &MediaCreateResult{
		Status:             "succeeded",
		Mode:               MediaCompletionSync,
		InlineBytes:        audioBytes,
		InlineContentType:  deepSeekTTSContentType(contentType, audioBytes),
		UpstreamStatusCode: statusCode,
		UpstreamRaw:        nil, // 音频字节即原文，不再重复留档（避免双倍内存）
		UpstreamEndpoint:   deepSeekTTSPath,
		BilledChars:        DeepSeekTTSBilledChars(text),
	}, nil
}

// deepSeekTTSContentType 决定返回给客户端的 MIME：优先上游 header（audio/wav、
// audio/pcm），缺失或非音频时按 WAV 魔数（"RIFF"…"WAVE"）判定，否则按裸 PCM。
func deepSeekTTSContentType(headerContentType string, audioBytes []byte) string {
	ct := strings.ToLower(strings.TrimSpace(strings.Split(headerContentType, ";")[0]))
	switch ct {
	case "audio/wav", "audio/x-wav", "audio/wave", "audio/pcm":
		return ct
	}
	if len(audioBytes) >= 12 && bytes.Equal(audioBytes[0:4], []byte("RIFF")) && bytes.Equal(audioBytes[8:12], []byte("WAVE")) {
		return "audio/wav"
	}
	return "audio/pcm"
}
