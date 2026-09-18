package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// URL 透传模式（开启异步但没有对象存储）
//
// 这是本轮新增的运行形态：uploader == nil 但 enabled == true。
// 语义是"产物必须全是 http URL，原样转发；一旦出现内嵌大二进制就必须明确失败"，
// 因为 Redis 里躺一个几 MB 的 b64 且保留 24 小时是不可接受的。
// 下面这一组用例锁住这个边界。
// ---------------------------------------------------------------------------

// newURLPassthroughImageTaskService 构造一个「已启用但没有对象存储」的服务。
func newURLPassthroughImageTaskService(store ImageTaskStore) *ImageTaskService {
	return NewImageTaskServiceWithResolver(store, func() (*ImageResultUploader, bool) {
		return nil, true
	}, time.Hour, time.Minute)
}

func TestImageTaskURLEnabledWithoutUploader(t *testing.T) {
	passthrough := newURLPassthroughImageTaskService(&imageTaskMemoryStore{})
	require.True(t, passthrough.Enabled(), "没有对象存储也要允许异步：改成 URL 透传而不是整体禁用")
	require.False(t, passthrough.OffloadEnabled())

	withUploader := NewImageTaskServiceWithResolver(&imageTaskMemoryStore{}, func() (*ImageResultUploader, bool) {
		return NewImageResultUploader(nil, "images/", 0, nil), true
	}, time.Hour, time.Minute)
	require.True(t, withUploader.Enabled())
	require.True(t, withUploader.OffloadEnabled())

	disabled := NewImageTaskServiceWithOptions(&imageTaskMemoryStore{}, time.Hour, time.Minute)
	require.False(t, disabled.Enabled())
	require.False(t, disabled.OffloadEnabled())
}

func TestImageTaskCompleteKeepsUpstreamURLsInPassthroughMode(t *testing.T) {
	store := &imageTaskMemoryStore{}
	svc := newURLPassthroughImageTaskService(store)

	task, err := svc.Create(context.Background(), ImageTaskOwner{UserID: 1, APIKeyID: 2})
	require.NoError(t, err)

	// 多张图也要全部保留，不能只留第一张（历史出现过 n>1 少发货的缺陷）。
	result := json.RawMessage(`{"created":1,"data":[
		{"url":"https://upstream.test/a.png","revised_prompt":"a"},
		{"url":"https://upstream.test/b.png"}
	]}`)
	require.NoError(t, svc.Complete(context.Background(), task.ID, http.StatusOK, result))

	require.Equal(t, ImageTaskStatusCompleted, store.task.Status)
	require.Empty(t, store.task.Error)
	require.JSONEq(t, string(result), string(store.task.Result), "URL 透传模式下原样落库，不做任何改写")
}

func TestImageTaskCompleteRejectsInlineBase64InPassthroughMode(t *testing.T) {
	inline := base64.StdEncoding.EncodeToString(pngBytes)
	cases := []struct {
		name   string
		result string
		want   string // 错误信息里必须出现的提示
	}{
		{
			name:   "b64_json",
			result: `{"created":1,"data":[{"b64_json":"` + inline + `"}]}`,
			want:   "object storage",
		},
		{
			name:   "data URI",
			result: `{"created":1,"data":[{"url":"data:image/png;base64,` + inline + `"}]}`,
			want:   "object storage",
		},
		{
			name:   "第二张是 b64（第一张正常也要整体失败）",
			result: `{"created":1,"data":[{"url":"https://upstream.test/a.png"},{"b64_json":"` + inline + `"}]}`,
			want:   "object storage",
		},
		{
			name:   "空 data 数组",
			result: `{"created":1,"data":[]}`,
			want:   "no image",
		},
		{
			name:   "既无 url 也无 b64_json",
			result: `{"created":1,"data":[{}]}`,
			want:   "neither url nor b64_json",
		},
		{
			name:   "url 为空串",
			result: `{"created":1,"data":[{"url":"  "}]}`,
			want:   "empty url",
		},
		{
			name:   "没有 data 字段",
			result: `{"created":1}`,
			want:   "no data field",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &imageTaskMemoryStore{}
			svc := newURLPassthroughImageTaskService(store)
			task, err := svc.Create(context.Background(), ImageTaskOwner{UserID: 1, APIKeyID: 2})
			require.NoError(t, err)

			require.NoError(t, svc.Complete(context.Background(), task.ID, http.StatusOK, json.RawMessage(tc.result)))

			require.Equal(t, ImageTaskStatusFailed, store.task.Status, "非法产物必须让任务明确失败，不能含糊通过")
			require.Nil(t, store.task.Result, "失败时不能把大 blob 留在 result 里")
			require.NotEmpty(t, store.task.Error)

			var taskErr struct {
				Message string `json:"message"`
			}
			require.NoError(t, json.Unmarshal(store.task.Error, &taskErr))
			require.Contains(t, taskErr.Message, tc.want, "错误信息要指明原因并给出可执行的出路")
			require.NotContains(t, string(store.task.Error), inline, "严禁把 base64 回显给调用方")
		})
	}
}

// 有对象存储时 b64 是被允许的：uploader 会把它转存成 URL，不走拒绝分支。
func TestImageTaskCompleteAllowsInlineBase64WithUploader(t *testing.T) {
	storage := &fakeImageStorage{}
	uploader := NewImageResultUploader(storage, "images/", 0, nil)
	store := &imageTaskMemoryStore{}
	svc := NewImageTaskServiceWithResolver(store, func() (*ImageResultUploader, bool) {
		return uploader, true
	}, time.Hour, time.Minute)

	task, err := svc.Create(context.Background(), ImageTaskOwner{UserID: 1, APIKeyID: 2})
	require.NoError(t, err)

	inline := base64.StdEncoding.EncodeToString(pngBytes)
	require.NoError(t, svc.Complete(context.Background(), task.ID, http.StatusOK,
		json.RawMessage(`{"created":1,"data":[{"b64_json":"`+inline+`"}]}`)))

	require.Equal(t, ImageTaskStatusCompleted, store.task.Status)
	require.Len(t, storage.saved, 1)
	require.NotContains(t, string(store.task.Result), inline, "转存后 result 里只剩 URL")
}
