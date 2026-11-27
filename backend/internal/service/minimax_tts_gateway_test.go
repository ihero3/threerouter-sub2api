package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// --- 测试替身 ---

type stubAccountRepo struct {
	AccountRepository
	account *Account
}

func (s *stubAccountRepo) GetByID(context.Context, int64) (*Account, error) {
	if s.account == nil {
		return nil, errors.New("account not found")
	}
	return s.account, nil
}

type memoryBindingStore struct {
	mu    sync.Mutex
	files map[string]MiniMaxFileBinding
	tasks map[string]MiniMaxTaskBinding
}

func newMemoryBindingStore() *memoryBindingStore {
	return &memoryBindingStore{
		files: map[string]MiniMaxFileBinding{},
		tasks: map[string]MiniMaxTaskBinding{},
	}
}

func (m *memoryBindingStore) SaveFile(_ context.Context, id string, b MiniMaxFileBinding, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[id] = b
	return nil
}

func (m *memoryBindingStore) GetFile(_ context.Context, id string) (*MiniMaxFileBinding, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.files[id]
	if !ok {
		return nil, nil
	}
	return &b, nil
}

func (m *memoryBindingStore) SaveTask(_ context.Context, id string, b MiniMaxTaskBinding, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tasks[id] = b
	return nil
}

func (m *memoryBindingStore) GetTask(_ context.Context, id string) (*MiniMaxTaskBinding, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.tasks[id]
	if !ok {
		return nil, nil
	}
	return &b, nil
}

func newMiniMaxTestService(account *Account, store MiniMaxBindingStore) *MediaTaskService {
	return &MediaTaskService{
		accountService:      NewAccountService(&stubAccountRepo{account: account}, nil),
		miniMaxBindingStore: store,
	}
}

func miniMaxTestAccount(baseURL string) *Account {
	return &Account{
		ID:          7,
		Platform:    PlatformMiniMax,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"base_url": baseURL,
			"api_key":  "upstream-key",
		},
	}
}

// --- token ---

func TestMiniMaxVoiceTokenRoundTripAndTamper(t *testing.T) {
	account := &Account{ID: 7, Credentials: map[string]any{"api_key": "upstream-key"}}
	token, err := signMiniMaxVoiceToken(account, miniMaxVoiceToken{UserID: 42, VoiceID: "my-voice"})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(token, miniMaxVoiceTokenPrefix))

	payload, body, sig, err := decodeMiniMaxVoiceToken(token)
	require.NoError(t, err)
	require.Equal(t, int64(7), payload.AccountID)
	require.Equal(t, int64(42), payload.UserID)
	require.Equal(t, "my-voice", payload.VoiceID)
	require.NoError(t, verifyMiniMaxVoiceToken(account, body, sig))

	// 换账号密钥无法验证。
	other := &Account{ID: 7, Credentials: map[string]any{"api_key": "another-key"}}
	require.ErrorIs(t, verifyMiniMaxVoiceToken(other, body, sig), ErrMiniMaxInvalidToken)
	// 篡改签名无法验证。
	tampered := append([]byte(nil), sig...)
	tampered[0] ^= 0xFF
	require.ErrorIs(t, verifyMiniMaxVoiceToken(account, body, tampered), ErrMiniMaxInvalidToken)
	// 非网关音色不能按 token 解析。
	_, _, _, err = decodeMiniMaxVoiceToken("longanyang")
	require.ErrorIs(t, err, ErrMiniMaxBadRequest)
}

func TestResolveMiniMaxVoicePin(t *testing.T) {
	account := miniMaxTestAccount("https://api.minimax.test")
	token, err := signMiniMaxVoiceToken(account, miniMaxVoiceToken{UserID: 42, VoiceID: "cloned-voice"})
	require.NoError(t, err)
	svc := newMiniMaxTestService(account, newMemoryBindingStore())

	req := &MediaCreateRequest{Extra: map[string]any{"voice": token}}
	pinned, err := svc.resolveMiniMaxVoicePin(context.Background(), 42, "speech-2.8-hd", req)
	require.NoError(t, err)
	require.NotNil(t, pinned)
	require.Equal(t, int64(7), pinned.ID)
	require.Equal(t, "cloned-voice", req.Extra["voice"], "网关音色 ID 必须还原成上游 voice_id")

	// 其它用户不能复用。
	req = &MediaCreateRequest{Extra: map[string]any{"voice": token}}
	_, err = svc.resolveMiniMaxVoicePin(context.Background(), 99, "speech-2.8-hd", req)
	require.ErrorIs(t, err, ErrMiniMaxInvalidToken)

	// 非 MiniMax 模型不允许使用网关音色。
	req = &MediaCreateRequest{Extra: map[string]any{"voice": token}}
	_, err = svc.resolveMiniMaxVoicePin(context.Background(), 42, "qwen-audio-3.1-tts-flash", req)
	require.ErrorIs(t, err, ErrMiniMaxBadRequest)

	// 普通上游音色原样放行。
	req = &MediaCreateRequest{Extra: map[string]any{"voice": "longanyang"}}
	pinned, err = svc.resolveMiniMaxVoicePin(context.Background(), 42, "speech-2.8-hd", req)
	require.NoError(t, err)
	require.Nil(t, pinned)
	require.Equal(t, "longanyang", req.Extra["voice"])

	// 原生 MiniMax 请求放在 voice_setting.voice_id 里，也要能识别并还原。
	req = &MediaCreateRequest{Extra: map[string]any{
		"voice_setting": map[string]any{"voice_id": token, "speed": 1.0},
	}}
	pinned, err = svc.resolveMiniMaxVoicePin(context.Background(), 42, "speech-2.8-hd", req)
	require.NoError(t, err)
	require.NotNil(t, pinned)
	require.Equal(t, "cloned-voice", req.Extra["voice_setting"].(map[string]any)["voice_id"])
	require.Equal(t, 1.0, req.Extra["voice_setting"].(map[string]any)["speed"])
}

