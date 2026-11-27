package service

// media_vendor_qwen_audio_adapter.go — 千问（Qwen）非实时语音合成 HTTP 接入。
//
// 覆盖千问平台文档里的四个 TTS 家族（均为同步 HTTP，非 WebSocket 实时链路）：
//
//  1. Qwen-Audio-TTS：qwen-audio-3.1/3.0-tts-{plus,flash}
//     端点 /api/v1/services/audio/tts/SpeechSynthesizer，
//     入参 input.text + input.voice + format / sample_rate / instruction / language_hints。
//  2. CosyVoice：cosyvoice-v3.5-{plus,flash}、cosyvoice-v3-{plus,flash}、cosyvoice-v2
//     与 Qwen-Audio-TTS 同一端点、同一入参形状，默认音色 longanyang。
//  3. 统一音频生成（Audiogen）：qwen-audio-3.1-tts-next
//     同一端点，但入参是 input.text_prompt，可带 input.references（@voice1..3）
//     一次生成多人对话 / 播客 / 音效；不再接受 input.text。
//  4. Qwen3-TTS：qwen3-tts-flash
//     端点 /api/v1/services/aigc/multimodal-generation/generation，
//     入参 input.text + input.voice（默认 Cherry）+ language_type / instructions。
//
// 四者都同步返回临时音频 URL（output.audio.url），本 adapter 负责下载成字节，
// 交给 /v1/audio/speech 直返，避免把 24 小时有效的上游签名 URL 暴露给客户端。
// 下载失败一律判 failed：音频是同步终态，succeeded 却无产物会导致「扣费没声音」。

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// qwenAudioTTSModels 是 Qwen-Audio-TTS 官方非实时模型（qwen-audio-3.1/3.0）。
var qwenAudioTTSModels = []string{
	"qwen-audio-3.1-tts-flash", "qwen-audio-3.1-tts-plus",
	"qwen-audio-3.0-tts-flash", "qwen-audio-3.0-tts-plus",
}

// qwenAudioTTSNextModels 是统一音频生成（Audiogen）模型。
var qwenAudioTTSNextModels = []string{
	"qwen-audio-3.1-tts-next",
}

// qwenCosyVoiceModels 是 CosyVoice 非实时模型（与 Qwen-Audio-TTS 同端点）。
var qwenCosyVoiceModels = []string{
	"cosyvoice-v3.5-plus", "cosyvoice-v3.5-flash",
	"cosyvoice-v3-plus", "cosyvoice-v3-flash",
	"cosyvoice-v2",
}

// qwen3TTSModels 是 Qwen3-TTS 非实时模型（走 multimodal-generation 端点）。
// qwen3-tts-flash-realtime 只支持 WebSocket 实时链路，不在本 adapter 范围内。
var qwen3TTSModels = []string{
	"qwen3-tts-flash",
}

const (
	// SpeechSynthesizer 是 Qwen-Audio-TTS / CosyVoice / tts-next 共用的同步合成端点。
	qwenAudioTTSCreatePath = "/api/v1/services/audio/tts/SpeechSynthesizer"
	// multimodal-generation 是 Qwen3-TTS 非实时合成端点。
	qwen3TTSCreatePath = "/api/v1/services/aigc/multimodal-generation/generation"

	qwenAudioTTSDefaultVoice          = "longanhuan_v3.1"
	qwenCosyVoiceDefaultVoice         = "longanyang"
	qwen3TTSDefaultVoice              = "Cherry"
	qwenAudioTTSDefaultFormat         = "wav"
	qwenAudioTTSDefaultSampleRate     = 24000
	qwenAudioTTSNextDefaultSampleRate = 48000
	qwenAudioTTSNextDefaultChannels   = 2

	// tts-next 播客场景单次最长 240 秒，加上排队与下载需要留足余量。
	qwenAudioTTSCreateTimeout = 300 * time.Second
	qwenAudioTTSDownloadTO    = 120 * time.Second
	qwenAudioTTSMaxAudioBytes = 64 << 20 // 240s/48kHz/双声道 wav 约 46MB，留出余量
)

