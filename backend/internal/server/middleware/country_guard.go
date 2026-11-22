package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

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
//     覆盖所有路径（含首页/静态前端/SPA），仅豁免 admin API 与健康检查，
//     见 countryGuardAppliesToPath；与 IP 黑名单的作用范围刻意区分。
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
	opsService     *service.OpsService
}

// NewCountryGuard 创建全站国家/地区封禁中间件。
func NewCountryGuard(settingService *service.SettingService, geoIP CountryLookup, opsService *service.OpsService) *CountryGuard {
	return &CountryGuard{settingService: settingService, geoIP: geoIP, opsService: opsService}
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
		if !countryGuardAppliesToPath(path) {
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
		blocked := strings.ToUpper(strings.TrimSpace(country))
		p.recordCountryBlock(c, blocked, path)
		abortCountryBlocklistBlocked(c, blocked, isAuthPath(path))
	}
}

// countryGuardAppliesToPath 判断国家封禁是否覆盖该路径。
//
// 与全站 IP 黑名单（ipBlacklistAppliesToPath）刻意分离：IP 黑名单只覆盖面板与
// 网关路由、豁免静态前端；国家封禁按运营要求覆盖**所有路径**——包括首页 `/`、
// 静态资源、SPA 前端路由、支付回调与所有 API/网关端点，命中即 403。
//
// 仅保留两类必要豁免：
//   - /api/v1/admin：管理员自助通道。管理员误把自己所在地区拉黑后，仍需能进后台
//     撤销，否则只能进数据库手工改。注意后台「页面」是 SPA 路由（挂在 `/`），
//     从被封地区仍打不开页面，但可用非被封出口或直接调豁免的 admin API 自救。
//   - /health、/setup/status：部署探针。若探针出口在被封地区，拦截会导致实例被
//     误判宕机并反复重启。
//
// 注意用前缀匹配覆盖所有 SPA 路由（`/login`、`/admin/settings`、`/foo` 等都会
// 返回 index.html），不能只精确匹配 `/`。
func countryGuardAppliesToPath(path string) bool {
	if strings.HasPrefix(path, "/api/v1/admin") {
		return false
	}
	if path == "/health" || path == "/setup/status" {
		return false
	}
	return true
}

// isAuthPath 判断路径是否属于登录/注册等认证入口；命中时拦截文案会点名登录/注册。
func isAuthPath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/auth")
}

// recordCountryBlock 把国家/地区封禁命中写入「错误请求」日志（ops_error_logs）。
// 登录/注册被拦截时通常没有已认证用户，归属统一记 admin(id=1)，便于在
// 「使用记录 → 错误请求」中观测，防止登录坍塌毫不知情。
func (p *CountryGuard) recordCountryBlock(c *gin.Context, country, path string) {
	if p.opsService == nil {
		return
	}
	clientIP := SecurityClientIP(c)
	uid := int64(authErrorDefaultUserID)
	cp := clientIP
	entry := &service.OpsInsertErrorLogInput{
		UserID:          &uid,
		ErrorPhase:      "auth",
		ErrorType:       "region_blocked",
		StatusCode:      http.StatusForbidden,
		ErrorMessage:    "Region blocked: " + country,
		ClientIP:        &cp,
		RequestPath:     path,
		InboundEndpoint: path,
		Platform:        platformForPath(path),
		UserAgent:       c.GetHeader("User-Agent"),
		CreatedAt:       time.Now(),
	}
	p.opsService.EnqueueError(entry)
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
//
// country 为命中的大写 ISO 3166-1 alpha-2 代码；authPath 为 true 时文案额外点名
// 登录/注册被禁，直接对应「CN 区域被禁止登录注册」的可读诉求。
func abortCountryBlocklistBlocked(c *gin.Context, country string, authPath bool) {
	message := "Access denied: your region (" + country + ") is blocked from this service."
	if authPath {
		message += " Account login and registration are disabled for this region."
	}
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
