package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeIPConsistencyCache struct {
	ips        []string
	err        error
	clearedKey string
	lastKey    string
	lastIP     string
	lastWindow time.Duration
}

func (f *fakeIPConsistencyCache) RecordAndListIPs(
	_ context.Context, key, clientIP string, _ time.Time, window time.Duration,
) ([]string, error) {
	f.lastKey = key
	f.lastIP = clientIP
	f.lastWindow = window
	return f.ips, f.err
}

func (f *fakeIPConsistencyCache) Clear(_ context.Context, key string) error {
	f.clearedKey = key
	return nil
}

type fakeIPConsistencySettings struct {
	settings IPConsistencySettings
}

func (f *fakeIPConsistencySettings) GetIPConsistencySettingsCached(context.Context) IPConsistencySettings {
	return f.settings
}

type fakeUserStatusUpdater struct {
	userID       int64
	status       string
	markedUserID int64
	markReason   string
	err          error
}

func (f *fakeUserStatusUpdater) UpdateStatus(_ context.Context, userID int64, status string) error {
	f.userID = userID
	f.status = status
	return f.err
}

func (f *fakeUserStatusUpdater) MarkIPInconsistent(_ context.Context, userID int64, reason string) {
	f.markedUserID = userID
	f.markReason = reason
}

func enabledSettings(windowSeconds int) *fakeIPConsistencySettings {
	return &fakeIPConsistencySettings{
		settings: IPConsistencySettings{Enabled: true, WindowSeconds: windowSeconds},
	}
}

func TestIPConsistency_DisabledSkipsCheck(t *testing.T) {
	cache := &fakeIPConsistencyCache{ips: []string{"1.1.1.1", "2.2.2.2"}}
	updater := &fakeUserStatusUpdater{}
	svc := NewIPConsistencyService(cache, &fakeIPConsistencySettings{
		settings: IPConsistencySettings{Enabled: false},
	}, updater)

	if got := svc.CheckUser(context.Background(), 1, "user", "1.1.1.1"); got != nil {
		t.Fatalf("expected no violation when disabled, got %+v", got)
	}
	if updater.status != "" {
		t.Fatalf("user should not be disabled when feature is off")
	}
}

func TestIPConsistency_SingleIPAllowed(t *testing.T) {
	cache := &fakeIPConsistencyCache{ips: []string{"1.1.1.1"}}
	updater := &fakeUserStatusUpdater{}
	svc := NewIPConsistencyService(cache, enabledSettings(600), updater)

	if got := svc.CheckUser(context.Background(), 7, "user", "1.1.1.1"); got != nil {
		t.Fatalf("expected no violation for a single ip, got %+v", got)
	}
	if cache.lastKey != "ipguard:user:7" {
		t.Fatalf("unexpected cache key: %s", cache.lastKey)
	}
	if cache.lastWindow != 600*time.Second {
		t.Fatalf("unexpected window: %s", cache.lastWindow)
	}
	if updater.status != "" {
		t.Fatalf("user should not be disabled for a single ip")
	}
}

func TestIPConsistency_TwoIPsMarksSuspicious(t *testing.T) {
	cache := &fakeIPConsistencyCache{ips: []string{"1.1.1.1", "2.2.2.2"}}
	updater := &fakeUserStatusUpdater{}
	svc := NewIPConsistencyService(cache, enabledSettings(600), updater)

	if got := svc.CheckUser(context.Background(), 42, "user", "2.2.2.2"); got != nil {
		t.Fatalf("two ips should not block the request, got %+v", got)
	}
	if updater.userID != 0 || updater.status != "" {
		t.Fatalf("user should not be disabled with two ips, got %d/%s", updater.userID, updater.status)
	}
	if updater.markedUserID != 42 || updater.markReason != ipInconsistencyReason {
		t.Fatalf("expected user 42 marked suspicious, got %d/%q", updater.markedUserID, updater.markReason)
	}
	if cache.clearedKey != "" {
		t.Fatalf("window should be kept for suspicious users, cleared %q", cache.clearedKey)
	}
}

func TestIPConsistency_ThreeIPsDisableUser(t *testing.T) {
	cache := &fakeIPConsistencyCache{ips: []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}
	updater := &fakeUserStatusUpdater{}
	svc := NewIPConsistencyService(cache, enabledSettings(600), updater)

	violation := svc.CheckUser(context.Background(), 42, "user", "3.3.3.3")
	if violation == nil {
		t.Fatal("expected a violation")
	}
	if violation.UserID != 42 || violation.CurrentIP != "3.3.3.3" {
		t.Fatalf("unexpected violation: %+v", violation)
	}
	if updater.userID != 42 || updater.status != StatusDisabled {
		t.Fatalf("expected user 42 to be disabled, got %d/%s", updater.userID, updater.status)
	}
	if cache.clearedKey != "ipguard:user:42" {
		t.Fatalf("expected window to be cleared, got %q", cache.clearedKey)
	}
}

