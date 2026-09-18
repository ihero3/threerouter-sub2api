package service

// media_adapters_extra.go — 图片 / 音频媒体 adapter。
// 图片与音频厂商接口大多走 OpenAI-compatible 异步或同步形态，
// 这里做通用实现；遇到差异巨大（如火山、阿里 DashScope）可再新增单独 adapter。

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// openAICompatMediaAdapter 通用 OpenAI-compatible 媒体 adapter。
// 根据 Kind 决定 POST/GET 路径与响应字段，兼容图片生成（/images/generations）
// 与语音合成（/audio/speech），异步任务形（任务 id + 轮询）与同步 URL 均支持。
type openAICompatMediaAdapter struct {
	kind       MediaKind
	supports   func(platform, model string) bool
	httpClient *http.Client
	// createPath 与 queryPath 可被 Kind 默认值覆盖
	createPath string
	queryPath  string
}

func newOpenAICompatMediaAdapter(kind MediaKind, supports func(string, string) bool) *openAICompatMediaAdapter {
	a := &openAICompatMediaAdapter{kind: kind, supports: supports}
	a.httpClient = &http.Client{Timeout: 120 * time.Second}
	switch kind {
	case MediaKindImage:
		a.createPath = "/v1/images/generations"
		a.queryPath = "/v1/images/generations/%s"
	case MediaKindAudio:
		a.createPath = "/v1/audio/speech"
		a.queryPath = "/v1/audio/speech/%s"
	}
	return a
}

func (a *openAICompatMediaAdapter) Kind() MediaKind { return a.kind }

func (a *openAICompatMediaAdapter) Supports(platform, model string) bool {
	if a.supports != nil {
		return a.supports(platform, model)
	}
	return false
}

func (a *openAICompatMediaAdapter) baseURL(account *Account) (string, error) {
	baseURL := strings.TrimRight(account.GetCredential("base_url"), "/")
	if baseURL == "" {
		return "", fmt.Errorf("media adapter: account %d has no base_url", account.ID)
	}
	return baseURL, nil
}

func (a *openAICompatMediaAdapter) apiKey(account *Account) (string, error) {
	apiKey := account.GetCredential("api_key")
	if apiKey == "" {
		return "", fmt.Errorf("media adapter: account %d has no api_key", account.ID)
	}
	return apiKey, nil
}

