package service

// media_vendor_image_adapter.go — 图片生成独立厂商 adapter（Seedance / Wan / MiniMax）。
// 图片生成接口大多为同步返回 URL（或 base64），与视频的异步任务不同。
// 每个厂商用独立 builder/parser，字段契约差异只影响本文件。
// 通用 OpenAI-compatible 图片 adapter 仍保留在 media_adapters_extra.go 作为 fallback。

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// mediaVendorImageAdapter 是图片生成厂商 adapter 底座。
type mediaVendorImageAdapter struct {
	name           string
	httpClient     *http.Client
	supports       func(platform, model string) bool
	buildCreate    func(MediaCreateRequest) []byte
	parseCreate    func(respBody []byte, statusCode int) (*MediaCreateResult, error)
	buildCreateURL func(account *Account) (string, error)
	createHeaders  func(account *Account) map[string]string
}

func newMediaVendorImageAdapter(name string) mediaVendorImageAdapter {
	return mediaVendorImageAdapter{
		name: name,
		httpClient: &http.Client{
			// 图片生成远慢于文本：千问图像 3.0 默认开启 prompt_extend 与思考模式，
			// 官方建议客户端超时从 600 秒起配。120 秒会在上游仍正常生成时提前断开，
			// 表现为"任务失败"但上游其实在计费。
			Timeout: 600 * time.Second,
		},
	}
}

func (a mediaVendorImageAdapter) Kind() MediaKind { return MediaKindImage }

func (a mediaVendorImageAdapter) Supports(platform, model string) bool {
	if a.supports != nil {
		return a.supports(platform, model)
	}
	return false
}

func (a mediaVendorImageAdapter) baseURL(account *Account) (string, error) {
	baseURL := strings.TrimRight(account.GetCredential("base_url"), "/")
	if baseURL == "" {
		return "", fmt.Errorf("%s image adapter: account %d has no base_url", a.name, account.ID)
	}
	return baseURL, nil
}

func (a mediaVendorImageAdapter) apiKey(account *Account) (string, error) {
	apiKey := account.GetCredential("api_key")
	if apiKey == "" {
		return "", fmt.Errorf("%s image adapter: account %d has no api_key", a.name, account.ID)
	}
	return apiKey, nil
}

func (a mediaVendorImageAdapter) do(ctx context.Context, account *Account, method, url string, body []byte) ([]byte, int, error) {
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

func (a mediaVendorImageAdapter) Create(ctx context.Context, account *Account, req MediaCreateRequest) (*MediaCreateResult, error) {
	if a.buildCreate == nil || a.parseCreate == nil {
		return nil, fmt.Errorf("%s image adapter is missing create handlers", a.name)
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
		url = baseURL + "/v1/images/generations"
	}
	respBody, statusCode, err := a.do(ctx, account, http.MethodPost, url, a.buildCreate(req))
	if err != nil {
		return nil, fmt.Errorf("%s image create request: %w", a.name, err)
	}
	return a.parseCreate(respBody, statusCode)
}

// GetResult 图片生成一般同步返回 URL；若上游返回 task_id（异步任务）则走查询。
// 这里默认返回未知 status，MediaTaskService 会把无 URL 且 status 非终态的任务
// 交给 Worker 轮询。若厂商图片接口确有异步任务，可覆写该默认。
func (a mediaVendorImageAdapter) GetResult(ctx context.Context, account *Account, upstreamTaskID string) (*MediaTaskResult, error) {
	baseURL, err := a.baseURL(account)
	if err != nil {
		return nil, err
	}
	apiKey, err := a.apiKey(account)
	if err != nil {
		return nil, err
	}
	url := baseURL + "/v1/images/generations/" + upstreamTaskID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%s image get request: %w", a.name, err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	account.ApplyHeaderOverrides(req.Header)
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s image get do: %w", a.name, err)
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
		return nil, fmt.Errorf("%s image unmarshal get response: %w", a.name, err)
	}
	result.Status = normalizeMediaTaskStatus(firstNonEmptyString(respData.Status, "processing"))
	result.URL = respData.URL
	if result.URL == "" && len(respData.Data) > 0 {
		result.URL = respData.Data[0].URL
	}
	return result, nil
}

// --- Seedance 图片 ---