func TestIPConsistency_AdminExempt(t *testing.T) {
	cache := &fakeIPConsistencyCache{ips: []string{"1.1.1.1", "2.2.2.2"}}
	updater := &fakeUserStatusUpdater{}
	svc := NewIPConsistencyService(cache, enabledSettings(600), updater)

	if got := svc.CheckUser(context.Background(), 1, RoleAdmin, "2.2.2.2"); got != nil {
		t.Fatalf("admin should be exempt, got %+v", got)
	}
	if cache.lastKey != "" {
		t.Fatal("admin requests should not touch the cache")
	}
}

func TestIPConsistency_WhitelistUserAndIP(t *testing.T) {
	cache := &fakeIPConsistencyCache{ips: []string{"1.1.1.1", "2.2.2.2"}}
	updater := &fakeUserStatusUpdater{}
	settings := &fakeIPConsistencySettings{
		settings: IPConsistencySettings{
			Enabled:          true,
			WindowSeconds:    600,
			WhitelistUserIDs: []int64{9},
			WhitelistIPs:     []string{"10.0.0.0/8"},
		},
	}
	svc := NewIPConsistencyService(cache, settings, updater)

	if got := svc.CheckUser(context.Background(), 9, "user", "2.2.2.2"); got != nil {
		t.Fatalf("whitelisted user should be exempt, got %+v", got)
	}
	cache.lastKey = ""
	if got := svc.CheckUser(context.Background(), 10, "user", "10.1.2.3"); got != nil {
		t.Fatalf("whitelisted ip should be exempt, got %+v", got)
	}
	if cache.lastKey != "" {
		t.Fatal("whitelisted ip should not touch the cache")
	}
	if updater.status != "" {
		t.Fatal("no user should be disabled")
	}
}

func TestIPConsistency_CacheErrorFailsOpen(t *testing.T) {
	cache := &fakeIPConsistencyCache{err: errors.New("redis down")}
	updater := &fakeUserStatusUpdater{}
	svc := NewIPConsistencyService(cache, enabledSettings(600), updater)

	if got := svc.CheckUser(context.Background(), 1, "user", "1.1.1.1"); got != nil {
		t.Fatalf("expected fail-open on cache error, got %+v", got)
	}
	if updater.status != "" {
		t.Fatal("user should not be disabled when cache fails")
	}
}

func TestIPConsistency_RegistrationMismatchRejected(t *testing.T) {
	cache := &fakeIPConsistencyCache{ips: []string{"1.1.1.1", "2.2.2.2"}}
	svc := NewIPConsistencyService(cache, enabledSettings(600), &fakeUserStatusUpdater{})

	err := svc.CheckRegistration(context.Background(), "USER@example.com", "2.2.2.2")
	if !errors.Is(err, ErrIPConsistencyBlocked) {
		t.Fatalf("expected ErrIPConsistencyBlocked, got %v", err)
	}
	if cache.lastKey == "" || cache.lastKey == "ipguard:register:2.2.2.2" {
		t.Fatalf("expected hashed registration key, got %q", cache.lastKey)
	}
}

func TestIPConsistency_RegistrationSameIPAllowed(t *testing.T) {
	cache := &fakeIPConsistencyCache{ips: []string{"1.1.1.1"}}
	svc := NewIPConsistencyService(cache, enabledSettings(600), &fakeUserStatusUpdater{})

	if err := svc.CheckRegistration(context.Background(), "user@example.com", "1.1.1.1"); err != nil {
		t.Fatalf("expected registration to pass, got %v", err)
	}
}

func TestIPConsistency_RecordUserIPSeedsWindow(t *testing.T) {
	cache := &fakeIPConsistencyCache{ips: []string{"1.1.1.1"}}
	svc := NewIPConsistencyService(cache, enabledSettings(600), &fakeUserStatusUpdater{})

	svc.RecordUserIP(context.Background(), 5, "1.1.1.1")
	if cache.lastKey != "ipguard:user:5" || cache.lastIP != "1.1.1.1" {
		t.Fatalf("unexpected record call: key=%q ip=%q", cache.lastKey, cache.lastIP)
	}
}

func TestNormalizeClientIP(t *testing.T) {
	cases := map[string]string{
		"  1.2.3.4 ":     "1.2.3.4",
		"::ffff:1.2.3.4": "1.2.3.4",
		"2001:db8::1":    "2001:db8::1",
		"":               "",
		"not-an-ip":      "not-an-ip",
	}
	for input, want := range cases {
		if got := normalizeClientIP(input); got != want {
			t.Errorf("normalizeClientIP(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestIPWhitelisted(t *testing.T) {
	whitelist := []string{"203.0.113.9", "198.51.100.0/24", "2001:db8::/32"}
	cases := map[string]bool{
		"203.0.113.9":   true,
		"198.51.100.42": true,
		"203.0.113.10":  false,
		"2001:db8::1":   true,
		"2001:dead::1":  false,
	}
	for ip, want := range cases {
		if got := ipWhitelisted(ip, whitelist); got != want {
			t.Errorf("ipWhitelisted(%q) = %v, want %v", ip, got, want)
		}
	}
}
