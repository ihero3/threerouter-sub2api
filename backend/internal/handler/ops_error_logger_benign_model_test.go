package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/stretchr/testify/require"
)

// TestShouldSkipBenignModelError 锁死三类「预期内、量大」模型错误的无条件过滤口径：
//   - 该模型所有账号暂时忙/被封（temporarily busy or blocked），模型名任意；
//   - 模型名乱写导致 404（does not exist / is not available for this group），模型名任意；
//   - 分组无账号支持该模型（not supported by any configured account），模型名任意。
//
// 模型名多变，只能匹配文案里的稳定短语；大小写不敏感。
func TestShouldSkipBenignModelError(t *testing.T) {
	cases := []struct {
		name    string
		message string
		body    string
		want    bool
	}{
		// 第一类：账号忙/被封，模型名多样
		{"busy blocked deepseek-flash", `All available accounts for model "deepseek-flash" are temporarily busy or blocked, please try again later`, "", true},
		{"busy blocked arbitrary model", `All available accounts for model "gpt-anything-9" are temporarily busy or blocked, please try again later`, "", true},
		{"busy blocked uppercase", `All available accounts for model "X" are TEMPORARILY BUSY OR BLOCKED, please try again later`, "", true},
		{"busy blocked in body", "", `{"error":{"message":"All available accounts for model \"plaza\" are temporarily busy or blocked, please try again later"}}`, true},
		// 第二类：模型名乱写
		{"not available plaza", `Model "plaza" does not exist or is not available for this group`, "", true},
		{"not available arbitrary model", `Model "zzz-made-up-name" does not exist or is not available for this group`, "", true},
		{"not available short variant", `Model "foo" is not available for this group`, "", true},
		{"not available uppercase", `MODEL "BAR" DOES NOT EXIST OR IS NOT AVAILABLE FOR THIS GROUP`, "", true},
		{"not available in body", "", `{"error":{"code":"model_not_found","message":"Model \"plaza\" does not exist or is not available for this group"}}`, true},
		// 第三类：分组无账号支持该模型（no_account_error.go 本地生成），模型名多样
		{"not supported deepseek", `Model "deepseek-v4.1-flash-1017" is not supported by any configured account in this group`, "", true},
		{"not supported arbitrary model", `Model "zzz-made-up-name" is not supported by any configured account in this group`, "", true},
		{"not supported uppercase", `MODEL "BAR" IS NOT SUPPORTED BY ANY CONFIGURED ACCOUNT IN THIS GROUP`, "", true},
		{"not supported in body", "", `{"error":{"type":"model_not_found","message":"Model \"plaza\" is not supported by any configured account in this group"}}`, true},
		// 正常错误 / 相似但不命中的错误必须保留
		{"all accounts exhausted stays", "All available accounts exhausted", "", false},
		{"rate limited stays", "All available accounts are currently rate-limited. Please retry later.", "", false},
		{"generic no available accounts stays", "no available accounts", "", false},
		{"retired model guidance stays", `Model "gpt-5.5" targets GPT-5.5, which retired from Codex with ChatGPT sign-in`, "", false},
		{"insufficient stays", "Insufficient balance", "", false},
		{"upstream timeout stays", "upstream timeout", "", false},
		{"rate limit stays", "rate_limit_exceeded", "", false},
		{"empty stays", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, shouldSkipBenignModelError(tc.message, tc.body, ""))
		})
	}
}

// TestShouldSkipBenignModelErrorIgnoresSettings 验证这两类过滤与任何后台开关无关：
// 所有开关全关时，主路径 shouldSkipOpsErrorLog 依然跳过。
func TestShouldSkipBenignModelErrorIgnoresSettings(t *testing.T) {
	svc := service.NewOpsService(nil, &opsAdvancedSettingsRepoStub{
		advanced: `{"schema_version":2}`,
	}, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	require.NoError(t, svc.RefreshRuntimeSettings(t.Context()))

	for _, msg := range []string{
		`All available accounts for model "deepseek-flash" are temporarily busy or blocked, please try again later`,
		`Model "plaza" does not exist or is not available for this group`,
		`Model "deepseek-v4.1-flash-1017" is not supported by any configured account in this group`,
	} {
		require.True(t, shouldSkipOpsErrorLog(svc, msg, "", "", "/v1/chat/completions"), msg)
	}
}

// TestShouldSkipEnqueuedBenignModelError 覆盖 2xx 恢复行等不经 shouldSkipOpsErrorLog
// 的落库路径：只看最终错误文本字段，不展开 UpstreamErrors，也不看 RequestPreview。
func TestShouldSkipEnqueuedBenignModelError(t *testing.T) {
	upstreamMsg := `Model "plaza" does not exist or is not available for this group`
	detail := `All available accounts for model "deepseek-flash" are temporarily busy or blocked, please try again later`
	cases := []struct {
		name  string
		entry *service.OpsInsertErrorLogInput
		want  bool
	}{
		{"error message", &service.OpsInsertErrorLogInput{ErrorMessage: `Model "x" is not available for this group`}, true},
		{"error body", &service.OpsInsertErrorLogInput{ErrorBody: `{"error":{"message":"are temporarily busy or blocked"}}`}, true},
		{"upstream message", &service.OpsInsertErrorLogInput{UpstreamErrorMessage: &upstreamMsg}, true},
		{"upstream detail", &service.OpsInsertErrorLogInput{UpstreamErrorDetail: &detail}, true},
		{"not supported message", &service.OpsInsertErrorLogInput{ErrorMessage: `Model "x" is not supported by any configured account in this group`}, true},
		{"normal error", &service.OpsInsertErrorLogInput{ErrorMessage: "upstream timeout", ErrorBody: `{"error":"timeout"}`}, false},
		{"exhausted stays", &service.OpsInsertErrorLogInput{ErrorMessage: "All available accounts exhausted"}, false},
		{"rate limited stays", &service.OpsInsertErrorLogInput{ErrorMessage: "All available accounts are currently rate-limited. Please retry later."}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, shouldSkipEnqueuedBenignModelError(tc.entry))
		})
	}
}
