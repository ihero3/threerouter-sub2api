package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	ippkg "github.com/Wei-Shaw/sub2api/internal/pkg/ip"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimitFailureMode Redis 故障策略
type RateLimitFailureMode int

const (
	RateLimitFailOpen RateLimitFailureMode = iota
	RateLimitFailClose
)

// RateLimitOptions 限流可选配置
type RateLimitOptions struct {
	FailureMode RateLimitFailureMode
}

var rateLimitScript = redis.NewScript(`
local current = redis.call('INCR', KEYS[1])
local ttl = redis.call('PTTL', KEYS[1])
local repaired = 0
if current == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
elseif ttl == -1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
  repaired = 1
end
return {current, repaired}
`)

// rateLimitRun 允许测试覆写脚本执行逻辑
var rateLimitRun = func(ctx context.Context, client *redis.Client, key string, windowMillis int64) (int64, bool, error) {
	values, err := rateLimitScript.Run(ctx, client, []string{key}, windowMillis).Slice()
	if err != nil {
		return 0, false, err
	}
	if len(values) < 2 {
		return 0, false, fmt.Errorf("rate limit script returned %d values", len(values))
	}
	count, err := parseInt64(values[0])
	if err != nil {
		return 0, false, err
	}
	repaired, err := parseInt64(values[1])
	if err != nil {
		return 0, false, err
	}
	return count, repaired == 1, nil
}

// RateLimiter Redis 速率限制器
type RateLimiter struct {
	redis  *redis.Client
	prefix string
	// fallback 进程内降级限流器：仅在 Redis 故障且调用方要求 fail-close 时使用，
	// 避免限流存储不可用（如 Redis OOM/宕机）把整条入口的所有用户一起锁死。
	fallback *localRateLimiter
}

// NewRateLimiter 创建速率限制器实例
func NewRateLimiter(redisClient *redis.Client) *RateLimiter {
	return &RateLimiter{
		redis:    redisClient,
		prefix:   "rate_limit:",
		fallback: newLocalRateLimiter(),
	}
}

// AllowResult 单次固定窗口限流判定结果。
type AllowResult struct {
	// Allowed 是否放行
	Allowed bool
	// Count 当前窗口内累计请求数（含本次）
	Count int64
	// RetryAfter 超限时距窗口重置的剩余时间（尽力而为；PTTL 不可用时回退为完整窗口）
	RetryAfter time.Duration
}

