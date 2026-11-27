package service

// minimax_tts_gateway.go — MiniMax 音色复刻 / 音色查询 / 异步长文本语音合成。
//
// 同步 T2A（/v1/audio/speech -> /v1/t2a_v2）由 media_vendor_audio_adapter 负责；
// 这里补齐 MiniMax 原生文档里的其余能力，并保持「一个网关 API Key + 一个分组」
// 就能用：
//
//   - POST /v1/files/upload             上传复刻音频 / 示例音频 / 异步文本文件
//   - POST /v1/voice_clone              音色快速复刻（返回网关音色 ID）
//   - POST /v1/get_voice                查询系统音色 + 当前账号的复刻音色
//   - POST /v1/t2a_async_v2             创建异步长文本语音合成任务
//   - GET  /v1/query/t2a_async_query_v2 查询任务状态（成功后按字符结算）
//   - GET  /v1/files/retrieve_content   下载异步产物
//
// 账号粘性：MiniMax 的 file_id / voice_id / task_id 都是账号维度的。file_id 与
// task_id 保持上游原始 int64（官方 SDK 按数字解析），其账号归属存 Redis（见
// minimax_tts_store.go）；voice_id 本身是字符串，直接签成网关 token，用于把
// 后续 TTS 固定打回签发它的账号。这样既保持协议类型，又不会因多账号轮转出现
// 「音色/文件/任务不存在」。

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	miniMaxVoiceTokenPrefix = "mmxvc_"

	miniMaxFileBindingTTL = 7 * 24 * time.Hour
	miniMaxTaskBindingTTL = 7 * 24 * time.Hour

	miniMaxDefaultTTSModel = "speech-2.8-hd"
	// 音色复刻/示例音频上游单文件上限 20MB；异步文本 zip 允许更大。
	miniMaxVoiceSampleMaxBytes = 20 << 20
	miniMaxUploadMaxBytes      = 100 << 20 // 异步文本 zip 可能较大；音色复刻单文件仍受上游 20MB 限制
	miniMaxAPITimeout          = 180 * time.Second
	miniMaxDownloadTimeout     = 30 * time.Minute
	miniMaxBindingTimeout      = 3 * time.Second
)

var (
	// ErrMiniMaxInvalidToken 表示音色 token 被篡改/不属于当前用户。
	ErrMiniMaxInvalidToken = errors.New("minimax: invalid or expired voice token")
	// ErrMiniMaxBadRequest 表示参数或 ID 不合法，handler 应映射为 400。
	ErrMiniMaxBadRequest = errors.New("minimax: invalid request")
	// ErrMiniMaxStoreUnavailable 表示归属映射存储不可用，handler 应映射为 502/503。
	ErrMiniMaxStoreUnavailable = errors.New("minimax: binding store unavailable")
)

// isMiniMaxTTSModel 判定 MiniMax T2A 模型（speech-* 与历史别名）。
// 独立成函数供 adapter 与账号粘性路由共用，避免两处口径漂移。
func isMiniMaxTTSModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	if strings.HasPrefix(m, "speech-") {
		return true
	}
	// "tts" 只做精确匹配：宽泛 Contains 会抢走 qwen-audio-...-tts-flash。
	return strings.Contains(m, "minimax-tts") || m == "speech-01" || m == "tts"
}

// --- 网关音色 token（voice_id 本身是字符串，可保持原生类型） ---

type miniMaxVoiceToken struct {
	AccountID int64  `json:"a"`
	UserID    int64  `json:"u"`
	VoiceID   string `json:"v"`
}

func miniMaxVoiceTokenKey(account *Account) []byte {
	if account == nil {
		return nil
	}
	seed := fmt.Sprintf("sub2api/minimax-voice-token/v1\x00%d\x00%s", account.ID, account.GetCredential("api_key"))
	sum := sha256.Sum256([]byte(seed))
	return sum[:]
}

func signMiniMaxVoiceToken(account *Account, payload miniMaxVoiceToken) (string, error) {
	if account == nil {
		return "", fmt.Errorf("%w: account is required", ErrMiniMaxBadRequest)
	}
	key := miniMaxVoiceTokenKey(account)
	if len(key) == 0 {
		return "", fmt.Errorf("%w: account %d has no api_key", ErrMiniMaxBadRequest, account.ID)
	}
	payload.AccountID = account.ID
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("minimax: marshal voice token: %w", err)
	}
	body := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(body))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return miniMaxVoiceTokenPrefix + body + "." + sig, nil
}

