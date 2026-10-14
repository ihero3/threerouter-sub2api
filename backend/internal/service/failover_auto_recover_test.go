package service

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func TestParseUpstreamResetMessage(t *testing.T) {
	now := time.Date(2026, 10, 14, 10, 0, 0, 0, time.FixedZone("CST", 8*3600))

	tests := []struct {
		name    string
		msg     string
		want    time.Time
		wantNil bool
	}{
		{
			name: "openai style with offset and zone abbreviation",
			msg:  "You have exceeded the 5-hour usage quota. It will reset at 2026-10-14 12:58:44 +0800 CST. We recommend upgrading your plan.",
			want: time.Date(2026, 10, 14, 12, 58, 44, 0, time.FixedZone("CST", 8*3600)),
		},
		{
			name: "offset only",
			msg:  "rate limited; reset at 2026-10-14 12:58:44 +0800",
			want: time.Date(2026, 10, 14, 12, 58, 44, 0, time.FixedZone("", 8*3600)),
		},
		{
			name: "rfc3339 with T separator",
			msg:  "It will reset at 2026-10-14T12:58:44+08:00",
			want: time.Date(2026, 10, 14, 12, 58, 44, 0, time.FixedZone("", 8*3600)),
		},
		{
			name: "naive local time",
			msg:  "It will reset at 2026-10-14 12:58:44",
			want: time.Date(2026, 10, 14, 12, 58, 44, 0, now.Location()),
		},
		{
			name:    "time already in the past",
			msg:     "It will reset at 2026-10-14 09:00:00 +0800 CST",
			wantNil: true,
		},
		{
			name:    "reset time beyond max age",
			msg:     "It will reset at 2026-10-30 12:58:44 +0800 CST",
			wantNil: true,
		},
		{
			name:    "no reset keyword",
			msg:     "something happened at 2026-10-14 12:58:44 +0800 CST",
			wantNil: true,
		},
		{
			name:    "zone abbreviation without offset is ambiguous",
			msg:     "It will reset at 2026-10-14 12:58:44 CST",
			wantNil: true,
		},
		{
			name:    "empty message",
			msg:     "",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseUpstreamResetMessage(tt.msg, now)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got %s", got.Format(time.RFC3339))
				}
				return
			}
			if got == nil {
				t.Fatalf("expected %s, got nil", tt.want.Format(time.RFC3339))
			}
			// 比较绝对时刻与偏移，避免 zone 名称差异干扰。
			if !got.Equal(tt.want) || got.Unix() != tt.want.Unix() {
				t.Fatalf("expected %s (%d), got %s (%d)", tt.want.Format(time.RFC3339), tt.want.Unix(), got.Format(time.RFC3339), got.Unix())
			}
		})
	}
}

func TestFailoverAutoRecoverAt(t *testing.T) {
	now := time.Date(2026, 10, 14, 10, 0, 0, 0, time.FixedZone("CST", 8*3600))

	if got := failoverAutoRecoverAt(http.StatusUnauthorized, "reset at 2026-10-14 12:58:44 +0000 UTC", nil, now); got != nil {
		t.Fatalf("non-429 must never auto-recover, got %v", got)
	}

	msg := "It will reset at 2026-10-14 12:58:44 +0800 CST"
	got := failoverAutoRecoverAt(http.StatusTooManyRequests, msg, nil, now)
	if got == nil {
		t.Fatal("expected recovery time from upstream message")
	}
	if wantUnix := time.Date(2026, 10, 14, 12, 58, 44, 0, time.FixedZone("CST", 8*3600)).Unix(); got.Unix() != wantUnix {
		t.Fatalf("expected unix %d, got %d", wantUnix, got.Unix())
	}

	// 无明确文案时退化到 Retry-After（相对秒数）。
	headers := http.Header{}
	headers.Set("Retry-After", "60")
	got = failoverAutoRecoverAt(http.StatusTooManyRequests, "rate limit exceeded", headers, now)
	if got == nil {
		t.Fatal("expected recovery time from Retry-After")
	}
	if got.Unix() != now.Add(60*time.Second).Unix() {
		t.Fatalf("expected now+60s, got %s", got.Format(time.RFC3339))
	}

	// 什么都拿不到 → nil，保持永久禁用。
	if got := failoverAutoRecoverAt(http.StatusTooManyRequests, "rate limit exceeded", nil, now); got != nil {
		t.Fatalf("expected nil without any reset hint, got %v", got)
	}
}

func TestEnsureFailoverAutoRecoverTag(t *testing.T) {
	recoverAt := time.Date(2026, 10, 14, 4, 58, 44, 0, time.UTC)
	tagged := ensureFailoverAutoRecoverTag("Auto-disabled on upstream failover: HTTP 429", recoverAt)
	if tagged == "Auto-disabled on upstream failover: HTTP 429" {
		t.Fatal("expected auto-recover tag appended")
	}
	if !strings.Contains(tagged, FailoverAutoRecoverMessageTag) {
		t.Fatalf("tag marker missing from %q", tagged)
	}
	// 幂等：已带标记不再追加。
	if again := ensureFailoverAutoRecoverTag(tagged, recoverAt); again != tagged {
		t.Fatalf("expected idempotent tagging, got %q", again)
	}
}

