package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// ---------------- 处罚调度（IPConsistencyService）测试 ----------------

type fakePenaltyLocker struct {
	mu            sync.Mutex
	claimResult   bool
	claimErr      error
	claimCalls    int
	releaseCalls  int
	lastKey       string
	lastTTL       time.Duration
	claimSet      bool
	returnOnCalls map[int]bool // 第 N 次调用的返回值，用于模拟「3 IP 抢到、4 IP 没抢到」
}

func (f *fakePenaltyLocker) TryClaimPenalty(_ context.Context, key string, ttl time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claimCalls++
	f.lastKey = key
	f.lastTTL = ttl
	if f.claimErr != nil {
		return false, f.claimErr
	}
	if f.returnOnCalls != nil {
		if v, ok := f.returnOnCalls[f.claimCalls]; ok {
			return v, nil
		}
	}
	if f.claimSet {
		return f.claimResult, nil
	}
	return true, nil // 默认抢到
}

func (f *fakePenaltyLocker) ReleasePenalty(_ context.Context, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.releaseCalls++
	return nil
}

func (f *fakePenaltyLocker) snapshot() (int, int, string, time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.claimCalls, f.releaseCalls, f.lastKey, f.lastTTL
}

type penaltyCall struct {
	userID int64
	floor  float64
	notes  string
	notify bool
}

type fakePenalty struct {
	mu      sync.Mutex
	calls   []penaltyCall
	err     error
	started chan struct{}
}

func (f *fakePenalty) ApplyIPBalancePenalty(_ context.Context, userID int64, floor float64, notes string, notify bool) error {
	f.mu.Lock()
	f.calls = append(f.calls, penaltyCall{userID: userID, floor: floor, notes: notes, notify: notify})
	f.mu.Unlock()
	if f.started != nil {
		select {
		case f.started <- struct{}{}:
		default:
		}
	}
	return f.err
}

func (f *fakePenalty) snapshot() []penaltyCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]penaltyCall, len(f.calls))
	copy(out, f.calls)
	return out
}

func penaltySettings() *fakeIPConsistencySettings {
	s := DefaultIPConsistencySettings()
	s.WindowSeconds = 600
	return &fakeIPConsistencySettings{settings: *s}
}

func newPenaltyFixture(ips []string) (*IPConsistencyService, *fakeIPConsistencyCache, *fakePenaltyLocker, *fakePenalty) {
	cache := &fakeIPConsistencyCache{ips: ips}
	locker := &fakePenaltyLocker{}
	penalty := &fakePenalty{started: make(chan struct{}, 4)}
	svc := NewIPConsistencyService(cache, penaltySettings(), &fakeUserStatusUpdater{})
	svc.SetPenaltyService(penalty, locker)
	return svc, cache, locker, penalty
}

func waitPenaltyCall(t *testing.T, p *fakePenalty) penaltyCall {
	t.Helper()
	select {
	case <-p.started:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for async penalty call")
	}
	calls := p.snapshot()
	if len(calls) == 0 {
		t.Fatal("penalty call recorded as started but no call payload")
	}
	return calls[0]
}

func TestIPConsistency_ThreeIPsSchedulesPenaltyWithContract(t *testing.T) {
	svc, _, locker, penalty := newPenaltyFixture([]string{"1.1.1.1", "2.2.2.2", "3.3.3.3"})

	if got := svc.CheckUser(context.Background(), 42, "user", "3.3.3.3"); got != nil {
		t.Fatalf("three ips must not block the request, got %+v", got)
	}
	calls, _, key, ttl := locker.snapshot()
	if calls != 1 {
		t.Fatalf("expected exactly one penalty claim, got %d", calls)
	}
	if key != "ipguard:penalty:42" {
		t.Fatalf("unexpected penalty lock key: %q", key)
	}
	if ttl != 24*time.Hour {
		t.Fatalf("unexpected penalty cooldown: %s", ttl)
	}

	call := waitPenaltyCall(t, penalty)
	if call.userID != 42 || call.floor != 0.03 {
		t.Fatalf("unexpected penalty payload: %+v", call)
	}
	if IPAbnormalityMessage != "The system detected an abnormality in your IP." {
		t.Fatalf("the agreed penalty message changed unexpectedly: %q", IPAbnormalityMessage)
	}
	if call.notes != IPAbnormalityMessage {
		t.Fatalf("penalty notes must use the agreed English message, got %q", call.notes)
	}
	if !call.notify {
		t.Fatal("notification email should be enabled by default")
	}
}

