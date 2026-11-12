package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// CountryLookup 是按客户端 IP 解析国家/地区代码的窄接口。
// *service.GeoIPService 天然满足；拆成窄接口便于中间件单测注入假实现，
// 也避免 middleware 包直接依赖 MaxMind  reader。
type CountryLookup interface {
	// Lookup 返回大写 ISO 3166-1 alpha-2 代码；无法判定时返回空字符串。
	Lookup(ip string) (string, error)
	// Available 报告 GeoIP 数据库是否就绪。未就绪时中间件一律放行。
	Available() bool
}

// CountryGuard 全站国家/地区封禁中间件。
//
// 设计要点：
//   - 挂在全局中间件链（SessionBindingContext、IPBlacklistGuard 之后），
//     作用路径与全站 IP 黑名单完全一致（见 ipBlacklistAppliesToPath）。
//   - 客户端 IP 取 SecurityClientIP（与审计日志 / 会话绑定同口径），
//     不信任客户端自报头；「信任转发 IP」开启会污染该值，须保持关闭。
//   - 默认关闭 + 空名单：功能关闭时完全不触碰 GeoIP，热路径零额外开销。
//   - fail-open：GeoIP 库缺失 / 损坏 / 未启用、私有 IP、查不到国家代码、
//     查询报错，一律放行，绝不因风控组件异常把整站打挂。
//   - 配置走进程内缓存（60s TTL + singleflight），热路径零 DB 访问。
//   - 命中时写结构化审计日志（audit=true），供事后回溯。
type CountryGuard struct {
	settingService *service.SettingService
	geoIP          CountryLookup
}

// NewCountryGuard 创建全站国家/地区封禁中间件。
func NewCountryGuard(settingService *service.SettingService, geoIP CountryLookup) *CountryGuard {
	return &CountryGuard{settingService: settingService, geoIP: geoIP}
}

// Middleware 返回 gin 中间件。应挂在 SessionBindingContext 之后。
func (p *CountryGuard) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if p == nil || p.settingService == nil {
			c.Next()
			return
		}
		settings := p.settingService.GetSecurityCountryBlocklistCached(c.Request.Context())
		if !settings.Enabled || len(settings.Countries) == 0 {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		if !ipBlacklistAppliesToPath(path) {
			c.Next()
			return
		}
		// GeoIP 不可用 / 未加载 / 未启用 → 放行（fail-open）
		if p.geoIP == nil || !p.geoIP.Available() {
			c.Next()
			return
		}
		clientIP := SecurityClientIP(c)
		if strings.TrimSpace(clientIP) == "" {
			c.Next()
			return
		}
		country, err := p.geoIP.Lookup(clientIP)
		if err != nil || strings.TrimSpace(country) == "" {
			c.Next()
			return
		}
		if !countryBlocklisted(country, settings.Countries) {
			c.Next()
			return
		}
		slog.Warn("request blocked by site-wide country blocklist",
			"audit", true,
			"client_ip", clientIP,
			"country", strings.ToUpper(strings.TrimSpace(country)),
			"method", c.Request.Method,
			"path", path,
		)
		abortCountryBlocklistBlocked(c)
	}
}

// countryBlocklisted 判断国家/地区代码是否命中名单（大小写不敏感）。
// 名单在写入时已规范化为大写，这里仍做 EqualFold 以兼容存量 JSON。
func countryBlocklisted(country string, list []string) bool {
	code := strings.ToUpper(strings.TrimSpace(country))
	if code == "" {
		return false
	}
	for _, entry := range list {
		if strings.EqualFold(strings.TrimSpace(entry), code) {
			return true
		}
	}
	return false
}

// abortCountryBlocklistBlocked 返回 403。客户端可见文案纯英语：
//   - 面板路径（/api/*）返回 {"detail": ...}；
//   - 网关路径返回 OpenAI 兼容错误格式，与全站 IP 黑名单 403 同款。
func abortCountryBlocklistBlocked(c *gin.Context) {
	const message = "Access denied: requests from your region are not accepted by this service."
	if strings.HasPrefix(c.Request.URL.Path, "/api/") {
		c.JSON(http.StatusForbidden, gin.H{"detail": message})
	} else {
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"message": message,
				"type":    "forbidden",
				"param":   nil,
				"code":    "region_blocked",
			},
		})
	}
	c.Abort()
}
