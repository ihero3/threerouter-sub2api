package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// ctxCheckingImageStore 模拟真实 Redis 存储：ctx 已取消时 Save 会失败。
// 用它才能暴露"落库沿用了会被取消的请求 context"这类缺陷。
type ctxCheckingImageStore struct{ inner *asyncImageMemoryStore }

func (s *ctxCheckingImageStore) Save(ctx context.Context, task *service.ImageTaskRecord, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.inner.Save(ctx, task, ttl)
}

func (s *ctxCheckingImageStore) Get(ctx context.Context, id string) (*service.ImageTaskRecord, error) {
	return s.inner.Get(ctx, id)
}

// withAsyncImageAPIKey 注入一个可指定 owner 的 API Key，便于验证跨用户隔离。
func withAsyncImageAPIKey(userID, apiKeyID int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		groupID := int64(3)
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
			ID:      apiKeyID,
			UserID:  userID,
			GroupID: &groupID,
			Group:   &service.Group{ID: groupID, Platform: service.PlatformOpenAI, AllowImageGeneration: true},
		})
		c.Next()
	}
}

// newAsyncImageTestHandler 装配一个启用状态的任务服务 + 可控执行体。
func newAsyncImageTestHandler(t *testing.T) (*AsyncImageHandler, *asyncImageMemoryStore, *int) {
	t.Helper()
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks := service.NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
	h := &AsyncImageHandler{tasks: tasks}
	runs := 0
	h.execute = func(_ string, c *gin.Context) {
		runs++
		c.JSON(http.StatusOK, gin.H{"created": 1, "data": []gin.H{{"url": "https://example.test/a.png"}}})
	}
	return h, store, &runs
}

// useMemoryIdempotency 用内存仓储装配幂等协调器，并保证测试结束后复位，
// 避免全局状态污染其它用例。
func useMemoryIdempotency(t *testing.T) {
	t.Helper()
	previous := service.DefaultIdempotencyCoordinator()
	service.SetDefaultIdempotencyCoordinator(
		service.NewIdempotencyCoordinator(newUserMemoryIdempotencyRepoStub(), service.DefaultIdempotencyConfig()))
	t.Cleanup(func() { service.SetDefaultIdempotencyCoordinator(previous) })
}

func (s *asyncImageMemoryStore) count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.tasks)
}

// 同一个 request_id 重复提交必须复用原任务：否则客户端在超时后重试会重复生成、
// 重复扣费——这正是引入幂等的目的。
func TestAsyncImageSubmitIsIdempotentByRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useMemoryIdempotency(t)
	h, store, _ := newAsyncImageTestHandler(t)

	router := gin.New()
	router.Use(withAsyncImageAPIKey(7, 9))
	router.POST("/v1/images/generations/async", h.Submit)

	payload := `{"model":"gpt-image-1","prompt":"cat"}`
	first := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "req-fixed-1")
	router.ServeHTTP(first, req)
	require.Equal(t, http.StatusAccepted, first.Code)

	var accepted struct {
		TaskID    string `json:"task_id"`
		RequestID string `json:"request_id"`
	}
	require.NoError(t, json.Unmarshal(first.Body.Bytes(), &accepted))
	require.NotEmpty(t, accepted.TaskID)
	require.Equal(t, "req-fixed-1", accepted.RequestID)
	require.Empty(t, first.Header().Get("X-Idempotency-Replayed"))

	// 完全相同的第二次提交
	second := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", strings.NewReader(payload))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", "req-fixed-1")
	router.ServeHTTP(second, req2)
	require.Equal(t, http.StatusAccepted, second.Code)

	var replayed struct {
		TaskID string `json:"task_id"`
	}
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &replayed))
	// 同一个任务，且明确告知调用方这是复用而非新建
	require.Equal(t, accepted.TaskID, replayed.TaskID)
	require.Equal(t, "true", second.Header().Get("X-Idempotency-Replayed"))
	require.Equal(t, 1, store.count(), "重复提交不得创建第二个任务")
}

