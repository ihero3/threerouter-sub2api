package handler

import (
	"strings"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// TestRPMLimitMessage_IncludesLimitAndRetrySeconds 429 文案必须带上 N（限额）与 X（等待秒数）。
//
// 背景：旧文案只有 "user requests-per-minute limit exceeded"——用户不知道超了多少、
// 也不知要等多久，只能判定"服务不稳定"，进而不断重试放大限流。
// 这是用户体感问题的直接修复点，必须固化。
func TestRPMLimitMessage_IncludesLimitAndRetrySeconds(t *testing.T) {
	err := service.ErrUserRPMExceeded.WithMetadata(map[string]string{
		"rpm_limit": "30",
		"rpm_scope": "user",
	})
	msg := rpmLimitMessage(err, 42)

	if !strings.Contains(msg, "limit is 30 requests/minute") {
		t.Errorf("missing English limit context, got: %s", msg)
	}
	if !strings.Contains(msg, "please retry after 42 seconds") {
		t.Errorf("missing English retry context, got: %s", msg)
	}
	if !strings.Contains(msg, "当前限额 30 次/分钟") {
		t.Errorf("missing Chinese limit context, got: %s", msg)
	}
	if !strings.Contains(msg, "请 42 秒后重试") {
		t.Errorf("missing Chinese retry context, got: %s", msg)
	}
}

// TestRPMLimitMessage_GroupScopeUsesSameFormat 分组维度命中时同样带上上下文。
// checkRPM 有三条命中路径（override / group / user），任一条漏挂 metadata 都会
// 让用户拿到无上下文的旧文案。
func TestRPMLimitMessage_GroupScopeUsesSameFormat(t *testing.T) {
	err := service.ErrGroupRPMExceeded.WithMetadata(map[string]string{
		"rpm_limit": "60",
		"rpm_scope": "group",
	})
	msg := rpmLimitMessage(err, 7)
	if !strings.Contains(msg, "当前限额 60 次/分钟") || !strings.Contains(msg, "请 7 秒后重试") {
		t.Errorf("group scope message = %s, want limit=60 retry=7", msg)
	}
}

// TestRPMLimitMessage_DegradesWithoutMetadata metadata 缺失时不能报错或露出 -1/0 之类脏值。
// 覆盖直接返回哨兵错误的旧路径（如自建的错误构造点未走 checkRPM）。
func TestRPMLimitMessage_DegradesWithoutMetadata(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"bare sentinel", service.ErrUserRPMExceeded},
		{"empty limit", service.ErrUserRPMExceeded.WithMetadata(map[string]string{"rpm_limit": ""})},
		{"non-numeric limit", service.ErrUserRPMExceeded.WithMetadata(map[string]string{"rpm_limit": "abc"})},
		{"zero limit", service.ErrUserRPMExceeded.WithMetadata(map[string]string{"rpm_limit": "0"})},
		{"negative limit", service.ErrUserRPMExceeded.WithMetadata(map[string]string{"rpm_limit": "-5"})},
		{"naked error", infraerrors.New(429, "USER_RPM_EXCEEDED", "user requests-per-minute limit exceeded")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := rpmLimitMessage(tc.err, 15)
			if strings.Contains(msg, "limit is") {
				t.Errorf("should omit limit when unavailable, got: %s", msg)
			}
			if strings.Contains(msg, "0 次/分钟") || strings.Contains(msg, "-") {
				t.Errorf("leaked invalid limit value, got: %s", msg)
			}
			if !strings.Contains(msg, "请 15 秒后重试") {
				t.Errorf("should still carry retry seconds, got: %s", msg)
			}
			if msg == "" {
				t.Fatal("message must not be empty")
			}
		})
	}
}

// TestRPMLimitMessage_RetrySecondsFloor 秒数为 0/负数时兜底为 1，避免"请 0 秒后重试"
// 诱导客户端立即重试、放大限流。
func TestRPMLimitMessage_RetrySecondsFloor(t *testing.T) {
	err := service.ErrUserRPMExceeded.WithMetadata(map[string]string{"rpm_limit": "10"})
	for _, secs := range []int{0, -3} {
		if msg := rpmLimitMessage(err, secs); !strings.Contains(msg, "请 1 秒后重试") {
			t.Errorf("retrySeconds=%d: got %s, want floor of 1 second", secs, msg)
		}
	}
}

// TestBillingErrorDetails_RPMUsesContextualMessage 端到端确认 billingErrorDetails 的
// RPM 分支确实替换了旧文案。billingErrorDetails 是 20+ 个端点共用的唯一出口，
// 这里漏改会让所有端点的 429 都退回无上下文的旧文案。
func TestBillingErrorDetails_RPMUsesContextualMessage(t *testing.T) {
	err := service.ErrUserRPMExceeded.WithMetadata(map[string]string{"rpm_limit": "30"})
	status, code, msg, retryAfter := billingErrorDetails(err)

	if status != 429 {
		t.Errorf("status = %d, want 429", status)
	}
	if code != "rate_limit_exceeded" {
		t.Errorf("code = %q, want rate_limit_exceeded", code)
	}
	if retryAfter < 1 || retryAfter > 60 {
		t.Errorf("retryAfter = %d, want 1..60", retryAfter)
	}
	if strings.Contains(msg, "requests-per-minute limit exceeded") {
		t.Errorf("old bare message leaked through: %s", msg)
	}
	if !strings.Contains(msg, "当前限额 30 次/分钟") {
		t.Errorf("missing limit context: %s", msg)
	}
	// 文案里的秒数必须与 Retry-After 头同源，否则客户端会拿到自相矛盾的指引。
	if !strings.Contains(msg, "requests/minute, please retry after") {
		t.Errorf("missing retry seconds: %s", msg)
	}
}
