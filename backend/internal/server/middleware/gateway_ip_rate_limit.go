package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"net/netip"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// gatewayIPRateLimitWindow 网关 IP 限流固定窗口时长（按每分钟计数）。
const gatewayIPRateLimitWindow = time.Minute

// gatewayIPRateLimitAllower 抽象底层限流原语，便于单测注入。
type gatewayIPRateLimitAllower interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (middleware.AllowResult, error)
}

// GatewayIPRateLimiter 网关 API（/v1/* 等模型转发链路）按客户端 IP 限流器。
//
// 设计要点：
//   - 按安全客户端 IP 维度计数：与审计日志/会话绑定同口径解析，回环/内网/
//     链路本地地址直接跳过，避免反代内部转发地址把整条链路合并进同一个桶。
//   - 白名单（精确 IP / CIDR）命中即跳过计数。
//   - 配置走进程内缓存（60s TTL），热路径零 DB 访问。
//   - Redis 异常一律 fail-open：限流是保护措施，不能反过来把网关打挂。
//   - 默认关闭，由管理员在系统设置中开启并配置阈值。
type GatewayIPRateLimiter struct {
	limiter        gatewayIPRateLimitAllower
	settingService *service.SettingService
}

// NewGatewayIPRateLimiter 创建网关 IP 限流器。
func NewGatewayIPRateLimiter(redisClient *redis.Client, settingService *service.SettingService) *GatewayIPRateLimiter {
	return &GatewayIPRateLimiter{
		limiter:        middleware.NewRateLimiter(redisClient),
		settingService: settingService,
	}
}

// Middleware 返回网关 IP 限流中间件，应挂在网关路由组最前（认证之前），
// 以便在认证/转发前就拦截单 IP 的聚合洪水。
func (p *GatewayIPRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if p == nil || p.limiter == nil || p.settingService == nil {
			c.Next()
			return
		}
		settings := p.settingService.GetGatewayIPRateLimitSettingsCached(c.Request.Context())
		if !settings.Enabled || settings.RPM <= 0 {
			c.Next()
			return
		}
		clientIP := SecurityClientIP(c)
		if !isPubliclyRoutableClientIP(clientIP) {
			c.Next()
			return
		}
		if gatewayIPWhitelisted(clientIP, settings.Whitelist) {
			c.Next()
			return
		}

		result, err := p.limiter.Allow(c.Request.Context(), "gateway:ip:"+clientIP, settings.RPM, gatewayIPRateLimitWindow)
		if err != nil {
			// fail-open：Redis 异常不阻断网关转发
			slog.Warn("gateway ip rate limit check failed, allowing request", "error", err)
			c.Next()
			return
		}
		if !result.Allowed {
			abortGatewayIPRateLimited(c, result.RetryAfter)
			return
		}
		c.Next()
	}
}

// gatewayIPWhitelisted 判断客户端 IP 是否命中白名单（精确 IP 或 CIDR）。
// 白名单条目在写入设置时已校验合法性，这里解析失败直接忽略该条目。
func gatewayIPWhitelisted(clientIP string, whitelist []string) bool {
	if len(whitelist) == 0 {
		return false
	}
	addr, err := netip.ParseAddr(clientIP)
	if err != nil {
		return false
	}
	for _, entry := range whitelist {
		if prefix, err := netip.ParsePrefix(entry); err == nil {
			if prefix.Contains(addr) {
				return true
			}
			continue
		}
		if whitelistedAddr, err := netip.ParseAddr(entry); err == nil && whitelistedAddr == addr {
			return true
		}
	}
	return false
}

// abortGatewayIPRateLimited 以 OpenAI 兼容错误格式返回 429。
// 网关在认证前限流，无法按平台区分错误格式；OpenAI 兼容格式带 type/code 字段，
// Anthropic/Gemini 客户端同样能识别 429 + Retry-After 语义并正确退避。
func abortGatewayIPRateLimited(c *gin.Context, retryAfter time.Duration) {
	if retryAfter <= 0 {
		retryAfter = gatewayIPRateLimitWindow
	}
	seconds := int64(retryAfter / time.Second)
	if retryAfter%time.Second > 0 {
		seconds++
	}
	c.Header("Retry-After", strconv.FormatInt(seconds, 10))
	c.JSON(http.StatusTooManyRequests, gin.H{
		"error": gin.H{
			"message": "Too many requests from this IP, please slow down and try again later",
			"type":    "rate_limit_error",
			"param":   nil,
			"code":    "rate_limit_exceeded",
		},
	})
	c.Abort()
}