// qwenAudioTTSFormatAllowList 是 Qwen-Audio-TTS / CosyVoice 可用的输出格式。
var qwenAudioTTSFormatAllowList = map[string]struct{}{
	"wav": {}, "mp3": {}, "pcm": {}, "opus": {},
}

// qwenAudioTTSNextFormatAllowList 是 tts-next 可用的输出格式（不支持 Opus 输出）。
var qwenAudioTTSNextFormatAllowList = map[string]struct{}{
	"wav": {}, "mp3": {}, "pcm": {},
}

type QwenAudioTTSAdapter struct{ mediaVendorAudioAdapter }

func NewQwenAudioTTSAdapter() *QwenAudioTTSAdapter {
	a := &QwenAudioTTSAdapter{}
	a.mediaVendorAudioAdapter = newMediaVendorAudioAdapter("qwen-audio-tts")
	a.httpClient.Timeout = qwenAudioTTSCreateTimeout
	a.supports = func(platform, model string) bool {
		return isQwenTTSModel(model)
	}
	a.buildCreate = buildQwenTTSBody
	a.parseCreate = parseQwenAudioTTSResult
	return a
}

// isQwenTTSModel 报告模型名是否属于本 adapter 负责的千问 TTS 家族。
// 只按模型名归属，不校验平台：媒体上游账号可能挂在 openai（最小配置）
// 或其它承载平台上，平台不影响协议。
func isQwenTTSModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "" {
		return false
	}
	for _, name := range qwenAudioTTSModels {
		if m == name {
			return true
		}
	}
	for _, name := range qwenAudioTTSNextModels {
		if m == name {
			return true
		}
	}
	for _, name := range qwenCosyVoiceModels {
		if m == name {
			return true
		}
	}
	for _, name := range qwen3TTSModels {
		if m == name {
			return true
		}
	}
	return false
}

func isQwenAudioTTSNextModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	for _, name := range qwenAudioTTSNextModels {
		if m == name {
			return true
		}
	}
	return false
}

func isQwen3TTSModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	for _, name := range qwen3TTSModels {
		if m == name {
			return true
		}
	}
	return false
}

func isQwenCosyVoiceModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	for _, name := range qwenCosyVoiceModels {
		if m == name {
			return true
		}
	}
	return false
}

// qwenTTSCreateURL 按模型家族选择上游端点：
// Qwen3-TTS 走 multimodal-generation，其余三个家族走 SpeechSynthesizer。
// 账号级 video_create_path 仍可覆盖（与其它媒体 adapter 的运维逃生口一致）。
func (a *QwenAudioTTSAdapter) qwenTTSCreateURL(account *Account, model string) (string, error) {
	baseURL := strings.TrimRight(account.GetCredential("base_url"), "/")
	if baseURL == "" {
		return "", fmt.Errorf("qwen audio tts adapter: account %d has no base_url", account.ID)
	}
	endpoint := qwenAudioTTSCreatePath
	if isQwen3TTSModel(model) {
		endpoint = qwen3TTSCreatePath
	}
	if override := account.GetCredential("video_create_path"); strings.TrimSpace(override) != "" {
		endpoint = normalizeVideoURLPath(override)
	}
	return baseURL + endpoint, nil
}

// buildQwenTTSBody 按模型家族把统一请求映射到各自的上游 wire 格式。
func buildQwenTTSBody(req MediaCreateRequest) []byte {
	switch {
	case isQwenAudioTTSNextModel(req.UpstreamModel):
		return buildQwenAudioTTSNextBody(req)
	case isQwen3TTSModel(req.UpstreamModel):
		return buildQwen3TTSBody(req)
	case isQwenCosyVoiceModel(req.UpstreamModel):
		return buildQwenAudioTTSBodyWithDefault(req, qwenCosyVoiceDefaultVoice)
	default:
		return buildQwenAudioTTSBodyWithDefault(req, qwenAudioTTSDefaultVoice)
	}
}

// buildQwenAudioTTSBody 保留旧入口（Qwen-Audio-TTS 默认音色），供既有用例与调用方使用。
func buildQwenAudioTTSBody(req MediaCreateRequest) []byte {
	return buildQwenAudioTTSBodyWithDefault(req, qwenAudioTTSDefaultVoice)
}

