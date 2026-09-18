package service

// media_vendor_audio_adapter.go — 音频生成独立厂商 adapter（MiniMax TTS / 火山 / 阿里）。
// 音频接口多为同步返回音频 URL（或 base64），与视频异步任务不同。
// 各家 path / 字段差异通过 builder 与 buildCreateURL 隔离。

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

// mediaVendorAudioAdapter 是音频生成厂商 adapter 底座。
type mediaVendorAudioAdapter struct {
	name           string
	httpClient     *http.Client
	supports       func(platform, model string) bool
	buildCreate    func(MediaCreateRequest) []byte
	parseCreate    func(respBody []byte, statusCode int) (*MediaCreateResult, error)
	buildCreateURL func(account *Account) (string, error)
	createHeaders  func(account *Account) map[string]string
}

func newMediaVendorAudioAdapter(name string) mediaVendorAudioAdapter {
	return mediaVendorAudioAdapter{
		name: name,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (a mediaVendorAudioAdapter) Kind() MediaKind { return MediaKindAudio }

func (a mediaVendorAudioAdapter) Supports(platform, model string) bool {
	if a.supports != nil {
		return a.supports(platform, model)
	}
	return false
}

func (a mediaVendorAudioAdapter) baseURL(account *Account) (string, error) {
	baseURL := strings.TrimRight(account.GetCredential("base_url"), "/")
	if baseURL == "" {
		return "", fmt.Errorf("%s audio adapter: account %d has no base_url", a.name, account.ID)
	}
	return baseURL, nil
}

func (a mediaVendorAudioAdapter) apiKey(account *Account) (string, error) {
	apiKey := account.GetCredential("api_key")
	if apiKey == "" {
		return "", fmt.Errorf("%s audio adapter: account %d has no api_key", a.name, account.ID)
	}
	return apiKey, nil
}

func (a mediaVendorAudioAdapter) do(ctx context.Context, account *Account, method, url string, body []byte) ([]byte, int, error) {
	apiKey, err := a.apiKey(account)
	if err != nil {
		return nil, 0, err
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	account.ApplyHeaderOverrides(req.Header)
	if body != nil && a.createHeaders != nil {
		for key, value := range a.createHeaders(account) {
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			if key != "" && value != "" {
				req.Header.Set(key, value)
			}
		}
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return respBody, resp.StatusCode, nil
}

func (a mediaVendorAudioAdapter) Create(ctx context.Context, account *Account, req MediaCreateRequest) (*MediaCreateResult, error) {
	if a.buildCreate == nil || a.parseCreate == nil {
		return nil, fmt.Errorf("%s audio adapter is missing create handlers", a.name)
	}
	baseURL, err := a.baseURL(account)
	if err != nil {
		return nil, err
	}
	var url string
	if req.Extra != nil {
		if override, _ := req.Extra["video_create_path"].(string); strings.TrimSpace(override) != "" {
			url = baseURL + normalizeVideoURLPath(override)
		}
	}
	if url == "" && a.buildCreateURL != nil {
		url, err = a.buildCreateURL(account)
		if err != nil {
			return nil, err
		}
	}
	if url == "" {
		url = baseURL + "/v1/audio/speech"
	}
	respBody, statusCode, err := a.do(ctx, account, http.MethodPost, url, a.buildCreate(req))
	if err != nil {
		return nil, fmt.Errorf("%s audio create request: %w", a.name, err)
	}
	return a.parseCreate(respBody, statusCode)
}

func (a mediaVendorAudioAdapter) GetResult(ctx context.Context, account *Account, upstreamTaskID string) (*MediaTaskResult, error) {
	baseURL, err := a.baseURL(account)
	if err != nil {
		return nil, err
	}
	apiKey, err := a.apiKey(account)
	if err != nil {
		return nil, err
	}
	url := baseURL + "/v1/audio/speech/" + upstreamTaskID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%s audio get request: %w", a.name, err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	account.ApplyHeaderOverrides(req.Header)
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s audio get do: %w", a.name, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	result := &MediaTaskResult{StatusCode: resp.StatusCode, UpstreamRaw: respBody}
	if resp.StatusCode >= 400 {
		result.Status = "failed"
		result.ErrorMessage = fmt.Sprintf("upstream returned %d: %s", resp.StatusCode, string(respBody))
		return result, nil
	}
	// 同步音频直接返回；无 URL 时标 processing。
	var respData struct {
		Status string `json:"status"`
		URL    string `json:"url"`
		Data   []struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &respData); err != nil {
		return nil, fmt.Errorf("%s audio unmarshal get response: %w", a.name, err)
	}
	result.Status = normalizeMediaTaskStatus(firstNonEmptyString(respData.Status, "processing"))
	result.URL = respData.URL
	if result.URL == "" && len(respData.Data) > 0 {
		result.URL = respData.Data[0].URL
	}
	return result, nil
}

// --- MiniMax TTS ---

type MiniMaxTTSAdapter struct{ mediaVendorAudioAdapter }

func NewMiniMaxTTSAdapter() *MiniMaxTTSAdapter {
	a := &MiniMaxTTSAdapter{}
	a.mediaVendorAudioAdapter = newMediaVendorAudioAdapter("minimax-tts")
	a.supports = func(platform, model string) bool {
		m := strings.ToLower(strings.TrimSpace(model))
		return strings.Contains(m, "minimax-tts") ||
			strings.Contains(m, "speech-01") ||
			strings.Contains(m, "tts")
	}
	a.buildCreate = buildMiniMaxTTSBody
	a.parseCreate = parseMiniMaxTTSResult
	a.buildCreateURL = func(account *Account) (string, error) {
		baseURL := strings.TrimRight(account.GetCredential("base_url"), "/")
		if baseURL == "" {
			return "", fmt.Errorf("minimax tts adapter: account %d has no base_url", account.ID)
		}
		path := "/v1/t2a_v2"
		if extra := account.GetCredential("video_create_path"); strings.TrimSpace(extra) != "" {
			path = normalizeVideoURLPath(extra)
		}
		return baseURL + path, nil
	}
	return a
}

func buildMiniMaxTTSBody(req MediaCreateRequest) []byte {
	body := map[string]any{
		"model": req.UpstreamModel,
		"text":  req.Prompt,
	}
	for k, v := range req.Extra {
		switch k {
		case "model", "prompt", "text", "media", "video_create_path",
			// 幂等标识不能进上游请求体，见 video_adapter.go 同处说明。
			"request_id":
			continue
		}
		body[k] = v
	}
	data, _ := json.Marshal(body)
	return data
}

func parseMiniMaxTTSResult(respBody []byte, statusCode int) (*MediaCreateResult, error) {
	if statusCode >= 400 {
		return &MediaCreateResult{
			Status: "failed", Mode: MediaCompletionFailed, UpstreamStatusCode: statusCode, UpstreamRaw: respBody,
			ErrorMessage: fmt.Sprintf("upstream returned %d: %s", statusCode, string(respBody)),
		}, nil
	}
	var data map[string]any
	if err := json.Unmarshal(respBody, &data); err != nil {
		return nil, fmt.Errorf("minimax tts unmarshal response: %w", err)
	}
	url := ""
	if audio, ok := data["data"].(map[string]any); ok {
		if u, ok := audio["audio"].(string); ok {
			url = u
		}
	}
	if url == "" {
		if u, ok := data["audio"].(string); ok {
			url = u
		}
	}
	// 没有产物就必须判失败：音频是同步终态，服务层会立刻真实扣费，
	// succeeded 却给不出 URL 等于"扣了钱没有声音"。与图片侧同口径。
	if url == "" {
		return &MediaCreateResult{
			Status: "failed", Mode: MediaCompletionFailed, UpstreamStatusCode: statusCode, UpstreamRaw: respBody,
			ErrorMessage: "minimax tts response contained no audio url",
		}, nil
	}
	return &MediaCreateResult{
		Status: "succeeded", Mode: MediaCompletionSync, InlineURL: url, UpstreamStatusCode: statusCode, UpstreamRaw: respBody,
	}, nil
}

// --- 火山 TTS ---

type VolcanoTTSAdapter struct{ mediaVendorAudioAdapter }

func NewVolcanoTTSAdapter() *VolcanoTTSAdapter {
	a := &VolcanoTTSAdapter{}
	a.mediaVendorAudioAdapter = newMediaVendorAudioAdapter("volcano-tts")
	a.supports = func(platform, model string) bool {
		m := strings.ToLower(strings.TrimSpace(model))
		return strings.Contains(m, "volc-tts") ||
			strings.Contains(m, "volcano-tts") ||
			strings.Contains(m, "doubao-tts")
	}
	a.buildCreate = buildVolcanoTTSBody
	a.parseCreate = parseVolcanoTTSResult
	a.buildCreateURL = func(account *Account) (string, error) {
		baseURL := strings.TrimRight(account.GetCredential("base_url"), "/")
		if baseURL == "" {
			return "", fmt.Errorf("volcano tts adapter: account %d has no base_url", account.ID)
		}
		path := "/api/v3/tts"
		if extra := account.GetCredential("video_create_path"); strings.TrimSpace(extra) != "" {
			path = normalizeVideoURLPath(extra)
		}
		return baseURL + path, nil
	}
	return a
}

func buildVolcanoTTSBody(req MediaCreateRequest) []byte {
	body := map[string]any{
		"app":     map[string]any{},
		"user":    map[string]any{},
		"request": map[string]any{"reqid": generateMediaLocalID(MediaKindAudio), "text": req.Prompt, "voice_type": req.UpstreamModel, "operation": "query"},
	}
	for k, v := range req.Extra {
		switch k {
		case "model", "prompt", "media", "video_create_path", "app", "user", "request",
			// 幂等标识不能进上游请求体，见 video_adapter.go 同处说明。
			"request_id":
			continue
		}
		body[k] = v
	}
	data, _ := json.Marshal(body)
	return data
}

func parseVolcanoTTSResult(respBody []byte, statusCode int) (*MediaCreateResult, error) {
	if statusCode >= 400 {
		return &MediaCreateResult{
			Status: "failed", Mode: MediaCompletionFailed, UpstreamStatusCode: statusCode, UpstreamRaw: respBody,
			ErrorMessage: fmt.Sprintf("upstream returned %d: %s", statusCode, string(respBody)),
		}, nil
	}
	var data map[string]any
	if err := json.Unmarshal(respBody, &data); err != nil {
		return nil, fmt.Errorf("volcano tts unmarshal response: %w", err)
	}
	url := ""
	if u, ok := data["data"].(string); ok {
		url = u
	}
	// 同上：火山把音频直接放在 data 字段，取不到就是没有产物，不能算 succeeded。
	if url == "" {
		return &MediaCreateResult{
			Status: "failed", Mode: MediaCompletionFailed, UpstreamStatusCode: statusCode, UpstreamRaw: respBody,
			ErrorMessage: "volcano tts response contained no audio url",
		}, nil
	}
	return &MediaCreateResult{
		Status: "succeeded", Mode: MediaCompletionSync, InlineURL: url, UpstreamStatusCode: statusCode, UpstreamRaw: respBody,
	}, nil
}

// --- 阿里 DashScope TTS ---

type AliyunTTSAdapter struct{ mediaVendorAudioAdapter }

func NewAliyunTTSAdapter() *AliyunTTSAdapter {
	a := &AliyunTTSAdapter{}
	a.mediaVendorAudioAdapter = newMediaVendorAudioAdapter("aliyun-tts")
	a.supports = func(platform, model string) bool {
		m := strings.ToLower(strings.TrimSpace(model))
		return strings.Contains(m, "aliyun-tts") ||
			strings.Contains(m, "dashscope-tts") ||
			strings.Contains(m, "cosyvoice") ||
			strings.Contains(m, "qwen-tts")
	}
	a.buildCreate = buildAliyunTTSBody
	a.parseCreate = parseAliyunTTSResult
	a.buildCreateURL = func(account *Account) (string, error) {
		baseURL := strings.TrimRight(account.GetCredential("base_url"), "/")
		if baseURL == "" {
			return "", fmt.Errorf("aliyun tts adapter: account %d has no base_url", account.ID)
		}
		path := "/api/v1/services/aigc/multimodal-generation/generation"
		if extra := account.GetCredential("video_create_path"); strings.TrimSpace(extra) != "" {
			path = normalizeVideoURLPath(extra)
		}
		return baseURL + path, nil
	}
	return a
}

func buildAliyunTTSBody(req MediaCreateRequest) []byte {
	messages := []map[string]any{{
		"role":    "user",
		"content": []map[string]any{{"text": req.Prompt}},
	}}
	body := map[string]any{
		"model": req.UpstreamModel,
		"input": map[string]any{"messages": messages},
	}
	params := map[string]any{}
	for k, v := range req.Extra {
		switch k {
		case "model", "prompt", "media", "video_create_path", "input", "parameters",
			// 幂等标识不能进上游请求体，见 video_adapter.go 同处说明。
			"request_id":
			continue
		}
		params[k] = v
	}
	if len(params) > 0 {
		body["parameters"] = params
	}
	data, _ := json.Marshal(body)
	return data
}

func parseAliyunTTSResult(respBody []byte, statusCode int) (*MediaCreateResult, error) {
	if statusCode >= 400 {
		return &MediaCreateResult{
			Status: "failed", Mode: MediaCompletionFailed, UpstreamStatusCode: statusCode, UpstreamRaw: respBody,
			ErrorMessage: fmt.Sprintf("upstream returned %d: %s", statusCode, string(respBody)),
		}, nil
	}
	var data map[string]any
	if err := json.Unmarshal(respBody, &data); err != nil {
		return nil, fmt.Errorf("aliyun tts unmarshal response: %w", err)
	}
	url := ""
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			for _, val := range t {
				walk(val)
			}
		case []any:
			for _, val := range t {
				walk(val)
			}
		case string:
			if strings.HasPrefix(t, "http") && url == "" {
				url = t
			}
		}
	}
	walk(data)
	// 同上：遍历兜底也没找到 http 开头的产物，就是没有产物。
	if url == "" {
		return &MediaCreateResult{
			Status: "failed", Mode: MediaCompletionFailed, UpstreamStatusCode: statusCode, UpstreamRaw: respBody,
			ErrorMessage: "dashscope tts response contained no audio url",
		}, nil
	}
	return &MediaCreateResult{
		Status: "succeeded", Mode: MediaCompletionSync, InlineURL: url, UpstreamStatusCode: statusCode, UpstreamRaw: respBody,
	}, nil
}
