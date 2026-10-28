package middleware

import (
	"errors"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/stretchr/testify/require"
)

func newGatewayIPRateLimitTestService(t *testing.T, settingsJSON string) *service.SettingService {
	t.Helper()
	repo := &panelRateLimitStubRepo{}
	if settingsJSON != "" {
		repo.values = map[string]string{service.SettingKeyGatewayIPRateLimitSettings: settingsJSON}
	}
	return service.NewSettingService(repo, &config.Config{})
}

func TestGatewayIPRateLimiterDisabledByDefault(t *testing.T) {
	// 无配置（默认关闭）：放行
	p := &GatewayIPRateLimiter{
		limiter:        &fakePanelAllower{},
		settingService: newGatewayIPRateLimitTestService(t, ""),
	}
	router := newPanelTestRouter(p.Middleware(), nil)
	for i := 0; i < 5; i++ {
		require.Equal(t, http.StatusOK, performPanelRequest(router, "203.0.113.9:1000").Code)
	}

	// 显式关闭：放行
	p2 := &GatewayIPRateLimiter{
		limiter:        &fakePanelAllower{},
		settingService: newGatewayIPRateLimitTestService(t, `{"enabled":false,"rpm":1}`),
	}
	router2 := newPanelTestRouter(p2.Middleware(), nil)
	for i := 0; i < 5; i++ {
		require.Equal(t, http.StatusOK, performPanelRequest(router2, "203.0.113.9:1000").Code)
	}

	// nil 限流器：放行
	var nilLimiter *GatewayIPRateLimiter
	nilRouter := newPanelTestRouter(nilLimiter.Middleware(), nil)
	require.Equal(t, http.StatusOK, performPanelRequest(nilRouter, "203.0.113.9:1000").Code)
}

func TestGatewayIPRateLimiterLimitsPublicIP(t *testing.T) {
	allower := &fakePanelAllower{}
	p := &GatewayIPRateLimiter{
		limiter:        allower,
		settingService: newGatewayIPRateLimitTestService(t, `{"enabled":true,"rpm":1}`),
	}
	router := newPanelTestRouter(p.Middleware(), nil)

	// 公网 IP：第二次被限，OpenAI 兼容错误格式 + Retry-After
	require.Equal(t, http.StatusOK, performPanelRequest(router, "203.0.113.9:1000").Code)
	limited := performPanelRequest(router, "203.0.113.9:1000")
	require.Equal(t, http.StatusTooManyRequests, limited.Code)
	require.NotEmpty(t, limited.Header().Get("Retry-After"))
	require.Contains(t, limited.Body.String(), "rate_limit_error")

	// 其他公网 IP 独立计数
	require.Equal(t, http.StatusOK, performPanelRequest(router, "198.51.100.7:1000").Code)

	// 回环/内网地址（反代内部转发地址）：跳过计数，绝不误拦
	for i := 0; i < 5; i++ {
		require.Equal(t, http.StatusOK, performPanelRequest(router, "127.0.0.1:1000").Code)
		require.Equal(t, http.StatusOK, performPanelRequest(router, "10.0.0.8:1000").Code)
		require.Equal(t, http.StatusOK, performPanelRequest(router, "192.168.1.30:1000").Code)
	}

	allower.mu.Lock()
	defer allower.mu.Unlock()
	require.Contains(t, allower.counts, "gateway:ip:203.0.113.9")
	require.Contains(t, allower.counts, "gateway:ip:198.51.100.7")
	for key := range allower.counts {
		require.NotContains(t, key, "127.0.0.1")
		require.NotContains(t, key, "10.0.0.8")
		require.NotContains(t, key, "192.168.1.30")
	}
}

func TestGatewayIPRateLimiterWhitelist(t *testing.T) {
	allower := &fakePanelAllower{}
	p := &GatewayIPRateLimiter{
		limiter:        allower,
		settingService: newGatewayIPRateLimitTestService(t, `{"enabled":true,"rpm":1,"whitelist":["203.0.113.9","198.51.100.0/24"]}`),
	}
	router := newPanelTestRouter(p.Middleware(), nil)

	// 精确 IP 命中白名单：不计数
	for i := 0; i < 3; i++ {
		require.Equal(t, http.StatusOK, performPanelRequest(router, "203.0.113.9:1000").Code)
	}
	// CIDR 命中白名单：不计数
	for i := 0; i < 3; i++ {
		require.Equal(t, http.StatusOK, performPanelRequest(router, "198.51.100.7:1000").Code)
	}
	// 未命中白名单：正常限流
	require.Equal(t, http.StatusOK, performPanelRequest(router, "8.8.8.8:1000").Code)
	require.Equal(t, http.StatusTooManyRequests, performPanelRequest(router, "8.8.8.8:1000").Code)

	allower.mu.Lock()
	defer allower.mu.Unlock()
	require.NotContains(t, allower.counts, "gateway:ip:203.0.113.9")
	require.NotContains(t, allower.counts, "gateway:ip:198.51.100.7")
	require.Contains(t, allower.counts, "gateway:ip:8.8.8.8")
}

func TestGatewayIPRateLimiterFailOpenOnRedisError(t *testing.T) {
	p := &GatewayIPRateLimiter{
		limiter:        &fakePanelAllower{err: errors.New("redis down")},
		settingService: newGatewayIPRateLimitTestService(t, `{"enabled":true,"rpm":1}`),
	}
	router := newPanelTestRouter(p.Middleware(), nil)
	for i := 0; i < 3; i++ {
		require.Equal(t, http.StatusOK, performPanelRequest(router, "203.0.113.9:1000").Code)
	}
}

func TestGatewayIPWhitelisted(t *testing.T) {
	require.True(t, gatewayIPWhitelisted("203.0.113.9", []string{"203.0.113.9"}))
	require.True(t, gatewayIPWhitelisted("203.0.113.9", []string{"203.0.113.0/24"}))
	require.True(t, gatewayIPWhitelisted("2001:db8::1", []string{"2001:db8::/32"}))
	require.False(t, gatewayIPWhitelisted("203.0.113.9", []string{"203.0.113.10"}))
	require.False(t, gatewayIPWhitelisted("203.0.113.9", []string{"198.51.100.0/24"}))
	require.False(t, gatewayIPWhitelisted("203.0.113.9", nil))
	// 非法条目被忽略，不影响其他条目
	require.True(t, gatewayIPWhitelisted("203.0.113.9", []string{"not-an-ip", "203.0.113.9"}))
	require.False(t, gatewayIPWhitelisted("not-an-ip", []string{"203.0.113.9"}))
}