func decodeMiniMaxVoiceToken(token string) (*miniMaxVoiceToken, string, []byte, error) {
	token = strings.TrimSpace(token)
	if !strings.HasPrefix(token, miniMaxVoiceTokenPrefix) {
		return nil, "", nil, fmt.Errorf("%w: not a gateway voice token", ErrMiniMaxBadRequest)
	}
	rest := strings.TrimPrefix(token, miniMaxVoiceTokenPrefix)
	dot := strings.LastIndex(rest, ".")
	if dot <= 0 || dot == len(rest)-1 {
		return nil, "", nil, ErrMiniMaxInvalidToken
	}
	body := rest[:dot]
	sig, err := base64.RawURLEncoding.DecodeString(rest[dot+1:])
	if err != nil {
		return nil, "", nil, ErrMiniMaxInvalidToken
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return nil, "", nil, ErrMiniMaxInvalidToken
	}
	var payload miniMaxVoiceToken
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, "", nil, ErrMiniMaxInvalidToken
	}
	if payload.AccountID <= 0 || strings.TrimSpace(payload.VoiceID) == "" {
		return nil, "", nil, ErrMiniMaxInvalidToken
	}
	return &payload, body, sig, nil
}

func verifyMiniMaxVoiceToken(account *Account, body string, sig []byte) error {
	key := miniMaxVoiceTokenKey(account)
	if len(key) == 0 {
		return ErrMiniMaxInvalidToken
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(body))
	if !hmac.Equal(mac.Sum(nil), sig) {
		return ErrMiniMaxInvalidToken
	}
	return nil
}

func (s *MediaTaskService) parseMiniMaxVoiceToken(
	ctx context.Context, token string, userID int64,
) (*miniMaxVoiceToken, *Account, error) {
	payload, body, sig, err := decodeMiniMaxVoiceToken(token)
	if err != nil {
		return nil, nil, err
	}
	if s == nil || s.accountService == nil {
		return nil, nil, fmt.Errorf("%w: account service unavailable", ErrMiniMaxInvalidToken)
	}
	account, err := s.accountService.GetByID(ctx, payload.AccountID)
	if err != nil || account == nil {
		return nil, nil, fmt.Errorf("%w: account not found", ErrMiniMaxInvalidToken)
	}
	if err := verifyMiniMaxVoiceToken(account, body, sig); err != nil {
		return nil, nil, err
	}
	if payload.UserID > 0 && payload.UserID != userID {
		return nil, nil, fmt.Errorf("%w: user mismatch", ErrMiniMaxInvalidToken)
	}
	return payload, account, nil
}

// --- 归属映射读写（Redis 由 repository 实现） ---

func (s *MediaTaskService) saveMiniMaxFileBinding(
	ctx context.Context, fileID string, account *Account, userID, apiKeyID int64, purpose string,
) error {
	if s == nil || s.miniMaxBindingStore == nil {
		return ErrMiniMaxStoreUnavailable
	}
	if account == nil || strings.TrimSpace(fileID) == "" {
		return fmt.Errorf("%w: file binding requires file id and account", ErrMiniMaxBadRequest)
	}
	saveCtx, cancel := context.WithTimeout(ctx, miniMaxBindingTimeout)
	defer cancel()
	if err := s.miniMaxBindingStore.SaveFile(saveCtx, fileID, MiniMaxFileBinding{
		AccountID: account.ID,
		UserID:    userID,
		APIKeyID:  apiKeyID,
		Purpose:   purpose,
	}, miniMaxFileBindingTTL); err != nil {
		return fmt.Errorf("%w: %v", ErrMiniMaxStoreUnavailable, err)
	}
	return nil
}

func (s *MediaTaskService) loadMiniMaxFileBinding(
	ctx context.Context, fileID string, userID int64,
) (*MiniMaxFileBinding, *Account, error) {
	if s == nil || s.miniMaxBindingStore == nil {
		return nil, nil, ErrMiniMaxStoreUnavailable
	}
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return nil, nil, fmt.Errorf("%w: file_id is required", ErrMiniMaxBadRequest)
	}
	loadCtx, cancel := context.WithTimeout(ctx, miniMaxBindingTimeout)
	defer cancel()
	binding, err := s.miniMaxBindingStore.GetFile(loadCtx, fileID)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrMiniMaxStoreUnavailable, err)
	}
	if binding == nil {
		return nil, nil, fmt.Errorf("%w: file_id not found or expired; upload it through this gateway first", ErrMiniMaxBadRequest)
	}
	if binding.UserID != 0 && binding.UserID != userID {
		return nil, nil, fmt.Errorf("%w: file_id belongs to another user", ErrMiniMaxInvalidToken)
	}
	if s.accountService == nil {
		return nil, nil, ErrMiniMaxStoreUnavailable
	}
	account, err := s.accountService.GetByID(ctx, binding.AccountID)
	if err != nil || account == nil {
		return nil, nil, fmt.Errorf("%w: account for file_id is unavailable", ErrMiniMaxBadRequest)
	}
	return binding, account, nil
}