// --- 原生 API 客户端 ---

func TestMiniMaxAPIClientEndpoints(t *testing.T) {
	var (
		uploadPurpose string
		uploaded      []byte
		voiceCloneReq map[string]any
		queryTaskID   string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/files/upload":
			require.NoError(t, r.ParseMultipartForm(8<<20))
			uploadPurpose = r.FormValue("purpose")
			file, _, err := r.FormFile("file")
			require.NoError(t, err)
			defer func() { _ = file.Close() }()
			uploaded, _ = io.ReadAll(file)
			_, _ = w.Write([]byte(`{"file":{"file_id":123,"purpose":"voice_clone"},"base_resp":{"status_code":0}}`))
		case "/v1/voice_clone":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&voiceCloneReq))
			_, _ = w.Write([]byte(`{"extra_info":{"usage_characters":9},"base_resp":{"status_code":0}}`))
		case "/v1/query/t2a_async_query_v2":
			queryTaskID = r.URL.Query().Get("task_id")
			_, _ = w.Write([]byte(`{"task_id":951,"status":"Success","file_id":952,"base_resp":{"status_code":0}}`))
		case "/v1/files/retrieve_content":
			_, _ = w.Write([]byte("RIFFfake"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client, err := newMiniMaxAPIClient(&Account{ID: 7, Credentials: map[string]any{
		"base_url": srv.URL,
		"api_key":  "k",
	}})
	require.NoError(t, err)
	client.httpClient = srv.Client()
	client.downloadClient = srv.Client()

	resp, status, err := client.postMultipart(context.Background(), "/v1/files/upload", "voice_clone", "a.mp3", []byte("ID3fake"))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "voice_clone", uploadPurpose)
	require.Equal(t, []byte("ID3fake"), uploaded)
	require.Contains(t, string(resp), "123")

	resp, status, err = client.postJSON(context.Background(), "/v1/voice_clone", map[string]any{"file_id": 123, "voice_id": "v1"})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, float64(123), voiceCloneReq["file_id"])
	require.Contains(t, string(resp), "usage_characters")

	queryResp, status, err := client.getBytes(context.Background(), "/v1/query/t2a_async_query_v2?task_id=951")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "951", queryTaskID)
	require.Contains(t, string(queryResp), "Success")

	download, err := client.get(context.Background(), "/v1/files/retrieve_content?file_id=952")
	require.NoError(t, err)
	defer func() { _ = download.Body.Close() }()
	body, _ := io.ReadAll(download.Body)
	require.Equal(t, "RIFFfake", string(body))
}

func TestMiniMaxEnsureOK(t *testing.T) {
	require.NoError(t, miniMaxEnsureOK([]byte(`{"base_resp":{"status_code":0}}`), http.StatusOK))
	require.Error(t, miniMaxEnsureOK([]byte(`{"base_resp":{"status_code":1004}}`), http.StatusOK))
	require.Error(t, miniMaxEnsureOK([]byte(`{"base_resp":{"status_code":0}}`), http.StatusBadRequest))
	require.Error(t, miniMaxEnsureOK([]byte(`not-json`), http.StatusOK))
}

// --- 业务流：异步创建 / 查询 / 下载 / 复刻 ---

