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

func TestNewUserFirstUseIP_MismatchStrictlyPunishes(t *testing.T) {
	svc, locker, penalty, updater, marker := newNewUserFixture()
	createdAt := time.Now().Add(-time.Minute)

	violation := svc.CheckNewUserFirstUseIP(context.Background(), 42, "user", "2.2.2.2", "1.1.1.1", createdAt)
	if violation == nil {
		t.Fatal("mismatched first-use ip must block the request")
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
	if calls != 2 {
		t.Fatalf("expected token claim + penalty claim, got %d", calls)
	}
	if lastKey != ipConsistencyNewUserPenaltyKeyPrefix+"42" {
		t.Fatalf("penalty key = %q, want %q", lastKey, ipConsistencyNewUserPenaltyKeyPrefix+"42")
	}
	penaltyCall := waitPenaltyCall(t, penalty)
	if penaltyCall.floor != DefaultIPBalancePenaltyFloor || !penaltyCall.notify {
		t.Fatalf("penalty contract mismatch: %+v", penaltyCall)
	}
}

func TestNewUserFirstUseIP_MatchingIPConsumesTokenForever(t *testing.T) {
	svc, locker, _, updater, marker := newNewUserFixture()
	createdAt := time.Now().Add(-time.Minute)
	// 模拟真实 SETNX：第二次抢占（换 IP 的请求）拿不到令牌。
	locker.returnOnCalls = map[int]bool{2: false}

	if got := svc.CheckNewUserFirstUseIP(context.Background(), 42, "user", "1.1.1.1", "1.1.1.1", createdAt); got != nil {
		t.Fatalf("matching first-use ip must pass, got %+v", got)
	}
	if got := svc.CheckNewUserFirstUseIP(context.Background(), 42, "user", "9.9.9.9", "1.1.1.1", createdAt); got != nil {
		t.Fatalf("later mismatch within window must not be punished (not first use), got %+v", got)
	}
	if updater.status != "" || len(marker.calls) != 0 {
		t.Fatalf("no punishment expected, got status=%q marks=%+v", updater.status, marker.calls)
	}
	calls, _, key, _ := locker.snapshot()
	if calls != 2 || key != ipConsistencyNewUserKeyPrefix+"42" {
		t.Fatalf("only token claims expected, calls=%d key=%q", calls, key)
	}
}

func TestNewUserFirstUseIP_SecondMismatchDoesNotRepeatPunishment(t *testing.T) {
	svc, locker, _, updater, marker := newNewUserFixture()
	createdAt := time.Now().Add(-time.Minute)
	locker.claimSet = true
	locker.claimResult = false // 令牌已被首调消费

	if got := svc.CheckNewUserFirstUseIP(context.Background(), 42, "user", "2.2.2.2", "1.1.1.1", createdAt); got != nil {
		t.Fatalf("already-handled first use must pass, got %+v", got)
	}
	if updater.status != "" || len(marker.calls) != 0 {
		t.Fatalf("no duplicate punishment expected, got status=%q marks=%+v", updater.status, marker.calls)
	}
	calls, _, key, _ := locker.snapshot()
	if calls != 1 || key != ipConsistencyNewUserKeyPrefix+"42" {
		t.Fatalf("only token claim expected, calls=%d key=%q", calls, key)
	}
}

func TestNewUserFirstUseIP_OutsideWindowAndMissingFieldsIgnored(t *testing.T) {
	cases := []struct {
		name      string
		createdAt time.Time
		clientIP  string
		register  string
	}{
		{name: "older than 10 minutes", createdAt: time.Now().Add(-11 * time.Minute), clientIP: "2.2.2.2", register: "1.1.1.1"},
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
			t.Fatalf("exempt user must not consume token, got %d claims", calls)
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
			t.Fatalf("whitelisted user must not consume token, got %d claims", calls)
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
			t.Fatalf("whitelisted ip must not consume token, got %d claims", calls)
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
			t.Fatalf("admin must not consume token, got %d claims", calls)
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
	if calls != 2 {
		t.Fatalf("strict rule must deduct independently of feature B, got %d claims", calls)
	}
}

func TestNewUserFirstUseIP_LockErrorFailsOpen(t *testing.T) {
	svc, locker, _, updater, marker := newNewUserFixture()
	locker.claimErr = errors.New("redis down")

	got := svc.CheckNewUserFirstUseIP(context.Background(), 42, "user", "2.2.2.2", "1.1.1.1", time.Now().Add(-time.Minute))
	if got != nil || updater.status != "" || len(marker.calls) != 0 {
		t.Fatalf("redis failure must fail open, got %+v", got)
	}
}
