package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeNewUserRiskMarker struct {
	calls []struct {
		userID  int64
		level   string
		reasons []string
	}
}

func (f *fakeNewUserRiskMarker) MarkUserRisk(_ context.Context, userID int64, level string, reasons []string) {
	f.calls = append(f.calls, struct {
		userID  int64
		level   string
		reasons []string
	}{userID: userID, level: level, reasons: reasons})
}

func newNewUserFixture() (*IPConsistencyService, *fakePenaltyLocker, *fakePenalty, *fakeUserStatusUpdater, *fakeNewUserRiskMarker) {
	locker := &fakePenaltyLocker{}
	penalty := &fakePenalty{started: make(chan struct{}, 4)}
	updater := &fakeUserStatusUpdater{}
	marker := &fakeNewUserRiskMarker{}
	svc := NewIPConsistencyService(&fakeIPConsistencyCache{}, penaltySettings(), updater)
	svc.SetPenaltyService(penalty, locker)
	svc.SetNewUserRiskMarker(marker)
	return svc, locker, penalty, updater, marker
}

// TestNewUserFirstUseIP_MismatchStrictlyPunishes 锁死「窗口内 IP 不一致 → 禁用+suspect+扣钱+邮件」。
// 新版只有一次扣减幂等键抢占（旧版还有一次首调令牌抢占，已移除）。
func TestNewUserFirstUseIP_MismatchStrictlyPunishes(t *testing.T) {
	svc, locker, penalty, updater, marker := newNewUserFixture()
	createdAt := time.Now().Add(-time.Minute)

	violation := svc.CheckNewUserFirstUseIP(context.Background(), 42, "user", "2.2.2.2", "1.1.1.1", createdAt)
	if violation == nil {
		t.Fatal("mismatched client ip must block the request")
	}
	if updater.status != StatusDisabled || updater.userID != 42 {
		t.Fatalf("user must be disabled, got userID=%d status=%q", updater.userID, updater.status)
	}
	if len(marker.calls) != 1 || marker.calls[0].level != AffiliateRiskLevelSuspect {
		t.Fatalf("must mark real suspect once, got %+v", marker.calls)
	}
	if len(marker.calls[0].reasons) != 1 || marker.calls[0].reasons[0] != newUserIPMismatchReason {
		t.Fatalf("reason mismatch: %+v", marker.calls[0].reasons)
	}

	calls, _, lastKey, _ := locker.snapshot()
	if calls != 1 {
		t.Fatalf("expected only penalty claim (no first-use token), got %d", calls)
	}
	if lastKey != ipConsistencyNewUserPenaltyKeyPrefix+"42" {
		t.Fatalf("penalty key = %q, want %q", lastKey, ipConsistencyNewUserPenaltyKeyPrefix+"42")
	}
	penaltyCall := waitPenaltyCall(t, penalty)
	if penaltyCall.floor != DefaultIPBalancePenaltyFloor || !penaltyCall.notify {
		t.Fatalf("penalty contract mismatch: %+v", penaltyCall)
	}
}

// TestNewUserFirstUseIP_MatchingIPDoesNotBlockLaterMismatch 是新旧语义的核心分水岭：
// 旧版首调 IP 一致就消费令牌，之后换 IP 不追罚；新版 IP 一致不占用任何键，
// 窗口内后续一旦出现不一致仍必须命中（用户拍板 2026-11-20）。
func TestNewUserFirstUseIP_MatchingIPDoesNotBlockLaterMismatch(t *testing.T) {
	svc, locker, _, updater, marker := newNewUserFixture()
	createdAt := time.Now().Add(-time.Minute)

	// 第一次：IP 一致，放行且不占用幂等键
	if got := svc.CheckNewUserFirstUseIP(context.Background(), 42, "user", "1.1.1.1", "1.1.1.1", createdAt); got != nil {
		t.Fatalf("matching ip must pass, got %+v", got)
	}
	calls, _, _, _ := locker.snapshot()
	if calls != 0 {
		t.Fatalf("matching ip must not consume penalty key, got %d claims", calls)
	}
	if updater.status != "" || len(marker.calls) != 0 {
		t.Fatalf("matching ip must not trigger punishment, got status=%q marks=%+v", updater.status, marker.calls)
	}

	// 第二次：同一窗口内换成不同 IP，必须命中
	if got := svc.CheckNewUserFirstUseIP(context.Background(), 42, "user", "9.9.9.9", "1.1.1.1", createdAt); got == nil {
		t.Fatalf("later mismatch within window must be punished under new semantics")
	}
	if updater.status != StatusDisabled || len(marker.calls) != 1 {
		t.Fatalf("later mismatch must disable + mark, got status=%q marks=%+v", updater.status, marker.calls)
	}
}