// buildQwenAudioTTSBodyWithDefault 生成 Qwen-Audio-TTS / CosyVoice 的请求体。
// OpenAI /v1/audio/speech 风格参数映射：input -> text、voice、response_format -> format、
// speed -> rate；其余千问原生字段（instruction / language_hints / hot_fix / seed /
// enable_ssml / enable_markdown_filter 等）原样透传到 input。
func buildQwenAudioTTSBodyWithDefault(req MediaCreateRequest, defaultVoice string) []byte {
	input := map[string]any{
		"text":        req.Prompt,
		"voice":       defaultVoice,
		"format":      qwenAudioTTSDefaultFormat,
		"sample_rate": qwenAudioTTSDefaultSampleRate,
	}
	if req.Extra != nil {
		if voice, ok := req.Extra["voice"].(string); ok && strings.TrimSpace(voice) != "" {
			input["voice"] = strings.TrimSpace(voice)
		}
		if format, ok := qwenAudioTTSResponseFormat(req.Extra, qwenAudioTTSFormatAllowList); ok {
			input["format"] = format
		}
		// OpenAI 的 speed 与千问的 rate 语义一致；显式传了 rate 时以客户端为准。
		if _, exists := req.Extra["rate"]; !exists {
			if rate, ok := qwenAudioTTSSpeedAsRate(req.Extra); ok {
				input["rate"] = rate
			}
		}
		for k, v := range req.Extra {
			switch k {
			case "model", "prompt", "text", "media", "video_create_path",
				// 幂等标识不能进上游请求体，见 video_adapter.go 同处说明。
				"request_id",
				// 已映射成 input 子字段或上游不认的 OpenAI 风格字段。
				"input", "voice", "response_format", "speed", "stream",
				// 音频参考输入只有 tts-next 支持，Qwen-Audio-TTS 用复刻音色 ID。
				"audio_url", "audio_urls", "references", "text_prompt":
				continue
			}
			input[k] = v
		}
	}
	data, _ := json.Marshal(map[string]any{
		"model": req.UpstreamModel,
		"input": input,
	})
	return data
}

// buildQwenAudioTTSNextBody 生成 qwen-audio-3.1-tts-next 的请求体。
// 该模型不是单句 TTS：input.text_prompt 是一段包含人物、台词、音效的完整描述，
// 参考音频按顺序放进 input.references，并在提示词里用 @voice1/@voice2/@voice3 引用。
func buildQwenAudioTTSNextBody(req MediaCreateRequest) []byte {
	input := map[string]any{
		"text_prompt": req.Prompt,
		"format":      qwenAudioTTSDefaultFormat,
		"sample_rate": qwenAudioTTSNextDefaultSampleRate,
		"channels":    qwenAudioTTSNextDefaultChannels,
	}
	if req.Extra != nil {
		if format, ok := qwenAudioTTSResponseFormat(req.Extra, qwenAudioTTSNextFormatAllowList); ok {
			input["format"] = format
		}
		if refs := qwenAudioTTSNextReferences(req); len(refs) > 0 {
			input["references"] = refs
		}
		if _, exists := req.Extra["rate"]; !exists {
			if rate, ok := qwenAudioTTSSpeedAsRate(req.Extra); ok {
				input["rate"] = rate
			}
		}
		for k, v := range req.Extra {
			switch k {
			case "model", "prompt", "text", "input", "media", "video_create_path",
				"request_id",
				// 已映射成 input 子字段或上游不认的 OpenAI 风格字段。
				"voice", "response_format", "speed", "stream",
				// 参考音频已归一化进 input.references。
				"references", "audio_url", "audio_urls", "text_prompt":
				continue
			}
			input[k] = v
		}
	}
	data, _ := json.Marshal(map[string]any{
		"model": req.UpstreamModel,
		"input": input,
	})
	return data
}

