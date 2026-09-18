package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// 本文件锁住「同步出图结果解析」的三条口径，它们各自对应一个已发生过的缺陷：
// 没有产物却判 succeeded（照扣费用但交付不了图）、只有 b64_json 被当成无产物、
// n>1 只交一张却按 n 张计费。三家厂商必须与 MiniMax 的既有口径一致。

func TestSeedanceImageCreateResultRejectsEmptyOutput(t *testing.T) {
	t.Parallel()

	// data 为空（或不存在）时不能再判 succeeded：MediaTaskService 的同步分支会
	// 立刻真实扣费，而 handler 随后因拿不到 URL 回 502，钱扣了图却没有。
	for _, body := range []string{
		`{"data":[]}`,
		`{"data":[{}]}`,
		`{"created":123}`,
	} {
		res, err := parseSeedanceImageCreateResult([]byte(body), http.StatusOK)
		require.NoError(t, err)
		require.Equal(t, "failed", res.Status, "body=%s", body)
		require.Equal(t, MediaCompletionFailed, res.Mode, "body=%s", body)
		require.Empty(t, res.InlineURL, "body=%s", body)
	}
}

func TestSeedanceImageCreateResultAcceptsB64JSON(t *testing.T) {
	t.Parallel()

	// 上游只回 b64_json 时没有可访问 URL，必须转成 data URI，
	// 否则这条成功响应会被当成"没有产物"而误判失败。
	res, err := parseSeedanceImageCreateResult(
		[]byte(`{"data":[{"b64_json":"AAAA"}]}`), http.StatusOK)
	require.NoError(t, err)
	require.Equal(t, "succeeded", res.Status)
	require.Equal(t, "data:image/png;base64,AAAA", res.InlineURL)
	require.Equal(t, []string{"data:image/png;base64,AAAA"}, res.InlineURLs)
}

func TestSeedanceImageCreateResultCollectsEveryImage(t *testing.T) {
	t.Parallel()

	// n>1 必须交付全部：只取 data[0] 会少给货，而 settledImageCount 仍按 n 张计费。
	res, err := parseSeedanceImageCreateResult([]byte(`{"data":[
		{"url":"https://cdn.test/a.png"},
		{"url":"https://cdn.test/b.png"},
		{"b64_json":"AAAA"}
	]}`), http.StatusOK)
	require.NoError(t, err)
	require.Equal(t, "succeeded", res.Status)
	require.Equal(t, "https://cdn.test/a.png", res.InlineURL)
	require.Len(t, res.InlineURLs, 3)
	require.Equal(t, "data:image/png;base64,AAAA", res.InlineURLs[2])
}

func TestSeedanceImageCreateResultKeepsUpstreamFailure(t *testing.T) {
	t.Parallel()

	res, err := parseSeedanceImageCreateResult([]byte(`{"error":"bad model"}`), http.StatusBadRequest)
	require.NoError(t, err)
	require.Equal(t, "failed", res.Status)
	require.Equal(t, http.StatusBadRequest, res.UpstreamStatusCode)
}

// newStubMediaAdapter 起一个返回固定响应体的临时上游。

func newStubMediaAdapter(t *testing.T, respBody string) (*openAICompatMediaAdapter, *Account) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(respBody))
	}))
	t.Cleanup(srv.Close)

	adapter := newOpenAICompatMediaAdapter(MediaKindImage, func(string, string) bool { return true })
	adapter.httpClient = srv.Client()
	account := &Account{
		ID:          1,
		Credentials: map[string]any{"base_url": srv.URL, "api_key": "stub-key"},
	}
	return adapter, account
}

func TestCompatImageCreateFailsWhenNoTaskAndNoOutput(t *testing.T) {
	t.Parallel()

	// 关键回归：Data 非空但只带 b64_json 时，旧逻辑不会 failed，
	// 任务变成「无 task_id 的 processing」—— PollTask 遇空 UpstreamTaskID 直接跳过，
	// 30 分钟超时兜底也不可达，预扣永久挂账且没有 usage_logs。
	adapter, account := newStubMediaAdapter(t, `{"data":[{"b64_json":"AAAA"}]}`)

	res, err := adapter.Create(context.Background(), account, MediaCreateRequest{UpstreamModel: "m", Prompt: "p"})
	require.NoError(t, err)
	require.Equal(t, "failed", res.Status)
	require.Equal(t, MediaCompletionFailed, res.Mode)
	require.Empty(t, res.TaskID)
	require.Contains(t, res.ErrorMessage, "missing task id or url")
}

func TestCompatImageCreateCollectsEveryImage(t *testing.T) {
	t.Parallel()

	adapter, account := newStubMediaAdapter(t, `{"data":[
		{"b64_json":"AAAA"},
		{"url":"https://cdn.test/a.png"}
	]}`)

	res, err := adapter.Create(context.Background(), account, MediaCreateRequest{UpstreamModel: "m", Prompt: "p"})
	require.NoError(t, err)
	require.Equal(t, "succeeded", res.Status)
	require.Equal(t, MediaCompletionSync, res.Mode)
	require.Equal(t, "data:image/png;base64,AAAA", res.InlineURL)
	require.Len(t, res.InlineURLs, 2)
}

func TestCompatImageCreateKeepsAsyncTask(t *testing.T) {
	t.Parallel()

	// 纯异步形态不能被误伤：有 task_id 就是 processing，产物为空是正常的。
	adapter, account := newStubMediaAdapter(t, `{"id":"task-123"}`)

	res, err := adapter.Create(context.Background(), account, MediaCreateRequest{UpstreamModel: "m", Prompt: "p"})
	require.NoError(t, err)
	require.Equal(t, "processing", res.Status)
	require.Equal(t, "task-123", res.TaskID)
	require.Empty(t, res.InlineURL)
	require.Less(t, len(strings.TrimSpace(res.InlineURLs[0]+res.InlineURLs[0])), 2)
}