// failoverDisableRepoStub 只承载 GetByID/Update，其余接口方法通过内嵌接口保持空实现。
type failoverDisableRepoStub struct {
	AccountRepository
	acct      *Account
	updated   *Account
	getCalled bool
}

func (r *failoverDisableRepoStub) GetByID(ctx context.Context, id int64) (*Account, error) {
	r.getCalled = true
	return r.acct, nil
}

func (r *failoverDisableRepoStub) Update(ctx context.Context, account *Account) error {
	r.updated = account
	return nil
}

// failoverSettingRepoStub 只提供 GetValue，其余 SettingRepository 方法通过内嵌接口留空。
type failoverSettingRepoStub struct {
	SettingRepository
	values map[string]string
}

func (r *failoverSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if v, ok := r.values[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}

func (r *failoverSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if v, ok := r.values[key]; ok {
			result[key] = v
		}
	}
	return result, nil
}

func newFailoverDisableSettingService(enabled bool) *SettingService {
	values := map[string]string{}
	if enabled {
		values[SettingKeyDisableFailedAccountOnFailover] = "true"
	}
	// 清空包级网关设置缓存，避免其他测试写入的缓存影响本用例的开关判定。
	gatewayForwardingSF.Forget("gateway_forwarding")
	gatewayForwardingCache.Store((*cachedGatewayForwardingSettings)(nil))
	return NewSettingService(&failoverSettingRepoStub{values: values}, &config.Config{})
}

func alwaysFailover(int) bool { return true }

func TestMaybeDisableAccountOnFailover_429WithResetTime_AutoRecoverable(t *testing.T) {
	repo := &failoverDisableRepoStub{acct: &Account{ID: 1, Status: StatusActive, Schedulable: true}}
	cst := time.FixedZone("CST", 8*3600)
	resetAt := time.Now().In(cst).Add(2 * time.Hour)
	msg := "You have exceeded the 5-hour usage quota. It will reset at " +
		resetAt.Format("2006-01-02 15:04:05 -0700 MST") + ". Please wait."

	maybeDisableAccountOnFailover(context.Background(), newFailoverDisableSettingService(true), repo,
		repo.acct, http.StatusTooManyRequests, nil, alwaysFailover, msg)

	if repo.updated == nil {
		t.Fatal("expected account to be updated")
	}
	if repo.updated.Status != StatusError {
		t.Fatalf("expected status error, got %s", repo.updated.Status)
	}
	if repo.updated.RateLimitResetAt == nil {
		t.Fatal("expected rate_limit_reset_at to be set for auto recovery")
	}
	if repo.updated.RateLimitedAt == nil {
		t.Fatal("expected rate_limited_at to be set")
	}
	if repo.updated.RateLimitResetAt.Unix() != resetAt.Unix() {
		t.Fatalf("expected reset at %d, got %d", resetAt.Unix(), repo.updated.RateLimitResetAt.Unix())
	}
	if !strings.Contains(repo.updated.ErrorMessage, FailoverAutoRecoverMessageTag) {
		t.Fatalf("expected auto-recover tag in error message, got %q", repo.updated.ErrorMessage)
	}
}

func TestMaybeDisableAccountOnFailover_429WithoutResetTime_Permanent(t *testing.T) {
	repo := &failoverDisableRepoStub{acct: &Account{ID: 1, Status: StatusActive, Schedulable: true}}

	maybeDisableAccountOnFailover(context.Background(), newFailoverDisableSettingService(true), repo,
		repo.acct, http.StatusTooManyRequests, nil, alwaysFailover, "rate limit exceeded")

	if repo.updated == nil {
		t.Fatal("expected account to be updated")
	}
	if repo.updated.Status != StatusError {
		t.Fatalf("expected status error, got %s", repo.updated.Status)
	}
	if repo.updated.RateLimitResetAt != nil {
		t.Fatalf("expected no reset time, got %s", repo.updated.RateLimitResetAt.Format(time.RFC3339))
	}
	if strings.Contains(repo.updated.ErrorMessage, FailoverAutoRecoverMessageTag) {
		t.Fatalf("permanent disable must not carry auto-recover tag, got %q", repo.updated.ErrorMessage)
	}
}

func TestMaybeDisableAccountOnFailover_401_Permanent(t *testing.T) {
	repo := &failoverDisableRepoStub{acct: &Account{ID: 1, Status: StatusActive, Schedulable: true}}

	maybeDisableAccountOnFailover(context.Background(), newFailoverDisableSettingService(true), repo,
		repo.acct, http.StatusUnauthorized, nil, alwaysFailover, "unauthorized")

	if repo.updated == nil || repo.updated.Status != StatusError {
		t.Fatal("expected account to be permanently disabled")
	}
	if repo.updated.RateLimitResetAt != nil {
		t.Fatal("401 must never schedule auto recovery")
	}
}

func TestMaybeDisableAccountOnFailover_SettingDisabled_NoOp(t *testing.T) {
	repo := &failoverDisableRepoStub{acct: &Account{ID: 1, Status: StatusActive, Schedulable: true}}

	maybeDisableAccountOnFailover(context.Background(), newFailoverDisableSettingService(false), repo,
		repo.acct, http.StatusTooManyRequests, nil, alwaysFailover, "reset at 2026-10-14 12:58:44 +0800 CST")

	if repo.getCalled || repo.updated != nil {
		t.Fatal("account must be untouched when the setting is disabled")
	}
}
