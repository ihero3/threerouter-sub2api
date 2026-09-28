package handler

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// #2 回归：未提供显式幂等键时，派生兜底键必须（1）对同一请求稳定一致（去重基础）、
// （2）按用户隔离不串键、（3）对请求体内容敏感不误合并。
func TestDeriveMediaAutoIdempotencyKey(t *testing.T) {
	body := map[string]any{"prompt": "a cat", "size": "1024x1024"}

	// 同用户输入 → 同键：这是去重能生效的前提。
	k1 := deriveMediaAutoIdempotencyKey("user:1", "POST", "/v1/images/generations", body)
	k2 := deriveMediaAutoIdempotencyKey("user:1", "POST", "/v1/images/generations", body)
	require.NotEmpty(t, k1)
	require.True(t, strings.HasPrefix(k1, "auto:"), "派生键必须以 auto: 前缀便于识别")
	require.Equal(t, k1, k2, "同用户输入必须派生同一键，否则超时重试无法去重")

	// 不同用户 → 不同键：防止跨用户串键（A 的派生键命中 B 的记录）。
	other := deriveMediaAutoIdempotencyKey("user:2", "POST", "/v1/images/generations", body)
	require.NotEqual(t, k1, other, "不同用户不能串键")

	// 不同请求体 → 不同键：防止把两次有意的不同提交误合并成一次。
	diff := deriveMediaAutoIdempotencyKey("user:1", "POST", "/v1/images/generations", map[string]any{"prompt": "a dog"})
	require.NotEqual(t, k1, diff, "不同内容不能误合并为同键")

	// 不同端点 → 不同键：/v1/images 与 /v1/videos 同源请求不应串。
	vid := deriveMediaAutoIdempotencyKey("user:1", "POST", "/v1/videos/generations", body)
	require.NotEqual(t, k1, vid, "不同端点路径不应串键")

	// 空 actorScope 退化为 anonymous，仍确定性。
	anon := deriveMediaAutoIdempotencyKey("", "POST", "/v1/images/generations", body)
	require.NotEmpty(t, anon)
	require.Equal(t, anon, deriveMediaAutoIdempotencyKey("", "POST", "/v1/images/generations", body))
}
