package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// 本文件锁住本轮扫描修掉的三个缺陷：
//   1. PollTask/GetTask 对「无上游任务号」的任务永不落终态，预扣永久挂账；
//   2. GetTask 刷新成功后仍返回刷新前的陈旧内存态；
//   3. 错误写出口未标记 response_committed，导致 handler 再追加一份错误体。

// recordingMediaTaskRepo 记录超时兜底是否真的走到写库。
type recordingMediaTaskRepo struct {
	fakeMediaTaskRepoForWait
	timeoutClaimed bool
	timeoutStatus  string
	timeoutErrMsg  string
}

func (r *recordingMediaTaskRepo) UpdateStatusIfProcessing(_ context.Context, _ int64, status, errorMsg string) (bool, error) {
	r.timeoutClaimed = true
	r.timeoutStatus = status
	r.timeoutErrMsg = errorMsg
	return true, nil
}

func newTimeoutProbeService(repo *recordingMediaTaskRepo) *MediaTaskService {
	return &MediaTaskService{
		mediaTaskRepo: repo,
		logger:        zap.NewNop(),
	}
}

func TestMediaTaskPollTask_NoUpstreamTaskIDStillTimesOut(t *testing.T) {
	t.Parallel()

	// 关键回归：UpstreamTaskID 为空的任务永远问不到上游结果。若 PollTask 先判空再判超时，
	// 它就永久停在 processing——既不交付也不退预扣，还缺一条 usage_logs。
	repo := &recordingMediaTaskRepo{fakeMediaTaskRepoForWait: fakeMediaTaskRepoForWait{
		record: &MediaTaskRecord{LocalID: "img_no_id", UserID: 7, Status: "processing"},
	}}
	svc := newTimeoutProbeService(repo)

	record := &MediaTaskRecord{
		ID:        1,
		LocalID:   "img_no_id",
		UserID:    7,
		Status:    "processing",
		MediaKind: MediaKindImage,
		// 空 UpstreamTaskID 是关键前提
		CreatedAt: time.Now().Add(-maxMediaTaskDurationBeforeFail - time.Minute),
	}

	require.NoError(t, svc.PollTask(context.Background(), record))
	require.True(t, repo.timeoutClaimed, "无上游任务号的老任务也必须走超时兜底")
	require.Equal(t, "failed", repo.timeoutStatus)
	require.Contains(t, repo.timeoutErrMsg, "timed out")
}

func TestMediaTaskPollTask_FreshTaskWithoutUpstreamTaskIDIsNotFailed(t *testing.T) {
	t.Parallel()

	// 反向护栏：刚创建的任务不能因为没任务号就被立刻判死（上游可能稍后才回填 task_id）。
	repo := &recordingMediaTaskRepo{fakeMediaTaskRepoForWait: fakeMediaTaskRepoForWait{
		record: &MediaTaskRecord{LocalID: "img_fresh", UserID: 7, Status: "processing"},
	}}
	svc := newTimeoutProbeService(repo)

	record := &MediaTaskRecord{
		ID:        2,
		LocalID:   "img_fresh",
		UserID:    7,
		Status:    "processing",
		MediaKind: MediaKindImage,
		CreatedAt: time.Now(),
	}

	require.NoError(t, svc.PollTask(context.Background(), record))
	require.False(t, repo.timeoutClaimed, "未超时的任务不能被误判 failed")
}

// 说明：GetTask「刷新成功后重读」的修复此处不写单测 —— 复现它需要 accountService 与
// adapter 两个非 nil 依赖，而本包 bare 层没有可复用的 AccountRepo 假实现
// （gateway_multiplatform_test.go 等 mock 都在 //go:build unit 层）。
// 该修复由 media_task_service.go GetTask 内的读写配对保证，已在 CR 中逐处核对。

func TestWriteAnthropicErrorMarksResponseCommitted(t *testing.T) {
	t.Parallel()

	// 横向一致性：writeChatCompletionsError / writeResponsesError 都会标记，
	// writeAnthropicError 曾漏掉 → ensureForwardErrorResponse 会再追加一份错误体。
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	writeAnthropicError(c, http.StatusBadRequest, "invalid_request_error", "bad request")

	require.True(t, IsResponseCommitted(c))
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWriteGrokMediaErrorResponseMarksResponseCommitted(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)

	writeGrokMediaErrorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream request failed")

	require.True(t, IsResponseCommitted(c))
	require.Equal(t, http.StatusBadGateway, w.Code)
}

func TestGrokMediaClientErrorMessageIsStatusDerivedOnly(t *testing.T) {
	t.Parallel()

	// 客户端文案只能由状态码推导，绝不能回显上游原文。
	cases := []struct {
		status int
		want   string
	}{
		{http.StatusBadRequest, "Upstream request failed"},
		{http.StatusUnauthorized, "Upstream authentication failed"},
		{http.StatusForbidden, "Upstream access denied"},
		{http.StatusNotFound, "Upstream resource not found"},
		{http.StatusTooManyRequests, "Upstream rate limit exceeded"},
		{http.StatusInternalServerError, "Upstream service temporarily unavailable"},
		{http.StatusServiceUnavailable, "Upstream service temporarily unavailable"},
	}
	for _, tc := range cases {
		got := grokMediaClientErrorMessage(tc.status)
		require.Equal(t, tc.want, got, "status=%d", tc.status)
		require.NotContains(t, got, "upstream:", "不得夹带上游原文")
	}
}
