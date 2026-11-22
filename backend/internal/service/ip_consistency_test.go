package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeIPConsistencyCache struct {
	ips        []string
	longIPs    []string // 长窗口独立返回值；nil 时复用 ips
	err        error
	clearedKey string
	lastKey    string
	lastIP     string
	lastWindow time.Duration
	keys       []string
}

func (f *fakeIPConsistencyCache) RecordAndListIPs(
	_ context.Context, key, clientIP string, _ time.Time, window time.Duration,
) ([]string, error) {
	f.lastKey = key
	f.lastIP = clientIP
	f.lastWindow = window
	f.keys = append(f.keys, key)
	if strings.HasPrefix(key, ipConsistencyUserLongKeyPrefix) && f.longIPs != nil {
		return f.longIPs, f.err
	}
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

func enabledSettings(windowSeconds int) *fakeIPConsistencySettings {
	return &fakeIPConsistencySettings{
		settings: IPConsistencySettings{
			Enabled:           true,
			WindowSeconds:     windowSeconds,
			LongWindowSeconds: defaultIPConsistencyLongWindowSeconds,
		},
	}
}

func TestIPConsistency_DisabledSkipsCheck(t *testing.T) {
	cache := &fakeIPConsistencyCache{ips: []string{"1.1.1.1", "2.2.2.2"}}
	svc := NewIPConsistencyService(cache, &fakeIPConsistencySettings{
		settings: IPConsistencySettings{Enabled: false},
	})

	if got := svc.CheckUser(context.Background(), 1, "user", "1.1.1.1"); got != nil {
		t.Fatalf("expected no violation when disabled, got %+v", got)
	}
}

func TestIPConsistency_SingleIPAllowed(t *testing.T) {
	cache := &fakeIPConsistencyCache{ips: []string{"1.1.1.1"}}
	svc := NewIPConsistencyService(cache, enabledSettings(600))

	if got := svc.CheckUser(context.Background(), 7, "user", "1.1.1.1"); got != nil {
		t.Fatalf("expected no violation for a single ip, got %+v", got)
	}
	if len(cache.keys) != 2 || cache.keys[0] != "ipguard:user:7" || cache.keys[1] != "ipguard:user-long:7" {
		t.Fatalf("both windows must be recorded, got %v", cache.keys)
	}
	if cache.lastKey != "ipguard:user-long:7" || cache.lastWindow != time.Hour {
		t.Fatalf("unexpected long-window record: key=%s window=%s", cache.lastKey, cache.lastWindow)
	}
}

func TestIPConsistency_TwoIPsNoAction(t *testing.T) {
	cache := &fakeIPConsistencyCache{ips: []string{"1.1.1.1", "2.2.2.2"}}
	marker := &fakeNewUserRiskMarker{}
	svc := NewIPConsistencyService(cache, enabledSettings(600))
	svc.SetNewUserRiskMarker(marker)

	if got := svc.CheckUser(context.Background(), 42, "user", "2.2.2.2"); got != nil {
		t.Fatalf("two ips should not block the request, got %+v", got)
	}
	if len(marker.calls) != 0 {
		t.Fatalf("two ips must not tag the user, got %+v", marker.calls)
	}
	if cache.clearedKey != "" {
		t.Fatalf("window should be kept, cleared %q", cache.clearedKey)
	}
}

// 2026-11-21 起 3 个 IP 即触发统一处置：suspect + 扣费 + 邮件 + 中断本次请求（不再只是轻标记放行）。
func TestIPConsistency_ThreeIPsMarksSuspectAndBlocks(t *testing.T) {
	cache := &fakeIPConsistencyCache{ips: []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}
	marker := &fakeNewUserRiskMarker{}
	svc := NewIPConsistencyService(cache, enabledSettings(600))
	svc.SetNewUserRiskMarker(marker)

	violation := svc.CheckUser(context.Background(), 42, "user", "3.3.3.3")
	if violation == nil || violation.UserID != 42 || violation.CurrentIP != "3.3.3.3" {
		t.Fatalf("three ips must block the request, got %+v", violation)
	}
	if len(marker.calls) != 1 || marker.calls[0].userID != 42 ||
		marker.calls[0].level != AffiliateRiskLevelSuspect || marker.calls[0].reasons[0] != ipInconsistencyReason {
		t.Fatalf("expected one real suspect mark, got %+v", marker.calls)
	}
	if cache.clearedKey != "" {
		t.Fatalf("window must not be cleared, got %q", cache.clearedKey)
	}
}

// 独立禁用档已取消：4/5 个 IP 与 3 个 IP 同档处置，窗口不清空，账号不置 disabled。
func TestIPConsistency_FourAndFiveIPsSameDisposition(t *testing.T) {
	for _, ips := range [][]string{
		{"1.1.1.1", "2.2.2.2", "3.3.3.3", "4.4.4.4"},
		{"1.1.1.1", "2.2.2.2", "3.3.3.3", "4.4.4.4", "5.5.5.5"},
	} {
		cache := &fakeIPConsistencyCache{ips: ips}
		marker := &fakeNewUserRiskMarker{}
		svc := NewIPConsistencyService(cache, enabledSettings(600))
		svc.SetNewUserRiskMarker(marker)

		violation := svc.CheckUser(context.Background(), 42, "user", "9.9.9.9")
		if violation == nil {
			t.Fatalf("expected a violation at %d ips", len(ips))
		}
		if len(marker.calls) != 1 || marker.calls[0].level != AffiliateRiskLevelSuspect {
			t.Fatalf("expected one suspect mark, got %+v", marker.calls)
		}
		if cache.clearedKey != "" {
			t.Fatalf("window must not be cleared, got %q", cache.clearedKey)
		}
	}
}

// 长窗口 1 小时内累计 3 个 IP，即使 10 分钟主窗口只有 1 个也必须命中。
func TestIPConsistency_LongWindowDetectsSlowRotation(t *testing.T) {
	cache := &fakeIPConsistencyCache{ips: []string{"1.1.1.1"}, longIPs: []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}
	marker := &fakeNewUserRiskMarker{}
	svc := NewIPConsistencyService(cache, enabledSettings(600))
	svc.SetNewUserRiskMarker(marker)

	violation := svc.CheckUser(context.Background(), 42, "user", "3.3.3.3")
	if violation == nil || len(violation.KnownIPs) != 3 {
		t.Fatalf("long-window hit must block with long-window ips, got %+v", violation)
	}
	if len(marker.calls) != 1 {
		t.Fatalf("long-window hit must mark suspect, got %+v", marker.calls)
	}
}

// 长窗口关闭（0）时只写主窗口，不做第二组 Redis 往返。
func TestIPConsistency_LongWindowDisabled(t *testing.T) {
	cache := &fakeIPConsistencyCache{ips: []string{"1.1.1.1"}}
	settings := enabledSettings(600)
	settings.settings.LongWindowSeconds = 0
	svc := NewIPConsistencyService(cache, settings)

	if got := svc.CheckUser(context.Background(), 42, "user", "1.1.1.1"); got != nil {
		t.Fatalf("single ip must pass, got %+v", got)
	}
	if len(cache.keys) != 1 || cache.keys[0] != "ipguard:user:42" {
		t.Fatalf("long window disabled must skip its cache call, got %v", cache.keys)
	}
}

func TestIPConsistency_AdminExempt(t *testing.T) {
	cache := &fakeIPConsistencyCache{ips: []string{"1.1.1.1", "2.2.2.2"}}
	svc := NewIPConsistencyService(cache, enabledSettings(600))

	if got := svc.CheckUser(context.Background(), 1, RoleAdmin, "2.2.2.2"); got != nil {
		t.Fatalf("admin should be exempt, got %+v", got)
	}
	if cache.lastKey != "" {
		t.Fatal("admin requests should not touch the cache")
	}
}

func TestIPConsistency_WhitelistUserAndIP(t *testing.T) {
	cache := &fakeIPConsistencyCache{ips: []string{"1.1.1.1", "2.2.2.2"}}
	settings := &fakeIPConsistencySettings{
		settings: IPConsistencySettings{
			Enabled:           true,
			WindowSeconds:     600,
			LongWindowSeconds: defaultIPConsistencyLongWindowSeconds,
			WhitelistUserIDs:  []int64{9},
			WhitelistIPs:      []string{"10.0.0.0/8"},
		},
	}
	svc := NewIPConsistencyService(cache, settings)

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
}

func TestIPConsistency_CacheErrorFailsOpen(t *testing.T) {
	cache := &fakeIPConsistencyCache{err: errors.New("redis down")}
	svc := NewIPConsistencyService(cache, enabledSettings(600))

	if got := svc.CheckUser(context.Background(), 1, "user", "1.1.1.1"); got != nil {
		t.Fatalf("expected fail-open on cache error, got %+v", got)
	}
}

func TestIPConsistency_RegistrationMismatchRejected(t *testing.T) {
	cache := &fakeIPConsistencyCache{ips: []string{"1.1.1.1", "2.2.2.2"}}
	svc := NewIPConsistencyService(cache, enabledSettings(600))

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
	svc := NewIPConsistencyService(cache, enabledSettings(600))

	if err := svc.CheckRegistration(context.Background(), "user@example.com", "1.1.1.1"); err != nil {
		t.Fatalf("expected registration to pass, got %v", err)
	}
}

func TestIPConsistency_RecordUserIPSeedsBothWindows(t *testing.T) {
	cache := &fakeIPConsistencyCache{ips: []string{"1.1.1.1"}}
	svc := NewIPConsistencyService(cache, enabledSettings(600))

	svc.RecordUserIP(context.Background(), 5, "1.1.1.1")
	if len(cache.keys) != 2 || cache.keys[0] != "ipguard:user:5" || cache.keys[1] != "ipguard:user-long:5" {
		t.Fatalf("registration ip must seed both windows, got %v", cache.keys)
	}
	if cache.lastIP != "1.1.1.1" || cache.lastWindow != time.Hour {
		t.Fatalf("unexpected long-window record: ip=%q window=%s", cache.lastIP, cache.lastWindow)
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