func (a *openAICompatMediaAdapter) Create(ctx context.Context, account *Account, req MediaCreateRequest) (*MediaCreateResult, error) {
	baseURL, err := a.baseURL(account)
	if err != nil {
		return nil, err
	}
	apiKey, err := a.apiKey(account)
	if err != nil {
		return nil, err
	}
	body := a.buildCreateBody(req)
	url := baseURL + a.createPath

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("media adapter: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	account.ApplyHeaderOverrides(httpReq.Header)

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("media adapter: create do: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return &MediaCreateResult{
			Status:             "failed",
			Mode:               MediaCompletionFailed,
			UpstreamStatusCode: resp.StatusCode,
			UpstreamRaw:        respBody,
			ErrorMessage:       fmt.Sprintf("upstream returned %d: %s", resp.StatusCode, string(respBody)),
		}, nil
	}

	// 音频 kind：OpenAI 兼容 /audio/speech 返回原始音频字节（audio/mpeg），非 JSON。
	// 此时直接把字节给调用方，标 succeeded（同步）。
	if a.kind == MediaKindAudio && !json.Valid(respBody) {
		return &MediaCreateResult{
			Status: "succeeded", Mode: MediaCompletionSync, InlineBytes: respBody,
			UpstreamStatusCode: resp.StatusCode, UpstreamRaw: respBody,
		}, nil
	}

	// 解析响应：优先 task_id / id；同步结果取 url / data[0].url / b64_json。
	var createResp struct {
		ID     string `json:"id"`
		TaskID string `json:"task_id"`
		URL    string `json:"url"`
		Data   []struct {
			URL     string `json:"url"`
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &createResp); err != nil {
		return nil, fmt.Errorf("media adapter: unmarshal create response: %w", err)
	}

	taskID := firstNonEmptyString(createResp.TaskID, createResp.ID)
	// 同步产物必须收集全部（含 b64_json 转成的 data URI）：
	// 只取 data[0].url 会漏掉纯 base64 响应，也会在 n>1 时只交付一张。
	patchURLs := collectOpenAIMediaImageURLs(respBody)
	if headerURL := strings.TrimSpace(createResp.URL); headerURL != "" {
		patchURLs = append([]string{headerURL}, patchURLs...)
	}
	// 判空必须用"实际可用产物数"，不能用 len(Data)：
	// Data 非空但每项只带 b64_json 时，旧逻辑不会 failed，任务却既无任务号
	// 也无 URL —— PollTask 遇空 UpstreamTaskID 直接跳过，超时兜底永远不可达，
	// 预扣就此挂账。这里堵死：既没有任务号又拿不到图，就是上游失败。
	if taskID == "" && len(patchURLs) == 0 {
		return &MediaCreateResult{
			Status:             "failed",
			Mode:               MediaCompletionFailed,
			UpstreamStatusCode: resp.StatusCode,
			UpstreamRaw:        respBody,
			ErrorMessage:       "upstream create response missing task id or url",
		}, nil
	}
	inlineURL := ""
	if len(patchURLs) > 0 {
		inlineURL = patchURLs[0]
	}
	status := "processing"
	if inlineURL != "" {
		status = "succeeded"
	}
	return &MediaCreateResult{
		TaskID:             taskID,
		InlineURL:          inlineURL,
		InlineURLs:         patchURLs,
		Status:             status,
		Mode:               mediaCompletionModeFromStatus(status),
		UpstreamStatusCode: resp.StatusCode,
		UpstreamRaw:        respBody,
	}, nil
}

func (a *openAICompatMediaAdapter) GetResult(ctx context.Context, account *Account, upstreamTaskID string) (*MediaTaskResult, error) {
	baseURL, err := a.baseURL(account)
	if err != nil {
		return nil, err
	}
	apiKey, err := a.apiKey(account)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf(baseURL+a.queryPath, upstreamTaskID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("media adapter: get request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	account.ApplyHeaderOverrides(httpReq.Header)

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("media adapter: get do: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	result := &MediaTaskResult{StatusCode: resp.StatusCode, UpstreamRaw: respBody}
	if resp.StatusCode >= 400 {
		result.Status = "failed"
		result.ErrorMessage = fmt.Sprintf("upstream returned %d: %s", resp.StatusCode, string(respBody))
		return result, nil
	}
	var respData struct {
		Status string `json:"status"`
		URL    string `json:"url"`
		Data   []struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &respData); err != nil {
		return nil, fmt.Errorf("media adapter: unmarshal get response: %w", err)
	}
	result.Status = normalizeMediaTaskStatus(firstNonEmptyString(respData.Status, "processing"))
	result.URL = respData.URL
	if result.URL == "" && len(respData.Data) > 0 {
		result.URL = respData.Data[0].URL
	}
	return result, nil
}

func (a *openAICompatMediaAdapter) buildCreateBody(req MediaCreateRequest) []byte {
	body := map[string]any{"model": req.UpstreamModel}
	switch a.kind {
	case MediaKindImage:
		body["prompt"] = req.Prompt
		if req.Resolution != "" {
			body["size"] = req.Resolution
		}
	case MediaKindAudio:
		body["input"] = req.Prompt
		if req.Prompt == "" && len(req.Media) > 0 {
			body["input"] = req.Media[0].URL
		}
	}
	for k, v := range req.Extra {
		switch k {
		case "model", "prompt", "input", "size", "resolution", "image_url", "image_urls", "media",
			// 幂等标识不能进上游请求体，见 video_adapter.go 同处说明。
			"request_id":
			continue
		}
		body[k] = v
	}
	data, _ := json.Marshal(body)
	return data
}

func mediaCompletionModeFromStatus(status string) MediaCompletionMode {
	switch mediaNormalizeStatus(status) {
	case "succeeded", "completed", "complete", "success", "done":
		return MediaCompletionSync
	case "failed", "error", "cancelled", "canceled", "expired", "unknown":
		return MediaCompletionFailed
	default:
		return MediaCompletionAsync
	}
}

func mediaNormalizeStatus(status string) string {
	return normalizeVideoTaskStatus(status)
}

func normalizeMediaTaskStatus(status string) string {
	return normalizeVideoTaskStatus(status)
}

// NewImageMediaAdapter 创建通用图片媒体 adapter。
// 覆盖 Grok imagine-image / OpenAI gpt-image / 国产图片模型。
func NewImageMediaAdapter() *openAICompatMediaAdapter {
	return newOpenAICompatMediaAdapter(MediaKindImage, func(platform, model string) bool {
		m := strings.ToLower(strings.TrimSpace(model))
		return strings.Contains(m, "grok-imagine-image") ||
			strings.Contains(m, "grok-imagine") ||
			strings.Contains(m, "gpt-image") ||
			strings.Contains(m, "dall-e") ||
			strings.Contains(m, "wan-image") ||
			strings.Contains(m, "seedance-image") ||
			strings.Contains(m, "t2i")
	})
}

// NewAudioMediaAdapter 创建通用音频媒体 adapter（TTS / STT）。
func NewAudioMediaAdapter() *openAICompatMediaAdapter {
	return newOpenAICompatMediaAdapter(MediaKindAudio, func(platform, model string) bool {
		m := strings.ToLower(strings.TrimSpace(model))
		return strings.Contains(m, "tts") ||
			strings.Contains(m, "stt") ||
			strings.Contains(m, "whisper") ||
			strings.Contains(m, "speech") ||
			strings.Contains(m, "realtime") ||
			strings.Contains(m, "voice")
	})
}
