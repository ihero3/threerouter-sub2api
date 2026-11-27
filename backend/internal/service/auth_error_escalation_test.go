package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ===== 测试桩（bare 层专用，勿加 //go:build unit）=====

type authEscalationRepoStub struct {
	AccountRepository
	setErrorCalls     int
	tempCalls         int
	rateLimitedCalls  int
	lastErrorMsg      string
	lastTempReason    string
	lastErrorID       int64
	lastTempID        int64
	lastRateLimitedID int64
	lastRateLimitedAt time.Time
	lastTempUntil     time.Time
}

func (r *authEscalationRepoStub) SetError(ctx context.Context, id int64, errorMsg string) error {
	r.setErrorCalls++
	r.lastErrorID = id
	r.lastErrorMsg = errorMsg
	return nil
}

func (r *authEscalationRepoStub) SetTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error {
	r.tempCalls++
	r.lastTempID = id
	r.lastTempReason = reason
	r.lastTempUntil = until
	return nil
}

func (r *authEscalationRepoStub) SetRateLimited(ctx context.Context, id int64, resetAt time.Time) error {
	r.rateLimitedCalls++
	r.lastRateLimitedID = id
	r.lastRateLimitedAt = resetAt
	return nil
}

// tempCooldownIsApprox 断言临时冷却时长约等于 want（容忍 1 分钟误差）。
func (r *authEscalationRepoStub) tempCooldownIsApprox(before time.Time, want time.Duration) bool {
	if r.lastTempUntil.IsZero() {
		return false
	}
	got := r.lastTempUntil.Sub(before)
	return got >= want-time.Minute && got <= want+time.Minute
}

type authEscalationCounterStub struct {
	counts     []int64
	increments int
	resets     []int64
}

func (c *authEscalationCounterStub) IncrementAuthErrorCount(ctx context.Context, accountID int64, statusCode int, windowMinutes int) (int64, error) {
	c.increments++
	if len(c.counts) == 0 {
		return 1, nil
	}
	count := c.counts[0]
	c.counts = c.counts[1:]
	return count, nil
}

func (c *authEscalationCounterStub) ResetAuthErrorCounters(ctx context.Context, accountID int64) error {
	c.resets = append(c.resets, accountID)
	return nil
}

// ===== 升级梯度 =====

func TestAuthErrorEscalationCooldownLadder(t *testing.T) {
	require.Equal(t, authErrorEscalationBaseCooldown, authErrorEscalationCooldownForCount(0))
	require.Equal(t, authErrorEscalationBaseCooldown, authErrorEscalationCooldownForCount(1))
	require.Equal(t, authErrorEscalationBaseCooldown, authErrorEscalationCooldownForCount(2))
	require.Equal(t, authErrorEscalationMidCooldown, authErrorEscalationCooldownForCount(3))
	require.Equal(t, authErrorEscalationMidCooldown, authErrorEscalationCooldownForCount(4))
	require.Equal(t, authErrorEscalationHighCooldown, authErrorEscalationCooldownForCount(5))
	require.Equal(t, authErrorEscalationHighCooldown, authErrorEscalationCooldownForCount(99))
}

// ===== handleAuthErrorEscalating =====

func newAuthEscalationService(repo *authEscalationRepoStub, counter *authEscalationCounterStub) *RateLimitService {
	svc := NewRateLimitService(repo, nil, nil, nil, nil)
	if counter != nil {
		svc.SetAuthErrorEscalationCache(counter)
	}
	return svc
}