// TestNewUserFirstUseIP_WindowBoundary 锁死 3 分钟窗口：2 分钟内命中，超过 3 分钟放行。
func TestNewUserFirstUseIP_WindowBoundary(t *testing.T) {
	tests := []struct {
		name      string
		createdAt time.Time
		wantHit   bool
	}{
		{"2 minutes ago hits", time.Now().Add(-2 * time.Minute), true},
		{"4 minutes ago passes", time.Now().Add(-4 * time.Minute), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _, updater, _ := newNewUserFixture()
			got := svc.CheckNewUserFirstUseIP(context.Background(), 42, "user", "2.2.2.2", "1.1.1.1", tc.createdAt)
			if tc.wantHit && got == nil {
				t.Fatal("within 3-minute window must hit")
			}
			if !tc.wantHit && got != nil {
				t.Fatalf("outside 3-minute window must pass, got %+v", got)
			}
			if tc.wantHit != (updater.status == StatusDisabled) {
				t.Fatalf("disable state mismatch: wantHit=%t status=%q", tc.wantHit, updater.status)
			}
		})
	}
}

// TestNewUserFirstUseIP_PenaltyKeyCooldownStopsDuplicateDeduction 验证扣减幂等：
// 冷却期内（penalty key 已被占用）再次命中不会重复扣钱，但禁用/标记仍照常（幂等无害）。
func TestNewUserFirstUseIP_PenaltyKeyCooldownStopsDuplicateDeduction(t *testing.T) {
	svc, locker, _, updater, marker := newNewUserFixture()
	locker.claimSet = true
	locker.claimResult = false // 扣减幂等键已被占用（24h 冷却内）

	got := svc.CheckNewUserFirstUseIP(context.Background(), 42, "user", "2.2.2.2", "1.1.1.1", time.Now().Add(-time.Minute))
	if got == nil {
		t.Fatal("mismatch must still hit even when penalty is on cooldown")
	}
	if updater.status != StatusDisabled || len(marker.calls) != 1 {
		t.Fatalf("disable + mark must still happen, got status=%q marks=%+v", updater.status, marker.calls)
	}
	calls, _, _, _ := locker.snapshot()
	if calls != 1 {
		t.Fatalf("only penalty claim expected, got %d", calls)
	}
}

func TestNewUserFirstUseIP_OutsideWindowAndMissingFieldsIgnored(t *testing.T) {
	cases := []struct {
		name      string
		createdAt time.Time
		clientIP  string
		register  string
	}{
		{name: "older than 3 minutes", createdAt: time.Now().Add(-4 * time.Minute), clientIP: "2.2.2.2", register: "1.1.1.1"},
		{name: "future timestamp", createdAt: time.Now().Add(time.Minute), clientIP: "2.2.2.2", register: "1.1.1.1"},
		{name: "zero timestamp", clientIP: "2.2.2.2", register: "1.1.1.1"},
		{name: "missing client ip", createdAt: time.Now().Add(-time.Minute), register: "1.1.1.1"},
		{name: "missing register ip", createdAt: time.Now().Add(-time.Minute), clientIP: "2.2.2.2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, locker, _, updater, marker := newNewUserFixture()
			if got := svc.CheckNewUserFirstUseIP(context.Background(), 42, "user", tc.clientIP, tc.register, tc.createdAt); got != nil {
				t.Fatalf("must fail-open, got %+v", got)
			}
			calls, _, _, _ := locker.snapshot()
			if calls != 0 || updater.status != "" || len(marker.calls) != 0 {
				t.Fatalf("no side effects expected, calls=%d status=%q marks=%+v", calls, updater.status, marker.calls)
			}
		})
	}
}

