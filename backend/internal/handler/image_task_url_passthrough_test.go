package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// newAsyncImageTestRouter 搭一个最小可用路由：注入 OpenAI 分组的 API Key。
func newAsyncImageTestRouter(tasks *service.ImageTaskService, h *AsyncImageHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		groupID := int64(3)
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
			ID:      9,
			UserID:  7,
			GroupID: &groupID,
			Group:   &service.Group{ID: groupID, Platform: service.PlatformOpenAI, AllowImageGeneration: true},
		})
		c.Next()
	})
	router.POST("/v1/images/generations/async", h.Submit)
	return router
}

func passthroughImageTasks(store *asyncImageMemoryStore) *service.ImageTaskService {
	// enabled=true 但 uploader=nil：URL 透传模式（无对象存储）。
	return service.NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
}

// 没有对象存储时，异步 + b64_json 必须在提交那一刻 400：
// 否则只能等上游生成完（已扣费）才在落库时失败——用户没拿到图，钱也没了。
func TestAsyncImageSubmitRejectsB64JSONWithoutObjectStorage(t *testing.T) {
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	h := &AsyncImageHandler{tasks: passthroughImageTasks(store)}
	// execute 只是占位：下面的请求应当被前置校验挡住，根本轮不到执行。
	// （Submit 在这之前会检查 execute 是否为 nil，故必须给一个空实现。）
	h.execute = func(_ string, _ *gin.Context) {}
	router := newAsyncImageTestRouter(nil, h)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async",
		strings.NewReader(`{"model":"gpt-image-1","prompt":"cat","response_format":"b64_json"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "b64_json")
	require.Contains(t, w.Body.String(), "object storage")
	require.Empty(t, store.tasks, "被拒的请求不能创建任务，也不能产生任何扣费")
}

// multipart（/v1/images/edits）同样要拦住：它的 response_format 写在表单字段里，
// 只检查 JSON body 会让这条路径漏过去。
func TestAsyncImageSubmitRejectsB64JSONMultipartWithoutObjectStorage(t *testing.T) {
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	h := &AsyncImageHandler{tasks: passthroughImageTasks(store)}
	h.execute = func(_ string, _ *gin.Context) {}
	router := newAsyncImageTestRouter(nil, h)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-1"))
	require.NoError(t, writer.WriteField("prompt", "cat"))
	require.NoError(t, writer.WriteField("response_format", "b64_json"))
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "object storage")
	require.Empty(t, store.tasks)
}

// 没有对象存储但产物是 URL 的请求必须照常放行——这正是 URL 透传模式存在的意义。
func TestAsyncImageSubmitAllowsURLRequestWithoutObjectStorage(t *testing.T) {
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	h := &AsyncImageHandler{tasks: passthroughImageTasks(store)}
	h.execute = func(_ string, c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"created": 1, "data": []gin.H{{"url": "https://upstream.test/a.png"}}})
	}
	router := newAsyncImageTestRouter(nil, h)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async",
		strings.NewReader(`{"model":"gpt-image-1","prompt":"cat"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)

	var accepted struct {
		TaskID string `json:"task_id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &accepted))
	require.Eventually(t, func() bool {
		got, err := h.tasks.Get(context.Background(), service.ImageTaskOwner{UserID: 7, APIKeyID: 9}, accepted.TaskID)
		return err == nil && got.Status == service.ImageTaskStatusSucceeded &&
			strings.Contains(string(got.Result), "https://upstream.test/a.png")
	}, 2*time.Second, 10*time.Millisecond)
}

// 有对象存储时 b64_json 是合法请求（uploader 会转存），不能被提交侧校验误伤。
func TestAsyncImageSubmitAllowsB64JSONWithObjectStorage(t *testing.T) {
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks := service.NewImageTaskServiceWithResolver(store, func() (*service.ImageResultUploader, bool) {
		return service.NewImageResultUploader(&stubPassthroughImageStorage{}, "images/", 0, nil), true
	}, time.Hour, time.Minute)
	h := &AsyncImageHandler{tasks: tasks}
	h.execute = func(_ string, c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"created": 1, "data": []gin.H{{"url": "https://upstream.test/a.png"}}})
	}
	router := newAsyncImageTestRouter(nil, h)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async",
		strings.NewReader(`{"model":"gpt-image-1","prompt":"cat","response_format":"b64_json"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
}

// stubPassthroughImageStorage 只用于让 OffloadEnabled() 为真，不参与真实转存。
type stubPassthroughImageStorage struct{}

func (s *stubPassthroughImageStorage) Save(_ context.Context, key, _ string, _ []byte) (string, error) {
	return "https://cdn.test/" + key, nil
}

var _ io.Reader = bytes.NewReader(nil)