func (s *MediaTaskService) saveMiniMaxTaskBinding(
	ctx context.Context, taskID string, binding MiniMaxTaskBinding,
) error {
	if s == nil || s.miniMaxBindingStore == nil {
		return ErrMiniMaxStoreUnavailable
	}
	if strings.TrimSpace(taskID) == "" {
		return fmt.Errorf("%w: task binding requires task id", ErrMiniMaxBadRequest)
	}
	saveCtx, cancel := context.WithTimeout(ctx, miniMaxBindingTimeout)
	defer cancel()
	if err := s.miniMaxBindingStore.SaveTask(saveCtx, taskID, binding, miniMaxTaskBindingTTL); err != nil {
		return fmt.Errorf("%w: %v", ErrMiniMaxStoreUnavailable, err)
	}
	return nil
}

func (s *MediaTaskService) loadMiniMaxTaskBinding(
	ctx context.Context, taskID string, userID int64,
) (*MiniMaxTaskBinding, *Account, error) {
	if s == nil || s.miniMaxBindingStore == nil {
		return nil, nil, ErrMiniMaxStoreUnavailable
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, nil, fmt.Errorf("%w: task_id is required", ErrMiniMaxBadRequest)
	}
	loadCtx, cancel := context.WithTimeout(ctx, miniMaxBindingTimeout)
	defer cancel()
	binding, err := s.miniMaxBindingStore.GetTask(loadCtx, taskID)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrMiniMaxStoreUnavailable, err)
	}
	if binding == nil {
		return nil, nil, fmt.Errorf("%w: task_id not found or expired", ErrMiniMaxBadRequest)
	}
	if binding.UserID != 0 && binding.UserID != userID {
		return nil, nil, fmt.Errorf("%w: task_id belongs to another user", ErrMiniMaxInvalidToken)
	}
	if s.accountService == nil {
		return nil, nil, ErrMiniMaxStoreUnavailable
	}
	account, err := s.accountService.GetByID(ctx, binding.AccountID)
	if err != nil || account == nil {
		return nil, nil, fmt.Errorf("%w: account for task_id is unavailable", ErrMiniMaxBadRequest)
	}
	return binding, account, nil
}

// --- MiniMax 原生 API 客户端 ---

type miniMaxAPIClient struct {
	baseURL        string
	apiKey         string
	account        *Account
	httpClient     *http.Client
	downloadClient *http.Client
}

func newMiniMaxAPIClient(account *Account) (*miniMaxAPIClient, error) {
	if account == nil {
		return nil, fmt.Errorf("%w: account is required", ErrMiniMaxBadRequest)
	}
	baseURL := strings.TrimRight(account.GetCredential("base_url"), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("minimax: account %d has no base_url", account.ID)
	}
	apiKey := account.GetCredential("api_key")
	if apiKey == "" {
		return nil, fmt.Errorf("minimax: account %d has no api_key", account.ID)
	}
	return &miniMaxAPIClient{
		baseURL:        baseURL,
		apiKey:         apiKey,
		account:        account,
		httpClient:     &http.Client{Timeout: miniMaxAPITimeout},
		downloadClient: &http.Client{Timeout: miniMaxDownloadTimeout},
	}, nil
}

func (c *miniMaxAPIClient) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.account != nil {
		c.account.ApplyHeaderOverrides(req.Header)
	}
	return req, nil
}

func (c *miniMaxAPIClient) postJSON(ctx context.Context, path string, payload any) ([]byte, int, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("minimax: marshal request: %w", err)
	}
	req, err := c.newRequest(ctx, http.MethodPost, path, bytes.NewReader(raw))
	if err != nil {
		return nil, 0, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	return respBody, resp.StatusCode, nil
}

