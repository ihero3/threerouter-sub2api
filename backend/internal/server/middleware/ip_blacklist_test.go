package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newIPBlacklistTestService(t *testing.T, settingsJSON string) *service.SettingService {
	t.Helper()
	repo := &panelRateLimitStubRepo{}
	if settingsJSON != "" {
		repo.values = map[string]string{service.SettingKeySecurityIPBlacklist: settingsJSON}
	}
	return service.NewSettingService(repo, &config.Config{})
}

// newIPBlacklistTestRouter 构造带 IP 黑名单中间件的多路径测试路由，
// 路径集合覆盖面板 / 网关 / 豁免三类。
func newIPBlacklistTestRouter(t *testing.T, settingsJSON string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(NewIPBlacklistGuard(newIPBlacklistTestService(t, settingsJSON), nil).Middleware())
	for _, path := range []string{
		"/api/v1/auth/register",
		"/api/v1/auth/login",
		"/v1/chat/completions",
		"/v1beta/models",
		"/api/v1/admin/settings",
		"/api/v1/payment/webhook/easypay",
		"/health",
		"/assets/index.js",
		"/",
	} {
		router.GET(path, func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})
		router.POST(path, func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})
	}
	return router
}

func performIPBlacklistRequest(router *gin.Engine, method, path, remoteAddr string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = remoteAddr
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestIPBlacklistDisabledByDefault(t *testing.T) {
	// 无配置（默认关闭）：放行
	router := newIPBlacklistTestRouter(t, "")
	require.Equal(t, http.StatusOK, performIPBlacklistRequest(router, http.MethodGet, "/api/v1/auth/register", "203.0.113.9:1000").Code)

	// 显式关闭：放行（名单保留但不拦截）
	router2 := newIPBlacklistTestRouter(t, `{"enabled":false,"blacklist":["203.0.113.9"]}`)
	require.Equal(t, http.StatusOK, performIPBlacklistRequest(router2, http.MethodGet, "/api/v1/auth/register", "203.0.113.9:1000").Code)

	// 开启但空名单：放行
	router3 := newIPBlacklistTestRouter(t, `{"enabled":true,"blacklist":[]}`)
	require.Equal(t, http.StatusOK, performIPBlacklistRequest(router3, http.MethodGet, "/api/v1/auth/register", "203.0.113.9:1000").Code)

	// nil 服务：放行
	gin.SetMode(gin.TestMode)
	nilRouter := gin.New()
	var nilGuard *IPBlacklistGuard
	nilRouter.Use(nilGuard.Middleware())
	nilRouter.GET("/api/v1/auth/register", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	require.Equal(t, http.StatusOK, performIPBlacklistRequest(nilRouter, http.MethodGet, "/api/v1/auth/register", "203.0.113.9:1000").Code)
}

func TestIPBlacklistBlocksPanelAndGateway(t *testing.T) {
	router := newIPBlacklistTestRouter(t, `{"enabled":true,"blacklist":["203.0.113.9","2a02:4780:5e:4f48::1","198.51.100.0/24"]}`)

	// 面板路径：精确 IPv4 命中 → 403，detail 错误格式
	resp := performIPBlacklistRequest(router, http.MethodPost, "/api/v1/auth/register", "203.0.113.9:1000")
	require.Equal(t, http.StatusForbidden, resp.Code)
	require.Contains(t, resp.Body.String(), "blocked by the site administrator")
	require.Contains(t, resp.Body.String(), "detail")

	// 网关路径：精确 IPv6 命中 → 403，OpenAI 兼容错误格式
	resp = performIPBlacklistRequest(router, http.MethodPost, "/v1/chat/completions", "[2a02:4780:5e:4f48::1]:1000")
	require.Equal(t, http.StatusForbidden, resp.Code)
	require.Contains(t, resp.Body.String(), "ip_blocked")

	// 网关路径：IPv4 CIDR 命中 → 403
	resp = performIPBlacklistRequest(router, http.MethodGet, "/v1beta/models", "198.51.100.7:1000")
	require.Equal(t, http.StatusForbidden, resp.Code)

	// 登录路径同样被拦（注册/登录/网关统一生效）
	resp = performIPBlacklistRequest(router, http.MethodPost, "/api/v1/auth/login", "203.0.113.9:1000")
	require.Equal(t, http.StatusForbidden, resp.Code)

	// 未命中 IP：放行
	require.Equal(t, http.StatusOK, performIPBlacklistRequest(router, http.MethodGet, "/api/v1/auth/register", "8.8.8.8:1000").Code)
	require.Equal(t, http.StatusOK, performIPBlacklistRequest(router, http.MethodPost, "/v1/chat/completions", "8.8.8.8:1000").Code)
}

func TestIPBlacklistExemptPaths(t *testing.T) {
	router := newIPBlacklistTestRouter(t, `{"enabled":true,"blacklist":["203.0.113.9"]}`)

	// 管理端：豁免（防管理员自锁）
	require.Equal(t, http.StatusOK, performIPBlacklistRequest(router, http.MethodGet, "/api/v1/admin/settings", "203.0.113.9:1000").Code)
	// 支付 webhook：豁免（回调来自支付服务商服务器）
	require.Equal(t, http.StatusOK, performIPBlacklistRequest(router, http.MethodPost, "/api/v1/payment/webhook/easypay", "203.0.113.9:1000").Code)
	// 静态前端 / 健康检查 / 根路径：豁免
	require.Equal(t, http.StatusOK, performIPBlacklistRequest(router, http.MethodGet, "/health", "203.0.113.9:1000").Code)
	require.Equal(t, http.StatusOK, performIPBlacklistRequest(router, http.MethodGet, "/assets/index.js", "203.0.113.9:1000").Code)
	require.Equal(t, http.StatusOK, performIPBlacklistRequest(router, http.MethodGet, "/", "203.0.113.9:1000").Code)
}

func TestSecurityIPBlacklisted(t *testing.T) {
	require.True(t, securityIPBlacklisted("203.0.113.9", []string{"203.0.113.9"}))
	require.True(t, securityIPBlacklisted("203.0.113.9", []string{"203.0.113.0/24"}))
	require.True(t, securityIPBlacklisted("2a02:4780:5e:4f48::1", []string{"2a02:4780:5e:4f48::1"}))
	require.True(t, securityIPBlacklisted("2a02:4780:5e:4f48::2", []string{"2a02:4780::/32"}))
	require.True(t, securityIPBlacklisted("2001:db8::1", []string{"2001:db8::/32"}))
	// v4-mapped 地址按 IPv4 口径匹配
	require.True(t, securityIPBlacklisted("::ffff:203.0.113.9", []string{"203.0.113.9"}))
	require.False(t, securityIPBlacklisted("203.0.113.9", []string{"203.0.113.10"}))
	require.False(t, securityIPBlacklisted("203.0.113.9", []string{"198.51.100.0/24"}))
	require.False(t, securityIPBlacklisted("203.0.113.9", nil))
	require.False(t, securityIPBlacklisted("203.0.113.9", []string{}))
	// 非法条目被忽略，不影响其他条目
	require.True(t, securityIPBlacklisted("203.0.113.9", []string{"not-an-ip", "203.0.113.9"}))
	require.False(t, securityIPBlacklisted("not-an-ip", []string{"203.0.113.9"}))
	// 带空白的条目/地址可容忍
	require.True(t, securityIPBlacklisted(" 203.0.113.9 ", []string{" 203.0.113.9 "}))
}

func TestIPBlacklistPathGate(t *testing.T) {
	// 豁免优先于覆盖：/api/v1/admin 前缀豁免
	require.False(t, ipBlacklistAppliesToPath("/api/v1/admin/settings"))
	require.False(t, ipBlacklistAppliesToPath("/api/v1/payment/webhook/stripe"))
	// 面板其余路径覆盖
	require.True(t, ipBlacklistAppliesToPath("/api/v1/auth/register"))
	require.True(t, ipBlacklistAppliesToPath("/api/v1/users/me"))
	// 网关前缀覆盖
	require.True(t, ipBlacklistAppliesToPath("/v1/chat/completions"))
	require.True(t, ipBlacklistAppliesToPath("/v1beta/models"))
	require.True(t, ipBlacklistAppliesToPath("/backend-api/codex/responses"))
	require.True(t, ipBlacklistAppliesToPath("/antigravity/v1/messages"))
	require.True(t, ipBlacklistAppliesToPath("/models"))
	// 静态/健康/未知路径豁免
	require.False(t, ipBlacklistAppliesToPath("/health"))
	require.False(t, ipBlacklistAppliesToPath("/setup/status"))
	require.False(t, ipBlacklistAppliesToPath("/assets/app.js"))
	require.False(t, ipBlacklistAppliesToPath("/"))
	// 注意：前缀匹配是字符串级，/v1-xxx 也会命中 /v1 前缀（无害，仅黑名单 IP 受影响）
	require.True(t, ipBlacklistAppliesToPath("/v1-not-a-gateway-prefix"))
	require.False(t, ipBlacklistAppliesToPath("/v2/something"))
}
