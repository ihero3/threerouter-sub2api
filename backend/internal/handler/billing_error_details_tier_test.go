package handler

import (
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 订阅档位耗尽错误映射：429 + Retry-After（window_resets_at metadata）。
func TestBillingErrorDetailsSubscriptionTierExceeded(t *testing.T) {
	resetAt := time.Now().Add(2 * time.Hour)
	err := service.ErrDailyLimitExceeded.WithMetadata(map[string]string{
		"window_resets_at":          resetAt.Format(time.RFC3339),
		"balance_fallback_rejected": "true",
	})

	status, code, message, retryAfter := billingErrorDetails(err)
	require.Equal(t, 429, status)
	require.Equal(t, "rate_limit_exceeded", code)
	require.Greater(t, retryAfter, 0)
	require.LessOrEqual(t, retryAfter, 7200)
	require.Contains(t, message, "Subscription quota and account balance are both insufficient")
	require.Contains(t, message, "resets automatically")
}

// 双耗尽但缺 window_resets_at：文案仍升级，Retry-After 走兜底秒数。
func TestBillingErrorDetailsSubscriptionTierWithoutResetTime(t *testing.T) {
	err := service.ErrWeeklyLimitExceeded.WithMetadata(map[string]string{
		"balance_fallback_rejected": "true",
	})

	status, _, message, retryAfter := billingErrorDetails(err)
	require.Equal(t, 429, status)
	require.GreaterOrEqual(t, retryAfter, 60)
	require.Contains(t, message, "Subscription quota and account balance are both insufficient")
}

// 单纯档位耗尽（回退开关关闭，无 metadata）：保持原文案，无升级后缀。
func TestBillingErrorDetailsSubscriptionTierPlain(t *testing.T) {
	status, code, message, retryAfter := billingErrorDetails(service.ErrMonthlyLimitExceeded)
	require.Equal(t, 429, status)
	require.Equal(t, "rate_limit_exceeded", code)
	require.NotContains(t, message, "balance")
	require.Equal(t, 60, retryAfter, "无 window_resets_at metadata 时走 60s 兜底退避")
}

// 非档位错误不被该分支吞掉。
func TestBillingErrorDetailsIgnoresNonTierErrors(t *testing.T) {
	status, _, _, _ := billingErrorDetails(service.ErrSubscriptionInvalid)
	require.NotEqual(t, 429, status)

	// 直接构造未映射错误走默认分支（与既有 TestBillingErrorDetails_UnknownErrorFallsBackTo403 一致）
	status, _, _, _ = billingErrorDetails(errors.New("boom"))
	require.Equal(t, 403, status)
}