func TestNewUserFirstUseIP_ExemptionsAndWhitelistIgnored(t *testing.T) {
	t.Run("ticket and review exemption", func(t *testing.T) {
		svc, locker, _, updater, marker := newNewUserFixture()
		svc.SetRiskExemptionChecker(&fakeExemptionCheck{exempt: map[int64]bool{42: true}})
		got := svc.CheckNewUserFirstUseIP(context.Background(), 42, "user", "2.2.2.2", "1.1.1.1", time.Now().Add(-time.Minute))
		if got != nil || updater.status != "" || len(marker.calls) != 0 {
			t.Fatalf("exempt user must be skipped, got %+v", got)
		}
		calls, _, _, _ := locker.snapshot()
		if calls != 0 {
			t.Fatalf("exempt user must not consume penalty key, got %d claims", calls)
		}
	})

	t.Run("whitelisted user", func(t *testing.T) {
		svc, locker, _, _, _ := newNewUserFixture()
		settings := penaltySettings()
		settings.settings.WhitelistUserIDs = []int64{42}
		svc.settingsProvider = settings
		got := svc.CheckNewUserFirstUseIP(context.Background(), 42, "user", "2.2.2.2", "1.1.1.1", time.Now().Add(-time.Minute))
		if got != nil {
			t.Fatalf("whitelisted user must be skipped, got %+v", got)
		}
		calls, _, _, _ := locker.snapshot()
		if calls != 0 {
			t.Fatalf("whitelisted user must not consume penalty key, got %d claims", calls)
		}
	})

	t.Run("whitelisted ip", func(t *testing.T) {
		svc, locker, _, _, _ := newNewUserFixture()
		settings := penaltySettings()
		settings.settings.WhitelistIPs = []string{"2.2.2.0/24"}
		svc.settingsProvider = settings
		got := svc.CheckNewUserFirstUseIP(context.Background(), 42, "user", "2.2.2.2", "1.1.1.1", time.Now().Add(-time.Minute))
		if got != nil {
			t.Fatalf("whitelisted ip must be skipped, got %+v", got)
		}
		calls, _, _, _ := locker.snapshot()
		if calls != 0 {
			t.Fatalf("whitelisted ip must not consume penalty key, got %d claims", calls)
		}
	})

	t.Run("admin", func(t *testing.T) {
		svc, locker, _, _, _ := newNewUserFixture()
		got := svc.CheckNewUserFirstUseIP(context.Background(), 42, RoleAdmin, "2.2.2.2", "1.1.1.1", time.Now().Add(-time.Minute))
		if got != nil {
			t.Fatalf("admin must be skipped, got %+v", got)
		}
		calls, _, _, _ := locker.snapshot()
		if calls != 0 {
			t.Fatalf("admin must not consume penalty key, got %d claims", calls)
		}
	})
}

func TestNewUserFirstUseIP_DeductsEvenWhenFeatureBTurnedOff(t *testing.T) {
	svc, locker, penalty, _, _ := newNewUserFixture()
	settings := penaltySettings()
	settings.settings.BalancePenaltyEnabled = false
	svc.settingsProvider = settings

	got := svc.CheckNewUserFirstUseIP(context.Background(), 42, "user", "2.2.2.2", "1.1.1.1", time.Now().Add(-time.Minute))
	if got == nil {
		t.Fatal("strict rule must still fire when feature A is enabled")
	}
	waitPenaltyCall(t, penalty)
	calls, _, _, _ := locker.snapshot()
	if calls != 1 {
		t.Fatalf("strict rule must deduct independently of feature B, got %d claims", calls)
	}
}

// TestNewUserFirstUseIP_LockErrorFailsOpen 验证幂等锁不可用时：禁用/标记照常（不依赖 Redis），
// 只跳过扣钱（扣钱依赖锁做幂等）。整条判定不再因锁故障而完全放行——这是安全优先的正确行为。
func TestNewUserFirstUseIP_LockErrorFailsOpen(t *testing.T) {
	svc, locker, _, updater, marker := newNewUserFixture()
	locker.claimErr = errors.New("redis down")

	got := svc.CheckNewUserFirstUseIP(context.Background(), 42, "user", "2.2.2.2", "1.1.1.1", time.Now().Add(-time.Minute))
	if got == nil {
		t.Fatal("mismatch must still hit when penalty lock unavailable")
	}
	if updater.status != StatusDisabled || len(marker.calls) != 1 {
		t.Fatalf("disable + mark must still happen, got status=%q marks=%+v", updater.status, marker.calls)
	}
}
