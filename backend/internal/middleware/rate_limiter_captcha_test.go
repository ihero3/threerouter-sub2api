package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// 与 routes/auth.go 中两条点击验证码路由的限流阈值保持一致（自 20/30 收紧而来）。
// 本文件处于 middleware 包内（bare 层），不能反向 import routes（会形成包循环），
// 因此用对应 limit 构造中间件，锁死新阈值下第 limit 次放行 / 第 limit+1 次 429 的边界语义。
const (
	captchaChallengeLimit = 10
	captchaVerifyLimit    = 15
)

// newCaptchaRateLimitTestRouter 用内存计数覆写 rateLimitRun，返回一个挂载了
// LimitWithOptions 的 gin 路由，以及记录到各限流桶 key 的请求计数表。
// 脚手架与同包 rate_limiter_test.go 一致；脚本不走真实 Redis。
func newCaptchaRateLimitTestRouter(t *testing.T, key string, limit int, opts RateLimitOptions) (*gin.Engine, map[string]int64) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	callCounts := make(map[string]int64)
	originalRun := rateLimitRun
	rateLimitRun = func(ctx context.Context, client *redis.Client, k string, windowMillis int64) (int64, bool, error) {
		callCounts[k]++
		return callCounts[k], false, nil
	}
	t.Cleanup(func() { rateLimitRun = originalRun })

	rdb := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:1",
		DialTimeout:  50 * time.Millisecond,
		ReadTimeout:  50 * time.Millisecond,
		WriteTimeout: 50 * time.Millisecond,
	})
	t.Cleanup(func() { _ = rdb.Close() })

	limiter := NewRateLimiter(rdb)
	router := gin.New()
	router.Use(limiter.LimitWithOptions(key, limit, time.Minute, opts))
	router.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })
	return router, callCounts
}

func performCaptchaRequest(router *gin.Engine, remoteAddr, userAgent string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = remoteAddr
	if userAgent == "" {
		req.Header.Del("User-Agent")
	} else {
		req.Header.Set("User-Agent", userAgent)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func captchaFailCloseOptions() RateLimitOptions {
	return RateLimitOptions{FailureMode: RateLimitFailClose}
}

// 关键回归：同一 IP 携带不同 User-Agent 必须共享同一个限流桶。
// 若把 UA 摘要拼进 key（route:ip:ua:...），每个新 UA 都会拿到全新额度，
// 本用例（第 11 次请求应 429）就会失败——这正是要锁死的放宽行为。
func TestCaptchaChallengeSameIPDifferentUserAgentSharesBucket(t *testing.T) {
	router, callCounts := newCaptchaRateLimitTestRouter(t,
		"auth-captcha-challenge", captchaChallengeLimit, captchaFailCloseOptions())

	// 10 次请求，每次换一个 User-Agent（含空 UA），全部来自同一 IP
	userAgents := []string{
		"Mozilla/5.0 (bot-A)",
		"Mozilla/5.0 (bot-B)",
		"curl/8.0",
		"python-requests/2.31",
		"", // 空 UA
		"Go-http-client/1.1",
		"PostmanRuntime/7.32",
		"Mozilla/5.0 (bot-C)",
		"node-fetch/3.3",
		"Wget/1.21",
	}
	for i, ua := range userAgents {
		rec := performCaptchaRequest(router, "203.0.113.9:1000", ua)
		require.Equal(t, http.StatusOK, rec.Code,
			"第 %d 次(UA=%q)应放行：不同 UA 不该各自开新桶", i+1, ua)
	}

	// 第 11 次：又一个全新 UA，仍应落在同一个 IP 桶并被限流
	rec := performCaptchaRequest(router, "203.0.113.9:1000", "brand-new-UA/1.0")
	require.Equal(t, http.StatusTooManyRequests, rec.Code,
		"第 11 次(换 UA)应 429：同一 IP 的所有 UA 必须共享一个桶")

	// 桶只有一个，且 key 是 route:ip 单一维度、不含任何 UA 段
	require.Len(t, callCounts, 1, "所有 UA 必须共存于单一 IP 桶")
	require.Contains(t, callCounts, "rate_limit:auth-captcha-challenge:203.0.113.9",
		"key 必须是 route:ip 单一维度")
	for capturedKey := range callCounts {
		require.NotContains(t, capturedKey, ":ua:", "限流 key 绝不能包含 User-Agent 维度")
	}
}

// 阈值确实收紧为 challenge=10 / verify=15：第 limit 次放行、第 limit+1 次 429。
func TestCaptchaRateLimitThresholdTightened(t *testing.T) {
	cases := []struct {
		name  string
		key   string
		limit int
	}{
		{"challenge", "auth-captcha-challenge", captchaChallengeLimit},
		{"verify", "auth-captcha-verify", captchaVerifyLimit},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, _ := newCaptchaRateLimitTestRouter(t, tc.key, tc.limit, captchaFailCloseOptions())

			// 第 1..limit 次全部放行（第 limit 次仍放行）
			for i := 1; i <= tc.limit; i++ {
				rec := performCaptchaRequest(router, "203.0.113.9:1000", "")
				require.Equal(t, http.StatusOK, rec.Code,
					"%s 第 %d 次应放行(limit=%d)", tc.name, i, tc.limit)
			}
			// 第 limit+1 次超限
			rec := performCaptchaRequest(router, "203.0.113.9:1000", "")
			require.Equal(t, http.StatusTooManyRequests, rec.Code,
				"%s 第 %d 次应 429(limit=%d)", tc.name, tc.limit+1, tc.limit)
		})
	}
}