// qwenAudioTTSNextReferences 归一化参考音频：
//   - 优先使用客户端按千问原生格式传入的 references（audio_url / audio_data）；
//   - 否则回退到统一请求解析出的 audio_url / audio_urls（data URI 按 audio_data 传）；
//   - 再回退到统一 media 数组里 type 含 audio 的素材项（与视频链路的 media 约定一致）。
//
// 最多保留 3 条，与官方限制一致；空值项直接丢弃。
func qwenAudioTTSNextReferences(req MediaCreateRequest) []map[string]any {
	if req.Extra != nil {
		if raw, ok := req.Extra["references"]; ok {
			if refs := qwenAudioTTSNormalizeReferences(raw); len(refs) > 0 {
				return refs
			}
		}
	}
	refs := make([]map[string]any, 0, len(req.AudioRefURLs))
	for _, rawURL := range req.AudioRefURLs {
		rawURL = strings.TrimSpace(rawURL)
		if rawURL == "" {
			continue
		}
		if strings.HasPrefix(rawURL, "data:") {
			refs = append(refs, map[string]any{"audio_data": rawURL})
		} else {
			refs = append(refs, map[string]any{"audio_url": rawURL})
		}
		if len(refs) == 3 {
			break
		}
	}
	if len(refs) == 0 {
		for _, item := range req.Media {
			if !strings.Contains(strings.ToLower(strings.TrimSpace(item.Type)), "audio") {
				continue
			}
			mediaURL := strings.TrimSpace(item.URL)
			if mediaURL == "" {
				continue
			}
			refs = append(refs, map[string]any{"audio_url": mediaURL})
			if len(refs) == 3 {
				break
			}
		}
	}
	return refs
}

// qwenAudioTTSNormalizeReferences 接受引用对象数组或字符串数组，输出官方结构。
func qwenAudioTTSNormalizeReferences(raw any) []map[string]any {
	items := make([]any, 0, 3)
	switch v := raw.(type) {
	case []any:
		items = v
	case []map[string]any:
		for _, item := range v {
			items = append(items, item)
		}
	case []string:
		for _, item := range v {
			items = append(items, item)
		}
	default:
		return nil
	}
	refs := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if len(refs) == 3 {
			break
		}
		switch v := item.(type) {
		case string:
			if u := strings.TrimSpace(v); u != "" {
				if strings.HasPrefix(u, "data:") {
					refs = append(refs, map[string]any{"audio_data": u})
				} else {
					refs = append(refs, map[string]any{"audio_url": u})
				}
			}
		case map[string]any:
			ref := map[string]any{}
			for _, key := range []string{"audio_url", "audio_data"} {
				if s, ok := v[key].(string); ok && strings.TrimSpace(s) != "" {
					ref[key] = strings.TrimSpace(s)
				}
			}
			// URL 与 Base64 二选一，同时出现时以 audio_url 为准。
			if _, hasURL := ref["audio_url"]; hasURL {
				delete(ref, "audio_data")
			}
			if len(ref) > 0 {
				refs = append(refs, ref)
			}
		}
	}
	return refs
}

// buildQwen3TTSBody 生成 Qwen3-TTS 的请求体：input.text + input.voice，
// 可选 language_type / instructions / optimize_instructions。
// 该端点不接受 format / sample_rate / rate，OpenAI 风格字段一律不下发。
func buildQwen3TTSBody(req MediaCreateRequest) []byte {
	input := map[string]any{
		"text":  req.Prompt,
		"voice": qwen3TTSDefaultVoice,
	}
	if req.Extra != nil {
		if voice, ok := req.Extra["voice"].(string); ok && strings.TrimSpace(voice) != "" {
			input["voice"] = strings.TrimSpace(voice)
		}
		for k, v := range req.Extra {
			switch k {
			case "model", "prompt", "text", "input", "media", "video_create_path",
				"request_id",
				"voice", "response_format", "speed", "stream",
				"format", "sample_rate", "rate", "channels", "volume",
				"audio_url", "audio_urls", "references", "text_prompt":
				continue
			}
			input[k] = v
		}
	}
	data, _ := json.Marshal(map[string]any{
		"model": req.UpstreamModel,
		"input": input,
	})
	return data
}

