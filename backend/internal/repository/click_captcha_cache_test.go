package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// 发码邮箱绑定的 Redis 实现：一次人机验证锁定一个收件地址，绑定不可释放。
// ---------------------------------------------------------------------------

func newClickCaptchaCacheForTest(t *testing.T) (*ClickCaptchaCache, *miniredis.Miniredis, *redis.Client) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewClickCaptchaCache(client), server, client
}

func TestClickCaptchaCache_ClaimTokenSendEmailBindsOnce(t *testing.T) {
	cache, _, _ := newClickCaptchaCacheForTest(t)
	ctx := context.Background()

	bound, err := cache.ClaimTokenSendEmail(ctx, "token-1", "user@example.com")
	require.NoError(t, err)
	require.Equal(t, "user@example.com", bound)

	// 同一地址重发不受限（邮件延迟重试）。
	bound, err = cache.ClaimTokenSendEmail(ctx, "token-1", "user@example.com")
	require.NoError(t, err)
	require.Equal(t, "user@example.com", bound)

	// 换地址时返回既有绑定，由 service 层转成 429。
	bound, err = cache.ClaimTokenSendEmail(ctx, "token-1", "victim@example.com")
	require.NoError(t, err)
	require.Equal(t, "user@example.com", bound)

	// 不同 token 互不干扰。
	bound, err = cache.ClaimTokenSendEmail(ctx, "token-2", "other@example.com")
	require.NoError(t, err)
	require.Equal(t, "other@example.com", bound)
}

// expireBindingBeforeGetHook 精确制造「SETNX 未抢到、GET 时绑定键已过期」的
// 竞态窗口：在 GET 命令真正发出前把 miniredis 的时钟推过绑定 TTL。
type expireBindingBeforeGetHook struct {
	server *miniredis.Miniredis
	once   sync.Once
}

func (h *expireBindingBeforeGetHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (h *expireBindingBeforeGetHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if cmd.Name() == "get" {
			h.once.Do(func() {
				h.server.FastForward(clickCaptchaTokenSendEmailTTL + time.Second)
			})
		}
		return next(ctx, cmd)
	}
}

func (h *expireBindingBeforeGetHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func TestClickCaptchaCache_ClaimTokenSendEmailFailClosesOnVanishedBinding(t *testing.T) {
	cache, server, client := newClickCaptchaCacheForTest(t)
	ctx := context.Background()

	// 预置一个已存在的绑定，迫使本次调用走 SETNX=false → GET 分支。
	key := clickCaptchaTokenSendEmailKeyPrefix + "token-race"
	require.NoError(t, client.Set(ctx, key, "first@example.com", clickCaptchaTokenSendEmailTTL).Err())

	client.AddHook(&expireBindingBeforeGetHook{server: server})

	// 修复前 GET 返回 redis.Nil 时直接 return email, nil：没有任何绑定被写入，
	// 却宣告本次赢得绑定，并发的多个不同邮箱可同时通过校验。必须 fail-close。
	_, err := cache.ClaimTokenSendEmail(ctx, "token-race", "attacker@example.com")
	require.Error(t, err, "绑定键在 SETNX 与 GET 之间消失时必须 fail-close，不能静默放行")
}