// 401/403 降级核心不变式：只临时冷却，绝不 SetError。
func TestHandleAuthErrorEscalating_TempInsteadOfSetError(t *testing.T) {
	repo := &authEscalationRepoStub{}
	counter := &authEscalationCounterStub{}
	svc := newAuthEscalationService(repo, counter)
	account := &Account{ID: 900, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	before := time.Now()

	svc.handleAuthErrorEscalating(context.Background(), account, http.StatusUnauthorized, "Authentication failed (401): bad key")

	require.Equal(t, 0, repo.setErrorCalls, "escalating path must never SetError")
	require.Equal(t, 1, repo.tempCalls)
	require.Equal(t, int64(900), repo.lastTempID)
	require.Contains(t, repo.lastTempReason, "bad key")
	require.Equal(t, 1, counter.increments)
	require.True(t, repo.tempCooldownIsApprox(before, authErrorEscalationBaseCooldown),
		"first hit should cool down ~30min, got until %s", repo.lastTempUntil)
}

func TestHandleAuthErrorEscalating_EscalatesWithCount(t *testing.T) {
	cases := []struct {
		count    int64
		cooldown time.Duration
	}{
		{count: 3, cooldown: authErrorEscalationMidCooldown},
		{count: 5, cooldown: authErrorEscalationHighCooldown},
	}
	for _, tc := range cases {
		repo := &authEscalationRepoStub{}
		counter := &authEscalationCounterStub{counts: []int64{tc.count}}
		svc := newAuthEscalationService(repo, counter)
		account := &Account{ID: 901, Platform: PlatformAnthropic, Type: AccountTypeAPIKey}
		before := time.Now()

		svc.handleAuthErrorEscalating(context.Background(), account, http.StatusForbidden, "Access forbidden (403): suspended")

		require.Equal(t, 0, repo.setErrorCalls)
		require.Equal(t, 1, repo.tempCalls)
		require.True(t, repo.tempCooldownIsApprox(before, tc.cooldown),
			"count %d should cool down %s, got until %s", tc.count, tc.cooldown, repo.lastTempUntil)
	}
}

// 中转站账号：走 handleAuthError 的 3min 固定短冷却分支，不消耗升级计数。
func TestHandleAuthErrorEscalating_RelayUpstreamShortCooldown(t *testing.T) {
	repo := &authEscalationRepoStub{}
	counter := &authEscalationCounterStub{}
	svc := newAuthEscalationService(repo, counter)
	account := &Account{
		ID:       902,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra:    map[string]any{AccountExtraUpstreamRelay: true},
	}
	before := time.Now()

	svc.handleAuthErrorEscalating(context.Background(), account, http.StatusUnauthorized, "upstream auth error")

	require.Equal(t, 0, repo.setErrorCalls)
	require.Equal(t, 1, repo.tempCalls)
	require.Equal(t, 0, counter.increments, "relay accounts must not consume the escalation counter")
	require.True(t, repo.tempCooldownIsApprox(before, relayUpstreamCooldown))
}

// 计数器未注入：退化为固定基础冷却，仍不 SetError。
func TestHandleAuthErrorEscalating_NoCacheFallback(t *testing.T) {
	repo := &authEscalationRepoStub{}
	svc := newAuthEscalationService(repo, nil)
	account := &Account{ID: 903, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	before := time.Now()

	svc.handleAuthErrorEscalating(context.Background(), account, http.StatusUnauthorized, "unauthorized")

	require.Equal(t, 0, repo.setErrorCalls)
	require.Equal(t, 1, repo.tempCalls)
	require.True(t, repo.tempCooldownIsApprox(before, authErrorEscalationBaseCooldown))
}

// ===== handlePaymentRequiredError =====

// 402 永久禁用但必须带自动复活探测：SetError + tag + rate_limit_reset_at = now + 24h。
func TestHandlePaymentRequiredError_CarriesRecoverProbe(t *testing.T) {
	repo := &authEscalationRepoStub{}
	svc := newAuthEscalationService(repo, nil)
	account := &Account{ID: 910, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	before := time.Now()

	svc.handlePaymentRequiredError(context.Background(), account, "Payment required (402): insufficient balance")

	require.Equal(t, 1, repo.setErrorCalls, "402 is the only allowed permanent disable")
	require.Equal(t, 1, repo.rateLimitedCalls, "402 must write rate_limit_reset_at for the auto-recovery scan")
	require.True(t, repo.lastRateLimitedAt.After(before.Add(failover402RecoverProbeInterval-time.Minute)),
		"probe interval must be ~24h, got %s", repo.lastRateLimitedAt)
	require.Contains(t, repo.lastErrorMsg, "insufficient balance")
	require.Contains(t, repo.lastErrorMsg, FailoverAutoRecoverMessageTag)
}

// 中转站账号 402：不再享受 3min 短冷却豁免——余额不足时换号治不了本，
// 池里每个中转站账号都会陆续 402，短冷却到期又被调度，形成无限报错循环。
// 因此与普通账号同口径：永久停用 + 24h 自动复活探测，充值后自动恢复。
func TestHandlePaymentRequiredError_RelayUpstreamDisabledWithProbe(t *testing.T) {
	repo := &authEscalationRepoStub{}
	svc := newAuthEscalationService(repo, nil)
	account := &Account{
		ID:       911,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra:    map[string]any{AccountExtraUpstreamRelay: true},
	}
	before := time.Now()

	svc.handlePaymentRequiredError(context.Background(), account, "Payment required (402)")

	require.Equal(t, 1, repo.setErrorCalls, "relay 402 must also permanently disable the account")
	require.Equal(t, 0, repo.tempCalls, "relay 402 must not fall back to the short temp cooldown")
	require.Equal(t, 1, repo.rateLimitedCalls, "relay 402 must write the auto-recovery probe time")
	require.True(t, repo.lastRateLimitedAt.After(before.Add(failover402RecoverProbeInterval-time.Minute)),
		"probe interval must be ~24h, got %s", repo.lastRateLimitedAt)
	require.Contains(t, repo.lastErrorMsg, "Payment required (402)")
	require.Contains(t, repo.lastErrorMsg, FailoverAutoRecoverMessageTag)
}

// 完整链路（HandleUpstreamError → 402）同样口径：中转站账号永久停用 + 探测，
// 不再走 relay 的 3 分钟短冷却——这是线上「没额度了账号一直回到调度池、一直报错」
// 的直接原因。
func TestHandleUpstreamError_RelayUpstream402_DisablesWithProbe(t *testing.T) {
	repo := &authEscalationRepoStub{}
	svc := newAuthEscalationService(repo, nil)
	account := &Account{
		ID:       912,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra:    map[string]any{AccountExtraUpstreamRelay: true},
	}
	before := time.Now()

	shouldDisable := svc.HandleUpstreamError(context.Background(), account, http.StatusPaymentRequired, http.Header{},
		[]byte(`{"error":{"message":"insufficient balance"}}`))

	require.True(t, shouldDisable)
	require.Equal(t, 1, repo.setErrorCalls, "relay 402 through HandleUpstreamError must disable the account")
	require.Equal(t, 0, repo.tempCalls, "relay 402 must not be downgraded to the short temp cooldown")
	require.Equal(t, 1, repo.rateLimitedCalls, "relay 402 must write the auto-recovery probe time")
	require.True(t, repo.lastRateLimitedAt.After(before.Add(failover402RecoverProbeInterval-time.Minute)),
		"probe interval must be ~24h, got %s", repo.lastRateLimitedAt)
	require.Contains(t, repo.lastErrorMsg, "insufficient balance")
	require.Contains(t, repo.lastErrorMsg, FailoverAutoRecoverMessageTag)
}

// ===== 成功/恢复清零升级计数 =====

func TestResetOpenAI403Counter_AlsoClearsEscalationCounters(t *testing.T) {
	repo := &authEscalationRepoStub{}
	counter := &authEscalationCounterStub{}
	svc := newAuthEscalationService(repo, counter)

	svc.ResetOpenAI403Counter(context.Background(), 920)

	require.Equal(t, []int64{920}, counter.resets, "403 counter reset must also clear escalation counters")
}

func TestResetOpenAI403Counter_NilCachesNoPanic(t *testing.T) {
	svc := NewRateLimitService(&authEscalationRepoStub{}, nil, nil, nil, nil)
	svc.ResetOpenAI403Counter(context.Background(), 921)
}