// qwenAudioTTSResponseFormat 读取 OpenAI 风格的 response_format，并按模型家族
// 允许列表过滤：上游不支持的格式如果原样下发会被 400，直接忽略更稳妥。
func qwenAudioTTSResponseFormat(extra map[string]any, allow map[string]struct{}) (string, bool) {
	raw, ok := extra["response_format"].(string)
	if !ok {
		return "", false
	}
	format := strings.ToLower(strings.TrimSpace(raw))
	if format == "" {
		return "", false
	}
	if _, ok := allow[format]; !ok {
		return "", false
	}
	return format, true
}

// qwenAudioTTSSpeedAsRate 把 OpenAI 的 speed 映射成千问的 rate。
// 非正数、非数字一律忽略，交由上游默认值处理。
func qwenAudioTTSSpeedAsRate(extra map[string]any) (float64, bool) {
	switch speed := extra["speed"].(type) {
	case float64:
		if speed > 0 {
			return speed, true
		}
	case float32:
		if speed > 0 {
			return float64(speed), true
		}
	case int:
		if speed > 0 {
			return float64(speed), true
		}
	case int64:
		if speed > 0 {
			return float64(speed), true
		}
	}
	return 0, false
}

// qwenAudioOutputFormat 返回请求对应的输出格式，用于给客户端声明真实 MIME。
// Qwen3-TTS 不接受 format 入参，按官方示例固定为 wav。
func qwenAudioOutputFormat(req *MediaCreateRequest) string {
	if req == nil {
		return qwenAudioTTSDefaultFormat
	}
	if isQwen3TTSModel(req.UpstreamModel) {
		return "wav"
	}
	allow := qwenAudioTTSFormatAllowList
	if isQwenAudioTTSNextModel(req.UpstreamModel) {
		allow = qwenAudioTTSNextFormatAllowList
	}
	if format, ok := qwenAudioTTSResponseFormat(req.Extra, allow); ok {
		return format
	}
	// 原生请求可能直接传 input.format（已平铺进 Extra），也要参与 MIME 推导。
	if req.Extra != nil {
		if format, ok := req.Extra["format"].(string); ok {
			format = strings.ToLower(strings.TrimSpace(format))
			if _, allowed := allow[format]; allowed {
				return format
			}
		}
	}
	return qwenAudioTTSDefaultFormat
}

// qwenAudioContentType 根据真实产物 URL / 请求格式给出 MIME。
// URL 扩展名优先（上游实际编码格式），缺失时回退请求格式，最后回退 wav。
func qwenAudioContentType(format, rawURL string) string {
	if ct := qwenAudioContentTypeFromURL(rawURL); ct != "" {
		return ct
	}
	switch format {
	case "wav":
		return "audio/wav"
	case "mp3":
		return "audio/mpeg"
	case "pcm":
		return "audio/pcm"
	case "opus", "ogg":
		return "audio/ogg"
	case "flac":
		return "audio/flac"
	case "aac":
		return "audio/aac"
	}
	return "audio/wav"
}

func qwenAudioContentTypeFromURL(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	switch strings.ToLower(path.Ext(u.Path)) {
	case ".wav":
		return "audio/wav"
	case ".mp3":
		return "audio/mpeg"
	case ".pcm":
		return "audio/pcm"
	case ".opus", ".ogg":
		return "audio/ogg"
	case ".flac":
		return "audio/flac"
	case ".aac":
		return "audio/aac"
	}
	return ""
}

