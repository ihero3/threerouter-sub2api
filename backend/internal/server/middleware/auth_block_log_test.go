package middleware

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// capturingOpsRepo 是 OpsRepository 的测试替身：实现监测落库所需的两个写入方法，
// 并把每条错误日志抓获到 entries，供断言校验。其余接口方法由内嵌的接口零值兜底。
type capturingOpsRepo struct {
	service.OpsRepository
	mu      sync.Mutex
	entries []*service.OpsInsertErrorLogInput
}

func (r *capturingOpsRepo) InsertErrorLog(_ context.Context, entry *service.OpsInsertErrorLogInput) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, entry)
	return 0, nil
}

func (r *capturingOpsRepo) BatchInsertErrorLogs(_ context.Context, entries []*service.OpsInsertErrorLogInput) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, entries...)
	return 0, nil
}

func (r *capturingOpsRepo) snapshot() []*service.OpsInsertErrorLogInput {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*service.OpsInsertErrorLogInput, len(r.entries))
	copy(out, r.entries)
	return out
}

// newCapturingOpsService 构造带捕获型仓库的 OpsService；显式开启监测以绕过 fail-open，
// 使 recordCountryBlock / recordIPBlock 经 EnqueueError 异步落库的条目可被断言。
func newCapturingOpsService() (*service.OpsService, *capturingOpsRepo) {
	repo := &capturingOpsRepo{}
	cfg := &config.Config{}
	cfg.Ops.Enabled = true
	ops := service.NewOpsService(repo, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	return ops, repo
}

// newCountryGuardTestRouterWithOps 同 newCountryGuardTestRouter，但注入可捕获落库的 OpsService。
func newCountryGuardTestRouterWithOps(t *testing.T, settingsJSON string, geoIP CountryLookup, ops *service.OpsService) (*gin.Engine, *service.SettingService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := &panelRateLimitStubRepo{}
	if settingsJSON != "" {
		repo.values = map[string]string{service.SettingKeySecurityCountryBlocklist: settingsJSON}
	}
	svc := service.NewSettingService(repo, &config.Config{})
	router := gin.New()
	router.Use(NewCountryGuard(svc, geoIP, ops).Middleware())
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

// newIPBlacklistTestRouterWithOps 同 newIPBlacklistTestRouter，但注入可捕获落库的 OpsService。
func newIPBlacklistTestRouterWithOps(t *testing.T, settingsJSON string, ops *service.OpsService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(NewIPBlacklistGuard(newIPBlacklistTestService(t, settingsJSON), ops).Middleware())
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

// TestCountryGuardBlockRecordsOpsErrorLog 验证国家/地区封禁命中时：
//   - 登录/注册入口 403 文案点名登录/注册；
//   - 命中写入「错误请求」日志，归属兜底记为 admin(id=1)，phase=auth，type=region_blocked；
//   - 网关路径同样落库，但 platform 留空。
func TestCountryGuardBlockRecordsOpsErrorLog(t *testing.T) {
	geo := &fakeCountryLookup{available: true, country: "CN"}
	ops, repo := newCapturingOpsService()
	router, _ := newCountryGuardTestRouterWithOps(t, `{"enabled":true,"countries":["CN"]}`, geo, ops)

	// 登录入口被封禁：文案点名登录/注册
	resp := performCountryGuardRequest(router, http.MethodPost, "/api/v1/auth/login", "203.0.113.9:1000")
	require.Equal(t, http.StatusForbidden, resp.Code)
	require.Contains(t, resp.Body.String(), "login")
	require.Contains(t, resp.Body.String(), "registration")

	// 异步落库：等待兜底记为 admin(id=1) 的「错误请求」日志出现
	require.Eventually(t, func() bool {
		return len(repo.snapshot()) >= 1
	}, 3*time.Second, 10*time.Millisecond)

	entries := repo.snapshot()
	require.Len(t, entries, 1)
	entry := entries[0]
	require.NotNil(t, entry.UserID, "登录封禁无法确定用户，必须兜底归属 admin")
	require.Equal(t, int64(authErrorDefaultUserID), *entry.UserID)
	require.Equal(t, "auth", entry.ErrorPhase)
	require.Equal(t, "region_blocked", entry.ErrorType)
	require.Equal(t, http.StatusForbidden, entry.StatusCode)
	require.Contains(t, entry.ErrorMessage, "Region blocked")
	require.Contains(t, entry.ErrorMessage, "CN")
	require.NotNil(t, entry.ClientIP)
	require.Equal(t, "panel", entry.Platform, "面板认证路径落库 platform 应标记 panel")

	// 网关路径同样落库，但 platform 留空（非面板）
	resp = performCountryGuardRequest(router, http.MethodPost, "/v1/chat/completions", "203.0.113.9:1000")
	require.Equal(t, http.StatusForbidden, resp.Code)
	require.Eventually(t, func() bool {
		return len(repo.snapshot()) >= 2
	}, 3*time.Second, 10*time.Millisecond)
	entries = repo.snapshot()
	require.Len(t, entries, 2)
	require.Equal(t, "", entries[1].Platform, "网关路径落库 platform 应留空")
	require.Equal(t, "region_blocked", entries[1].ErrorType)
}

// TestIPBlacklistBlockRecordsOpsErrorLog 验证 IP 黑名单命中时写入「错误请求」日志，
// 归属兜底记为 admin(id=1)，phase=auth，type=ip_blocked。
func TestIPBlacklistBlockRecordsOpsErrorLog(t *testing.T) {
	ops, repo := newCapturingOpsService()
	router := newIPBlacklistTestRouterWithOps(t, `{"enabled":true,"blacklist":["203.0.113.9"]}`, ops)

	// 注册入口被封禁：文案点名注册
	resp := performIPBlacklistRequest(router, http.MethodPost, "/api/v1/auth/register", "203.0.113.9:1000")
	require.Equal(t, http.StatusForbidden, resp.Code)
	require.Contains(t, resp.Body.String(), "registration")

	require.Eventually(t, func() bool {
		return len(repo.snapshot()) >= 1
	}, 3*time.Second, 10*time.Millisecond)

	entries := repo.snapshot()
	require.Len(t, entries, 1)
	entry := entries[0]
	require.NotNil(t, entry.UserID, "注册封禁无法确定用户，必须兜底归属 admin")
	require.Equal(t, int64(authErrorDefaultUserID), *entry.UserID)
	require.Equal(t, "auth", entry.ErrorPhase)
	require.Equal(t, "ip_blocked", entry.ErrorType)
	require.Equal(t, http.StatusForbidden, entry.StatusCode)
	require.Contains(t, entry.ErrorMessage, "IP blocked")
	require.Contains(t, entry.ErrorMessage, "203.0.113.9")
	require.NotNil(t, entry.ClientIP)
	require.Equal(t, "panel", entry.Platform)
}
