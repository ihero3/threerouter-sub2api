package handler

import (
	"os"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/stretchr/testify/require"
)

func enabledInsufficientFilterSettings() service.OpsAdvancedSettings {
	return service.OpsAdvancedSettings{
		IgnoreInsufficientBalanceErrors: true,
		SchemaVersion:                   2,
	}
}

// TestShouldSkipInsufficientErrorMatchesAllVariants 锁死「凡含 insufficient 都过滤」口径：
// 旧的三个短语白名单会漏掉下划线错误码、credits/funds 等变体，这里逐条回归。
func TestShouldSkipInsufficientErrorMatchesAllVariants(t *testing.T) {
	settings := enabledInsufficientFilterSettings()
	cases := []struct {
		name    string
		message string
		body    string
		code    string
	}{
		{"user balance message", "Insufficient balance", `{"error":{"message":"Insufficient balance"}}`, ""},
		{"uppercase code only", "", "", "INSUFFICIENT_BALANCE"},
		{"underscore code in body", "", `{"code":"INSUFFICIENT_BALANCE","message":"..."}`, ""},
		{"quota", "insufficient_quota", "", ""},
		{"credits", "Insufficient credits", "", ""},
		{"funds", "You have insufficient funds", "", ""},
		{"upstream out of credit", "Upstream payment required: insufficient balance or billing issue", "", ""},
		// 用户明确选择「所有含 Insufficient 都过滤」后的已知连带影响，勿当成回归。
		{"oauth scope", "insufficient authentication scopes", "", ""},
		{"mixed case", "INSUFFICIENT Account Balance", "", ""},
		{"body only", "", `{"error":{"message":"insufficient account balance"}}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.True(t, shouldSkipInsufficientError(settings, tc.message, tc.body, tc.code))
		})
	}
}

func TestShouldSkipInsufficientErrorLeavesNormalErrors(t *testing.T) {
	settings := enabledInsufficientFilterSettings()
	cases := []struct {
		message string
		body    string
		code    string
	}{
		{"rate limit exceeded", `{"error":{"message":"rate limit exceeded"}}`, "rate_limit_exceeded"},
		{"internal server error", "", "internal_error"},
		{"", `{"code":"billing_error","message":"Billing error"}`, "billing_error"},
		{"no available accounts", `{"error":{"message":"no available accounts"}}`, ""},
	}
	for i, tc := range cases {
		require.False(t, shouldSkipInsufficientError(settings, tc.message, tc.body, tc.code),
			"正常错误必须保留，case %d", i)
	}
}

func TestShouldSkipInsufficientErrorRespectsSwitch(t *testing.T) {
	off := service.OpsAdvancedSettings{
		IgnoreInsufficientBalanceErrors: false,
		SchemaVersion:                   2,
	}
	require.False(t, shouldSkipInsufficientError(off, "Insufficient balance", "", "INSUFFICIENT_BALANCE"),
		"管理员关闭开关后必须恢复记录")
}

func TestShouldSkipOpsErrorLogNilOps(t *testing.T) {
	require.False(t, shouldSkipOpsErrorLog(nil, "Insufficient balance", "", "INSUFFICIENT_BALANCE", "/v1/chat/completions"))
}

// TestShouldSkipEnqueuedInsufficientError 覆盖 2xx 上游恢复行等
// 不经 shouldSkipOpsErrorLog 的落库路径。
func TestShouldSkipEnqueuedInsufficientError(t *testing.T) {
	settings := enabledInsufficientFilterSettings()
	upstreamMsg := "insufficient credits"
	upstreamDetail := "account has insufficient balance"
	cases := []struct {
		name  string
		entry *service.OpsInsertErrorLogInput
		want  bool
	}{
		{"error message", &service.OpsInsertErrorLogInput{ErrorMessage: "Insufficient balance"}, true},
		{"error body", &service.OpsInsertErrorLogInput{ErrorBody: `{"code":"INSUFFICIENT_BALANCE"}`}, true},
		{"upstream message (2xx recovery row)", &service.OpsInsertErrorLogInput{UpstreamErrorMessage: &upstreamMsg}, true},
		{"upstream detail", &service.OpsInsertErrorLogInput{UpstreamErrorDetail: &upstreamDetail}, true},
		{"normal error", &service.OpsInsertErrorLogInput{ErrorMessage: "upstream timeout", ErrorBody: `{"error":"timeout"}`}, false},
		// 用户输入里恰好出现该词不该吞掉错误日志
		{"request preview must not gate", &service.OpsInsertErrorLogInput{
			RequestPreview: "please explain insufficient funds",
			ErrorMessage:   "upstream timeout",
		}, false},
		{"nil entry", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, shouldSkipEnqueuedInsufficientError(settings, tc.entry))
		})
	}

	off := service.OpsAdvancedSettings{IgnoreInsufficientBalanceErrors: false, SchemaVersion: 2}
	require.False(t, shouldSkipEnqueuedInsufficientError(off,
		&service.OpsInsertErrorLogInput{ErrorMessage: "Insufficient balance"}))
}

// TestEnqueueOpsErrorLogKeepsInsufficientGuard 防止后人重构时删掉兜底过滤，
// 导致 2xx 恢复行 / 专用记录路径又绕过设置项（历史上出现过类似静默失效）。
func TestEnqueueOpsErrorLogKeepsInsufficientGuard(t *testing.T) {
	src, err := os.ReadFile("ops_error_logger.go")
	require.NoError(t, err)
	body := string(src)
	start := strings.Index(body, "func enqueueOpsErrorLog(")
	require.GreaterOrEqual(t, start, 0)
	rest := body[start:]
	if end := strings.Index(rest, "\nfunc "); end > 0 {
		rest = rest[:end]
	}
	require.Contains(t, rest, "shouldSkipEnqueuedInsufficientError",
		"所有错误日志入队路径都必须经过兜底过滤")
}
