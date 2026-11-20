package server

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestWireGenSideEffectProvidersArePresent 锁死两个「声明了但没装配」的副作用 provider。
//
// 本仓库 wire_gen.go 必须手工同步，而纯副作用 provider（返回值无人依赖）最容易被
// Wire 生成器和人工同步同时漏掉：
//   - ProvideUserComplianceProfileHook：用户合规画像不注入 → ZDR 写 usage_logs、
//     按用户 enabled_rule_ids 过滤规则恒走全量 fallback，静默不生效；
//   - ProvideTeamAggregationService：不构造 → team/dept/consumer/model 四张每日
//     聚合表的定时任务从不启动，团队分析页面永远空/陈旧。
func TestWireGenSideEffectProvidersArePresent(t *testing.T) {
	providerSrc, err := os.ReadFile("../../internal/service/wire.go")
	require.NoError(t, err)
	require.Contains(t, string(providerSrc), "ProvideUserComplianceProfileHook,")
	require.Contains(t, string(providerSrc), "ProvideTeamAggregationService,")

	wireSrc, err := os.ReadFile("../../cmd/server/wire_gen.go")
	require.NoError(t, err)
	src := string(wireSrc)
	require.Contains(t, src, "service.ProvideUserComplianceProfileHook(",
		"启动装配必须注入用户合规画像，否则规则过滤与 ZDR 静默失效")
	require.Contains(t, src, "service.ProvideTeamAggregationService(",
		"启动装配必须构造并启动团队每日聚合服务")

	// 顺带保护此前已手工同步的可选依赖，避免修复时误删。
	require.Contains(t, src, "settingHandler.SetGeoIPStatus(geoIPService)")
	require.Contains(t, src, "SetPenaltyService(")

	// 全局豁免检查器 + 共享 IP 风险扫描的装配同样只能手工同步：漏了会导致
	// 工单/复核豁免不生效、共享 IP 扫描从不启动。
	require.Contains(t, string(providerSrc), "NewRiskExemptionChecker,")
	require.Contains(t, string(providerSrc), "NewSharedIPRiskScanService,")
	require.Contains(t, src, "service.NewRiskExemptionChecker(")
	require.Contains(t, src, "SetRiskExemptionChecker(")
	require.Contains(t, src, "SetRiskExemptionInvalidator(")
	require.Contains(t, src, "service.NewSharedIPRiskScanService(")
	require.Contains(t, src, "sharedIPRiskScanService.Start()")
	require.Contains(t, src, "sharedIPRiskScan.Stop()")

	// 多维关联团伙风险扫描同样是手工同步：漏了会导致该规则从不启动。
	require.Contains(t, string(providerSrc), "NewLinkedClusterRiskScanService,")
	require.Contains(t, src, "service.NewLinkedClusterRiskScanService(")
	require.Contains(t, src, "linkedClusterRiskScanService.Start()")
	require.Contains(t, src, "linkedClusterRiskScan.Stop()")

	// 新用户首调 IP 严格规则：打标器必须手工注入，漏了会只禁用扣钱但不进复核队列。
	require.Contains(t, src, "SetNewUserRiskMarker(")
	routeSrc, err := os.ReadFile("routes/gateway.go")
	require.NoError(t, err)
	require.Contains(t, string(routeSrc), "NewUserFirstUseIPGuard(ipConsistencyGuard, true)")
}