func TestIPConsistency_FourIPsDisablesButNeverDoublePenalizes(t *testing.T) {
	svc, _, locker, penalty := newPenaltyFixture([]string{"1.1.1.1", "2.2.2.2", "3.3.3.3", "4.4.4.4"})
	locker.returnOnCalls = map[int]bool{1: false} // 冷却期内已处罚过

	violation := svc.CheckUser(context.Background(), 42, "user", "4.4.4.4")
	if violation == nil {
		t.Fatal("expected a violation at 4 ips")
	}
	if calls, _, _, _ := locker.snapshot(); calls != 1 {
		t.Fatalf("expected one claim attempt, got %d", calls)
	}
	// 没抢到锁就不应启动异步处罚；给一点时间让可能的 goroutine 暴露出来。
	select {
	case <-penalty.started:
		t.Fatal("penalty must not run when the cooldown lock was not claimed")
	case <-time.After(60 * time.Millisecond):
	}
}

func TestIPConsistency_TwoIPsNeverSchedulesPenalty(t *testing.T) {
	svc, _, locker, penalty := newPenaltyFixture([]string{"1.1.1.1", "2.2.2.2"})

	if got := svc.CheckUser(context.Background(), 42, "user", "2.2.2.2"); got != nil {
		t.Fatalf("two ips must not block, got %+v", got)
	}
	if calls, _, _, _ := locker.snapshot(); calls != 0 {
		t.Fatalf("penalty lock must not be touched below 3 ips, got %d claims", calls)
	}
	if len(penalty.snapshot()) != 0 {
		t.Fatal("penalty must not run below 3 ips")
	}
}

func TestIPConsistency_PenaltyLockFailureFailsOpen(t *testing.T) {
	svc, _, locker, penalty := newPenaltyFixture([]string{"1.1.1.1", "2.2.2.2", "3.3.3.3"})
	locker.claimErr = errors.New("redis down")

	if got := svc.CheckUser(context.Background(), 42, "user", "3.3.3.3"); got != nil {
		t.Fatalf("lock failure must never block the request, got %+v", got)
	}
	select {
	case <-penalty.started:
		t.Fatal("penalty must not run when the lock failed")
	case <-time.After(60 * time.Millisecond):
	}
}