type SeedanceImageAdapter struct{ mediaVendorImageAdapter }

func NewSeedanceImageAdapter() *SeedanceImageAdapter {
	a := &SeedanceImageAdapter{}
	a.mediaVendorImageAdapter = newMediaVendorImageAdapter("seedance-image")
	a.supports = func(platform, model string) bool {
		m := strings.ToLower(strings.TrimSpace(model))
		return strings.Contains(m, "seedream") ||
			strings.Contains(m, "doubao-seedream") ||
			strings.Contains(m, "seedance-image") ||
			strings.Contains(m, "jimeng-image")
	}
	a.buildCreate = buildSeedanceImageCreateBody
	a.parseCreate = parseSeedanceImageCreateResult
	a.buildCreateURL = func(account *Account) (string, error) {
		baseURL := strings.TrimRight(account.GetCredential("base_url"), "/")
		if baseURL == "" {
			return "", fmt.Errorf("seedance image adapter: account %d has no base_url", account.ID)
		}
		path := "/api/v3/images/generations"
		if extra := account.GetCredential("video_create_path"); strings.TrimSpace(extra) != "" {
			path = normalizeVideoURLPath(extra)
		}
		return baseURL + path, nil
	}
	return a
}

func buildSeedanceImageCreateBody(req MediaCreateRequest) []byte {
	body := map[string]any{
		"model":  req.UpstreamModel,
		"prompt": req.Prompt,
	}
	if len(req.ImageRefURLs) > 0 {
		body["image"] = req.ImageRefURLs
	}
	if req.Resolution != "" {
		body["size"] = req.Resolution
	}
	if req.Seed != nil {
		body["seed"] = *req.Seed
	}
	for k, v := range req.Extra {
		switch k {
		case "model", "prompt", "image", "image_urls", "size", "resolution", "seed", "media", "video_create_path":
			continue
		}
		body[k] = v
	}
	data, _ := json.Marshal(body)
	return data
}