// 同一个幂等键配不同请求体属于调用方误用（键被复用到别的请求上），
// 必须明确拒绝而不是默默返回另一个任务的结果。
func TestAsyncImageSubmitSameKeyDifferentPayloadConflicts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useMemoryIdempotency(t)
	h, store, _ := newAsyncImageTestHandler(t)

	router := gin.New()
	router.Use(withAsyncImageAPIKey(7, 9))
	router.POST("/v1/images/generations/async", h.Submit)

	post := func(prompt string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async",
			strings.NewReader(`{"model":"gpt-image-1","prompt":"`+prompt+`"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "req-conflict")
		router.ServeHTTP(w, req)
		return w
	}

	require.Equal(t, http.StatusAccepted, post("cat").Code)
	conflict := post("dog")
	require.Equal(t, http.StatusConflict, conflict.Code)
	require.Equal(t, 1, store.count())
}

// 不带幂等键时保持原有行为：每次提交都是独立任务，不做任何强制。
func TestAsyncImageSubmitWithoutKeyKeepsLegacyBehavior(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useMemoryIdempotency(t)
	h, store, _ := newAsyncImageTestHandler(t)

	router := gin.New()
	router.Use(withAsyncImageAPIKey(7, 9))
	router.POST("/v1/images/generations/async", h.Submit)

	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async",
			strings.NewReader(`{"model":"gpt-image-1","prompt":"cat"}`))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusAccepted, w.Code)
	}
	require.Equal(t, 2, store.count())
}

// 提交响应丢失后，客户端只能凭 request_id 找回原任务。这条路径是"不重复提交"
// 的最后保障，必须能真的拿到任务状态。
func TestAsyncImageGetByRequestRecoversTask(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useMemoryIdempotency(t)
	h, _, _ := newAsyncImageTestHandler(t)

	router := gin.New()
	router.Use(withAsyncImageAPIKey(7, 9))
	router.POST("/v1/images/generations/async", h.Submit)
	router.GET("/v1/images/generations/by-request/:request_id", h.GetByRequest)

	submit := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async",
		strings.NewReader(`{"model":"gpt-image-1","prompt":"cat"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "req-recover")
	router.ServeHTTP(submit, req)
	require.Equal(t, http.StatusAccepted, submit.Code)

	var accepted struct {
		TaskID string `json:"task_id"`
	}
	require.NoError(t, json.Unmarshal(submit.Body.Bytes(), &accepted))

	get := httptest.NewRecorder()
	router.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/v1/images/generations/by-request/req-recover", nil))
	require.Equal(t, http.StatusOK, get.Code)

	var recovered struct {
		TaskID    string `json:"task_id"`
		RequestID string `json:"request_id"`
		Status    string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(get.Body.Bytes(), &recovered))
	require.Equal(t, accepted.TaskID, recovered.TaskID)
	require.Equal(t, "req-recover", recovered.RequestID)
}

// request_id 查询必须受 owner 隔离：否则猜到别人的 request_id 就能读到别人的任务。
func TestAsyncImageGetByRequestIsolatesOwners(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useMemoryIdempotency(t)
	h, _, _ := newAsyncImageTestHandler(t)

	ownerRouter := gin.New()
	ownerRouter.Use(withAsyncImageAPIKey(7, 9))
	ownerRouter.POST("/v1/images/generations/async", h.Submit)
	submit := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async",
		strings.NewReader(`{"model":"gpt-image-1","prompt":"cat"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "req-private")
	ownerRouter.ServeHTTP(submit, req)
	require.Equal(t, http.StatusAccepted, submit.Code)

	// 换一个用户，用同一个 request_id 查询
	otherRouter := gin.New()
	otherRouter.Use(withAsyncImageAPIKey(8, 10))
	otherRouter.GET("/v1/images/generations/by-request/:request_id", h.GetByRequest)
	get := httptest.NewRecorder()
	otherRouter.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/v1/images/generations/by-request/req-private", nil))
	require.Equal(t, http.StatusNotFound, get.Code)
}

// 未知 request_id 返回 404，而不是 500 或空任务。
func TestAsyncImageGetByRequestUnknownReturns404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useMemoryIdempotency(t)
	h, _, _ := newAsyncImageTestHandler(t)

	router := gin.New()
	router.Use(withAsyncImageAPIKey(7, 9))
	router.GET("/v1/images/generations/by-request/:request_id", h.GetByRequest)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/images/generations/by-request/never-seen", nil))
	require.Equal(t, http.StatusNotFound, w.Code)
}

// request_id 是本网关的幂等标识，不能透传给上游：OpenAI 官方 API 会因未知参数
// 直接拒绝整个请求（Unrecognized request argument）。
func TestStripImageTaskRequestIDRemovesFieldBeforeForwarding(t *testing.T) {
	body := []byte(`{"model":"gpt-image-1","prompt":"cat","request_id":"req-1","n":2}`)
	stripped, requestID := stripImageTaskRequestID("application/json", body)
	require.Equal(t, "req-1", requestID)
	require.NotContains(t, string(stripped), "request_id")
	// 其它字段必须原样保留
	require.Contains(t, string(stripped), `"prompt":"cat"`)
	require.Contains(t, string(stripped), `"n":2`)
	require.Contains(t, string(stripped), `"model":"gpt-image-1"`)

	// 没有 request_id 时原样返回
	plain := []byte(`{"model":"gpt-image-1","prompt":"cat"}`)
	got, id := stripImageTaskRequestID("application/json", plain)
	require.Empty(t, id)
	require.Equal(t, plain, got)

	// 畸形 multipart：解析不出结构时原样放行，不能把 body 改坏
	gotMulti, idMulti := stripImageTaskRequestID("multipart/form-data; boundary=x", []byte(`request_id`))
	require.Empty(t, idMulti)
	require.Equal(t, []byte(`request_id`), gotMulti)

	// 非 JSON body 不应被误改
	gotRaw, idRaw := stripImageTaskRequestID("image/png", []byte{0x89, 0x50, 0x4E, 0x47})
	require.Empty(t, idRaw)
	require.Equal(t, []byte{0x89, 0x50, 0x4E, 0x47}, gotRaw)
}

// 幂等域必须按 owner 隔离，否则不同用户写同一个 request_id 会互相撞 409。
func TestImageTaskIdempotencyScopeIsOwnerScoped(t *testing.T) {
	a := service.ImageTaskIdempotencyScope("user:1")
	b := service.ImageTaskIdempotencyScope("user:2")
	require.NotEqual(t, a, b)
	require.Equal(t, a, service.ImageTaskIdempotencyScope("user:1"))
	// 缺失 owner 时退化为固定域而不是空串（空 scope 会被协调器直接拒绝）
	require.NotEmpty(t, service.ImageTaskIdempotencyScope(""))
	require.True(t, strings.HasPrefix(a, service.ImageTaskIdempotencyScopeBase))
}

// 提交一旦被接受，就必须留下可找回的任务。客户端在提交瞬间断开恰恰是本端点
// 要服务的场景（"提交后立刻断开、稍后再取"），此时落库若沿用请求 context 会
// 直接失败：既无任务可查，幂等键还被占住，调用方反而更难恢复。
func TestAsyncImageSubmitSurvivesClientDisconnect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useMemoryIdempotency(t)

	inner := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks := service.NewImageTaskServiceWithUploader(&ctxCheckingImageStore{inner: inner}, nil, time.Hour, time.Minute)
	h := &AsyncImageHandler{tasks: tasks}
	h.execute = func(_ string, c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"created": 1, "data": []gin.H{{"url": "https://example.test/a.png"}}})
	}

	router := gin.New()
	router.Use(withAsyncImageAPIKey(7, 9))
	router.POST("/v1/images/generations/async", h.Submit)

	requestCtx, cancel := context.WithCancel(context.Background())
	cancel() // 客户端已断开
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async",
		strings.NewReader(`{"model":"gpt-image-1","prompt":"cat"}`)).WithContext(requestCtx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "req-disconnect")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	require.Equal(t, 1, inner.count(), "断连也必须留下可找回的任务")
}

// multipart 提交同样要支持请求体形式的 request_id，并且必须把该字段从下发上游的
// body 中摘掉：multipart 是 /v1/images/edits 的标准形态，两个问题都真实存在。
func TestStripImageTaskRequestIDMultipart(t *testing.T) {
	body, contentType := buildAsyncImageMultipart(t, map[string]string{
		"model":      "gpt-image-1",
		"prompt":     "turn it blue",
		"request_id": " req-mp-1 ",
	}, []byte("fake-png"))

	stripped, requestID := stripImageTaskRequestID(contentType, body)
	require.Equal(t, "req-mp-1", requestID)

	fields, file := parseAsyncImageMultipart(t, stripped, contentType)
	require.NotContains(t, fields, "request_id")
	require.Equal(t, "gpt-image-1", fields["model"])
	require.Equal(t, "turn it blue", fields["prompt"])
	require.Equal(t, []byte("fake-png"), file)

	// 没有该字段时必须原样返回：不重建 body、不改 boundary，避免给所有
	// multipart 上传都付一次重建开销。
	clean, cleanType := buildAsyncImageMultipart(t, map[string]string{
		"model":  "gpt-image-1",
		"prompt": "no key here",
	}, []byte("fake-png"))
	afterStrip, noID := stripImageTaskRequestID(cleanType, clean)
	require.Empty(t, noID)
	require.Equal(t, clean, afterStrip)
}

func buildAsyncImageMultipart(t *testing.T, fields map[string]string, fileContent []byte) ([]byte, string) {
	t.Helper()
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	for name, value := range fields {
		require.NoError(t, writer.WriteField(name, value))
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="image"; filename="a.png"`)
	header.Set("Content-Type", "image/png")
	part, err := writer.CreatePart(header)
	require.NoError(t, err)
	_, err = part.Write(fileContent)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return buffer.Bytes(), writer.FormDataContentType()
}

func parseAsyncImageMultipart(t *testing.T, body []byte, contentType string) (map[string]string, []byte) {
	t.Helper()
	_, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)
	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	fields := map[string]string{}
	var file []byte
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		data, err := io.ReadAll(part)
		require.NoError(t, err)
		require.NoError(t, part.Close())
		if part.FileName() != "" {
			file = data
			continue
		}
		fields[part.FormName()] = string(data)
	}
	return fields, file
}