func (c *miniMaxAPIClient) postMultipart(
	ctx context.Context, path, purpose, filename string, data []byte,
) ([]byte, int, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, 0, fmt.Errorf("minimax: create form file: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return nil, 0, fmt.Errorf("minimax: write form file: %w", err)
	}
	if purpose != "" {
		if err := writer.WriteField("purpose", purpose); err != nil {
			return nil, 0, fmt.Errorf("minimax: write purpose: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, 0, fmt.Errorf("minimax: close multipart: %w", err)
	}
	req, err := c.newRequest(ctx, http.MethodPost, path, &buf)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	return respBody, resp.StatusCode, nil
}

func (c *miniMaxAPIClient) get(ctx context.Context, path string) (*http.Response, error) {
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return c.downloadClient.Do(req)
}

func (c *miniMaxAPIClient) getBytes(ctx context.Context, path string) ([]byte, int, error) {
	resp, err := c.get(ctx, path)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	return body, resp.StatusCode, nil
}

// miniMaxEnsureOK 把 HTTP 状态与 MiniMax base_resp.status_code 统一成错误。
// 上游 message 不回显给客户端，只保留状态码供 handler 映射。
func miniMaxEnsureOK(respBody []byte, statusCode int) error {
	if statusCode >= http.StatusBadRequest {
		return fmt.Errorf("minimax upstream returned HTTP %d", statusCode)
	}
	var payload struct {
		BaseResp struct {
			StatusCode int64 `json:"status_code"`
		} `json:"base_resp"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return fmt.Errorf("minimax upstream returned invalid json")
	}
	if payload.BaseResp.StatusCode != 0 {
		return fmt.Errorf("minimax upstream rejected the request (code %d)", payload.BaseResp.StatusCode)
	}
	return nil
}

// --- 小工具 ---

func miniMaxAnyID(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strings.TrimSpace(strconv.FormatFloat(x, 'f', -1, 64))
	case int64:
		return strconv.FormatInt(x, 10)
	case int:
		return strconv.Itoa(x)
	case json.Number:
		return x.String()
	default:
		return ""
	}
}

func miniMaxAnyInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int64:
		return int(x)
	case int:
		return x
	case json.Number:
		if n, err := x.Int64(); err == nil {
			return int(n)
		}
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(x)); err == nil {
			return n
		}
	}
	return 0
}

func miniMaxIDJSONValue(id string) any {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	if n, err := strconv.ParseInt(id, 10, 64); err == nil {
		return n
	}
	return id
}

func miniMaxCloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func miniMaxString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// --- 账号选择 / 音色粘性 ---

// selectMiniMaxAccount 按模型选账号；model 为空时依次尝试常用 T2A 模型，
// 让「上传文件 / 查询音色」这类没有 model 的原生接口也能落到 TTS 账号。
func (s *MediaTaskService) selectMiniMaxAccount(
	ctx context.Context, groupID *int64, preferredModel string, excluded map[int64]struct{},
) (*Account, error) {
	if s == nil || s.gatewayService == nil {
		return nil, fmt.Errorf("minimax: gateway service unavailable")
	}
	candidates := make([]string, 0, 6)
	seen := map[string]struct{}{}
	add := func(m string) {
		m = strings.TrimSpace(m)
		if m == "" {
			return
		}
		if _, ok := seen[m]; ok {
			return
		}
		seen[m] = struct{}{}
		candidates = append(candidates, m)
	}
	add(preferredModel)
	for _, fallback := range []string{
		miniMaxDefaultTTSModel, "speech-2.8-turbo", "speech-2.6-hd",
		"speech-02-hd", "speech-01-hd",
	} {
		add(fallback)
	}
	var lastErr error
	for _, model := range candidates {
		account, err := s.gatewayService.SelectAccountForModelWithExclusions(ctx, groupID, "", model, excluded)
		if err == nil && account != nil {
			return account, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("no available account")
	}
	return nil, fmt.Errorf("minimax: select account: %w", lastErr)
}

// resolveMiniMaxVoicePin 识别网关音色 ID：命中时返回签发它的上游账号，并把
// req.Extra["voice"] 还原成上游真实 voice_id。调用方应优先使用返回的账号且
// 不再跨账号 failover（换账号后音色一定不存在）。
func (s *MediaTaskService) resolveMiniMaxVoicePin(
	ctx context.Context, userID int64, publicModel string, req *MediaCreateRequest,
) (*Account, error) {
	if req == nil || req.Extra == nil {
		return nil, nil
	}
	voice := strings.TrimSpace(miniMaxString(req.Extra["voice"]))
	// 原生 MiniMax 请求把音色放在 voice_setting.voice_id，也要能识别网关音色。
	if voice == "" {
		if vs, ok := req.Extra["voice_setting"].(map[string]any); ok {
			voice = strings.TrimSpace(miniMaxString(vs["voice_id"]))
		}
	}
	if !strings.HasPrefix(voice, miniMaxVoiceTokenPrefix) {
		return nil, nil
	}
	if !isMiniMaxTTSModel(publicModel) {
		return nil, fmt.Errorf("%w: gateway voice id is only valid for MiniMax speech models", ErrMiniMaxBadRequest)
	}
	payload, account, err := s.parseMiniMaxVoiceToken(ctx, voice, userID)
	if err != nil {
		return nil, err
	}
	if account == nil || !account.IsSchedulable() {
		return nil, fmt.Errorf("%w: the account owning this voice is unavailable", ErrMiniMaxBadRequest)
	}
	if !account.IsModelSupported(publicModel) {
		return nil, fmt.Errorf("%w: the account owning this voice does not support model %s", ErrMiniMaxBadRequest, publicModel)
	}
	req.Extra["voice"] = payload.VoiceID
	if vs, ok := req.Extra["voice_setting"].(map[string]any); ok {
		cloned := miniMaxCloneMap(vs)
		cloned["voice_id"] = payload.VoiceID
		req.Extra["voice_setting"] = cloned
	}
	return account, nil
}

// settleMiniMaxAudioByChars 按字符结算 MiniMax 试听 / 异步任务费用。
// 幂等由 localID 派生的 usage request id 保证；结算失败不影响上游结果的交付。
func (s *MediaTaskService) settleMiniMaxAudioByChars(
	ctx context.Context,
	localID string,
	userID, apiKeyID int64,
	account *Account,
	publicModel, upstreamModel string,
	billedChars int,
) {
	if s == nil || account == nil || billedChars <= 0 || strings.TrimSpace(localID) == "" {
		return
	}
	if s.billingService == nil || s.apiKeyService == nil || s.openAIGatewayService == nil {
		return
	}
	billingCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	settleMediaAudioTaskSuccess(billingCtx, s.billingDeps(), &mediaAudioBillingInput{
		LocalID:       localID,
		UserID:        userID,
		APIKeyID:      apiKeyID,
		AccountID:     account.ID,
		Account:       account,
		Model:         publicModel,
		UpstreamModel: upstreamModel,
		BilledChars:   billedChars,
	})
}

// --- 文件上传 ---

func (s *MediaTaskService) MiniMaxUploadFile(
	ctx context.Context,
	groupID *int64,
	userID, apiKeyID int64,
	model, purpose, filename string,
	data []byte,
) (map[string]any, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: file is required", ErrMiniMaxBadRequest)
	}
	purpose = strings.TrimSpace(purpose)
	if purpose == "" {
		purpose = "voice_clone"
	}
	if purpose != "t2a_async_input" && len(data) > miniMaxVoiceSampleMaxBytes {
		return nil, fmt.Errorf("%w: voice sample exceeds %d bytes", ErrMiniMaxBadRequest, miniMaxVoiceSampleMaxBytes)
	}
	if len(data) > miniMaxUploadMaxBytes {
		return nil, fmt.Errorf("%w: file exceeds %d bytes", ErrMiniMaxBadRequest, miniMaxUploadMaxBytes)
	}
	if filename == "" {
		filename = "upload.bin"
	}
	account, err := s.selectMiniMaxAccount(ctx, groupID, model, nil)
	if err != nil {
		return nil, err
	}
	client, err := newMiniMaxAPIClient(account)
	if err != nil {
		return nil, err
	}
	respBody, statusCode, err := client.postMultipart(ctx, "/v1/files/upload", purpose, filename, data)
	if err != nil {
		return nil, fmt.Errorf("minimax upload: %w", err)
	}
	if err := miniMaxEnsureOK(respBody, statusCode); err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("minimax upload: invalid response")
	}
	fileObj, _ := out["file"].(map[string]any)
	if fileObj == nil {
		return nil, fmt.Errorf("minimax upload: response has no file object")
	}
	fileID := miniMaxAnyID(fileObj["file_id"])
	if fileID == "" {
		return nil, fmt.Errorf("minimax upload: response has no file_id")
	}
	if err := s.saveMiniMaxFileBinding(ctx, fileID, account, userID, apiKeyID, purpose); err != nil {
		return nil, err
	}
	// file_id 保持上游原始数字，兼容官方 SDK 的 int64 解析。
	return out, nil
}

// --- 音色复刻 ---

func (s *MediaTaskService) MiniMaxVoiceClone(
	ctx context.Context,
	userID, apiKeyID int64,
	body map[string]any,
) (map[string]any, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf("%w: request body is empty", ErrMiniMaxBadRequest)
	}
	req := miniMaxCloneMap(body)
	newVoiceID := strings.TrimSpace(miniMaxString(req["voice_id"]))
	if newVoiceID == "" {
		return nil, fmt.Errorf("%w: voice_id is required", ErrMiniMaxBadRequest)
	}
	if strings.HasPrefix(newVoiceID, miniMaxVoiceTokenPrefix) {
		return nil, fmt.Errorf("%w: voice_id must be a new upstream voice id, not a gateway voice token", ErrMiniMaxBadRequest)
	}

	fileID := miniMaxAnyID(req["file_id"])
	_, account, err := s.loadMiniMaxFileBinding(ctx, fileID, userID)
	if err != nil {
		return nil, err
	}
	req["file_id"] = miniMaxIDJSONValue(fileID)

	if clonePrompt, ok := req["clone_prompt"].(map[string]any); ok {
		cp := miniMaxCloneMap(clonePrompt)
		if promptAudio := miniMaxAnyID(cp["prompt_audio"]); promptAudio != "" {
			_, promptAccount, perr := s.loadMiniMaxFileBinding(ctx, promptAudio, userID)
			if perr != nil {
				return nil, perr
			}
			if promptAccount != nil && account != nil && promptAccount.ID != account.ID {
				return nil, fmt.Errorf("%w: file_id and clone_prompt.prompt_audio belong to different accounts", ErrMiniMaxBadRequest)
			}
			cp["prompt_audio"] = miniMaxIDJSONValue(promptAudio)
		}
		req["clone_prompt"] = cp
	}

	if !account.IsSchedulable() {
		return nil, fmt.Errorf("%w: the selected account is unavailable", ErrMiniMaxBadRequest)
	}
	client, err := newMiniMaxAPIClient(account)
	if err != nil {
		return nil, err
	}
	respBody, statusCode, err := client.postJSON(ctx, "/v1/voice_clone", req)
	if err != nil {
		return nil, fmt.Errorf("minimax voice clone: %w", err)
	}
	if err := miniMaxEnsureOK(respBody, statusCode); err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("minimax voice clone: invalid response")
	}

	voiceToken, err := signMiniMaxVoiceToken(account, miniMaxVoiceToken{
		UserID:  userID,
		VoiceID: newVoiceID,
	})
	if err != nil {
		return nil, err
	}
	out["voice_id"] = voiceToken
	out["upstream_voice_id"] = newVoiceID

	// 带试听文本时上游会收 TTS 费；usage_characters 与 Sync TTS 同口径。
	if extra, ok := out["extra_info"].(map[string]any); ok {
		if chars := miniMaxAnyInt(extra["usage_characters"]); chars > 0 {
			model := strings.TrimSpace(miniMaxString(req["model"]))
			if model == "" {
				model = miniMaxDefaultTTSModel
			}
			upstreamModel := account.GetMappedModel(model)
			if upstreamModel == "" {
				upstreamModel = model
			}
			text := strings.TrimSpace(miniMaxString(req["text"]))
			sum := sha256.Sum256([]byte(newVoiceID + "\x00" + text))
			s.settleMiniMaxAudioByChars(ctx,
				"mmxclone-"+strconv.FormatInt(account.ID, 10)+"-"+hex.EncodeToString(sum[:8]),
				userID, apiKeyID, account, model, upstreamModel, chars)
		}
	}
	return out, nil
}

// --- 音色查询 ---

func (s *MediaTaskService) MiniMaxGetVoices(
	ctx context.Context,
	groupID *int64,
	userID int64,
	model string,
	body map[string]any,
) (map[string]any, error) {
	account, err := s.selectMiniMaxAccount(ctx, groupID, model, nil)
	if err != nil {
		return nil, err
	}
	req := miniMaxCloneMap(body)
	delete(req, "model")
	if strings.TrimSpace(miniMaxString(req["voice_type"])) == "" {
		req["voice_type"] = "all"
	}
	client, err := newMiniMaxAPIClient(account)
	if err != nil {
		return nil, err
	}
	respBody, statusCode, err := client.postJSON(ctx, "/v1/get_voice", req)
	if err != nil {
		return nil, fmt.Errorf("minimax get_voice: %w", err)
	}
	if err := miniMaxEnsureOK(respBody, statusCode); err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("minimax get_voice: invalid response")
	}
	// 复刻/生成音色是账号维度的；包成网关 token 后，后续 TTS 才能路由回同一账号。
	for _, key := range []string{"voice_cloning", "voice_generation"} {
		items, _ := out[key].([]any)
		for _, item := range items {
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			upstreamVoiceID := strings.TrimSpace(miniMaxAnyID(obj["voice_id"]))
			if upstreamVoiceID == "" {
				continue
			}
			token, signErr := signMiniMaxVoiceToken(account, miniMaxVoiceToken{
				UserID:  userID,
				VoiceID: upstreamVoiceID,
			})
			if signErr != nil {
				continue
			}
			obj["voice_id"] = token
			obj["upstream_voice_id"] = upstreamVoiceID
		}
	}
	return out, nil
}

// --- 异步语音合成 ---

func (s *MediaTaskService) MiniMaxCreateAsyncTTS(
	ctx context.Context,
	groupID *int64,
	userID, apiKeyID int64,
	body map[string]any,
) (map[string]any, error) {
	req := miniMaxCloneMap(body)
	model := strings.TrimSpace(miniMaxString(req["model"]))
	if model == "" {
		return nil, fmt.Errorf("%w: model is required", ErrMiniMaxBadRequest)
	}

	var pinned *Account
	// 音色可能是网关 token：解出账号并还原上游 voice_id。
	voiceRaw := ""
	if vs, ok := req["voice_setting"].(map[string]any); ok {
		voiceRaw = strings.TrimSpace(miniMaxString(vs["voice_id"]))
	}
	if voiceRaw == "" {
		voiceRaw = strings.TrimSpace(miniMaxString(req["voice"]))
	}
	if voiceRaw != "" {
		upstreamVoiceID := voiceRaw
		if strings.HasPrefix(voiceRaw, miniMaxVoiceTokenPrefix) {
			payload, account, perr := s.parseMiniMaxVoiceToken(ctx, voiceRaw, userID)
			if perr != nil {
				return nil, perr
			}
			pinned = account
			upstreamVoiceID = payload.VoiceID
		}
		vs, _ := req["voice_setting"].(map[string]any)
		if vs == nil {
			vs = map[string]any{}
		} else {
			vs = miniMaxCloneMap(vs)
		}
		vs["voice_id"] = upstreamVoiceID
		req["voice_setting"] = vs
	}
	if _, ok := req["voice"]; ok {
		// 网关便捷字段；MiniMax 只认 voice_setting.voice_id。
		delete(req, "voice")
	}

	// 文本文件必须是本网关上传的：先从归属映射里取账号，保证与音色同账号。
	if textFileID := miniMaxAnyID(req["text_file_id"]); textFileID != "" {
		_, fileAccount, ferr := s.loadMiniMaxFileBinding(ctx, textFileID, userID)
		if ferr != nil {
			return nil, ferr
		}
		if pinned != nil && fileAccount != nil && fileAccount.ID != pinned.ID {
			return nil, fmt.Errorf("%w: text_file_id and voice belong to different accounts", ErrMiniMaxBadRequest)
		}
		pinned = fileAccount
		req["text_file_id"] = miniMaxIDJSONValue(textFileID)
	}

	if pinned == nil {
		account, err := s.selectMiniMaxAccount(ctx, groupID, model, nil)
		if err != nil {
			return nil, err
		}
		pinned = account
	}
	if pinned == nil || !pinned.IsSchedulable() {
		return nil, fmt.Errorf("%w: the selected account is unavailable", ErrMiniMaxBadRequest)
	}
	if !pinned.IsModelSupported(model) {
		return nil, fmt.Errorf("%w: the selected account does not support model %s", ErrMiniMaxBadRequest, model)
	}
	upstreamModel := pinned.GetMappedModel(model)
	if upstreamModel == "" {
		upstreamModel = model
	}
	req["model"] = upstreamModel

	client, err := newMiniMaxAPIClient(pinned)
	if err != nil {
		return nil, err
	}
	respBody, statusCode, err := client.postJSON(ctx, "/v1/t2a_async_v2", req)
	if err != nil {
		return nil, fmt.Errorf("minimax async tts create: %w", err)
	}
	if err := miniMaxEnsureOK(respBody, statusCode); err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("minimax async tts create: invalid response")
	}
	taskID := miniMaxAnyID(out["task_id"])
	if taskID == "" {
		return nil, fmt.Errorf("minimax async tts create: response has no task_id")
	}
	fileID := miniMaxAnyID(out["file_id"])
	billedChars := miniMaxAnyInt(out["usage_characters"])

	if err := s.saveMiniMaxTaskBinding(ctx, taskID, MiniMaxTaskBinding{
		AccountID:     pinned.ID,
		UserID:        userID,
		APIKeyID:      apiKeyID,
		Model:         model,
		UpstreamModel: upstreamModel,
		BilledChars:   billedChars,
		FileID:        fileID,
	}); err != nil {
		return nil, err
	}
	if fileID != "" {
		if err := s.saveMiniMaxFileBinding(ctx, fileID, pinned, userID, apiKeyID, "t2a_async"); err != nil {
			return nil, err
		}
	}
	// 异步任务在创建时就已由上游受理并返回 usage_characters；若等到查询才计费，
	// 用户可以只提交不查询，让网关单方面承担上游成本。这里创建即按字符结算；
	// 查询成功时用同一 localID 再结算一次是幂等的（作为临时故障的安全网）。
	if billedChars > 0 {
		s.settleMiniMaxAudioByChars(ctx,
			"mmxasync-"+strconv.FormatInt(pinned.ID, 10)+"-"+taskID,
			userID, apiKeyID, pinned, model, upstreamModel, billedChars)
	}
	// 上游 task_token 是账号凭据的一部分，不下发给终端用户；task_id/file_id 保持数字。
	delete(out, "task_token")
	return out, nil
}

func (s *MediaTaskService) MiniMaxQueryAsyncTTS(
	ctx context.Context,
	userID int64,
	taskID string,
) (map[string]any, error) {
	binding, account, err := s.loadMiniMaxTaskBinding(ctx, taskID, userID)
	if err != nil {
		return nil, err
	}
	client, err := newMiniMaxAPIClient(account)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("task_id", strings.TrimSpace(taskID))
	respBody, statusCode, err := client.getBytes(ctx, "/v1/query/t2a_async_query_v2?"+query.Encode())
	if err != nil {
		return nil, fmt.Errorf("minimax async tts query: %w", err)
	}
	if err := miniMaxEnsureOK(respBody, statusCode); err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("minimax async tts query: invalid response")
	}
	status := strings.ToLower(strings.TrimSpace(miniMaxString(out["status"])))
	fileID := miniMaxAnyID(out["file_id"])
	if fileID == "" {
		fileID = binding.FileID
	}
	if fileID != "" {
		if err := s.saveMiniMaxFileBinding(ctx, fileID, account, binding.UserID, binding.APIKeyID, "t2a_async"); err != nil {
			return nil, err
		}
	}
	// 只有成功终态才结算：失败/过期/处理中都不计费。localID 稳定，重复轮询不重复扣。
	if status == "success" && binding.BilledChars > 0 {
		localID := "mmxasync-" + strconv.FormatInt(account.ID, 10) + "-" + strings.TrimSpace(taskID)
		model := binding.Model
		if model == "" {
			model = miniMaxDefaultTTSModel
		}
		s.settleMiniMaxAudioByChars(ctx, localID, binding.UserID, binding.APIKeyID, account, model, binding.UpstreamModel, binding.BilledChars)
	}
	return out, nil
}

// miniMaxDownload 是下载端点的流式响应：调用方负责关闭 Body。
type miniMaxDownload struct {
	Body          io.ReadCloser
	ContentType   string
	ContentLength int64
}

func (s *MediaTaskService) MiniMaxDownloadFile(
	ctx context.Context,
	userID int64,
	fileID string,
) (*miniMaxDownload, error) {
	_, account, err := s.loadMiniMaxFileBinding(ctx, fileID, userID)
	if err != nil {
		return nil, err
	}
	client, err := newMiniMaxAPIClient(account)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("file_id", strings.TrimSpace(fileID))
	resp, err := client.get(ctx, "/v1/files/retrieve_content?"+query.Encode())
	if err != nil {
		return nil, fmt.Errorf("minimax file download: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		_ = resp.Body.Close()
		return nil, fmt.Errorf("minimax file download: upstream returned HTTP %d", resp.StatusCode)
	}
	contentType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	if strings.Contains(strings.ToLower(contentType), "application/json") {
		// MiniMax 部分错误是 HTTP 200 + JSON；不能把错误 JSON 当音频回给客户端。
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		_ = resp.Body.Close()
		return nil, fmt.Errorf("minimax file download: upstream returned json instead of audio")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return &miniMaxDownload{
		Body:          resp.Body,
		ContentType:   contentType,
		ContentLength: resp.ContentLength,
	}, nil
}
