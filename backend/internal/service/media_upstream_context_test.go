package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 回归：上游 create 曾经直接用 gin 的 request context。客户端（或其反代，常见
// 60 秒超时）一断开，gin 立刻取消该 context，已发出的上游请求被连带掐断，
// 表现为 502 + "context canceled"，且上游可能已建好任务却无人认领。
//
// 本测试锁住修复后的不变量：从客户端 ctx 派生的上游 ctx 必须脱离取消链。
func TestUpstreamCreateContextSurvivesClientDisconnect(t *testing.T) {
	t.Parallel()

	released := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-released
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"task_id":"task-1"}`))
	}))
	defer server.Close()

	// 模拟客户端连接：gin 在客户端断开时会取消它。
	clientCtx, cancelClient := context.WithCancel(context.Background())
	upstreamBase := context.WithoutCancel(clientCtx)

	req, err := http.NewRequestWithContext(upstreamBase, http.MethodPost, server.URL, nil)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()
		}
		done <- err
	}()

	// 客户端在请求飞行途中断开。
	cancelClient()
	time.Sleep(20 * time.Millisecond)
	close(released)

	select {
	case err := <-done:
		require.NoError(t, err, "upstream create must not be canceled by client disconnect")
	case <-time.After(5 * time.Second):
		t.Fatal("upstream request did not complete")
	}
}

// 上游 create 的超时窗口必须显著大于落库/结算窗口，否则上游慢时会先把
// persistCtx 耗光，导致任务建好了却写不进库（孤儿任务 + 预扣无法结算）。
func TestMediaTimeoutWindowsAreOrdered(t *testing.T) {
	t.Parallel()
	require.Greater(t, mediaUpstreamCreateTimeout, mediaPersistTimeout,
		"upstream create window must exceed persist window")
	require.GreaterOrEqual(t, mediaUpstreamCreateTimeout, 120*time.Second)
}
