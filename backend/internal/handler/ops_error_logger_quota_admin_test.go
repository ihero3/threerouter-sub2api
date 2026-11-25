package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestShouldSkipAPIKeyQuotaExhausted 锁死「API key 额度已用完」无条件过滤口径：
// 错误码或文案任一命中即跳过，且不受任何后台开关影响（这是与 insufficient 过滤
// 刻意不同的语义——额度耗尽是本地预期限制，运营不需要在日志里看到）。
func TestShouldSkipAPIKeyQuotaExhausted(t *testing.T) {
	cases := []struct {
		name    string
		message string
		body    string
		code    string
		want    bool
	}{
		{"exact message", "API key quota exhausted", "", "", true},
		{"lowercase message", "api key quota exhausted", "", "", true},
		{"message in body", "", `{"error":{"message":"API key quota exhausted"}}`, "", true},
		{"error code only", "", "", "API_KEY_QUOTA_EXHAUSTED", true},
		{"lowercase error code", "", "", "api_key_quota_exhausted", true},
		// 正常错误必须保留，尤其是同为 429 / 业务限制的其他口径
		{"rate limit", "rate limit exceeded", "", "rate_limit_exceeded", false},
		{"insufficient balance", "Insufficient balance", "", "INSUFFICIENT_BALANCE", false},
		{"daily quota wording", "api key daily rate limit exceeded", "", "", false},
		{"empty", "", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, shouldSkipAPIKeyQuotaExhausted(tc.message, tc.body, tc.code))
		})
	}
}

// TestShouldSkipAPIKeyQuotaExhaustedIgnoresSettings 验证该过滤与开关无关：
// 即使所有开关都为 false，额度耗尽依然被跳过。
func TestShouldSkipAPIKeyQuotaExhaustedIgnoresSettings(t *testing.T) {
	settings := service.OpsAdvancedSettings{
		IgnoreInsufficientBalanceErrors: false,
		IgnoreAuthFallbackAdminErrors:   false,
		SchemaVersion:                   2,
	}
	require.False(t, settings.IgnoreInsufficientBalanceErrors)
	// 主路径：shouldSkipOpsErrorLog 内部先做无条件判断，不受 settings 影响。
	// 用一个所有开关都关闭的真实 OpsService 验证——额度耗尽仍必须被跳过。
	svc := service.NewOpsService(nil, &opsAdvancedSettingsRepoStub{
		advanced: `{"ignore_insufficient_balance_errors":false,"ignore_auth_fallback_admin_errors":false,"schema_version":2}`,
	}, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	require.NoError(t, svc.RefreshRuntimeSettings(t.Context()))
	require.False(t, svc.OpsAdvancedSettingsSnapshot().IgnoreInsufficientBalanceErrors)
	require.False(t, svc.OpsAdvancedSettingsSnapshot().IgnoreAuthFallbackAdminErrors)
	require.True(t, shouldSkipOpsErrorLog(svc, "API key quota exhausted", "", "", "/v1/chat/completions"))
}

// TestShouldSkipEnqueuedAPIKeyQuotaExhausted 覆盖 2xx 恢复行等不经
// shouldSkipOpsErrorLog 的落库路径（与主路径同一口径）。
func TestShouldSkipEnqueuedAPIKeyQuotaExhausted(t *testing.T) {
	upstreamMsg := "api key quota exhausted"
	upstreamDetail := "API key quota exhausted"
	cases := []struct {
		name  string
		entry *service.OpsInsertErrorLogInput
		want  bool
	}{
		{"error message", &service.OpsInsertErrorLogInput{ErrorMessage: "API key quota exhausted"}, true},
		{"error body", &service.OpsInsertErrorLogInput{ErrorBody: `{"message":"api key quota exhausted"}`}, true},
		{"upstream message", &service.OpsInsertErrorLogInput{UpstreamErrorMessage: &upstreamMsg}, true},
		{"upstream detail", &service.OpsInsertErrorLogInput{UpstreamErrorDetail: &upstreamDetail}, true},
		{"normal error", &service.OpsInsertErrorLogInput{ErrorMessage: "upstream timeout"}, false},
		{"request preview must not gate", &service.OpsInsertErrorLogInput{
			RequestPreview: "api key quota exhausted",
			ErrorMessage:   "upstream timeout",
		}, false},
		{"nil entry", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, shouldSkipEnqueuedAPIKeyQuotaExhausted(tc.entry))
		})
	}
}

// TestEnqueueOpsErrorLogKeepsQuotaGuard 防止重构删掉 enqueue 兜底过滤。
func TestEnqueueOpsErrorLogKeepsQuotaGuard(t *testing.T) {
	src, err := os.ReadFile("ops_error_logger.go")
	require.NoError(t, err)
	body := string(src)
	start := strings.Index(body, "func enqueueOpsErrorLog(")
	require.GreaterOrEqual(t, start, 0)
	rest := body[start:]
	if end := strings.Index(rest, "\nfunc "); end > 0 {
		rest = rest[:end]
	}
	require.Contains(t, rest, "shouldSkipEnqueuedAPIKeyQuotaExhausted",
		"所有错误日志入队路径都必须经过额度耗尽兜底过滤")
}

