package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// fakeCountryLookup 固定返回某个国家/地区代码，不依赖真实 GeoLite2 文件。
type fakeCountryLookup struct {
	available bool
	country   string
	err       error
	lookups   int
}

func (f *fakeCountryLookup) Lookup(_ string) (string, error) {
	f.lookups++
	return f.country, f.err
}

func (f *fakeCountryLookup) Available() bool { return f.available }

func newCountryGuardTestRouter(t *testing.T, settingsJSON string, geoIP CountryLookup) (*gin.Engine, *service.SettingService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := &panelRateLimitStubRepo{}
	if settingsJSON != "" {
		repo.values = map[string]string{service.SettingKeySecurityCountryBlocklist: settingsJSON}
	}
	svc := service.NewSettingService(repo, &config.Config{})
	router := gin.New()
	router.Use(NewCountryGuard(svc, geoIP).Middleware())
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
		router.GET(path, func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
		router.POST(path, func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	}
	return router, svc
}

func performCountryGuardRequest(router *gin.Engine, method, path, remoteAddr string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = remoteAddr
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestCountryGuardDisabledByDefaultDoesNotTouchGeoIP(t *testing.T) {
	cases := []string{
		"",                                     // 键缺失
		`{"enabled":false,"countries":["CN"]}`, // 显式关闭
		`{"enabled":true,"countries":[]}`,      // 开启但空名单
		`{"enabled":true,"countries":null}`,    // null 名单
	}
	for _, settingsJSON := range cases {
		geo := &fakeCountryLookup{available: true, country: "CN"}
		router, _ := newCountryGuardTestRouter(t, settingsJSON, geo)
		require.Equal(t, http.StatusOK,
			performCountryGuardRequest(router, http.MethodPost, "/api/v1/auth/register", "203.0.113.9:1000").Code)
		require.Zero(t, geo.lookups, "功能未真正启用时绝不能触碰 GeoIP：%s", settingsJSON)
	}

	// nil guard：放行
	gin.SetMode(gin.TestMode)
	nilRouter := gin.New()
	var nilGuard *CountryGuard
	nilRouter.Use(nilGuard.Middleware())
	nilRouter.GET("/api/v1/auth/register", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	require.Equal(t, http.StatusOK,
		performCountryGuardRequest(nilRouter, http.MethodGet, "/api/v1/auth/register", "203.0.113.9:1000").Code)
}

func TestCountryGuardBlocksPanelAndGateway(t *testing.T) {
	geo := &fakeCountryLookup{available: true, country: "CN"}
	router, _ := newCountryGuardTestRouter(t, `{"enabled":true,"countries":["CN"]}`, geo)

	// 面板：403 + detail 格式 + 纯英语文案
	resp := performCountryGuardRequest(router, http.MethodPost, "/api/v1/auth/register", "203.0.113.9:1000")
	require.Equal(t, http.StatusForbidden, resp.Code)
	require.Contains(t, resp.Body.String(), "detail")
	require.Contains(t, resp.Body.String(), "your region")

	// 网关：403 + OpenAI 兼容格式
	resp = performCountryGuardRequest(router, http.MethodPost, "/v1/chat/completions", "203.0.113.9:1000")
	require.Equal(t, http.StatusForbidden, resp.Code)
	require.Contains(t, resp.Body.String(), "region_blocked")

	// 登录与 /v1beta 同样被拦
	require.Equal(t, http.StatusForbidden,
		performCountryGuardRequest(router, http.MethodPost, "/api/v1/auth/login", "203.0.113.9:1000").Code)
	require.Equal(t, http.StatusForbidden,
		performCountryGuardRequest(router, http.MethodGet, "/v1beta/models", "203.0.113.9:1000").Code)
}

func TestCountryGuardAllowsNonListedCountry(t *testing.T) {
	geo := &fakeCountryLookup{available: true, country: "US"}
	router, _ := newCountryGuardTestRouter(t, `{"enabled":true,"countries":["CN","DE"]}`, geo)

	require.Equal(t, http.StatusOK,
		performCountryGuardRequest(router, http.MethodPost, "/api/v1/auth/register", "8.8.8.8:1000").Code)
	require.Equal(t, http.StatusOK,
		performCountryGuardRequest(router, http.MethodPost, "/v1/chat/completions", "8.8.8.8:1000").Code)
	require.Equal(t, 2, geo.lookups)
}

func TestCountryGuardExemptPaths(t *testing.T) {
	geo := &fakeCountryLookup{available: true, country: "CN"}
	router, _ := newCountryGuardTestRouter(t, `{"enabled":true,"countries":["CN"]}`, geo)

	// 管理端豁免：封禁自己所在地区时，管理员仍能进后台撤销
	require.Equal(t, http.StatusOK,
		performCountryGuardRequest(router, http.MethodGet, "/api/v1/admin/settings", "203.0.113.9:1000").Code)
	require.Equal(t, http.StatusOK,
		performCountryGuardRequest(router, http.MethodPost, "/api/v1/payment/webhook/easypay", "203.0.113.9:1000").Code)
	require.Equal(t, http.StatusOK,
		performCountryGuardRequest(router, http.MethodGet, "/health", "203.0.113.9:1000").Code)
	require.Equal(t, http.StatusOK,
		performCountryGuardRequest(router, http.MethodGet, "/assets/index.js", "203.0.113.9:1000").Code)
	require.Equal(t, http.StatusOK,
		performCountryGuardRequest(router, http.MethodGet, "/", "203.0.113.9:1000").Code)
	require.Zero(t, geo.lookups, "豁免路径不该触发 GeoIP 查询")
}

func TestCountryGuardFailOpenOnGeoIPProblems(t *testing.T) {
	const settingsJSON = `{"enabled":true,"countries":["CN"]}`

	// GeoIP 数据库未就绪 / 未启用
	unavailable := &fakeCountryLookup{available: false, country: "CN"}
	router, _ := newCountryGuardTestRouter(t, settingsJSON, unavailable)
	require.Equal(t, http.StatusOK,
		performCountryGuardRequest(router, http.MethodPost, "/api/v1/auth/register", "203.0.113.9:1000").Code)
	require.Zero(t, unavailable.lookups)

	// 查询报错
	broken := &fakeCountryLookup{available: true, country: "CN", err: errors.New("mmdb read failed")}
	router, _ = newCountryGuardTestRouter(t, settingsJSON, broken)
	require.Equal(t, http.StatusOK,
		performCountryGuardRequest(router, http.MethodPost, "/api/v1/auth/register", "203.0.113.9:1000").Code)

	// 查不到国家（私有 IP / 未知归属地）
	unknown := &fakeCountryLookup{available: true, country: ""}
	router, _ = newCountryGuardTestRouter(t, settingsJSON, unknown)
	require.Equal(t, http.StatusOK,
		performCountryGuardRequest(router, http.MethodPost, "/api/v1/auth/register", "203.0.113.9:1000").Code)

	// nil geoIP 依赖
	router, _ = newCountryGuardTestRouter(t, settingsJSON, nil)
	require.Equal(t, http.StatusOK,
		performCountryGuardRequest(router, http.MethodPost, "/api/v1/auth/register", "203.0.113.9:1000").Code)

	// typed-nil：接口值非 nil 但底层指针为空，全局中间件绝不能 panic
	var typedNil *service.GeoIPService
	router, _ = newCountryGuardTestRouter(t, settingsJSON, typedNil)
	require.Equal(t, http.StatusOK,
		performCountryGuardRequest(router, http.MethodPost, "/api/v1/auth/register", "203.0.113.9:1000").Code)
}

func TestCountryGuardSettingsSaveTakesEffectImmediately(t *testing.T) {
	geo := &fakeCountryLookup{available: true, country: "CN"}
	router, svc := newCountryGuardTestRouter(t, "", geo)

	// 先走一次请求，把默认关闭的配置装进 60s 缓存
	require.Equal(t, http.StatusOK,
		performCountryGuardRequest(router, http.MethodPost, "/api/v1/auth/register", "203.0.113.9:1000").Code)

	require.NoError(t, svc.SetSecurityCountryBlocklist(context.Background(),
		&service.SecurityCountryBlocklistSettings{Enabled: true, Countries: []string{"cn"}}))

	require.Equal(t, http.StatusForbidden,
		performCountryGuardRequest(router, http.MethodPost, "/api/v1/auth/register", "203.0.113.9:1000").Code,
		"保存后必须立即生效，不能等 60s 缓存过期")
}

func TestCountryBlocklisted(t *testing.T) {
	require.True(t, countryBlocklisted("CN", []string{"CN"}))
	require.True(t, countryBlocklisted("cn", []string{"CN"}))
	require.True(t, countryBlocklisted(" CN ", []string{" cn "}))
	require.True(t, countryBlocklisted("TW", []string{"CN", "HK", "TW"}))
	require.False(t, countryBlocklisted("US", []string{"CN"}))
	require.False(t, countryBlocklisted("", []string{"CN"}))
	require.False(t, countryBlocklisted("CN", nil))
}
