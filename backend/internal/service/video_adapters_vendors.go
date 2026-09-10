package service

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

// vendorVideoAdapter is a thin HTTP adapter used by vendor-specific video
// implementations. It keeps the base URL, authorization and status parsing
// logic in one place, while vendor-specific builders translate wire fields.
//
// The concrete vendor adapters below are kept in separate small types (rather
// than one giant switch) so an upstream API change can be isolated to one
// file and one builder pair.
type vendorVideoAdapter struct {
	name           string
	plugin         string
	httpClient     *http.Client
	supports       func(platform, model string) bool
	buildCreate    func(VideoCreateRequest) []byte
	parseCreate    func(respBody []byte, statusCode int) (*VideoCreateResult, error)
	buildCreateURL func(account *Account) (string, error)
	buildQuery     func(account *Account, upstreamTaskID string) (string, error)
	createHeaders  func(account *Account) map[string]string
	parseQuery     func(respBody []byte, statusCode int) (*VideoTaskResult, error)
}

func newVendorVideoAdapter(name string) vendorVideoAdapter {
	return vendorVideoAdapter{
		name:   name,
		plugin: "video",
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (a vendorVideoAdapter) baseURL(account *Account) (string, error) {
	baseURL := strings.TrimRight(account.GetCredential("base_url"), "/")
	if baseURL == "" {
		return "", fmt.Errorf("%s video adapter: account %d has no base_url", a.name, account.ID)
	}
	return baseURL, nil
}

func (a vendorVideoAdapter) apiKey(account *Account) (string, error) {
	apiKey := account.GetCredential("api_key")
	if apiKey == "" {
		return "", fmt.Errorf("%s video adapter: account %d has no api_key", a.name, account.ID)
	}
	return apiKey, nil
}

func (a vendorVideoAdapter) do(ctx context.Context, account *Account, method, url string, body []byte) ([]byte, int, error) {
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

func (a vendorVideoAdapter) Create(ctx context.Context, account *Account, req VideoCreateRequest) (*VideoCreateResult, error) {
	if a.buildCreate == nil || a.parseCreate == nil {
		return nil, fmt.Errorf("%s video adapter is missing create handlers", a.name)
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
		url = baseURL + "/v1/images/generations/video"
	}

	respBody, statusCode, err := a.do(ctx, account, http.MethodPost, url, a.buildCreate(req))
	if err != nil {
		return nil, fmt.Errorf("%s video create request: %w", a.name, err)
	}
	return a.parseCreate(respBody, statusCode)
}

// Cancel 通过 DELETE 到任务查询端点尝试取消上游任务（很多厂商支持删除/取消）。
// 由于部分厂商不支持，失败仅记录日志，不向上抛。由 MediaTaskService 尽力调用。
func (a vendorVideoAdapter) Cancel(ctx context.Context, account *Account, upstreamTaskID string) error {
	if a.buildQuery == nil {
		return fmt.Errorf("%s video adapter has no query builder for cancel", a.name)
	}
	url, err := a.buildQuery(account, upstreamTaskID)
	if err != nil {
		return fmt.Errorf("%s video adapter build cancel url: %w", a.name, err)
	}
	respBody, statusCode, err := a.do(ctx, account, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("%s video adapter cancel do: %w", a.name, err)
	}
	if statusCode >= 400 {
		return fmt.Errorf("%s video adapter cancel returned %d: %s", a.name, statusCode, string(respBody))
	}
	return nil
}

func (a vendorVideoAdapter) GetResult(ctx context.Context, account *Account, upstreamTaskID string) (*VideoTaskResult, error) {
	if a.buildQuery == nil || a.parseQuery == nil {
		return nil, fmt.Errorf("%s video adapter is missing query handlers", a.name)
	}
	url, err := a.buildQuery(account, upstreamTaskID)
	if err != nil {
		return nil, err
	}
	respBody, statusCode, err := a.do(ctx, account, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%s video query request: %w", a.name, err)
	}
	result, err := a.parseQuery(respBody, statusCode)
	if result != nil {
		result.UpstreamRaw = respBody
	}
	return result, err
}

func normalizeVideoURLPath(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		return "/" + p
	}
	return p
}

// --- Vendors ---

// SeedanceVideoAdapter supports ByteDance Seedance / Jimeng video models.
// The official public API tends to follow an OpenAI-compatible create/query
// shape, but several channels still differ in field names. All conversion is
// kept in build/parse helpers below so a channel change only affects this
// section.
type SeedanceVideoAdapter struct{ vendorVideoAdapter }

func NewSeedanceVideoAdapter() *SeedanceVideoAdapter {
	a := &SeedanceVideoAdapter{}
	a.vendorVideoAdapter = newVendorVideoAdapter("seedance")
	a.supports = func(platform, model string) bool {
		m := strings.ToLower(strings.TrimSpace(model))
		return strings.Contains(m, "seedance") ||
			strings.HasPrefix(m, "doubao-seedance") ||
			strings.Contains(m, "jimeng-video")
	}
	a.buildCreate = buildSeedanceVideoCreateBody
	a.parseCreate = parseSeedanceVideoCreateResult
	a.buildCreateURL = buildSeedanceVideoCreateURL
	a.buildQuery = buildSeedanceVideoQueryURL
	a.parseQuery = parseSeedanceVideoQueryResult
	return a
}

func (*SeedanceVideoAdapter) Supports(platform, model string) bool {
	a := NewSeedanceVideoAdapter()
	return a.supports(platform, model)
}

// MiniMaxVideoAdapter supports MiniMax Hailuo / H3 video models.
// Public contract (2026-08):
//
//	POST /v2/video_generation
//	GET  /v2/query/video_generation/{task_id}
//
// The create payload uses a multi-modal `content` array rather than flat
// top-level fields. The adapter translates the neutral request into that
// shape here, so MiniMax API changes stay isolated to this file.
type MiniMaxVideoAdapter struct{ vendorVideoAdapter }

func NewMiniMaxVideoAdapter() *MiniMaxVideoAdapter {
	a := &MiniMaxVideoAdapter{}
	a.vendorVideoAdapter = newVendorVideoAdapter("minimax-video")
	a.supports = func(platform, model string) bool {
		m := strings.ToLower(strings.TrimSpace(model))
		return strings.HasPrefix(m, "minimax-hailuo") ||
			strings.HasPrefix(m, "minimax-video") ||
			strings.HasPrefix(m, "minimax-h3") ||
			(strings.HasPrefix(m, "video-") && strings.Contains(m, "hailuo"))
	}
	a.buildCreate = buildMiniMaxVideoCreateBody
	a.parseCreate = parseMiniMaxVideoCreateResult
	a.buildCreateURL = buildMiniMaxVideoCreateURL
	a.buildQuery = buildMiniMaxVideoQueryURL
	a.parseQuery = parseMiniMaxVideoQueryResult
	return a
}

func (*MiniMaxVideoAdapter) Supports(platform, model string) bool {
	a := NewMiniMaxVideoAdapter()
	return a.supports(platform, model)
}

// WanVideoAdapter supports Alibaba Wan / Wanx / Wan2.x video models.
// Alibaba DashScope uses model-specific paths and an async task endpoint,
// which is more divergent than MiniMax or Seedance. The adapter therefore
// resolves create/query paths through explicit helper functions.
type WanVideoAdapter struct{ vendorVideoAdapter }

func NewWanVideoAdapter() *WanVideoAdapter {
	a := &WanVideoAdapter{}
	a.vendorVideoAdapter = newVendorVideoAdapter("wan-video")
	a.supports = func(platform, model string) bool {
		m := strings.ToLower(strings.TrimSpace(model))
		return strings.HasPrefix(m, "wan") &&
			(strings.Contains(m, "video") || strings.Contains(m, "wanx") ||
				strings.Contains(m, "t2v") || strings.HasPrefix(m, "wan2") || strings.HasPrefix(m, "wan3"))
	}
	a.buildCreate = buildWanVideoCreateBody
	a.parseCreate = parseWanVideoCreateResult
	a.buildCreateURL = buildWanVideoCreateURL
	a.buildQuery = buildWanVideoQueryURL
	a.parseQuery = parseWanVideoQueryResult
	a.createHeaders = func(*Account) map[string]string {
		return map[string]string{"X-DashScope-Async": "enable"}
	}
	return a
}

func (*WanVideoAdapter) Supports(platform, model string) bool {
	a := NewWanVideoAdapter()
	return a.supports(platform, model)
}


// --- Shared request builders ---

// seedanceVideoContent converts the unified request to Volcano Ark content items.
// Explicit media is preserved; otherwise the flattened image/video/audio fields
// are used. Prompt is omitted when empty so reference-only requests stay valid.
func seedanceVideoContent(req VideoCreateRequest) []map[string]any {
	content := make([]map[string]any, 0, len(req.Media)+len(req.ImageRefURLs)+len(req.VideoRefURLs)+len(req.AudioRefURLs)+1)
	if strings.TrimSpace(req.Prompt) != "" {
		content = append(content, map[string]any{"type": "text", "text": req.Prompt})
	}
	if len(req.Media) > 0 {
		for _, item := range req.Media {
			switch strings.ToLower(strings.TrimSpace(item.Type)) {
			case "first_frame", "last_frame", "reference_image":
				content = append(content, map[string]any{
					"type": "image_url", "role": item.Type, "image_url": map[string]any{"url": item.URL},
				})
			case "reference_video":
				content = append(content, map[string]any{
					"type": "video_url", "role": item.Type, "video_url": map[string]any{"url": item.URL},
				})
			case "reference_audio":
				content = append(content, map[string]any{
					"type": "audio_url", "role": item.Type, "audio_url": map[string]any{"url": item.URL},
				})
			}
		}
		return content
	}
	for i, imageURL := range req.ImageRefURLs {
		role := "reference_image"
		if i == 0 {
			role = "first_frame"
		}
		content = append(content, map[string]any{
			"type": "image_url", "role": role, "image_url": map[string]any{"url": imageURL},
		})
	}
	for _, videoURL := range req.VideoRefURLs {
		content = append(content, map[string]any{
			"type": "video_url", "role": "reference_video", "video_url": map[string]any{"url": videoURL},
		})
	}
	for _, audioURL := range req.AudioRefURLs {
		content = append(content, map[string]any{
			"type": "audio_url", "role": "reference_audio", "audio_url": map[string]any{"url": audioURL},
		})
	}
	return content
}

// minimaxVideoContent converts the unified request to MiniMax v2 content items.
// MiniMax requires a non-empty text prompt; media roles are mapped onto the
// content item role field. Unsupported file/link types are ignored.
func minimaxVideoContent(req VideoCreateRequest) []map[string]any {
	content := make([]map[string]any, 0, len(req.Media)+len(req.ImageRefURLs)+len(req.VideoRefURLs)+len(req.AudioRefURLs)+1)
	if strings.TrimSpace(req.Prompt) != "" {
		content = append(content, map[string]any{"type": "text", "text": req.Prompt})
	} else {
		content = append(content, map[string]any{"type": "text", "text": ""})
	}
	if len(req.Media) > 0 {
		for _, item := range req.Media {
			switch strings.ToLower(strings.TrimSpace(item.Type)) {
			case "first_frame", "last_frame", "reference_image":
				content = append(content, map[string]any{
					"type": "image_url", "role": item.Type, "image_url": map[string]any{"url": item.URL},
				})
			case "reference_video":
				content = append(content, map[string]any{
					"type": "video_url", "role": item.Type, "video_url": map[string]any{"url": item.URL},
				})
			case "reference_audio":
				content = append(content, map[string]any{
					"type": "audio_url", "role": item.Type, "audio_url": map[string]any{"url": item.URL},
				})
			}
		}
		return content
	}
	for i, imageURL := range req.ImageRefURLs {
		role := "reference_image"
		if i == 0 {
			role = "first_frame"
		}
		content = append(content, map[string]any{
			"type": "image_url", "role": role, "image_url": map[string]any{"url": imageURL},
		})
	}
	for _, videoURL := range req.VideoRefURLs {
		content = append(content, map[string]any{
			"type": "video_url", "role": "reference_video", "video_url": map[string]any{"url": videoURL},
		})
	}
	for _, audioURL := range req.AudioRefURLs {
		content = append(content, map[string]any{
			"type": "audio_url", "role": "reference_audio", "audio_url": map[string]any{"url": audioURL},
		})
	}
	return content
}

func buildSeedanceVideoCreateBody(req VideoCreateRequest) []byte {
	content := seedanceVideoContent(req)
	body := map[string]any{
		"model":   req.UpstreamModel,
		"content": content,
	}
	if req.Resolution != "" {
		body["resolution"] = req.Resolution
	}
	if req.Ratio != "" {
		body["ratio"] = req.Ratio
	}
	if req.DurationSec > 0 {
		body["duration"] = req.DurationSec
	}
	if req.Seed != nil {
		body["seed"] = *req.Seed
	}
	mergeVideoExtra(body, req.Extra)
	data, _ := json.Marshal(body)
	return data
}

func buildMiniMaxVideoCreateBody(req VideoCreateRequest) []byte {
	content := minimaxVideoContent(req)
	body := map[string]any{
		"model":   req.UpstreamModel,
		"content": content,
	}
	if req.Resolution != "" {
		body["resolution"] = req.Resolution
	}
	if req.DurationSec > 0 {
		body["duration"] = req.DurationSec
	}
	if req.Extra != nil {
		if ratio, ok := req.Extra["ratio"].(string); ok && strings.TrimSpace(ratio) != "" {
			body["ratio"] = ratio
		}
	}
	mergeVideoExtra(body, req.Extra)
	data, _ := json.Marshal(body)
	return data
}

func buildWanVideoCreateBody(req VideoCreateRequest) []byte {
	input := map[string]any{}
	if strings.TrimSpace(req.Prompt) != "" {
		input["prompt"] = req.Prompt
	}
	if media := wanVideoMedia(req); len(media) > 0 {
		input["media"] = media
	}
	body := map[string]any{
		"model": req.UpstreamModel,
		"input": input,
	}

	params := map[string]any{}
	if req.Resolution != "" {
		params["resolution"] = req.Resolution
	}
	if req.Ratio != "" {
		params["ratio"] = req.Ratio
	}
	if req.DurationSec != 0 {
		params["duration"] = req.DurationSec
	}
	if req.Seed != nil {
		params["seed"] = *req.Seed
	}
	if len(params) > 0 {
		body["parameters"] = params
	}
	mergeVideoExtra(body, req.Extra)
	data, _ := json.Marshal(body)
	return data
}


// wanVideoMedia converts the unified request into DashScope Wan media objects.
// Explicit `media` (from all-in-one callers) is preserved as-is; the flattened
// image/video/audio fields are mapped onto first_frame/reference_video/
// reference_audio semantics so simple OpenAI-style clients still work.
func wanVideoMedia(req VideoCreateRequest) []map[string]any {
	media := make([]map[string]any, 0, len(req.Media)+len(req.ImageRefURLs)+len(req.VideoRefURLs)+len(req.AudioRefURLs))
	for _, item := range req.Media {
		if strings.TrimSpace(item.Type) == "" || strings.TrimSpace(item.URL) == "" {
			continue
		}
		media = append(media, map[string]any{"type": item.Type, "url": item.URL})
	}
	if len(req.Media) == 0 {
		for i, url := range req.ImageRefURLs {
			mediaType := "reference_image"
			if i == 0 {
				mediaType = "first_frame"
			}
			media = append(media, map[string]any{"type": mediaType, "url": url})
		}
	}
	for _, url := range req.VideoRefURLs {
		media = append(media, map[string]any{"type": "reference_video", "url": url})
	}
	for _, url := range req.AudioRefURLs {
		media = append(media, map[string]any{"type": "reference_audio", "url": url})
	}
	return media
}

func mergeVideoExtra(body map[string]any, extra map[string]any) {
	// 这些字段已经由统一解析或 adapter 显式构造，原始请求值不能覆盖规范化结果。
	reserved := map[string]struct{}{
		"model": {}, "content": {}, "input": {}, "parameters": {},
		"prompt": {}, "negative_prompt": {},
		"image": {}, "image_url": {}, "image_urls": {}, "video_url": {}, "video_urls": {},
		"audio_url": {}, "audio_urls": {}, "media": {},
		"resolution": {}, "ratio": {}, "duration": {}, "duration_sec": {}, "seed": {},
		"video_create_path": {}, "video_query_path": {},
	}
	for k, v := range extra {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		if _, ok := reserved[key]; ok {
			continue
		}
		body[key] = v
	}
}

func buildSeedanceVideoCreateURL(account *Account) (string, error) {
	baseURL := strings.TrimRight(account.GetCredential("base_url"), "/")
	if baseURL == "" {
		return "", fmt.Errorf("video adapter: account %d has no base_url", account.ID)
	}
	path := "/api/v3/contents/generations/tasks"
	if extra := account.GetCredential("video_create_path"); strings.TrimSpace(extra) != "" {
		path = normalizeVideoURLPath(extra)
	}
	return baseURL + path, nil
}

func buildMiniMaxVideoCreateURL(account *Account) (string, error) {
	baseURL := strings.TrimRight(account.GetCredential("base_url"), "/")
	if baseURL == "" {
		return "", fmt.Errorf("video adapter: account %d has no base_url", account.ID)
	}
	path := "/v2/video_generation"
	if extra := account.GetCredential("video_create_path"); strings.TrimSpace(extra) != "" {
		path = normalizeVideoURLPath(extra)
	}
	return baseURL + path, nil
}

func buildWanVideoCreateURL(account *Account) (string, error) {
	baseURL := strings.TrimRight(account.GetCredential("base_url"), "/")
	if baseURL == "" {
		return "", fmt.Errorf("video adapter: account %d has no base_url", account.ID)
	}
	path := "/api/v1/services/aigc/video-generation/video-synthesis"
	if extra := account.GetCredential("video_create_path"); strings.TrimSpace(extra) != "" {
		path = normalizeVideoURLPath(extra)
	}
	return baseURL + path, nil
}

// buildSeedanceVideoQueryURL targets the Volcano Ark v3 task endpoint.
func buildSeedanceVideoQueryURL(account *Account, upstreamTaskID string) (string, error) {
	baseURL := strings.TrimRight(account.GetCredential("base_url"), "/")
	if baseURL == "" {
		return "", fmt.Errorf("video adapter: account %d has no base_url", account.ID)
	}
	path := "/api/v3/contents/generations/tasks/" + upstreamTaskID
	if extra := account.GetCredential("video_query_path"); strings.TrimSpace(extra) != "" {
		path = normalizeVideoURLPath(extra) + "/" + upstreamTaskID
	}
	return baseURL + path, nil
}

// --- Shared query builders ---

func buildGenericVideoQueryURL(account *Account, upstreamTaskID string) (string, error) {
	baseURL := strings.TrimRight(account.GetCredential("base_url"), "/")
	if baseURL == "" {
		return "", fmt.Errorf("video adapter: account %d has no base_url", account.ID)
	}
	if extra := account.GetCredential("video_query_path"); strings.TrimSpace(extra) != "" {
		return baseURL + normalizeVideoURLPath(extra) + "/" + upstreamTaskID, nil
	}
	return baseURL + "/v1/videos/" + upstreamTaskID, nil
}

func buildMiniMaxVideoQueryURL(account *Account, upstreamTaskID string) (string, error) {
	baseURL := strings.TrimRight(account.GetCredential("base_url"), "/")
	if baseURL == "" {
		return "", fmt.Errorf("video adapter: account %d has no base_url", account.ID)
	}
	path := "/v2/query/video_generation/" + upstreamTaskID
	if extra := account.GetCredential("video_query_path"); strings.TrimSpace(extra) != "" {
		path = normalizeVideoURLPath(extra) + "/" + upstreamTaskID
	}
	return baseURL + path, nil
}

func buildWanVideoQueryURL(account *Account, upstreamTaskID string) (string, error) {
	baseURL := strings.TrimRight(account.GetCredential("base_url"), "/")
	if baseURL == "" {
		return "", fmt.Errorf("video adapter: account %d has no base_url", account.ID)
	}
	path := "/api/v1/tasks/" + upstreamTaskID
	if extra := account.GetCredential("video_query_path"); strings.TrimSpace(extra) != "" {
		path = normalizeVideoURLPath(extra) + "/" + upstreamTaskID
	}
	return baseURL + path, nil
}

// --- Shared response parsers ---

func parseGenericVideoCreateResult(respBody []byte, statusCode int) (*VideoCreateResult, error) {
	if statusCode >= 400 {
		return &VideoCreateResult{
			Status:             "failed",
			Mode:               VideoCompletionFailed,
			UpstreamStatusCode: statusCode,
			UpstreamRaw:        respBody,
			ErrorMessage:       fmt.Sprintf("upstream returned %d: %s", statusCode, string(respBody)),
		}, nil
	}
	var resp struct {
		ID     string `json:"id"`
		TaskID string `json:"task_id"`
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, err
	}
	taskID := firstNonEmptyString(resp.ID, resp.TaskID)
	if taskID == "" {
		return &VideoCreateResult{
			Status:             "failed",
			Mode:               VideoCompletionFailed,
			UpstreamStatusCode: statusCode,
			UpstreamRaw:        respBody,
			ErrorMessage:       "upstream create response missing task id",
		}, nil
	}
	status := normalizeVideoTaskStatus(firstNonEmptyString(resp.Status, "processing"))
	result := &VideoCreateResult{
		TaskID:             taskID,
		Status:             status,
		Mode:               videoCompletionModeForStatus(status),
		UpstreamStatusCode: statusCode,
		UpstreamRaw:        respBody,
		ErrorMessage:       resp.Error,
	}
	return result, nil
}

func parseSeedanceVideoCreateResult(respBody []byte, statusCode int) (*VideoCreateResult, error) {
	return parseGenericVideoCreateResultWithPaths(respBody, statusCode, "task_id", "data.task_id", "id")
}

func parseSeedanceVideoQueryResult(respBody []byte, statusCode int) (*VideoTaskResult, error) {
	result := &VideoTaskResult{StatusCode: statusCode, UpstreamRaw: respBody}
	if statusCode >= 400 {
		result.Status = "failed"
		result.ErrorMessage = fmt.Sprintf("upstream returned %d: %s", statusCode, string(respBody))
		return result, nil
	}
	var data map[string]any
	_ = json.Unmarshal(respBody, &data)
	result.Status = normalizeVideoTaskStatus(firstNonEmptyString(stringAtPath(data, "status", "data.status"), "processing"))
	result.VideoURL = stringAtPath(data, "content.video_url", "data.video_url", "video_url")
	result.ThumbnailURL = stringAtPath(data, "content.thumbnail_url", "data.thumbnail_url", "thumbnail_url")
	result.DurationSec = intAtPath(data, "duration", "data.duration", "output.duration")
	result.ErrorMessage = stringAtPath(data, "error.message", "message")
	return result, nil
}

func parseMiniMaxVideoCreateResult(respBody []byte, statusCode int) (*VideoCreateResult, error) {
	// MiniMax returns task in `task_id` or `data.task_id`.
	base, err := parseGenericVideoCreateResult(respBody, statusCode)
	if err != nil {
		return nil, err
	}
	_ = base
	return parseGenericVideoCreateResultWithPaths(respBody, statusCode, "data.task_id", "task_id")
}

func parseGenericVideoCreateResultWithPaths(respBody []byte, statusCode int, paths ...string) (*VideoCreateResult, error) {
	if statusCode >= 400 {
		return parseGenericVideoCreateResult(respBody, statusCode)
	}
	var data map[string]any
	if err := json.Unmarshal(respBody, &data); err != nil {
		return nil, err
	}
	taskID := firstStringAtPath(data, paths...)
	if taskID == "" {
		return &VideoCreateResult{
			Status:             "failed",
			Mode:               VideoCompletionFailed,
			UpstreamStatusCode: statusCode,
			UpstreamRaw:        respBody,
			ErrorMessage:       "upstream create response missing task id",
		}, nil
	}
	status := normalizeVideoTaskStatus(firstNonEmptyString(stringAtPath(data, "status", "data.status"), "processing"))
	return &VideoCreateResult{
		TaskID:             taskID,
		Status:             status,
		Mode:               videoCompletionModeForStatus(status),
		UpstreamStatusCode: statusCode,
		UpstreamRaw:        respBody,
		ErrorMessage:       stringAtPath(data, "error.message", "message"),
	}, nil
}

func parseGenericVideoQueryResult(respBody []byte, statusCode int) (*VideoTaskResult, error) {
	result := &VideoTaskResult{StatusCode: statusCode, UpstreamRaw: respBody}
	if statusCode >= 400 {
		result.Status = "failed"
		result.ErrorMessage = fmt.Sprintf("upstream returned %d: %s", statusCode, string(respBody))
		return result, nil
	}
	var resp struct {
		Status       string `json:"status"`
		VideoURL     string `json:"video_url"`
		URL          string `json:"url"`
		ThumbnailURL string `json:"thumbnail_url"`
		Duration     int    `json:"duration_sec"`
		DurationSec  int    `json:"durationSec"`
		Error        string `json:"error"`
		ErrorMessage string `json:"error_message"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, err
	}
	result.Status = normalizeVideoTaskStatus(firstNonEmptyString(resp.Status, "processing"))
	result.VideoURL = firstNonEmptyString(resp.VideoURL, resp.URL)
	result.ThumbnailURL = resp.ThumbnailURL
	result.DurationSec = firstNonZero(resp.Duration, resp.DurationSec)
	result.ErrorMessage = firstNonEmptyString(resp.Error, resp.ErrorMessage)
	return result, nil
}

func parseMiniMaxVideoQueryResult(respBody []byte, statusCode int) (*VideoTaskResult, error) {
	result := &VideoTaskResult{StatusCode: statusCode, UpstreamRaw: respBody}
	if statusCode >= 400 {
		result.Status = "failed"
		result.ErrorMessage = fmt.Sprintf("upstream returned %d: %s", statusCode, string(respBody))
		return result, nil
	}
	var data map[string]any
	_ = json.Unmarshal(respBody, &data)
	result.Status = normalizeVideoTaskStatus(firstNonEmptyString(stringAtPath(data, "task.status", "status"), "processing"))
	result.VideoURL = stringAtPath(data, "task.content.url", "content.url", "video_url")
	result.ThumbnailURL = stringAtPath(data, "task.content.thumbnail_url", "thumbnail_url")
	result.DurationSec = intAtPath(data, "task.duration", "duration")
	result.ErrorMessage = stringAtPath(data, "task.error.message", "error.message", "message")
	return result, nil
}

func parseWanVideoCreateResult(respBody []byte, statusCode int) (*VideoCreateResult, error) {
	return parseGenericVideoCreateResultWithPaths(respBody, statusCode, "output.task_id", "data.task_id", "task_id")
}

func parseWanVideoQueryResult(respBody []byte, statusCode int) (*VideoTaskResult, error) {
	result := &VideoTaskResult{StatusCode: statusCode, UpstreamRaw: respBody}
	if statusCode >= 400 {
		result.Status = "failed"
		result.ErrorMessage = fmt.Sprintf("upstream returned %d: %s", statusCode, string(respBody))
		return result, nil
	}
	var data map[string]any
	_ = json.Unmarshal(respBody, &data)
	result.Status = normalizeVideoTaskStatus(firstNonEmptyString(stringAtPath(data, "output.task_status", "output.status", "task_status", "status"), "processing"))
	result.VideoURL = stringAtPath(data, "output.video_url", "video_url")
	result.ThumbnailURL = stringAtPath(data, "output.thumbnail_url", "thumbnail_url")
	result.DurationSec = intAtPath(data, "usage.duration", "output.duration", "duration")
	result.ErrorMessage = stringAtPath(data, "message", "output.message", "error.message")
	return result, nil
}


// --- small path helpers ---

func stringAtPath(data map[string]any, paths ...string) string {
	for _, path := range paths {
		if v := valueAtPath(data, path); v != "" {
			return v
		}
	}
	return ""
}

func intAtPath(data map[string]any, paths ...string) int {
	for _, path := range paths {
		if v := valueAtPath(data, path); v != "" {
			var n int
			if _, err := fmt.Sscan(v, &n); err == nil {
				return n
			}
		}
	}
	return 0
}

func firstStringAtPath(data map[string]any, paths ...string) string {
	return stringAtPath(data, paths...)
}

func valueAtPath(data map[string]any, path string) string {
	parts := strings.Split(path, ".")
	var current any = data
	for _, p := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = m[p]
	}
	switch v := current.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f", v)
	case json.Number:
		return v.String()
	default:
		return ""
	}
}

func firstNonZero(values ...int) int {
	for _, v := range values {
		if v != 0 {
			return v
		}
	}
	return 0
}
