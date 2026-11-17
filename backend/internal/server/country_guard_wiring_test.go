package server

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCountryGuardWiringIsPresent 静态断言启动时确实装配了国家/地区封禁中间件。
//
// 本仓库 wire_gen.go 不能用 go generate 重生成（缺既有 provider），历史上出现过
// 「改了 ProviderSet 但忘记手工同步 wire_gen.go，可选依赖 fail-open 导致功能
// 静默不生效」的问题。这里把装配链锁死，漏改即测试红。
func TestCountryGuardWiringIsPresent(t *testing.T) {
	routerSrc, err := os.ReadFile("router.go")
	require.NoError(t, err)
	require.Contains(t, string(routerSrc), "NewCountryGuard(settingService, geoIPService)",
		"全局中间件链必须挂载 CountryGuard")
	require.Contains(t, string(routerSrc), "NewIPBlacklistGuard(settingService).Middleware()",
		"CountryGuard 应与 IP 黑名单相邻、同在 SessionBindingContext 之后")

	httpSrc, err := os.ReadFile("http.go")
	require.NoError(t, err)
	require.Contains(t, string(httpSrc), "ipConsistencyService, geoIPService)",
		"ProvideRouter 必须把 GeoIPService 透传给 SetupRouter")

	wireSrc, err := os.ReadFile("../../cmd/server/wire_gen.go")
	require.NoError(t, err)
	require.Contains(t, string(wireSrc), "ipConsistencyService, geoIPService)",
		"wire_gen.go 的 ProvideRouter 调用必须手工补上 geoIPService 参数")
	require.Contains(t, string(wireSrc), "settingHandler.SetGeoIPStatus(geoIPService)",
		"后台设置页的 GeoIP 就绪状态必须注入，否则永远显示未就绪")
}