// Redis 故障时 RateLimitFailClose 仍通过进程内降级按 IP 计数限流，不会被放宽为全部放行。
func TestCaptchaRateLimitFailCloseStillLimitsOnRedisError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalRun := rateLimitRun
	rateLimitRun = func(ctx context.Context, client *redis.Client, key string, windowMillis int64) (int64, bool, error) {
		return 0, false, errors.New("redis down")
	}
	t.Cleanup(func() { rateLimitRun = originalRun })

	rdb := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:1",
		DialTimeout:  50 * time.Millisecond,
		ReadTimeout:  50 * time.Millisecond,
		WriteTimeout: 50 * time.Millisecond,
	})
	t.Cleanup(func() { _ = rdb.Close() })

	limiter := NewRateLimiter(rdb)
	router := gin.New()
	router.Use(limiter.LimitWithOptions("auth-captcha-challenge", captchaChallengeLimit, time.Minute, captchaFailCloseOptions()))
	router.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	// 即便 Redis 故障，fail-close 仍保留防爆破能力：额度用尽即 429
	for i := 1; i <= captchaChallengeLimit; i++ {
		rec := performCaptchaRequest(router, "203.0.113.9:1000", "")
		require.Equal(t, http.StatusOK, rec.Code, "第 %d 次应放行", i)
	}
	rec := performCaptchaRequest(router, "203.0.113.9:1000", "")
	require.Equal(t, http.StatusTooManyRequests, rec.Code, "额度用尽后应 429")

	// 降级限流仍按 IP 分桶，不会把其它 IP 一起锁死
	other := performCaptchaRequest(router, "203.0.113.10:1000", "")
	require.Equal(t, http.StatusOK, other.Code, "其它 IP 应独立放行")
}

// IPv6 前缀归并仍生效：同一 /64 内轮换地址（且换 UA）仍共享同一额度。
func TestCaptchaRateLimitIPv6PrefixStillSharesBudget(t *testing.T) {
	opts := RateLimitOptions{FailureMode: RateLimitFailClose, IPv6PrefixBits: 64}
	router, callCounts := newCaptchaRateLimitTestRouter(t, "auth-captcha-challenge", 1, opts)

	// 同一 /64 内轮换地址、并换 UA：仍应共享同一桶
	require.Equal(t, http.StatusOK,
		performCaptchaRequest(router, "[2001:db8::1]:1234", "bot-A").Code)
	require.Equal(t, http.StatusTooManyRequests,
		performCaptchaRequest(router, "[2001:db8::2]:1234", "bot-B").Code,
		"同 /64 轮换地址+换 UA 仍应共享额度，第 2 次即 429")

	// 不同 /64：独立桶，放行
	require.Equal(t, http.StatusOK,
		performCaptchaRequest(router, "[2001:db8:0:1::1]:1234", "bot-A").Code)

	// 归并后的 key 必须是 /64 前缀、单一 IP 维度，不含 UA 段
	var foundPrefix bool
	for capturedKey := range callCounts {
		require.NotContains(t, capturedKey, ":ua:", "限流 key 绝不能包含 User-Agent 维度")
		if capturedKey == "rate_limit:auth-captcha-challenge:2001:db8::/64" {
			foundPrefix = true
		}
	}
	require.True(t, foundPrefix, "IP 段应为归并后的 /64 前缀")
}