func parseQwenAudioTTSResult(respBody []byte, statusCode int) (*MediaCreateResult, error) {
	if statusCode >= 400 {
		return &MediaCreateResult{
			Status: "failed", Mode: MediaCompletionFailed, UpstreamStatusCode: statusCode, UpstreamRaw: respBody,
			ErrorMessage: sanitizeMediaUpstreamErrorMessage(statusCode, respBody),
		}, nil
	}
	var data map[string]any
	if err := json.Unmarshal(respBody, &data); err != nil {
		return nil, fmt.Errorf("qwen audio tts unmarshal response: %w", err)
	}
	// DashScope 风格业务失败：HTTP 200 + 顶层 code != Success。
	// 原文只留 UpstreamRaw，绝不把 message 回显给终端用户。
	if code, ok := data["code"].(string); ok && code != "" && !strings.EqualFold(code, "Success") {
		return &MediaCreateResult{
			Status: "failed", Mode: MediaCompletionFailed, UpstreamStatusCode: statusCode, UpstreamRaw: respBody,
			ErrorMessage: "qwen audio tts upstream rejected the request",
		}, nil
	}
	audioURL := ""
	if out, ok := data["output"].(map[string]any); ok {
		if audio, ok := out["audio"].(map[string]any); ok {
			audioURL, _ = audio["url"].(string)
		}
	}
	// 没有产物就必须判失败：音频是同步终态，服务层会立刻真实扣费。
	if audioURL == "" {
		return &MediaCreateResult{
			Status: "failed", Mode: MediaCompletionFailed, UpstreamStatusCode: statusCode, UpstreamRaw: respBody,
			ErrorMessage: "qwen audio tts response contained no audio url",
		}, nil
	}
	return &MediaCreateResult{
		Status: "succeeded", Mode: MediaCompletionSync, InlineURL: audioURL,
		UpstreamStatusCode: statusCode, UpstreamRaw: respBody,
	}, nil
}

// Create 覆盖底座：按模型家族选端点，先拿合成结果里的临时音频 URL，
// 再下载成字节交给同步端点直返，并按真实格式声明 MIME。
// 下载失败一律判 failed，绝不返回 succeeded 空产物让服务层扣费。
func (a *QwenAudioTTSAdapter) Create(ctx context.Context, account *Account, req MediaCreateRequest) (*MediaCreateResult, error) {
	createURL, err := a.qwenTTSCreateURL(account, req.UpstreamModel)
	if err != nil {
		return nil, err
	}
	respBody, statusCode, err := a.do(ctx, account, http.MethodPost, createURL, a.buildCreate(req))
	if err != nil {
		return nil, fmt.Errorf("qwen audio tts create request: %w", err)
	}
	result, err := a.parseCreate(respBody, statusCode)
	if err != nil || result == nil || result.Status != "succeeded" || result.InlineURL == "" {
		return result, err
	}
	audioBytes, err := a.downloadAudio(ctx, result.InlineURL)
	if err != nil {
		return &MediaCreateResult{
			Status: "failed", Mode: MediaCompletionFailed, UpstreamStatusCode: statusCode, UpstreamRaw: respBody,
			ErrorMessage: "qwen audio tts audio download failed",
		}, nil
	}
	result.InlineContentType = qwenAudioContentType(qwenAudioOutputFormat(&req), result.InlineURL)
	result.InlineBytes = audioBytes
	result.InlineURL = ""
	return result, nil
}

// qwenAudioDownloadURLAllowed 只接受 http(s)。千问返回的是预签名临时 URL，
// 文档示例中同时存在 http 与 https 两种 CDN 域名，不能只放行 https，
// 否则合法产物会被判成下载失败（用户被拒付但上游已计费）。
func qwenAudioDownloadURLAllowed(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return false
	}
	return u.Scheme == "https" || u.Scheme == "http"
}

func (a *QwenAudioTTSAdapter) downloadAudio(ctx context.Context, rawURL string) ([]byte, error) {
	if !qwenAudioDownloadURLAllowed(rawURL) {
		return nil, fmt.Errorf("qwen audio tts: invalid audio url")
	}
	dlCtx, cancel := context.WithTimeout(ctx, qwenAudioTTSDownloadTO)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(dlCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	// 上游返回的是临时预签名 URL，不再附带 API Key。
	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("qwen audio tts: audio download status %d", resp.StatusCode)
	}
	audio, err := io.ReadAll(io.LimitReader(resp.Body, qwenAudioTTSMaxAudioBytes+1))
	if err != nil {
		return nil, err
	}
	if len(audio) == 0 || len(audio) > qwenAudioTTSMaxAudioBytes {
		return nil, fmt.Errorf("qwen audio tts: empty or oversized audio")
	}
	return audio, nil
}