// Allow 对给定 key（不含 "rate_limit:" 前缀）执行一次固定窗口计数判定。
// 供需要自定义限流维度（如按用户 ID）的调用方使用；Redis 错误由调用方决定 fail-open/close。
func (r *RateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (AllowResult, error) {
	redisKey := r.prefix + key
	windowMillis := windowTTLMillis(window)

	count, repaired, err := rateLimitRun(ctx, r.redis, redisKey, windowMillis)
	if err != nil {
		return AllowResult{}, err
	}
	if repaired {
		log.Printf("[RateLimit] ttl repaired: key=%s window_ms=%d", redisKey, windowMillis)
	}

	result := AllowResult{Allowed: count <= int64(limit), Count: count}
	if !result.Allowed {
		result.RetryAfter = window
		if ttl, ttlErr := r.redis.PTTL(ctx, redisKey).Result(); ttlErr == nil && ttl > 0 {
			result.RetryAfter = ttl
		}
	}
	return result, nil
}

// clientIPForRateLimit 返回 IP 维度限流使用的客户端地址。
// 与聊天/会话绑定/审计日志/IP 限制链路同口径：走自研转发头解析（默认信任
// X-Real-IP / X-Forwarded-For），避免反代部署下 gin ClientIP() 恒等于本机
// 127.0.0.1、所有用户坍缩进同一个限流桶造成整体误拦截。
func clientIPForRateLimit(c *gin.Context) string {
	return ippkg.GetClientIP(c)
}

// Limit 返回速率限制中间件
// key: 限制类型标识
// limit: 时间窗口内最大请求数
// window: 时间窗口
func (r *RateLimiter) Limit(key string, limit int, window time.Duration) gin.HandlerFunc {
	return r.LimitWithOptions(key, limit, window, RateLimitOptions{})
}

// LimitWithOptions 返回速率限制中间件（带可选配置）
func (r *RateLimiter) LimitWithOptions(key string, limit int, window time.Duration, opts RateLimitOptions) gin.HandlerFunc {
	failureMode := opts.FailureMode
	if failureMode != RateLimitFailClose {
		failureMode = RateLimitFailOpen
	}

	return func(c *gin.Context) {
		scopedKey := key + ":" + clientIPForRateLimit(c)
		result, err := r.Allow(c.Request.Context(), scopedKey, limit, window)
		if err != nil {
			log.Printf("[RateLimit] redis error: key=%s mode=%s err=%v", r.prefix+key, failureModeLabel(failureMode), err)
			if failureMode == RateLimitFailClose {
				// 限流存储故障（如 Redis OOM / 宕机）时降级为进程内限流：
				// 仍保留防爆破能力，同时避免把整站用户一起挡在门外。
				if r.fallback != nil && !r.fallback.allow(scopedKey, limit, window, time.Now()) {
					abortRateLimit(c, limit, window)
					return
				}
				c.Next()
				return
			}
			// Redis 错误时放行，避免影响正常服务
			c.Next()
			return
		}

		// 超过限制
		if !result.Allowed {
			abortRateLimit(c, limit, result.RetryAfter)
			return
		}

		c.Next()
	}
}

const (
	// localRateLimiterMaxKeys 限制降级限流表的最大条目数，防止极端情况（大量伪造 IP）拖垮进程内存。
	localRateLimiterMaxKeys = 20000
	// localRateLimiterSweepInterval 定期清理过期窗口的最小间隔。
	localRateLimiterSweepInterval = time.Minute
)

// localRateLimiter 是 Redis 不可用时的进程内固定窗口限流器。
// 语义与 Redis 版本一致：窗口从首次请求开始计时，窗口内计数超过 limit 即拒绝。
type localRateLimiter struct {
	mu        sync.Mutex
	windows   map[string]localRateWindow
	lastSweep time.Time
}

type localRateWindow struct {
	start  time.Time
	window time.Duration
	count  int
}

func newLocalRateLimiter() *localRateLimiter {
	return &localRateLimiter{
		windows:   make(map[string]localRateWindow),
		lastSweep: time.Now(),
	}
}

// allow 判定并计数；超过上限返回 false。
func (l *localRateLimiter) allow(key string, limit int, window time.Duration, now time.Time) bool {
	if l == nil || limit <= 0 || window <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweep(now)

	entry, ok := l.windows[key]
	if !ok || now.Sub(entry.start) >= entry.window {
		l.windows[key] = localRateWindow{start: now, window: window, count: 1}
		return true
	}
	if entry.count >= limit {
		return false
	}
	entry.count++
	l.windows[key] = entry
	return true
}

// sweep 定期清理过期窗口，避免长期运行时映射无限增长。
func (l *localRateLimiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < localRateLimiterSweepInterval && len(l.windows) < localRateLimiterMaxKeys {
		return
	}
	for k, entry := range l.windows {
		if now.Sub(entry.start) >= entry.window {
			delete(l.windows, k)
		}
	}
	l.lastSweep = now
	if len(l.windows) > localRateLimiterMaxKeys {
		l.windows = make(map[string]localRateWindow)
	}
}

func windowTTLMillis(window time.Duration) int64 {
	ttl := window.Milliseconds()
	if ttl < 1 {
		return 1
	}
	return ttl
}

func abortRateLimit(c *gin.Context, limit int, retryAfter time.Duration) {
	retrySeconds := 0
	if retryAfter > 0 {
		// 向上取整：不足 1 秒按 1 秒计，避免客户端拿到 0 后立即重试。
		seconds := int64(retryAfter / time.Second)
		if retryAfter%time.Second > 0 {
			seconds++
		}
		retrySeconds = int(seconds)
		c.Header("Retry-After", strconv.FormatInt(seconds, 10))
	}
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
		"error": "rate limit exceeded",
		// 与 gateway 侧 RPM 超限同口径：带上限额与等待秒数，让调用方知道
		// 该等多久、超限到什么程度，而不是当成一次随机的服务故障。
		"message": rateLimitMessage(limit, retrySeconds),
	})
}

// rateLimitMessage 生成 IP 维度限流的 429 文案（中英双语）。
// 无有效限额时降级为不含 N 的文案。
func rateLimitMessage(limit, retrySeconds int) string {
	if retrySeconds < 1 {
		retrySeconds = 1
	}
	if limit > 0 {
		return fmt.Sprintf(
			"Rate limit exceeded: current limit is %d requests/minute, please retry after %d seconds. "+
				"当前限额 %d 次/分钟，请 %d 秒后重试。",
			limit, retrySeconds, limit, retrySeconds,
		)
	}
	return fmt.Sprintf(
		"Rate limit exceeded: please retry after %d seconds. 请求过于频繁，请 %d 秒后重试。",
		retrySeconds, retrySeconds,
	)
}

func failureModeLabel(mode RateLimitFailureMode) string {
	if mode == RateLimitFailClose {
		return "fail-close"
	}
	return "fail-open"
}

func parseInt64(value any) (int64, error) {
	switch v := value.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unexpected value type %T", value)
	}
}
