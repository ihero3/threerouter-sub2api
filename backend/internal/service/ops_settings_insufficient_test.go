package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"

	"github.com/stretchr/testify/require"
)

// newOpsInsufficientTestService 用最小依赖构造 OpsService；
// 复用 bare 层 ipBlacklistStubRepo（无 build tag，避免跨标签依赖）。
func newOpsInsufficientTestService(values map[string]string) (*OpsService, *ipBlacklistStubRepo) {
	repo := &ipBlacklistStubRepo{values: values}
	svc := NewOpsService(nil, repo, &config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil)
	return svc, repo
}

func TestDefaultOpsAdvancedSettingsIgnoresInsufficientErrors(t *testing.T) {
	d := defaultOpsAdvancedSettings()
	require.True(t, d.IgnoreInsufficientBalanceErrors, "默认必须开启过滤，否则线上继续刷屏")
	require.Equal(t, opsAdvancedSettingsSchemaVersion, d.SchemaVersion)
}

func TestMigrateOpsAdvancedSettings(t *testing.T) {
	cases := []struct {
		name        string
		in          *OpsAdvancedSettings
		wantIgnored bool
		wantVersion int
	}{
		{"v1 explicit false", &OpsAdvancedSettings{SchemaVersion: 0, IgnoreInsufficientBalanceErrors: false}, true, opsAdvancedSettingsSchemaVersion},
		{"v1 already true", &OpsAdvancedSettings{SchemaVersion: 1, IgnoreInsufficientBalanceErrors: true}, true, opsAdvancedSettingsSchemaVersion},
		{"v2 explicit false must stay false", &OpsAdvancedSettings{SchemaVersion: 2, IgnoreInsufficientBalanceErrors: false}, false, 2},
		{"v2 true", &OpsAdvancedSettings{SchemaVersion: 2, IgnoreInsufficientBalanceErrors: true}, true, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			migrateOpsAdvancedSettings(tc.in)
			require.Equal(t, tc.wantIgnored, tc.in.IgnoreInsufficientBalanceErrors)
			require.Equal(t, tc.wantVersion, tc.in.SchemaVersion)
		})
	}
	require.NotPanics(t, func() { migrateOpsAdvancedSettings(nil) })
}

func TestRefreshRuntimeSettingsMigratesLegacyInsufficientSetting(t *testing.T) {
	// 存量 v1 配置：无 schema_version，且显式关闭过滤
	legacy := `{"ignore_count_tokens_errors":true,"ignore_context_canceled":true,"ignore_no_available_accounts":false,"ignore_invalid_api_key_errors":true,"ignore_insufficient_balance_errors":false,"display_alert_events":true,"auto_refresh_enabled":false,"auto_refresh_interval_seconds":30,"data_retention":{"cleanup_enabled":true,"cleanup_schedule":"0 3 * * *","error_log_retention_days":30,"minute_metrics_retention_days":30,"hourly_metrics_retention_days":30}}`
	svc, _ := newOpsInsufficientTestService(map[string]string{
		SettingKeyOpsAdvancedSettings: legacy,
	})

	require.NoError(t, svc.RefreshRuntimeSettings(context.Background()))
	snap := svc.OpsAdvancedSettingsSnapshot()
	require.True(t, snap.IgnoreInsufficientBalanceErrors, "存量显式 false 必须被一次性迁移为 true")
	require.Equal(t, opsAdvancedSettingsSchemaVersion, snap.SchemaVersion)
}

func TestRefreshRuntimeSettingsRespectsExplicitV2Off(t *testing.T) {
	v2 := `{"ignore_insufficient_balance_errors":false,"schema_version":2,"auto_refresh_interval_seconds":30,"data_retention":{"error_log_retention_days":30,"minute_metrics_retention_days":30,"hourly_metrics_retention_days":30}}`
	svc, _ := newOpsInsufficientTestService(map[string]string{
		SettingKeyOpsAdvancedSettings: v2,
	})

	require.NoError(t, svc.RefreshRuntimeSettings(context.Background()))
	require.False(t, svc.OpsAdvancedSettingsSnapshot().IgnoreInsufficientBalanceErrors,
		"管理员保存过的 v2 显式关闭不能被迁移覆盖")
}

func TestUpdateOpsAdvancedSettingsStampsVersionAndKeepsExplicitOff(t *testing.T) {
	svc, repo := newOpsInsufficientTestService(nil)
	cfg := defaultOpsAdvancedSettings()
	cfg.SchemaVersion = 0 // 模拟旧前端不带版本号
	cfg.IgnoreInsufficientBalanceErrors = false

	updated, err := svc.UpdateOpsAdvancedSettings(context.Background(), cfg)
	require.NoError(t, err)
	require.Equal(t, opsAdvancedSettingsSchemaVersion, updated.SchemaVersion)
	require.False(t, updated.IgnoreInsufficientBalanceErrors)

	raw, err := repo.GetValue(context.Background(), SettingKeyOpsAdvancedSettings)
	require.NoError(t, err)
	var stored OpsAdvancedSettings
	require.NoError(t, json.Unmarshal([]byte(raw), &stored))
	require.Equal(t, opsAdvancedSettingsSchemaVersion, stored.SchemaVersion,
		"写入必须打 v2 版本号，否则下次启动又会被迁移")
	require.False(t, stored.IgnoreInsufficientBalanceErrors)

	// 用同一份 DB 值再走一次读取路径，确认不会被迁移改回 true
	require.NoError(t, svc.RefreshRuntimeSettings(context.Background()))
	require.False(t, svc.OpsAdvancedSettingsSnapshot().IgnoreInsufficientBalanceErrors)
}

func TestRefreshRuntimeSettingsWithoutStoredConfigKeepsDefault(t *testing.T) {
	svc, _ := newOpsInsufficientTestService(nil)
	require.NoError(t, svc.RefreshRuntimeSettings(context.Background()))
	snap := svc.OpsAdvancedSettingsSnapshot()
	require.True(t, snap.IgnoreInsufficientBalanceErrors)
	require.Equal(t, opsAdvancedSettingsSchemaVersion, snap.SchemaVersion)
}
