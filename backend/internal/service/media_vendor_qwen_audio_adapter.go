package service

// media_vendor_qwen_audio_adapter.go — 千问 Qwen-Audio-TTS 非实时语音合成。
// 走 DashScope/MaaS 的 SpeechSynthesizer 端点，与旧 AliyunTTSAdapter 的
// multimodal-generation 协议不同，故单独建 adapter 复用 mediaVendorAudioAdapter 底座。

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// qwenAudioTTSModels 是千问非实时语音合成官方模型（qwen-audio-3.1/3.0）。
var qwenAudioTTSModels = []string{
	"qwen-audio-3.1-tts-flash", "qwen-audio-3.1-tts-plus",
	"qwen-audio-3.0-tts-flash", "qwen-audio-3.0-tts-plus",
}

const (
	qwenAudioTTSCreatePath    = "/api/v1/services/audio/tts/SpeechSynthesizer"
	qwenAudioTTSDefaultVoice  = "longanhuan_v3.1"
	qwenAudioTTSDefaultFormat = "wav"
	qwenAudioTTSDownloadTO    = 30 * time.Second
	qwenAudioTTSMaxAudioBytes = 64 << 20 // 单段音频上限，防止上游返回异常大响应
)

type QwenAudioTTSAdapter struct{ mediaVendorAudioAdapter }

func NewQwenAudioTTSAdapter() *QwenAudioTTSAdapter {
	a := &QwenAudioTTSAdapter{}
	a.mediaVendorAudioAdapter = newMediaVendorAudioAdapter("qwen-audio-tts")
	a.supports = func(platform, model string) bool {
		m := strings.ToLower(strings.TrimSpace(model))
		for _, name := range qwenAudioTTSModels {
			if m == name {
				return true
			}
		}
		return false
	}
	a.buildCreate = buildQwenAudioTTSBody
	a.parseCreate = parseQwenAudioTTSResult
	a.buildCreateURL = func(account *Account) (string, error) {
		baseURL := strings.TrimRight(account.GetCredential("base_url"), "/")
		if baseURL == "" {
			return "", fmt.Errorf("qwen audio tts adapter: account %d has no base_url", account.ID)
		}
		path := qwenAudioTTSCreatePath
		if extra := account.GetCredential("video_create_path"); strings.TrimSpace(extra) != "" {
			path = normalizeVideoURLPath(extra)
		}
		return baseURL + path, nil
	}
	return a
}

// buildQwenAudioTTSBody 把 OpenAI /v1/audio/speech 风格参数映射到千问 input 对象。
// instruction / language_hints / hot_fix 等千问原生字段原样透传到 input。
func buildQwenAudioTTSBody(req MediaCreateRequest) []byte {
	input := map[string]any{
		"text":        req.Prompt,
		"voice":       qwenAudioTTSDefaultVoice,
		"format":      qwenAudioTTSDefaultFormat,
		"sample_rate": 24000,
	}
	if voice, ok := req.Extra["voice"].(string); ok && strings.TrimSpace(voice) != "" {
		input["voice"] = strings.TrimSpace(voice)
	}
	if format, ok := req.Extra["response_format"].(string); ok {
		format = strings.ToLower(strings.TrimSpace(format))
		switch format {
		case "wav", "mp3", "flac", "pcm", "aac", "ogg", "opus":
			input["format"] = format
		}
	}
	for k, v := range req.Extra {
		switch k {
		case "model", "prompt", "text", "media", "video_create_path",
			// 幂等标识不能进上游请求体，见 video_adapter.go 同处说明。
			"request_id",
			// 已映射成 input 子字段或上游不认的 OpenAI 风格字段。
			"input", "voice", "response_format", "speed", "stream":
			continue
		}
		input[k] = v
	}
	data, _ := json.Marshal(map[string]any{
		"model": req.UpstreamModel,
		"input": input,
	})
	return data
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

// Create 覆盖底座：先拿合成结果里的临时音频 URL，再下载成字节交给同步端点直返。
// 下载失败一律判 failed，绝不返回 succeeded 空产物让服务层扣费。
func (a *QwenAudioTTSAdapter) Create(ctx context.Context, account *Account, req MediaCreateRequest) (*MediaCreateResult, error) {
	createURL, err := a.buildCreateURL(account)
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
	result.InlineBytes = audioBytes
	result.InlineURL = ""
	return result, nil
}

func qwenAudioDownloadURLAllowed(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return false
	}
	return true
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