// TestDefaultOpsAdvancedSettingsIgnoresAuthFallbackAdmin 锁定 auth 兜底开关默认开启。
func TestDefaultOpsAdvancedSettingsIgnoresAuthFallbackAdmin(t *testing.T) {
	// 无存储配置时 RefreshRuntimeSettings 走默认快照，新开关必须为 true。
	svc := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	require.NoError(t, svc.RefreshRuntimeSettings(t.Context()))
	require.True(t, svc.OpsAdvancedSettingsSnapshot().IgnoreAuthFallbackAdminErrors,
		"auth 兜底到 admin 的错误默认必须不写日志，避免登录/注册失败刷屏")
}

// TestAuthFallbackAdminSwitchReadFromStoredConfig 验证存量配置（无该字段）读取时
// 继承默认 true，管理员显式写 false 后能被尊重。
func TestAuthFallbackAdminSwitchReadFromStoredConfig(t *testing.T) {
	// 存量配置没有 ignore_auth_fallback_admin_errors 字段：反序列化后应为默认 true。
	svc := service.NewOpsService(nil, &opsAdvancedSettingsRepoStub{
		advanced: `{"schema_version":2}`,
	}, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	require.NoError(t, svc.RefreshRuntimeSettings(t.Context()))
	require.True(t, svc.OpsAdvancedSettingsSnapshot().IgnoreAuthFallbackAdminErrors,
		"存量配置缺失该字段时必须继承默认 true")

	// 管理员显式关闭后应被尊重。
	svc2 := service.NewOpsService(nil, &opsAdvancedSettingsRepoStub{
		advanced: `{"ignore_auth_fallback_admin_errors":false,"schema_version":2}`,
	}, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	require.NoError(t, svc2.RefreshRuntimeSettings(t.Context()))
	require.False(t, svc2.OpsAdvancedSettingsSnapshot().IgnoreAuthFallbackAdminErrors,
		"管理员显式写 false 后必须被尊重")
}

// serveAuthFallbackError 模拟 auth 入口返回一条「无法确定用户」的 400 错误，
// 返回该请求实际入队的错误日志数量与队首条目。
func serveAuthFallbackError(t *testing.T, ops *service.OpsService) (*service.OpsInsertErrorLogInput, int64) {
	t.Helper()
	setupOpsErrorLogTestQueue(t, 2)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(OpsErrorLoggerMiddlewareWithDefaultUser(ops, 1))
	router.POST("/api/v1/auth/send-verify-code", func(c *gin.Context) {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "INVALID_VERIFY_CODE",
			"message": "invalid or expired verification code",
		})
	})
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(
		http.MethodPost, "/api/v1/auth/send-verify-code", nil))
	queued := OpsErrorLogQueueLength()
	if queued == 0 {
		return nil, 0
	}
	return (<-opsErrorLogQueue).entry, queued
}

// TestAuthFallbackAdminMiddlewareSwitch 端到端验证 auth 兜底开关：
// 默认开启时无法归属用户的登录/注册失败不落库；关闭后落库且归属 admin(id=1)。
func TestAuthFallbackAdminMiddlewareSwitch(t *testing.T) {
	t.Run("default on skips auth fallback errors", func(t *testing.T) {
		ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		require.NoError(t, ops.RefreshRuntimeSettings(t.Context()))
		require.True(t, ops.OpsAdvancedSettingsSnapshot().IgnoreAuthFallbackAdminErrors)

		entry, queued := serveAuthFallbackError(t, ops)
		require.Zero(t, queued, "默认开关开启时 auth 兜底错误不得写日志")
		require.Nil(t, entry)
	})

	t.Run("switch off records with admin user id", func(t *testing.T) {
		ops := service.NewOpsService(nil, &opsAdvancedSettingsRepoStub{
			advanced: `{"ignore_auth_fallback_admin_errors":false,"schema_version":2}`,
		}, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		require.NoError(t, ops.RefreshRuntimeSettings(t.Context()))
		require.False(t, ops.OpsAdvancedSettingsSnapshot().IgnoreAuthFallbackAdminErrors)

		entry, queued := serveAuthFallbackError(t, ops)
		require.Equal(t, int64(1), queued, "关闭开关后 auth 兜底错误应恢复记录")
		require.NotNil(t, entry)
		require.NotNil(t, entry.UserID)
		require.Equal(t, int64(1), *entry.UserID, "归属应兜底为 admin(id=1)")
		require.Contains(t, entry.ErrorMessage, "invalid or expired verification code")
	})
}
