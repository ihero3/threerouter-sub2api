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

	"github.com/tidwall/gjson"
)

// mediaVendorImageAdapter 是图片生成厂商 adapter 底座。
type mediaVendorImageAdapter struct {
	name           string
	httpClient     *http.Client
	supports       func(platform, model string) bool
	validateCreate func(MediaCreateRequest) error
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
	// 厂商侧参数上限前置校验：已知必失败的请求不打上游，错误也更好懂。
	if a.validateCreate != nil {
		if err := a.validateCreate(req); err != nil {
			return nil, err
		}
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
	result, err := a.parseCreate(respBody, statusCode)
	if result != nil {
		// 记录上游端点路径供 usage_logs 明细展示（与文本链路 upstream_endpoint 同口径）。
		result.UpstreamEndpoint = upstreamEndpointPath(url)
	}
	return result, err
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
		result.ErrorMessage = sanitizeMediaUpstreamErrorMessage(resp.StatusCode, respBody)
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
		case "model", "prompt", "image", "image_urls", "size", "resolution", "seed", "media", "video_create_path",
			// 幂等标识不能进上游请求体，见 video_adapter.go 同处说明。
			"request_id":
			continue
		}
		body[k] = v
	}
	data, _ := json.Marshal(body)
	return data
}

// collectOpenAIMediaImageURLs 从 OpenAI 风格响应的 data[] 里收集**全部**图片产物。
//
// 每条要点都对应一个已发生过的缺陷：
//  1. b64_json 没有可访问 URL，必须转成 data URI —— 否则只回 base64 的上游会被
//     当成"没有产物"：Seedance 因此判 succeeded 却交付不了图（照扣费用），
//     通用 adapter 因此永远停在 processing（预扣挂账）。
//  2. 收集全部而不是只取 data[0] —— n>1 时只交一张却按 n 张计费，属于少给货。
//  3. 返回空切片代表"上游没交付任何图"，由调用方判 failed，
//     避免"任务成功 + 立即扣费 + 调用方拿不到图"这种组合。
//
// 与 MiniMax 的 ImageBase64 处理保持同一口径，三家厂商落到同一套语义。
func collectOpenAIMediaImageURLs(respBody []byte) []string {
	urls := make([]string, 0, 4)
	data := gjson.GetBytes(respBody, "data")
	if !data.Exists() || !data.IsArray() {
		return urls
	}
	for _, item := range data.Array() {
		if u := strings.TrimSpace(item.Get("url").String()); u != "" {
			urls = append(urls, u)
			continue
		}
		if b64 := strings.TrimSpace(item.Get("b64_json").String()); b64 != "" {
			urls = append(urls, "data:image/png;base64,"+b64)
		}
	}
	return urls
}