func parseSeedanceImageCreateResult(respBody []byte, statusCode int) (*MediaCreateResult, error) {
	if statusCode >= 400 {
		return &MediaCreateResult{
			Status: "failed", Mode: MediaCompletionFailed, UpstreamStatusCode: statusCode, UpstreamRaw: respBody,
			ErrorMessage: fmt.Sprintf("upstream returned %d: %s", statusCode, string(respBody)),
		}, nil
	}
	var resp struct {
		Data []struct {
			URL     string `json:"url"`
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("seedance image unmarshal response: %w", err)
	}
	url := ""
	if len(resp.Data) > 0 {
		url = resp.Data[0].URL
	}
	return &MediaCreateResult{
		Status: "succeeded", Mode: MediaCompletionSync, InlineURL: url, UpstreamStatusCode: statusCode, UpstreamRaw: respBody,
	}, nil
}

// --- Wan 图片 ---

type WanImageAdapter struct{ mediaVendorImageAdapter }

func NewWanImageAdapter() *WanImageAdapter {
	a := &WanImageAdapter{}
	a.mediaVendorImageAdapter = newMediaVendorImageAdapter("wan-image")
	a.supports = func(platform, model string) bool {
		m := strings.ToLower(strings.TrimSpace(model))
		return strings.HasPrefix(m, "wan2") ||
			strings.HasPrefix(m, "wanx") ||
			strings.Contains(m, "qwen-image") ||
			strings.Contains(m, "t2i")
	}
	a.buildCreate = buildWanImageCreateBody
	a.parseCreate = parseWanImageCreateResult
	a.buildCreateURL = func(account *Account) (string, error) {
		baseURL := strings.TrimRight(account.GetCredential("base_url"), "/")
		if baseURL == "" {
			return "", fmt.Errorf("wan image adapter: account %d has no base_url", account.ID)
		}
		path := "/api/v1/services/aigc/multimodal-generation/generation"
		if extra := account.GetCredential("video_create_path"); strings.TrimSpace(extra) != "" {
			path = normalizeVideoURLPath(extra)
		}
		return baseURL + path, nil
	}
	return a
}

// --- DashScope 图片公共契约（wanx / qwen-image 同属阿里 DashScope 系）---

const (
	// dashScopeImageMaxCount 是单次生成允许的最大张数（千问图像 3.0 为 1-6）。
	dashScopeImageMaxCount = 6
	// dashScopeImageMaxRefs 是图生图允许的最大参考图数量（官方为 1-3 张）。
	dashScopeImageMaxRefs = 3
)

// normalizeDashScopeImageSize 把 OpenAI 写法的 "1024x1024" 转成 DashScope 的 "1024*1024"。
// 两种协议的分隔符不同：OpenAI 用字母 x，DashScope 用星号。原样下发 x 会被上游
// 判为参数非法，官方迁移文档亦专门提示过这一点。
func normalizeDashScopeImageSize(size string) string {
	trimmed := strings.TrimSpace(size)
	if trimmed == "" || !strings.ContainsAny(trimmed, "xX") {
		return trimmed
	}
	replaced := strings.ReplaceAll(strings.ReplaceAll(trimmed, "x", "*"), "X", "*")
	return replaced
}

// clampDashScopeImageCount 把生成张数收敛到 DashScope 允许的 [1,6]。
func clampDashScopeImageCount(n int) int {
	if n < 1 {
		return 1
	}
	if n > dashScopeImageMaxCount {
		return dashScopeImageMaxCount
	}
	return n
}

func buildWanImageCreateBody(req MediaCreateRequest) []byte {
	content := make([]map[string]any, 0, dashScopeImageMaxRefs+1)
	// 图生图：DashScope 通过 content 里的 {"image": ...} 传参考图（官方 1-3 张）。
	// 参考图必须排在文本之前，否则模型无法定位待编辑的主体。
	refs := 0
	for _, ref := range req.ImageRefURLs {
		if trimmed := strings.TrimSpace(ref); trimmed != "" {
			content = append(content, map[string]any{"image": trimmed})
			refs++
			if refs >= dashScopeImageMaxRefs {
				break
			}
		}
	}
	content = append(content, map[string]any{"text": req.Prompt})
	body := map[string]any{
		"model": req.UpstreamModel,
		"input": map[string]any{"messages": []map[string]any{{"role": "user", "content": content}}},
	}
	params := map[string]any{"n": clampDashScopeImageCount(req.ImageCount)}
	if size := normalizeDashScopeImageSize(req.Resolution); size != "" {
		params["size"] = size
	}
	if req.Seed != nil {
		params["seed"] = *req.Seed
	}
	for k, v := range req.Extra {
		switch k {
		case "model", "prompt", "size", "resolution", "seed", "media", "video_create_path",
			"input", "parameters", "image", "image_url", "image_urls":
			continue
		}
		params[k] = v
	}
	body["parameters"] = params
	data, _ := json.Marshal(body)
	return data
}

// dashScopeImageResponse 是 DashScope 图片生成的同步响应。
// 失败时是 HTTP 200 + code/message（如 InvalidApiKey），只看状态码会误判为成功。
type dashScopeImageResponse struct {
	Output struct {
		Choices []struct {
			Message struct {
				Content []struct {
					Image string `json:"image"`
				} `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	} `json:"output"`
	Usage struct {
		OutputWidth  int `json:"output_width"`
		OutputHeight int `json:"output_height"`
	} `json:"usage"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

func parseWanImageCreateResult(respBody []byte, statusCode int) (*MediaCreateResult, error) {
	failed := func(message string) *MediaCreateResult {
		return &MediaCreateResult{
			Status:             "failed",
			Mode:               MediaCompletionFailed,
			UpstreamStatusCode: statusCode,
			UpstreamRaw:        respBody,
			ErrorMessage:       message,
		}
	}
	if statusCode >= 400 {
		return failed(fmt.Sprintf("upstream returned %d: %s", statusCode, string(respBody))), nil
	}
	var resp dashScopeImageResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("dashscope image unmarshal response: %w", err)
	}
	if code := strings.TrimSpace(resp.Code); code != "" {
		// 错误码必须进消息：运营排障时只有 message 无法定位失败类型。
		message := "dashscope " + code
		if msg := strings.TrimSpace(resp.Message); msg != "" {
			message += ": " + msg
		}
		return failed(message), nil
	}
	urls := make([]string, 0, len(resp.Output.Choices))
	for _, choice := range resp.Output.Choices {
		for _, part := range choice.Message.Content {
			if u := strings.TrimSpace(part.Image); u != "" {
				urls = append(urls, u)
			}
		}
	}
	if len(urls) == 0 {
		// 没有 URL 的"成功"无法交付给调用方，且会静默吞掉上游异常。
		return failed("dashscope image response contained no image url"), nil
	}
	result := &MediaCreateResult{
		Status:             "succeeded",
		Mode:               MediaCompletionSync,
		InlineURL:          urls[0],
		InlineURLs:         urls,
		UpstreamStatusCode: statusCode,
		UpstreamRaw:        respBody,
	}
	// 真实输出尺寸用于计费：不传 size 时模型自行推荐分辨率，按请求值收会错档。
	if resp.Usage.OutputWidth > 0 && resp.Usage.OutputHeight > 0 {
		result.UpstreamSize = fmt.Sprintf("%dx%d", resp.Usage.OutputWidth, resp.Usage.OutputHeight)
	}
	return result, nil
}

// --- MiniMax 图片 ---

type MiniMaxImageAdapter struct{ mediaVendorImageAdapter }

func NewMiniMaxImageAdapter() *MiniMaxImageAdapter {
	a := &MiniMaxImageAdapter{}
	a.mediaVendorImageAdapter = newMediaVendorImageAdapter("minimax-image")
	a.supports = func(platform, model string) bool {
		m := strings.ToLower(strings.TrimSpace(model))
		return strings.Contains(m, "image-01") ||
			strings.Contains(m, "minimax-image") ||
			strings.Contains(m, "hailuo-image")
	}
	a.buildCreate = buildMiniMaxImageCreateBody
	a.parseCreate = parseMiniMaxImageCreateResult
	a.buildCreateURL = func(account *Account) (string, error) {
		baseURL := strings.TrimRight(account.GetCredential("base_url"), "/")
		if baseURL == "" {
			return "", fmt.Errorf("minimax image adapter: account %d has no base_url", account.ID)
		}
		path := "/v1/image_generation"
		if extra := account.GetCredential("video_create_path"); strings.TrimSpace(extra) != "" {
			path = normalizeVideoURLPath(extra)
		}
		return baseURL + path, nil
	}
	return a
}

// normalizeMiniMaxImageModel 把带厂商前缀的别名收敛成 MiniMax 官方枚举值。
//
// MiniMax /v1/image_generation 的 model 是**枚举**，官方只接受 image-01 / image-01-live
// （platform.minimaxi.com 文档：model 可选项 image-01、image-01-live）。而本仓为了与
// 其它厂商统一命名，对外暴露的是 minimax-image-01 这类带前缀的名字。选号阶段若账号没配
// model_mapping，UpstreamModel 就等于客户端传的 public model，原样发给上游会被判
// 非法参数——表现为 HTTP 200 + base_resp.status_code != 0（或 400），图片永远出不来，
// 也就永远不会有成功的用量记录。这里做归一，让"客户端写 minimax-image-01"也能出图。
//
// 只收敛"带 minimax/hailuo 字样"的别名，其余名字（含渠道私有命名、image-01-live）
// 原样透传；需要指定任意其它名字时用账号 model_mapping 即可。
func normalizeMiniMaxImageModel(model string) string {
	m := strings.ToLower(strings.TrimSpace(model))
	switch m {
	case "", "image-01", "image-01-live":
		return m
	}
	if strings.Contains(m, "minimax") || strings.Contains(m, "hailuo") {
		if strings.Contains(m, "live") {
			return "image-01-live"
		}
		return "image-01"
	}
	return model
}

func buildMiniMaxImageCreateBody(req MediaCreateRequest) []byte {
	model := normalizeMiniMaxImageModel(req.UpstreamModel)
	body := map[string]any{
		"model":  model,
		"prompt": req.Prompt,
	}
	if req.Resolution != "" {
		if strings.Contains(req.Resolution, "x") {
			body["width"] = firstIntBeforeX(req.Resolution)
			body["height"] = firstIntAfterX(req.Resolution)
		} else {
			body["aspect_ratio"] = req.Resolution
		}
	}
	if req.Seed != nil {
		body["seed"] = *req.Seed
	}
	// n：MiniMax 取值 [1,9]。计费侧按同一张数收，避免上游出 9 张我们只收 1 张。
	if req.ImageCount > 1 {
		body["n"] = clampImageCount(req.ImageCount)
	}
	for k, v := range req.Extra {
		switch k {
		case "model", "prompt", "size", "resolution", "seed", "media", "video_create_path", "width", "height", "aspect_ratio", "n", "subject_reference":
			continue
		}
		body[k] = v
	}
	// 图生图：MiniMax 用 subject_reference 传参考图，且每次仅支持一张。
	// 用户显式传了 subject_reference 时不覆盖，保留其自定义（如 type 不是 character）。
	if _, exists := body["subject_reference"]; !exists {
		for _, ref := range req.ImageRefURLs {
			if trimmed := strings.TrimSpace(ref); trimmed != "" {
				body["subject_reference"] = []map[string]any{{"type": "character", "image_file": trimmed}}
				break
			}
		}
	}
	data, _ := json.Marshal(body)
	return data
}

// clampImageCount 把图片张数收敛到 MiniMax 允许的 [1,9]。
func clampImageCount(n int) int {
	if n < 1 {
		return 1
	}
	if n > 9 {
		return 9
	}
	return n
}

// miniMaxImageResponse 是 MiniMax /v1/image_generation 的响应结构。
// data 是对象而非数组：url 模式返回 image_urls，base64 模式返回 image_base64。
// 另外 MiniMax 用 HTTP 200 + base_resp.status_code 表达业务失败（如内容安全拦截），
// 只看 HTTP 状态码会把失败当成成功，最终得到一个没有 URL 的"成功"任务。
type miniMaxImageResponse struct {
	Data struct {
		ImageURLs   []string `json:"image_urls"`
		ImageBase64 []string `json:"image_base64"`
	} `json:"data"`
	BaseResp struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
}

func parseMiniMaxImageCreateResult(respBody []byte, statusCode int) (*MediaCreateResult, error) {
	failed := func(message string) *MediaCreateResult {
		return &MediaCreateResult{
			Status:             "failed",
			Mode:               MediaCompletionFailed,
			UpstreamStatusCode: statusCode,
			UpstreamRaw:        respBody,
			ErrorMessage:       message,
		}
	}
	if statusCode >= 400 {
		return failed(fmt.Sprintf("upstream returned %d: %s", statusCode, string(respBody))), nil
	}
	var resp miniMaxImageResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("minimax image unmarshal response: %w", err)
	}
	if resp.BaseResp.StatusCode != 0 {
		// 错误码保留在消息里：运营排障时只有 status_msg 无法定位到具体失败类型。
		message := "minimax " + strconv.Itoa(resp.BaseResp.StatusCode)
		if msg := strings.TrimSpace(resp.BaseResp.StatusMsg); msg != "" {
			message += ": " + msg
		}
		return failed(message), nil
	}
	urls := make([]string, 0, len(resp.Data.ImageURLs))
	for _, u := range resp.Data.ImageURLs {
		if trimmed := strings.TrimSpace(u); trimmed != "" {
			urls = append(urls, trimmed)
		}
	}
	// base64 模式没有可访问 URL，转成 data URI，让下游统一按 URL 处理。
	for _, b64 := range resp.Data.ImageBase64 {
		if trimmed := strings.TrimSpace(b64); trimmed != "" {
			urls = append(urls, "data:image/jpeg;base64,"+trimmed)
		}
	}
	if len(urls) == 0 {
		return failed("minimax image response contains no image url"), nil
	}
	return &MediaCreateResult{
		Status:             "succeeded",
		Mode:               MediaCompletionSync,
		InlineURL:          urls[0],
		InlineURLs:         urls,
		UpstreamStatusCode: statusCode,
		UpstreamRaw:        respBody,
	}, nil
}

// firstIntBeforeX 从 "1280x1024" 提取宽度。
func firstIntBeforeX(size string) int {
	parts := strings.SplitN(size, "x", 2)
	n := 0
	if len(parts) > 0 {
		_, _ = fmt.Sscan(parts[0], &n)
	}
	return n
}

// firstIntAfterX 从 "1280x1024" 提取高度。
func firstIntAfterX(size string) int {
	parts := strings.SplitN(size, "x", 2)
	n := 0
	if len(parts) > 1 {
		_, _ = fmt.Sscan(parts[1], &n)
	}
	return n
}