func TestIPConsistency_PenaltyFailureReleasesLock(t *testing.T) {
	svc, _, locker, penalty := newPenaltyFixture([]string{"1.1.1.1", "2.2.2.2", "3.3.3.3"})
	penalty.err = errors.New("db down")

	svc.CheckUser(context.Background(), 42, "user", "3.3.3.3")
	waitPenaltyCall(t, penalty)

	deadline := time.After(3 * time.Second)
	for {
		_, releases, _, _ := locker.snapshot()
		if releases > 0 {
			return
		}
		select {
		case <-deadline:
			t.Fatal("lock should be released after a penalty failure so a later request can retry")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestIPConsistency_FeatureBOnlyNeverDisables(t *testing.T) {
	cache := &fakeIPConsistencyCache{ips: []string{"1.1.1.1", "2.2.2.2", "3.3.3.3", "4.4.4.4"}}
	updater := &fakeUserStatusUpdater{}
	settings := DefaultIPConsistencySettings()
	settings.Enabled = false // 功能A 关闭
	settings.WindowSeconds = 600
	svc := NewIPConsistencyService(cache, &fakeIPConsistencySettings{settings: *settings}, updater)
	locker := &fakePenaltyLocker{}
	svc.SetPenaltyService(&fakePenalty{started: make(chan struct{}, 1)}, locker)

	if got := svc.CheckUser(context.Background(), 42, "user", "4.4.4.4"); got != nil {
		t.Fatalf("feature B alone must never disable or block the account, got %+v", got)
	}
	if updater.status != "" || updater.markedUserID != 0 {
		t.Fatalf("feature A is off: no status change or tagging expected, got %q/%d", updater.status, updater.markedUserID)
	}
	if calls, _, _, _ := locker.snapshot(); calls != 1 {
		t.Fatalf("feature B should still claim the penalty lock, got %d", calls)
	}
}

func TestIPConsistency_BothFeaturesOffSkipsCache(t *testing.T) {
	cache := &fakeIPConsistencyCache{ips: []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}
	settings := DefaultIPConsistencySettings()
	settings.Enabled = false
	settings.BalancePenaltyEnabled = false
	svc := NewIPConsistencyService(cache, &fakeIPConsistencySettings{settings: *settings}, &fakeUserStatusUpdater{})

	if got := svc.CheckUser(context.Background(), 42, "user", "3.3.3.3"); got != nil {
		t.Fatalf("both features off should pass through, got %+v", got)
	}
	if cache.lastKey != "" {
		t.Fatal("both features off must not even touch Redis")
	}
}

// ---------------- 设置默认值与 v1→v2 迁移 ----------------

func TestIPConsistencySettings_DefaultsBothFeaturesOn(t *testing.T) {
	s := DefaultIPConsistencySettings()
	if !s.Enabled || !s.BalancePenaltyEnabled || !s.PenaltyNotifyEmail {
		t.Fatalf("both features and email notification must default on: %+v", s)
	}
	if s.BalancePenaltyFloor != 0.03 || s.WindowSeconds != 600 || s.SchemaVersion != 2 {
		t.Fatalf("unexpected defaults: %+v", s)
	}
}

func TestIPConsistencySettings_V1MigratesToBothFeaturesOn(t *testing.T) {
	// 模拟线上存量配置：管理员曾显式关闭功能A。
	v1 := &IPConsistencySettings{
		Enabled:          false,
		WindowSeconds:    900,
		WhitelistUserIDs: []int64{7},
		WhitelistIPs:     []string{" 10.0.0.0/8 ", "", "203.0.113.9"},
	}
	normalizeIPConsistencySettings(v1)

	if !v1.Enabled || !v1.BalancePenaltyEnabled || !v1.PenaltyNotifyEmail {
		t.Fatalf("v1 must migrate to both features on, got %+v", v1)
	}
	if v1.SchemaVersion != 2 || v1.BalancePenaltyFloor != 0.03 {
		t.Fatalf("unexpected migrated version/floor: %+v", v1)
	}
	if v1.WindowSeconds != 900 || len(v1.WhitelistUserIDs) != 1 || v1.WhitelistUserIDs[0] != 7 {
		t.Fatalf("migration must preserve window and user whitelist: %+v", v1)
	}
	if len(v1.WhitelistIPs) != 2 || v1.WhitelistIPs[0] != "10.0.0.0/8" || v1.WhitelistIPs[1] != "203.0.113.9" {
		t.Fatalf("migration must preserve and clean IP whitelist: %+v", v1.WhitelistIPs)
	}
}

func TestIPConsistencySettings_V2ExplicitOffIsRespected(t *testing.T) {
	v2 := &IPConsistencySettings{
		SchemaVersion:         2,
		Enabled:               false,
		BalancePenaltyEnabled: false,
		BalancePenaltyFloor:   0.03,
		PenaltyNotifyEmail:    false,
		WindowSeconds:         600,
	}
	normalizeIPConsistencySettings(v2)
	if v2.Enabled || v2.BalancePenaltyEnabled || v2.PenaltyNotifyEmail {
		t.Fatalf("explicit v2 choices must not be overwritten: %+v", v2)
	}
}

func TestIPConsistencySettings_InvalidFloorFallsBack(t *testing.T) {
	for _, floor := range []float64{0, -1, 101} {
		s := &IPConsistencySettings{SchemaVersion: 2, WindowSeconds: 600, BalancePenaltyFloor: floor}
		normalizeIPConsistencySettings(s)
		if s.BalancePenaltyFloor != DefaultIPBalancePenaltyFloor {
			t.Errorf("floor %v should fall back to %v, got %v", floor, DefaultIPBalancePenaltyFloor, s.BalancePenaltyFloor)
		}
	}
}

// ---------------- 处罚执行器 ----------------

type fakePenaltyAdmin struct {
	email    string
	applied  bool
	err      error
	gotNote  string
	gotFloor float64
}

func (f *fakePenaltyAdmin) ApplyIPBalancePenalty(_ context.Context, _ int64, floor float64, notes string) (string, bool, error) {
	f.gotFloor = floor
	f.gotNote = notes
	return f.email, f.applied, f.err
}

type fakePenaltySender struct {
	calls   int
	to      string
	subject string
	body    string
	err     error
}

func (f *fakePenaltySender) SendEmail(_ context.Context, to, subject, body string) error {
	f.calls++
	f.to, f.subject, f.body = to, subject, body
	return f.err
}

func TestIPConsistencyPenalty_SendsAgreedMessageOnlyWhenApplied(t *testing.T) {
	admin := &fakePenaltyAdmin{email: "user@example.com", applied: true}
	sender := &fakePenaltySender{}
	svc := &IPConsistencyPenaltyService{admin: admin, emailService: sender}

	if err := svc.ApplyIPBalancePenalty(context.Background(), 42, 0.03, IPAbnormalityMessage, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sender.calls != 1 || sender.to != "user@example.com" {
		t.Fatalf("expected one email to the user, got %+v", sender)
	}
	if sender.body != "The system detected an abnormality in your IP." {
		t.Fatalf("email body must be the agreed message, got %q", sender.body)
	}
	if admin.gotNote != IPAbnormalityMessage || admin.gotFloor != 0.03 {
		t.Fatalf("penalty contract broken: floor=%v notes=%q", admin.gotFloor, admin.gotNote)
	}
}

func TestIPConsistencyPenalty_NoDeductionNoEmail(t *testing.T) {
	admin := &fakePenaltyAdmin{applied: false}
	sender := &fakePenaltySender{}
	svc := &IPConsistencyPenaltyService{admin: admin, emailService: sender}

	if err := svc.ApplyIPBalancePenalty(context.Background(), 42, 0.03, IPAbnormalityMessage, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sender.calls != 0 {
		t.Fatal("must not email a user whose balance was already at or below the floor")
	}
}

func TestIPConsistencyPenalty_EmailFailureDoesNotFail(t *testing.T) {
	admin := &fakePenaltyAdmin{email: "user@example.com", applied: true}
	sender := &fakePenaltySender{err: errors.New("smtp down")}
	svc := &IPConsistencyPenaltyService{admin: admin, emailService: sender}

	// 返回 error 会让上层释放幂等锁、可能重复处罚，因此邮件失败必须吞掉。
	if err := svc.ApplyIPBalancePenalty(context.Background(), 42, 0.03, IPAbnormalityMessage, true); err != nil {
		t.Fatalf("email failure must not be reported as penalty failure: %v", err)
	}
}

func TestIPConsistencyPenalty_NotifyOffOrMissingCapability(t *testing.T) {
	admin := &fakePenaltyAdmin{email: "user@example.com", applied: true}
	sender := &fakePenaltySender{}
	svc := &IPConsistencyPenaltyService{admin: admin, emailService: sender}
	if err := svc.ApplyIPBalancePenalty(context.Background(), 42, 0.03, IPAbnormalityMessage, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sender.calls != 0 {
		t.Fatal("notify=false must not send an email")
	}

	if err := (&IPConsistencyPenaltyService{}).ApplyIPBalancePenalty(context.Background(), 42, 0.03, IPAbnormalityMessage, true); err == nil {
		t.Fatal("missing admin capability must fail open by returning an error (logged by caller)")
	}

	admin.err = errors.New("db down")
	if err := svc.ApplyIPBalancePenalty(context.Background(), 42, 0.03, IPAbnormalityMessage, true); err == nil {
		t.Fatal("admin failure must propagate so the caller releases the idempotency lock")
	}
}

func TestIPConsistencyPenaltyLocker_ProviderReturnsNilForPlainCache(t *testing.T) {
	if got := ProvideIPConsistencyPenaltyLocker(&fakeIPConsistencyCache{}); got != nil {
		t.Fatalf("plain cache without SETNX must not be treated as a locker, got %T", got)
	}
}

// 轻标记与「待复核冻结」必须严格区分：ip_suspect 只在管理端展示，账号照常调用。
func TestIPConsistency_IPSuspectDoesNotFreezeAccount(t *testing.T) {
	if (&User{RiskLevel: AffiliateRiskLevelIPSuspect}).RiskAccessBlocked() {
		t.Fatal("ip_suspect must NOT trigger the pending-review freeze (key creation / gateway 403)")
	}
	for _, level := range []string{AffiliateRiskLevelSuspect, AffiliateRiskLevelAbnormal} {
		if !(&User{RiskLevel: level}).RiskAccessBlocked() {
			t.Errorf("%q must still be blocked by the pending-review gate", level)
		}
	}
	if (&User{RiskLevel: ""}).RiskAccessBlocked() {
		t.Fatal("normal users must never be blocked")
	}
}

// ---------------- 端到端：存量配置升级后立即生效 ----------------

// ipGuardSettingRepo 是 bare 层自包含的 SettingRepository 桩。
// 不能复用 auth_service_register_test.go 里的 settingRepoStub：那个文件带
// //go:build unit，bare 测试编译时不可见（本项目约定新增测试不加 build tag）。
type ipGuardSettingRepo struct {
	values map[string]string
}

var _ SettingRepository = (*ipGuardSettingRepo)(nil)

func (r *ipGuardSettingRepo) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (r *ipGuardSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	if v, ok := r.values[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}

func (r *ipGuardSettingRepo) Set(_ context.Context, key, value string) error {
	if r.values == nil {
		r.values = map[string]string{}
	}
	r.values[key] = value
	return nil
}

func (r *ipGuardSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if v, ok := r.values[key]; ok {
			out[key] = v
		}
	}
	return out, nil
}

func (r *ipGuardSettingRepo) SetMultiple(ctx context.Context, settings map[string]string) error {
	for key, value := range settings {
		if err := r.Set(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}

func (r *ipGuardSettingRepo) GetAll(context.Context) (map[string]string, error) {
	return r.values, nil
}

func (r *ipGuardSettingRepo) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}

func TestIPConsistencySettingsCached_StoredV1MigratesImmediately(t *testing.T) {
	repo := &ipGuardSettingRepo{values: map[string]string{
		SettingKeyIPConsistencySettings: `{"enabled":false,"window_seconds":900,"whitelist_ips":["10.0.0.0/8"]}`,
	}}
	svc := &SettingService{settingRepo: repo}

	got := svc.GetIPConsistencySettingsCached(context.Background())
	if !got.Enabled || !got.BalancePenaltyEnabled || !got.PenaltyNotifyEmail {
		t.Fatalf("stored v1 config must migrate to both features on at read time: %+v", got)
	}
	if got.SchemaVersion != ipConsistencySettingsVersion || got.BalancePenaltyFloor != 0.03 {
		t.Fatalf("unexpected migrated defaults: %+v", got)
	}
	if got.WindowSeconds != 900 || len(got.WhitelistIPs) != 1 || got.WhitelistIPs[0] != "10.0.0.0/8" {
		t.Fatalf("migration must preserve window and IP whitelist: %+v", got)
	}
}

func TestIPConsistencySettingsCached_MissingKeyUsesV2Defaults(t *testing.T) {
	svc := &SettingService{settingRepo: &ipGuardSettingRepo{values: map[string]string{}}}
	got := svc.GetIPConsistencySettingsCached(context.Background())
	if !got.Enabled || !got.BalancePenaltyEnabled || !got.PenaltyNotifyEmail || got.BalancePenaltyFloor != 0.03 {
		t.Fatalf("missing setting must use v2 defaults (both features on): %+v", got)
	}
}

func TestIPConsistencySettings_WriteRespectsExplicitOff(t *testing.T) {
	repo := &ipGuardSettingRepo{values: map[string]string{}}
	svc := &SettingService{settingRepo: repo}

	if err := svc.SetIPConsistencySettings(context.Background(), &IPConsistencySettings{
		Enabled:               false,
		BalancePenaltyEnabled: false,
		PenaltyNotifyEmail:    false,
		WindowSeconds:         600,
		BalancePenaltyFloor:   0.03,
	}); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	got, err := svc.GetIPConsistencySettings(context.Background())
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if got.Enabled || got.BalancePenaltyEnabled || got.PenaltyNotifyEmail {
		t.Fatalf("explicit v2 off must be persisted, not overwritten by v1 migration: %+v", got)
	}
}
