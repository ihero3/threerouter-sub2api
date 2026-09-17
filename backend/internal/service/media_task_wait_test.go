package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fakeMediaTaskRepoForWait 只实现等待测试需要的查询能力：
// 按 local_id 返回固定记录，其余方法返回零值。
type fakeMediaTaskRepoForWait struct {
	record *MediaTaskRecord
}

func (f *fakeMediaTaskRepoForWait) Create(ctx context.Context, task *MediaTaskRecord) (*MediaTaskRecord, error) {
	return task, nil
}

func (f *fakeMediaTaskRepoForWait) GetByLocalID(ctx context.Context, localID string) (*MediaTaskRecord, error) {
	if f.record == nil || f.record.LocalID != localID {
		return nil, errMediaTaskNotFoundForTest
	}
	return f.record, nil
}

func (f *fakeMediaTaskRepoForWait) GetByID(ctx context.Context, id int64) (*MediaTaskRecord, error) {
	return f.record, nil
}

func (f *fakeMediaTaskRepoForWait) UpdateStatusIfProcessing(ctx context.Context, id int64, status, errorMsg string) (bool, error) {
	return true, nil
}

func (f *fakeMediaTaskRepoForWait) UpdateResult(ctx context.Context, id int64, status, mediaURL, thumbnailURL string, durationSec int, costUSD float64) (bool, error) {
	return true, nil
}

func (f *fakeMediaTaskRepoForWait) UpdateUpstreamTaskID(ctx context.Context, id int64, upstreamTaskID string) error {
	return nil
}

func (f *fakeMediaTaskRepoForWait) ListByUserID(ctx context.Context, userID int64, limit, offset int) ([]*MediaTaskRecord, int, error) {
	return nil, 0, nil
}

func (f *fakeMediaTaskRepoForWait) ListProcessingTasks(ctx context.Context, before time.Time, limit int) ([]*MediaTaskRecord, error) {
	return nil, nil
}

func (f *fakeMediaTaskRepoForWait) ListAdmin(ctx context.Context, userID int64, status, mediaKind string, limit, offset int) ([]*MediaTaskRecord, int, error) {
	return nil, 0, nil
}

var errMediaTaskNotFoundForTest = &mediaTaskNotFound{}

type mediaTaskNotFound struct{}

func (e *mediaTaskNotFound) Error() string { return "media task not found" }

func TestAwaitTerminal_TerminalStatusReturnsImmediately(t *testing.T) {
	svc := &MediaTaskService{mediaTaskRepo: &fakeMediaTaskRepoForWait{
		record: &MediaTaskRecord{LocalID: "img_1", UserID: 7, Status: "succeeded", MediaURL: "https://example.com/a.png"},
	}}
	start := time.Now()
	record, err := svc.AwaitTerminal(context.Background(), "img_1", 7, 30*time.Second)
	require.NoError(t, err)
	require.Equal(t, "succeeded", record.Status)
	require.Less(t, time.Since(start), time.Second, "终态任务不该进入轮询")
}

func TestAwaitTerminal_ZeroTimeoutOnlyRefreshesOnce(t *testing.T) {
	svc := &MediaTaskService{mediaTaskRepo: &fakeMediaTaskRepoForWait{
		// 无 UpstreamTaskID：GetTask 内部不会触发上游刷新，等待循环走纯轮询分支。
		record: &MediaTaskRecord{LocalID: "img_2", UserID: 7, Status: "processing"},
	}}
	start := time.Now()
	record, err := svc.AwaitTerminal(context.Background(), "img_2", 7, 0)
	require.NoError(t, err)
	require.Equal(t, "processing", record.Status)
	require.Less(t, time.Since(start), time.Second, "不传 wait 时不应额外等待")
}

func TestAwaitTerminal_StopsAtDeadline(t *testing.T) {
	svc := &MediaTaskService{mediaTaskRepo: &fakeMediaTaskRepoForWait{
		record: &MediaTaskRecord{LocalID: "img_3", UserID: 7, Status: "processing"},
	}}
	record, err := svc.AwaitTerminal(context.Background(), "img_3", 7, 3*time.Second)
	require.NoError(t, err)
	require.Equal(t, "processing", record.Status, "超时后仍应返回最后一次已知状态")
}

func TestAwaitTerminal_OwnershipMismatch(t *testing.T) {
	svc := &MediaTaskService{mediaTaskRepo: &fakeMediaTaskRepoForWait{
		record: &MediaTaskRecord{LocalID: "img_4", UserID: 7, Status: "succeeded"},
	}}
	_, err := svc.AwaitTerminal(context.Background(), "img_4", 8, time.Second)
	require.Error(t, err, "不能查到别人的任务")
}

func TestIsMediaTaskTerminal(t *testing.T) {
	require.True(t, IsMediaTaskTerminal("succeeded"))
	require.True(t, IsMediaTaskTerminal("failed"))
	require.True(t, IsMediaTaskTerminal("cancelled"))
	require.False(t, IsMediaTaskTerminal("processing"))
}

func TestDownloadMediaBytes_RejectsNonHTTPScheme(t *testing.T) {
	_, _, err := DownloadMediaBytes(context.Background(), "file:///tmp/a.png")
	require.Error(t, err)
}

func TestDownloadMediaBytesLimit_RespectsLimit(t *testing.T) {
	// Content-Length 超过上限时必须直接拒绝：先完整下载再判断会白白消耗
	// 出口带宽，还会让客户端多等几十秒才拿到一个必然失败的结果。
	payload := make([]byte, 4096)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	data, _, err := DownloadMediaBytesLimit(context.Background(), server.URL+"/blob", 8192)
	require.NoError(t, err)
	require.Len(t, data, 4096)

	_, _, err = DownloadMediaBytesLimit(context.Background(), server.URL+"/blob", 1024)
	require.Error(t, err, "声明长度超过上限必须报错")
	require.Contains(t, err.Error(), "exceeds limit")

	_, _, err = DownloadMediaBytesLimit(context.Background(), server.URL+"/blob", 0)
	require.Error(t, err, "无效上限必须报错")
}

func TestDownloadMediaBytesLimit_TruncatedBodyDetected(t *testing.T) {
	// 没有 Content-Length 时读满 limit 并不代表"刚好到上限"，必须靠多读一字节
	// 才能区分，否则超限文件会被静默截断成损坏数据。
	payload := make([]byte, 4096)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Transfer-Encoding", "chunked")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	_, _, err := DownloadMediaBytesLimit(context.Background(), server.URL+"/blob", 1024)
	require.Error(t, err, "无 Content-Length 的超限响应也必须报错")
}

func TestMediaStorageKeyDistinguishesIndexedImages(t *testing.T) {
	record := &MediaTaskRecord{LocalID: "img_abc123", MediaKind: MediaKindImage}
	// 同一任务不同序号必须落到不同 key（否则后写覆盖先写）。
	k0 := mediaStorageKey(record, "image/png")
	k1 := mediaStorageKey(record, "image/png", "1")
	k2 := mediaStorageKey(record, "image/png", "2")
	require.NotEqual(t, k0, k1)
	require.NotEqual(t, k1, k2)
	// 无序号时保持幂等（同一 URL 重复轮询不重复存储）。
	require.Equal(t, k0, mediaStorageKey(record, "image/png"))
	// 序号相同的不同记录（不同 localID）也天然不同。
	other := &MediaTaskRecord{LocalID: "img_def456", MediaKind: MediaKindImage}
	require.NotEqual(t, mediaStorageKey(other, "image/png", "1"), k1)
}