func TestMiniMaxAsyncTTSFlowAndBindings(t *testing.T) {
	var seenCreate map[string]any
	var seenQueryTask string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/t2a_async_v2":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&seenCreate))
			_, _ = w.Write([]byte(`{"task_id":951,"file_id":952,"usage_characters":11,"task_token":"secret","base_resp":{"status_code":0}}`))
		case "/v1/query/t2a_async_query_v2":
			seenQueryTask = r.URL.Query().Get("task_id")
			_, _ = w.Write([]byte(`{"task_id":951,"status":"Success","file_id":952,"base_resp":{"status_code":0}}`))
		case "/v1/files/retrieve_content":
			_, _ = w.Write([]byte("RIFFaudio"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	account := miniMaxTestAccount(srv.URL)
	store := newMemoryBindingStore()
	svc := newMiniMaxTestService(account, store)

	voiceToken, err := signMiniMaxVoiceToken(account, miniMaxVoiceToken{UserID: 42, VoiceID: "cloned-voice"})
	require.NoError(t, err)

	created, err := svc.MiniMaxCreateAsyncTTS(context.Background(), nil, 42, 100, map[string]any{
		"model": "speech-2.8-hd",
		"text":  "hello",
		"voice_setting": map[string]any{
			"voice_id": voiceToken,
		},
	})
	require.NoError(t, err)
	require.Equal(t, "cloned-voice", seenCreate["voice_setting"].(map[string]any)["voice_id"])
	require.Equal(t, "speech-2.8-hd", seenCreate["model"])
	require.NotContains(t, created, "task_token", "上游 task_token 不能下发给用户")
	require.Equal(t, float64(951), created["task_id"])
	require.Equal(t, float64(952), created["file_id"])

	// 数字 task_id / file_id 的账号归属必须落库。
	taskBinding, ok := store.tasks["951"]
	require.True(t, ok)
	require.Equal(t, int64(7), taskBinding.AccountID)
	require.Equal(t, 11, taskBinding.BilledChars)
	fileBinding, ok := store.files["952"]
	require.True(t, ok)
	require.Equal(t, int64(7), fileBinding.AccountID)

	queried, err := svc.MiniMaxQueryAsyncTTS(context.Background(), 42, "951")
	require.NoError(t, err)
	require.Equal(t, "951", seenQueryTask)
	require.Equal(t, "Success", queried["status"])
	require.Equal(t, float64(952), queried["file_id"])

	download, err := svc.MiniMaxDownloadFile(context.Background(), 42, "952")
	require.NoError(t, err)
	defer func() { _ = download.Body.Close() }()
	audio, _ := io.ReadAll(download.Body)
	require.Equal(t, "RIFFaudio", string(audio))

	// 其它用户不能查询/下载。
	_, err = svc.MiniMaxQueryAsyncTTS(context.Background(), 99, "951")
	require.Error(t, err)
	_, err = svc.MiniMaxDownloadFile(context.Background(), 99, "952")
	require.Error(t, err)
}

func TestMiniMaxVoiceCloneUsesFileBindingAccount(t *testing.T) {
	var seenClone map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/voice_clone" {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&seenClone))
			_, _ = w.Write([]byte(`{"demo_audio":"https://cdn.test/demo.mp3","extra_info":{"usage_characters":5},"base_resp":{"status_code":0}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	account := miniMaxTestAccount(srv.URL)
	store := newMemoryBindingStore()
	svc := newMiniMaxTestService(account, store)
	require.NoError(t, store.SaveFile(context.Background(), "123", MiniMaxFileBinding{
		AccountID: account.ID, UserID: 42, APIKeyID: 100, Purpose: "voice_clone",
	}, time.Hour))
	require.NoError(t, store.SaveFile(context.Background(), "124", MiniMaxFileBinding{
		AccountID: account.ID, UserID: 42, APIKeyID: 100, Purpose: "prompt_audio",
	}, time.Hour))

	out, err := svc.MiniMaxVoiceClone(context.Background(), 42, 100, map[string]any{
		"file_id":  float64(123),
		"voice_id": "MiniMax001",
		"model":    "speech-2.8-hd",
		"text":     "hello",
		"clone_prompt": map[string]any{
			"prompt_audio": float64(124),
			"prompt_text":  "hello prompt",
		},
	})
	require.NoError(t, err)
	require.Equal(t, float64(123), seenClone["file_id"])
	require.Equal(t, float64(124), seenClone["clone_prompt"].(map[string]any)["prompt_audio"])
	require.True(t, strings.HasPrefix(out["voice_id"].(string), miniMaxVoiceTokenPrefix))
	require.Equal(t, "MiniMax001", out["upstream_voice_id"])
	require.Equal(t, "https://cdn.test/demo.mp3", out["demo_audio"])
}

func TestMiniMaxBindingMissingIDFails(t *testing.T) {
	account := miniMaxTestAccount("https://api.minimax.test")
	svc := newMiniMaxTestService(account, newMemoryBindingStore())
	_, err := svc.MiniMaxVoiceClone(context.Background(), 42, 100, map[string]any{
		"file_id":  float64(999),
		"voice_id": "MiniMax001",
	})
	require.ErrorIs(t, err, ErrMiniMaxBadRequest)

	_, err = svc.MiniMaxQueryAsyncTTS(context.Background(), 42, "999")
	require.ErrorIs(t, err, ErrMiniMaxBadRequest)

	_, err = svc.MiniMaxDownloadFile(context.Background(), 42, "999")
	require.ErrorIs(t, err, ErrMiniMaxBadRequest)
}
