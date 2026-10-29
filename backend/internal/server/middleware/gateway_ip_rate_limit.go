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
//   - 黑名单（精确 IP / CIDR）命中 → 直接 403 Forbidden，不参与限流计数。
//     不在黑名单的所有 IP 一视同仁参与限流，不搞任何豁免。
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
		// 黑名单优先：命中直接 403，不进入限流计数。
		// 设计原则：不搞任何白名单豁免——所有没被拉黑的 IP 一视同仁参与限流。
		if gatewayIPBlacklisted(clientIP, settings.Blacklist) {
			abortGatewayIPRateBlocked(c)
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

// gatewayIPBlacklisted 判断客户端 IP 是否命中黑名单（精确 IP 或 CIDR）。
// 黑名单条目在写入设置时已校验合法性，这里解析失败直接忽略该条目。
func gatewayIPBlacklisted(clientIP string, blacklist []string) bool {
	if len(blacklist) == 0 {
		return false
	}
	addr, err := netip.ParseAddr(clientIP)
	if err != nil {
		return false
	}
	for _, entry := range blacklist {
		if prefix, err := netip.ParsePrefix(entry); err == nil {
			if prefix.Contains(addr) {
				return true
			}
			continue
		}
		if blacklistedAddr, err := netip.ParseAddr(entry); err == nil && blacklistedAddr == addr {
			return true
		}
	}
	return false
}

// abortGatewayIPRateBlocked 返回 403 Forbidden——IP 命中网关级黑名单，直接拒绝访问。
func abortGatewayIPRateBlocked(c *gin.Context) {
	c.Status(http.StatusForbidden)
	c.JSON(http.StatusForbidden, gin.H{
		"error": gin.H{
			"message": "This IP address is blocked from accessing the gateway. Contact the site admin if this is a mistake.",
			"type":    "blocked",
			"param":   nil,
			"code":    "ip_blocked",
		},
	})
	c.Abort()
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