func parseSeedanceImageCreateResult(respBody []byte, statusCode int) (*MediaCreateResult, error) {
	if statusCode >= 400 {
		return &MediaCreateResult{
			Status: "failed", Mode: MediaCompletionFailed, UpstreamStatusCode: statusCode, UpstreamRaw: respBody,
			ErrorMessage: sanitizeMediaUpstreamErrorMessage(statusCode, respBody),
		}, nil
	}
	// 这里只做 JSON 合法性校验（沿用历史错误语义），产物统一由
	// collectOpenAIMediaImageURLs 收集，避免 struct 字段漏读。
	var resp struct {
		Data []struct {
			URL     string `json:"url"`
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("seedance image unmarshal response: %w", err)
	}
	urls := collectOpenAIMediaImageURLs(respBody)
	if len(urls) == 0 {
		// 判 succeeded 的代价是立刻真实扣费（MediaTaskService 同步分支），
		// 而调用方随后在 handler 里拿到 502 —— 钱扣了、图没有、失败结算也不触发。
		// 所以这里必须与 Wan / MiniMax 口径一致：没有产物就是失败。
		return &MediaCreateResult{
			Status: "failed", Mode: MediaCompletionFailed, UpstreamStatusCode: statusCode, UpstreamRaw: respBody,
			ErrorMessage: "seedance image response contained no image url or b64_json",
		}, nil
	}
	return &MediaCreateResult{
		Status: "succeeded", Mode: MediaCompletionSync, InlineURL: urls[0], InlineURLs: urls,
		UpstreamStatusCode: statusCode, UpstreamRaw: respBody,
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
			"input", "parameters", "image", "image_url", "image_urls",
			// 幂等标识不能进上游请求体，见 video_adapter.go 同处说明。
			"request_id":
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
		return failed(sanitizeMediaUpstreamErrorMessage(statusCode, respBody)), nil
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
	// 官方限制 prompt 最长 1500 字符（按字符数，中文同计），超长上游报
	// 2013 invalid params。前置拒绝：省一次必然失败的调用，错误信息直接可读。
	a.validateCreate = func(req MediaCreateRequest) error {
		if n := len([]rune(req.Prompt)); n > 1500 {
			return fmt.Errorf("minimax image: prompt length %d exceeds the 1500-character limit; please shorten the prompt", n)
		}
		return nil
	}
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
		// subject_reference 故意不在排除列表：它是 MiniMax 原生字段，必须能透传，
		// 否则下面的「用户显式传了就不覆盖」判断永远看不到它，用户自定义会被静默丢弃。
		case "model", "prompt", "size", "resolution", "seed", "media", "video_create_path",
			"width", "height", "aspect_ratio", "n",
			// 幂等标识不能进上游请求体，见 video_adapter.go 同处说明。
			"request_id",
			// 以下字段已转成结构化字段、或 MiniMax 根本不支持：
			// 原样透传会污染请求体，轻则被忽略，重则触发参数类型错误。
			// image/image_url/image_urls 已由 ImageRefURLs 转成 subject_reference，
			// 再发一份同义的 image 字段对上游是无意义噪声。
			"image", "image_url", "image_urls", "image_file",
			"negative_prompt", "quality":
			continue
		}
		// style 两边语义不同：OpenAI 用字符串（vivid/natural），MiniMax 用对象
		// （{style_type, style_weight}）。字符串形态对 MiniMax 是非法类型，直接丢弃；
		// 对象形态是 MiniMax 原生写法，保留透传，不误伤按官方文档调用的用户。
		if k == "style" {
			if _, ok := v.(map[string]any); !ok {
				continue
			}
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
		return failed(sanitizeMediaUpstreamErrorMessage(statusCode, respBody)), nil
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
		// 光有数字调用方无法自助排障，补一句可执行的排查指引。
		if hint := miniMaxImageStatusHint(resp.BaseResp.StatusCode); hint != "" {
			message += " (" + hint + ")"
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

// miniMaxImageStatusHint 把 MiniMax 业务错误码翻成一句可执行的排查指引。
//
// 官方文档只给了码值表，调用方拿到 "minimax 1026" 不知道该改 prompt 还是改参考图，
// 只能来问运营。这里对高频码补一句说明，让错误能自助闭环。
// 注意 1026 是内容安全拦截，而 MiniMax 的参考图（subject_reference）只接受单人正面
// 人像，传风景/物品图同样会落到这里——不加这句说明，用户会误判成 prompt 违规。
func miniMaxImageStatusHint(code int) string {
	switch code {
	case 1002:
		return "触发限流，请稍后重试或降低并发"
	case 1004, 2049:
		return "API Key 鉴权失败，请检查该账号的接口密钥是否正确、是否已过期"
	case 1008:
		return "账号余额不足，请充值后重试"
	case 1026:
		return "内容安全拦截：请调整 prompt 措辞；若带了参考图，注意 MiniMax 图生图仅支持单人正面人像照片，非人像参考图同样会被拒"
	case 2013:
		return "参数异常：请检查 prompt 是否超过 1500 字符、model 是否为 image-01/image-01-live、size 是否为 8 的倍数且在 [512,2048]、参考图是否为可公网访问的 JPG/PNG 且小于 10MB"
	}
	return ""
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

// --- MiniMax H3 图片（MiniMax-H3-Image / MiniMax-H3-Image-Highspeed）---
//
// 契约（platform.minimaxi.com 文档，2026-11）与旧版 image-01 完全不同：
//   - 文生图：POST /v1/images/generations（OpenAI 风格：model/prompt/size/n/...）
//   - 图片编辑：POST /v1/images/edits（images[].image_url 传 1~16 张参考图）
//   - 响应：data[] 数组，部分失败时只要有一张成功即 200（失败项只带 error）；
//     全部失败才是 422（内容安全）/402（余额）/500（其他）
//
// 尺寸两选一：size 传 "宽x高"（面积 655,360~20,000,000、宽高比 1:3~3:1），
// 或省略 size 用 aspect_ratio + area（1K/2K/4K 档位）。同一请求两者同传返回 400。
// 图片编辑始终开启 prompt 增强；response_format 的 url 链接 24 小时有效。

type MiniMaxH3ImageAdapter struct{ mediaVendorImageAdapter }

// miniMaxH3ImageMaxPromptChars 是 H3 图片 prompt 的官方上限（≤ 32000 字符）。
const miniMaxH3ImageMaxPromptChars = 32000

// miniMaxH3ImageMaxRefs 是图片编辑参考图的官方上限（1~16 张）。
const miniMaxH3ImageMaxRefs = 16

func NewMiniMaxH3ImageAdapter() *MiniMaxH3ImageAdapter {
	a := &MiniMaxH3ImageAdapter{}
	a.mediaVendorImageAdapter = newMediaVendorImageAdapter("minimax-h3-image")
	// 注意与旧版 image-01 adapter 的边界：minimax-h3-image 不含 "minimax-image"
	// / "image-01" / "hailuo-image" 子串，旧 adapter 接不住，必须由本 adapter 显式
	// 认领，否则会落到通用 OpenAI 兜底 adapter（路径 /v1/images/generations 恰好
	// 相同，但 edits 场景与错误语义都不对）。
	a.supports = func(_, model string) bool { return isMiniMaxH3ImageModel(model) }
	a.buildCreate = buildMiniMaxH3ImageCreateBody
	a.parseCreate = parseMiniMaxH3ImageCreateResult
	a.validateCreate = validateMiniMaxH3ImageCreate
	return a
}

// isMiniMaxH3ImageModel 判定 MiniMax H3 图片系列（含 Highspeed 高速版）。
// 必须「同时」含厂商字样（minimax/hailuo）、h3 与 image：只含 minimax-h 前缀的
// 是 H3 **视频**（minimax-h3 → /v2/video_generation），不能被图片 adapter 抢走。
// 路由侧 IsKnownImageVendorModel 靠 "-image" 子串先行命中图片判定（wanx 同款
// 先例），这里与路由口径保持互斥。
func isMiniMaxH3ImageModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "" {
		return false
	}
	if !strings.Contains(m, "minimax") && !strings.Contains(m, "hailuo") {
		return false
	}
	return strings.Contains(m, "h3") && strings.Contains(m, "image")
}

// validateMiniMaxH3ImageCreate 厂商侧上限前置校验：已知必失败的请求不打上游。
func validateMiniMaxH3ImageCreate(req MediaCreateRequest) error {
	if n := len([]rune(req.Prompt)); n > miniMaxH3ImageMaxPromptChars {
		return fmt.Errorf("minimax h3 image: prompt length %d exceeds the %d-character limit; please shorten the prompt", n, miniMaxH3ImageMaxPromptChars)
	}
	if refs := miniMaxH3ImageRefs(req); len(refs) > miniMaxH3ImageMaxRefs {
		return fmt.Errorf("minimax h3 image: %d reference images exceed the limit of %d", len(refs), miniMaxH3ImageMaxRefs)
	}
	return nil
}

// normalizeMiniMaxH3ImageModel 把别名收敛成官方枚举值。
// H3 图片的 model 是具体型号名（区分大小写），客户端写小写或带厂商前缀的别名
// 会被上游判非法参数。只收敛「明确指向 MiniMax/Hailuo H3 图片」的名字，
// 渠道私有命名原样透传（用账号 model_mapping 自定义）。
func normalizeMiniMaxH3ImageModel(model string) string {
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "" {
		return m
	}
	if !strings.Contains(m, "minimax") && !strings.Contains(m, "hailuo") {
		return model
	}
	if strings.Contains(m, "highspeed") || strings.Contains(m, "high-speed") {
		return "MiniMax-H3-Image-Highspeed"
	}
	return "MiniMax-H3-Image"
}

// miniMaxH3ImageRefs 汇总参考图：用户显式传的官方 images[]（对象数组）优先，
// 否则用链路统一解析出的 ImageRefURLs（来自 OpenAI 风格 image/image_url 字段，
// multipart 上传已转成 data URI 或对象存储 URL）。两者不可同发，会重复计费侧
// 语义（同一张参考图出现两次）。
func miniMaxH3ImageRefs(req MediaCreateRequest) []string {
	if req.Extra != nil {
		if raw, ok := req.Extra["images"]; ok {
			var refs []string
			switch typed := raw.(type) {
			case []any:
				for _, item := range typed {
					if obj, ok := item.(map[string]any); ok {
						if u, _ := obj["image_url"].(string); strings.TrimSpace(u) != "" {
							refs = append(refs, strings.TrimSpace(u))
						}
						continue
					}
					if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
						refs = append(refs, strings.TrimSpace(s))
					}
				}
			case []string:
				for _, s := range typed {
					if trimmed := strings.TrimSpace(s); trimmed != "" {
						refs = append(refs, trimmed)
					}
				}
			}
			if len(refs) > 0 {
				return refs
			}
		}
	}
	refs := make([]string, 0, len(req.ImageRefURLs))
	for _, ref := range req.ImageRefURLs {
		if trimmed := strings.TrimSpace(ref); trimmed != "" {
			refs = append(refs, trimmed)
		}
	}
	return refs
}

// buildMiniMaxH3ImageCreateBody 构造 H3 图片请求体（文生图与图片编辑共用）。
func buildMiniMaxH3ImageCreateBody(req MediaCreateRequest) []byte {
	body := map[string]any{
		"model":  normalizeMiniMaxH3ImageModel(req.UpstreamModel),
		"prompt": req.Prompt,
	}
	// n：官方区间 [1,10]。链路统一的 parseMediaImageCount 已收敛到 [1,9]
	//（与旧版 image-01 的 n 上限共用 clamp），这里沿用同一收敛，宁可少生成一张
	// 也不出现「计费口径与上游实际张数不一致」的档位漂移。
	if req.ImageCount > 1 {
		body["n"] = clampImageCount(req.ImageCount)
	}
	// size：链路把 OpenAI 风格的 size/resolution 统一进 req.Resolution。
	// aspect_ratio + area 档位写法是 H3 原生字段，经 Extra 透传（见下方白名单）。
	if size := strings.TrimSpace(req.Resolution); size != "" {
		body["size"] = size
	}
	for k, v := range req.Extra {
		switch k {
		case "model", "prompt", "size", "resolution", "n", "media", "video_create_path",
			// 幂等标识不能进上游请求体，见 video_adapter.go 同处说明。
			"request_id",
			// images 已由 miniMaxH3ImageRefs 转成结构化字段，再透传会与产物字段
			// 重复出现；image/image_url/image_urls 是 OpenAI 风格别名，同理由。
			"images", "image", "image_url", "image_urls", "image_file",
			// H3 文档未提供这些 OpenAI 专有字段；原样透传轻则被忽略、重则触发
			// 参数类型错误（与旧版 image-01 的拦截口径一致）。
			"seed", "quality", "negative_prompt", "style",
			// response_format 由 handler 层消费（决定回 url 还是 b64_json），
			// 不能以客户端的偏好改写上游返回形态。
			"response_format",
			// stream：H3 支持 SSE 逐张推送，但本链路是同步语义（解析 JSON 响应）。
			// 若透传 stream=true，上游会改回 SSE 文本流，JSON 解析必炸、任务误判
			// 失败 —— 必须拦下，按同步语义调用。
			"stream":
			continue
		}
		body[k] = v
	}
	// 图片编辑：参考图通过 images[].image_url 下发（官方契约）。
	if refs := miniMaxH3ImageRefs(req); len(refs) > 0 {
		items := make([]map[string]any, 0, len(refs))
		for _, ref := range refs {
			items = append(items, map[string]any{"image_url": ref})
		}
		body["images"] = items
	}
	data, _ := json.Marshal(body)
	return data
}

// parseMiniMaxH3ImageCreateResult 解析 H3 图片响应。
// 响应契约（对齐官方文档）：
//   - data 是数组；部分失败时失败项只带 error（code/message），HTTP 仍 200；
//   - output_format 是全部产物的统一格式（jpeg/png/webp），b64 转码 MIME 用它；
//   - 全部失败时返回错误响应（422 内容安全 / 402 余额不足 / 500 其他），
//     错误体沿用 MiniMax base_resp 习惯，也附带每张的 data 与 usage。
func parseMiniMaxH3ImageCreateResult(respBody []byte, statusCode int) (*MediaCreateResult, error) {
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
		// 全部失败：422（内容安全）/ 402（余额不足）/ 500（其他）。
		// 优先取 base_resp 的错误码与说明（MiniMax 全系习惯），取不到再回落
		// 到通用清洗逻辑 —— 只有状态码没有语义的报文，调用方无从自助排障。
		if message := miniMaxH3ImageUpstreamErrorMessage(statusCode, respBody); message != "" {
			return failed(message), nil
		}
		return failed(sanitizeMediaUpstreamErrorMessage(statusCode, respBody)), nil
	}

	// 用 gjson 做宽容解析：data 数组内 url / b64_json / size / error 均按项读取，
	// 避免 struct 漏字段导致「上游交付了产物却被判失败」（真实扣费不给货的反面）。
	outputFormat := strings.ToLower(strings.TrimSpace(gjson.GetBytes(respBody, "output_format").String()))
	mimeType := miniMaxH3ImageMIMEType(outputFormat)
	urls := make([]string, 0, 4)
	var firstSize string
	var itemErrors []string
	data := gjson.GetBytes(respBody, "data")
	if data.Exists() && data.IsArray() {
		for _, item := range data.Array() {
			if errObj := item.Get("error"); errObj.Exists() {
				code := strings.TrimSpace(item.Get("error.code").String())
				message := strings.TrimSpace(item.Get("error.message").String())
				if code != "" || message != "" {
					itemErrors = append(itemErrors, strings.TrimSpace(code+" "+message))
				}
				continue
			}
			if u := strings.TrimSpace(item.Get("url").String()); u != "" {
				urls = append(urls, u)
			} else if b64 := strings.TrimSpace(item.Get("b64_json").String()); b64 != "" {
				// b64_json 没有可访问 URL，转成 data URI，与旧版 image-01 /
				// collectOpenAIMediaImageURLs 同一口径（下游统一按 URL 处理）。
				urls = append(urls, "data:"+mimeType+";base64,"+b64)
			}
			if size := strings.TrimSpace(item.Get("size").String()); size != "" && firstSize == "" {
				firstSize = size
			}
		}
	}
	if len(urls) == 0 {
		// 200 但一张都没拿到：把每项的失败原因拼进错误消息；一条都提不出来时
		// 才用兜底文案。判 succeeded 的代价是立刻真实扣费 + 调用方拿 502。
		message := "minimax h3 image response contained no image url or b64_json"
		if len(itemErrors) > 0 {
			message += ": " + strings.Join(itemErrors, "; ")
		}
		return failed(message), nil
	}
	result := &MediaCreateResult{
		Status:             "succeeded",
		Mode:               MediaCompletionSync,
		InlineURL:          urls[0],
		InlineURLs:         urls,
		UpstreamStatusCode: statusCode,
		UpstreamRaw:        respBody,
	}
	// 真实输出尺寸用于计费分档（1K/2K/4K）：按 aspect_ratio+area 生成时每张实际
	// 宽高由服务端决定，按请求值分档会错档（与 DashScope usage 口径同理）。
	if firstSize != "" {
		result.UpstreamSize = firstSize
	}
	return result, nil
}

// miniMaxH3ImageMIMEType 把 output_format（jpeg/jpg/png/webp）映射成 data URI MIME。
func miniMaxH3ImageMIMEType(outputFormat string) string {
	switch strings.ToLower(strings.TrimSpace(outputFormat)) {
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	default:
		// jpeg 是官方默认格式（文档亦允许写作 jpg）。
		return "image/jpeg"
	}
}

// miniMaxH3ImageUpstreamErrorMessage 从全部失败的错误响应里提取可读语义。
// 返回空串表示没提取到，调用方回落到通用清洗。
func miniMaxH3ImageUpstreamErrorMessage(statusCode int, respBody []byte) string {
	parts := make([]string, 0, 2)
	if code := gjson.GetBytes(respBody, "base_resp.status_code").Int(); code != 0 {
		parts = append(parts, "minimax "+strconv.Itoa(int(code)))
	}
	if msg := strings.TrimSpace(gjson.GetBytes(respBody, "base_resp.status_msg").String()); msg != "" {
		parts = append(parts, msg)
	} else if msg := strings.TrimSpace(gjson.GetBytes(respBody, "error.message").String()); msg != "" {
		parts = append(parts, msg)
	}
	if len(parts) == 0 {
		return ""
	}
	message := strings.Join(parts, ": ")
	switch statusCode {
	case http.StatusPaymentRequired:
		message += " (insufficient balance)"
	case http.StatusUnprocessableEntity:
		message += " (all images failed the content safety check)"
	}
	return message
}

// Create 覆写底座：H3 按有无参考图在两个官方端点间切换（底座的单端点
// buildCreateURL 模型表达不了这个决策），HTTP 收发复用底座 do/parse。
// 请求体交给 buildMiniMaxH3ImageCreateBody（两端点共用，靠 images 字段区分）。
func (a *MiniMaxH3ImageAdapter) Create(ctx context.Context, account *Account, req MediaCreateRequest) (*MediaCreateResult, error) {
	// 厂商侧参数上限前置校验：已知必失败的请求不打上游，错误也更好懂。
	if a.validateCreate != nil {
		if err := a.validateCreate(req); err != nil {
			return nil, err
		}
	}
	baseURL, err := a.baseURL(account)
	if err != nil {
		return nil, err
	}
	// 有参考图 → 图片编辑端点；纯文生图 → 生成端点。
	// video_create_path 是渠道自定义端点覆写（账号凭证优先、客户端 Extra 次之），
	// 与其他厂商 adapter 同一语义，仅作用于文生图路径。
	path := "/v1/images/generations"
	if len(miniMaxH3ImageRefs(req)) > 0 {
		path = "/v1/images/edits"
	} else if extra := account.GetCredential("video_create_path"); strings.TrimSpace(extra) != "" {
		path = normalizeVideoURLPath(extra)
	}
	if req.Extra != nil {
		if override, _ := req.Extra["video_create_path"].(string); strings.TrimSpace(override) != "" {
			path = normalizeVideoURLPath(override)
		}
	}
	url := baseURL + path
	respBody, statusCode, err := a.do(ctx, account, http.MethodPost, url, a.buildCreate(req))
	if err != nil {
		return nil, fmt.Errorf("%s image create request: %w", a.name, err)
	}
	result, err := a.parseCreate(respBody, statusCode)
	if result != nil {
		// 记录上游端点路径供 usage_logs 明细展示（与文本链路 upstream_endpoint 同口径）。
		result.UpstreamEndpoint = upstreamEndpointPath(url)
	}
	return result, err
}
