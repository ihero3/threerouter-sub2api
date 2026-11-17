package middleware

import (
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// IPBlacklistGuard 全站 IP 黑名单中间件。
//
// 设计要点：
//   - 挂在全局中间件链（SessionBindingContext 之后），覆盖注册/登录面板与
//     模型网关两个入口；命中黑名单（精确 IP / IPv4/IPv6 CIDR）→ 403。
//   - 客户端 IP 与审计日志/会话绑定同口径（SecurityClientIP），反代场景下
//     依赖 server.trusted_proxies 解析，不信任客户端自报头。
//   - 豁免：管理端 /api/v1/admin（防管理员把自己锁在门外）、支付 webhook
//     /api/v1/payment/webhook（回调来自支付服务商服务器，不经过终端用户 IP）、
//     以及非 API / 非网关前缀的静态前端与健康检查。
//   - 配置走进程内缓存（60s TTL + singleflight），热路径零 DB 访问；
//     默认关闭，管理员在系统设置中开启。
//   - 命中时写结构化审计日志（audit=true），供事后回溯。
type IPBlacklistGuard struct {
	settingService *service.SettingService
	opsService     *service.OpsService
}

// NewIPBlacklistGuard 创建全站 IP 黑名单中间件。
func NewIPBlacklistGuard(settingService *service.SettingService, opsService *service.OpsService) *IPBlacklistGuard {
	return &IPBlacklistGuard{settingService: settingService, opsService: opsService}
}

// Middleware 返回 gin 中间件。应挂在 SessionBindingContext 之后（依赖其注入的
// 会话绑定 IP 快照），其余全局中间件之前，尽早拦截。
func (p *IPBlacklistGuard) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if p == nil || p.settingService == nil {
			c.Next()
			return
		}
		settings := p.settingService.GetSecurityIPBlacklistCached(c.Request.Context())
		if !settings.Enabled || len(settings.Blacklist) == 0 {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		if !ipBlacklistAppliesToPath(path) {
			c.Next()
			return
		}
		clientIP := SecurityClientIP(c)
		if !securityIPBlacklisted(clientIP, settings.Blacklist) {
			c.Next()
			return
		}
		slog.Warn("request blocked by site-wide IP blacklist",
			"audit", true,
			"client_ip", clientIP,
			"method", c.Request.Method,
			"path", path,
		)
		p.recordIPBlock(c, clientIP, path)
		abortIPBlacklistBlocked(c, isAuthPath(path))
	}
}

// recordIPBlock 把 IP 黑名单命中写入「错误请求」日志（ops_error_logs）。
// 登录/注册被拦截时通常没有已认证用户，归属统一记 admin(id=1)，便于在
// 「使用记录 → 错误请求」中观测，防止登录坍塌毫不知情。
func (p *IPBlacklistGuard) recordIPBlock(c *gin.Context, clientIP, path string) {
	if p.opsService == nil {
		return
	}
	uid := int64(authErrorDefaultUserID)
	cp := clientIP
	entry := &service.OpsInsertErrorLogInput{
		UserID:          &uid,
		ErrorPhase:      "auth",
		ErrorType:       "ip_blocked",
		StatusCode:      http.StatusForbidden,
		ErrorMessage:    "IP blocked: " + clientIP,
		ClientIP:        &cp,
		RequestPath:     path,
		InboundEndpoint: path,
		Platform:        platformForPath(path),
		UserAgent:       c.GetHeader("User-Agent"),
		CreatedAt:       time.Now(),
	}
	p.opsService.EnqueueError(entry)
}

// ipBlacklistGuardPathPrefixes 是黑名单需要覆盖的网关路由前缀（挂在 gin 根路由）。
// 面板 /api/* 单独判断；其余路径（静态前端、/health、/setup 等）一律豁免。
var ipBlacklistGuardPathPrefixes = []string{
	"/v1",
	"/v1beta",
	"/backend-api",
	"/antigravity",
	"/models",
}

// ipBlacklistExemptPathPrefixes 是面板侧必须豁免的前缀：
//   - /api/v1/admin：管理员接口。若不豁免，管理员把自己的出口 IP 拉黑后
//     将无法再通过后台撤回黑名单（只能进 DB 手工改）。
//   - /api/v1/payment/webhook：支付回调由支付服务商服务器发起，签名校验
//     已足够，按 IP 拦截只会让掉单静默发生。
var ipBlacklistExemptPathPrefixes = []string{
	"/api/v1/admin",
	"/api/v1/payment/webhook",
}

func ipBlacklistAppliesToPath(path string) bool {
	for _, exempt := range ipBlacklistExemptPathPrefixes {
		if strings.HasPrefix(path, exempt) {
			return false
		}
	}
	if strings.HasPrefix(path, "/api/") {
		return true
	}
	for _, prefix := range ipBlacklistGuardPathPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// securityIPBlacklisted 判断客户端 IP 是否命中黑名单（精确 IP 或 CIDR）。
// 条目在写入设置时已校验合法性，这里解析失败直接忽略该条目。
func securityIPBlacklisted(clientIP string, blacklist []string) bool {
	if len(blacklist) == 0 {
		return false
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(clientIP))
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, entry := range blacklist {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if prefix, err := netip.ParsePrefix(entry); err == nil {
			if prefix.Contains(addr) {
				return true
			}
			continue
		}
		if blocked, err := netip.ParseAddr(entry); err == nil && blocked.Unmap() == addr {
			return true
		}
	}
	return false
}

// abortIPBlacklistBlocked 返回 403。错误文案纯英语（客户端可见）：
//   - 面板路径（/api/*）返回 {"detail": ...}，与管理端/面板错误格式一致；
//   - 网关路径返回 OpenAI 兼容错误格式，与网关 IP 限流 403 同款。
//
// authPath 为 true 时文案额外点名登录/注册被禁。
func abortIPBlacklistBlocked(c *gin.Context, authPath bool) {
	message := "Access denied: your IP address has been blocked by the site administrator."
	if authPath {
		message += " Account login and registration are disabled for blocked addresses."
	}
	if strings.HasPrefix(c.Request.URL.Path, "/api/") {
		c.JSON(http.StatusForbidden, gin.H{"detail": message})
	} else {
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"message": message,
				"type":    "forbidden",
				"param":   nil,
				"code":    "ip_blocked",
			},
		})
	}
	c.Abort()
}
