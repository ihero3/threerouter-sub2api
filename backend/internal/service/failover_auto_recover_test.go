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

// failoverDisableRepoStub 只承载 GetByID/Update/SetTempUnschedulable，
// 其余接口方法通过内嵌接口保持空实现。
type failoverDisableRepoStub struct {
	AccountRepository
	acct      *Account
	updated   *Account
	getCalled bool

	tempCalled bool
	tempID     int64
	tempUntil  time.Time
	tempReason string
}

func (r *failoverDisableRepoStub) GetByID(ctx context.Context, id int64) (*Account, error) {
	r.getCalled = true
	return r.acct, nil
}

func (r *failoverDisableRepoStub) Update(ctx context.Context, account *Account) error {
	r.updated = account
	return nil
}

func (r *failoverDisableRepoStub) SetTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error {
	r.tempCalled = true
	r.tempID = id
	r.tempUntil = until
	r.tempReason = reason
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
		repo.acct, http.StatusTooManyRequests, nil, alwaysFailover, msg, nil)

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

// 429 无可信 reset：不再永久禁用（方案 A 收敛），降级为 30min 临时冷却。
func TestMaybeDisableAccountOnFailover_429WithoutResetTime_TempCooldown(t *testing.T) {
	repo := &failoverDisableRepoStub{acct: &Account{ID: 1, Status: StatusActive, Schedulable: true}}

	maybeDisableAccountOnFailover(context.Background(), newFailoverDisableSettingService(true), repo,
		repo.acct, http.StatusTooManyRequests, nil, alwaysFailover, "rate limit exceeded", nil)

	if repo.updated != nil {
		t.Fatalf("429 without reset must not permanently disable, got status %s", repo.updated.Status)
	}
	if !repo.tempCalled {
		t.Fatal("429 without reset must temp-cooldown the account")
	}
	if repo.tempUntil.Before(time.Now().Add(failoverClientErrorTempCooldown - time.Second)) {
		t.Fatalf("cooldown window too short: %s", repo.tempUntil)
	}
	if strings.Contains(repo.tempReason, FailoverAutoRecoverMessageTag) {
		t.Fatalf("temp cooldown must not carry auto-recover tag, got %q", repo.tempReason)
	}
}

// 401：不再永久禁用（方案 A 收敛），降级为 30min 临时冷却，到期自动恢复调度。
func TestMaybeDisableAccountOnFailover_401_TempCooldown(t *testing.T) {
	repo := &failoverDisableRepoStub{acct: &Account{ID: 1, Status: StatusActive, Schedulable: true}}

	maybeDisableAccountOnFailover(context.Background(), newFailoverDisableSettingService(true), repo,
		repo.acct, http.StatusUnauthorized, nil, alwaysFailover, "unauthorized", nil)

	if repo.updated != nil {
		t.Fatalf("401 must not permanently disable, got status %s", repo.updated.Status)
	}
	if !repo.tempCalled {
		t.Fatal("401 must temp-cooldown the account")
	}
}

// 5xx：不再永久禁用（方案 A 收敛），降级为 10min 短冷却（瞬时故障）。
func TestMaybeDisableAccountOnFailover_5xx_TempCooldown(t *testing.T) {
	for _, statusCode := range []int{http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable} {
		repo := &failoverDisableRepoStub{acct: &Account{ID: 1, Status: StatusActive, Schedulable: true}}

		maybeDisableAccountOnFailover(context.Background(), newFailoverDisableSettingService(true), repo,
			repo.acct, statusCode, nil, alwaysFailover, "upstream error", nil)

		if repo.updated != nil {
			t.Fatalf("HTTP %d must not permanently disable, got status %s", statusCode, repo.updated.Status)
		}
		if !repo.tempCalled {
			t.Fatalf("HTTP %d must temp-cooldown the account", statusCode)
		}
		if repo.tempUntil.Before(time.Now().Add(failoverServerErrorTempCooldown-time.Second)) ||
			repo.tempUntil.After(time.Now().Add(failoverServerErrorTempCooldown+time.Minute)) {
			t.Fatalf("HTTP %d: unexpected cooldown until %s", statusCode, repo.tempUntil.Format(time.RFC3339))
		}
	}
}

// 402（余额/计费不足）是唯一保留永久禁用的状态码，但必须带自动复活探测：
// error_message 写 [auto-recover at ...] 标记 + rate_limit_reset_at = now + 24h，
// AccountAutoRecoveryService 每 24h 放回调度池试一次，充值后无需人工恢复。
func TestMaybeDisableAccountOnFailover_402_PermanentWithRecoverProbe(t *testing.T) {
	repo := &failoverDisableRepoStub{acct: &Account{ID: 1, Status: StatusActive, Schedulable: true}}
	before := time.Now()

	maybeDisableAccountOnFailover(context.Background(), newFailoverDisableSettingService(true), repo,
		repo.acct, http.StatusPaymentRequired, nil, alwaysFailover, "insufficient balance", nil)

	if repo.updated == nil || repo.updated.Status != StatusError {
		t.Fatal("402 must permanently disable the account")
	}
	if repo.updated.RateLimitResetAt == nil {
		t.Fatal("402 must schedule the 24h auto-recover probe")
	}
	if repo.updated.RateLimitResetAt.Before(before.Add(failover402RecoverProbeInterval - time.Minute)) {
		t.Fatalf("probe interval too short: %s", repo.updated.RateLimitResetAt.Format(time.RFC3339))
	}
	if !strings.Contains(repo.updated.ErrorMessage, FailoverAutoRecoverMessageTag) {
		t.Fatalf("402 error message must carry auto-recover tag, got %q", repo.updated.ErrorMessage)
	}
	if strings.Contains(repo.updated.ErrorMessage, "insufficient balance") == false {
		t.Fatalf("402 error message must keep upstream detail, got %q", repo.updated.ErrorMessage)
	}
}

// 402 的处罚与 "Disable Failed Account on Failover" 开关解耦：开关只管其余状态码，
// 402（没费用）始终停用账号——否则没有额度的账号会一直留在调度池里，每次请求
// 都在它身上拿到 402，再把同样的错误换号抛给下一个账号。
func TestMaybeDisableAccountOnFailover_402_DisablesEvenWhenSettingDisabled(t *testing.T) {
	repo := &failoverDisableRepoStub{acct: &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true}}
	before := time.Now()

	maybeDisableAccountOnFailover(context.Background(), newFailoverDisableSettingService(false), repo,
		repo.acct, http.StatusPaymentRequired, nil, alwaysFailover, "insufficient balance", nil)

	if repo.updated == nil || repo.updated.Status != StatusError {
		t.Fatal("402 must permanently disable the account even when the switch is off")
	}
	if repo.updated.RateLimitResetAt == nil ||
		repo.updated.RateLimitResetAt.Before(before.Add(failover402RecoverProbeInterval-time.Minute)) {
		t.Fatal("402 must still schedule the 24h auto-recover probe when the switch is off")
	}
	if !strings.Contains(repo.updated.ErrorMessage, FailoverAutoRecoverMessageTag) {
		t.Fatalf("402 error message must carry auto-recover tag, got %q", repo.updated.ErrorMessage)
	}
}

// 中转站账号 402：不再降级为 3 分钟短冷却。短冷却到期又被调度、又 402，会形成
// 无限报错循环；余额不足是确定事实，只有停用 + 24h 探测才能在充值后自动恢复。
func TestMaybeDisableAccountOnFailover_RelayUpstream_402_PermanentWithRecoverProbe(t *testing.T) {
	repo := &failoverDisableRepoStub{acct: newRelayAccount(3)}
	before := time.Now()

	maybeDisableAccountOnFailover(context.Background(), newFailoverDisableSettingService(false), repo,
		repo.acct, http.StatusPaymentRequired, nil, alwaysFailover, "insufficient balance", nil)

	if repo.tempCalled {
		t.Fatal("relay 402 must not fall back to the relay short cooldown")
	}
	if repo.updated == nil || repo.updated.Status != StatusError {
		t.Fatal("relay 402 must permanently disable the account")
	}
	if repo.updated.RateLimitResetAt == nil ||
		repo.updated.RateLimitResetAt.Before(before.Add(failover402RecoverProbeInterval-time.Minute)) {
		t.Fatal("relay 402 must schedule the 24h auto-recover probe")
	}
	if !strings.Contains(repo.updated.ErrorMessage, FailoverAutoRecoverMessageTag) {
		t.Fatalf("relay 402 error message must carry auto-recover tag, got %q", repo.updated.ErrorMessage)
	}
}

// 自带周期余额探测的账号（国产供应商 / Cline / Command Code / OpenCode Zen）：
// 402 交给专属钱包分支做可自愈的临时停调，failover 处罚不得永久置 error
// （探测任务只清自己写的临时停调，救不回 status=error 的账号）。
func TestMaybeDisableAccountOnFailover_402_BalanceManagedAccountSkips(t *testing.T) {
	for _, platform := range []string{PlatformDeepseek, PlatformKimi, PlatformCline, PlatformCommandCode} {
		acct := &Account{ID: 4, Platform: platform, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true}
		repo := &failoverDisableRepoStub{acct: acct}

		maybeDisableAccountOnFailover(context.Background(), newFailoverDisableSettingService(true), repo,
			acct, http.StatusPaymentRequired, nil, alwaysFailover, "insufficient balance", nil)

		if repo.updated != nil || repo.tempCalled {
			t.Fatalf("platform %s: balance-managed account must be left to its wallet/balance probe on 402", platform)
		}
	}
}

// 池模式账号完全跳过账号级惩罚：与 HandleUpstreamError 的池模式豁免口径一致，
// 修复此前开关开启时池模式账号仍会被永久禁用的一致性缺口。
func TestMaybeDisableAccountOnFailover_PoolMode_NoOp(t *testing.T) {
	acct := &Account{
		ID:          1,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{"pool_mode": true},
	}
	repo := &failoverDisableRepoStub{acct: acct}

	for _, statusCode := range []int{http.StatusUnauthorized, http.StatusPaymentRequired, http.StatusInternalServerError} {
		repo.getCalled, repo.updated, repo.tempCalled = false, nil, false
		maybeDisableAccountOnFailover(context.Background(), newFailoverDisableSettingService(true), repo,
			repo.acct, statusCode, nil, alwaysFailover, "upstream error", nil)

		if repo.updated != nil || repo.tempCalled {
			t.Fatalf("HTTP %d: pool-mode account must be untouched by failover disable", statusCode)
		}
	}
}

func TestMaybeDisableAccountOnFailover_SettingDisabled_NoOp(t *testing.T) {
	repo := &failoverDisableRepoStub{acct: &Account{ID: 1, Status: StatusActive, Schedulable: true}}

	maybeDisableAccountOnFailover(context.Background(), newFailoverDisableSettingService(false), repo,
		repo.acct, http.StatusTooManyRequests, nil, alwaysFailover, "reset at 2026-10-14 12:58:44 +0800 CST", nil)

	if repo.getCalled || repo.updated != nil {
		t.Fatal("account must be untouched when the setting is disabled")
	}
}

func TestMaybeDisableAccountOnFailover_404ModelNotSupportedByAnyAccount_NoDisable(t *testing.T) {
	repo := &failoverDisableRepoStub{acct: &Account{ID: 1, Status: StatusActive, Schedulable: true}}
	// sub2api/one-api 类中转站的模型缺失错误：分组内无账号支持该模型。
	// 语义是（账号, 模型）组合在上游无能力，不是账号失效——不得永久禁用整个账号。
	body := []byte(`{"error":{"message":"Model \"deepseek-v4-flash\" is not supported by any configured account in this group","type":"invalid_request_error"}}`)

	maybeDisableAccountOnFailover(context.Background(), newFailoverDisableSettingService(true), repo,
		repo.acct, http.StatusNotFound, nil, alwaysFailover, "Model \"deepseek-v4-flash\" is not supported by any configured account in this group", body)

	if repo.updated != nil {
		t.Fatalf("404 model-not-supported must not disable account, got status %s msg %q", repo.updated.Status, repo.updated.ErrorMessage)
	}
}

// 2026-11-26 事故回归：端点级 404（"Invalid URL (POST /v1/chat/completions/v1/embeddings)"、
// "Not Found (request id: ...)"）不含 model 文案，此前一次命中即永久禁用，
// 一个 embeddings 请求经 failover 把同组健康账号逐个扫禁停服。现在必须降级为临时冷却。
func TestMaybeDisableAccountOnFailover_404EndpointNotFound_TempCooldown(t *testing.T) {
	bodies := [][]byte{
		[]byte(`{"error":{"code":"invalid_request_error","message":"Invalid URL (POST /v1/chat/completions/v1/embeddings)"}}`),
		[]byte(`{"error":{"message":"Not Found (request id: 3ce408cd-1921-4ac7-92a1-b183440f1fca)"}}`),
		[]byte(`404 page not found`),
	}
	for i, body := range bodies {
		repo := &failoverDisableRepoStub{acct: &Account{ID: 1, Status: StatusActive, Schedulable: true}}
		before := time.Now()

		maybeDisableAccountOnFailover(context.Background(), newFailoverDisableSettingService(true), repo,
			repo.acct, http.StatusNotFound, nil, alwaysFailover, "Invalid URL (POST /v1/chat/completions/v1/embeddings)", body)

		if repo.updated != nil {
			t.Fatalf("case %d: endpoint-level 404 must not permanently disable account, got status %s", i, repo.updated.Status)
		}
		if !repo.tempCalled {
			t.Fatalf("case %d: endpoint-level 404 must temp-cooldown the account", i)
		}
		if repo.tempID != 1 {
			t.Fatalf("case %d: cooldown applied to wrong account %d", i, repo.tempID)
		}
		if repo.tempUntil.Before(before.Add(failoverEndpointNotFoundCooldown - time.Second)) {
			t.Fatalf("case %d: cooldown window too short: %s", i, repo.tempUntil)
		}
		if !strings.HasPrefix(repo.tempReason, "Temp-cooled on upstream failover: HTTP 404") {
			t.Fatalf("case %d: unexpected reason %q", i, repo.tempReason)
		}
	}
}

func newRelayAccount(id int64) *Account {
	return &Account{
		ID:          id,
		Status:      StatusActive,
		Schedulable: true,
		Extra:       map[string]any{AccountExtraUpstreamRelay: true},
	}
}

// 中转站账号遇到 401 这类账号级错误：只做 3 分钟临时冷却，账号保持 active，
// 到期自动恢复调度——上游 Key 失效是中转站内部的事，不能永久禁用本地账号。
func TestMaybeDisableAccountOnFailover_RelayUpstream_401TempCooldown(t *testing.T) {
	for _, statusCode := range []int{http.StatusUnauthorized, http.StatusInternalServerError} {
		repo := &failoverDisableRepoStub{acct: newRelayAccount(1)}
		before := time.Now()

		maybeDisableAccountOnFailover(context.Background(), newFailoverDisableSettingService(true), repo,
			repo.acct, statusCode, nil, alwaysFailover, "upstream error", nil)

		if repo.updated != nil {
			t.Fatalf("HTTP %d must not permanently disable a relay account, got status %s", statusCode, repo.updated.Status)
		}
		if !repo.tempCalled {
			t.Fatalf("HTTP %d must set temp-unschedulable cooldown", statusCode)
		}
		if repo.tempID != repo.acct.ID {
			t.Fatalf("expected temp cooldown on account %d, got %d", repo.acct.ID, repo.tempID)
		}
		if repo.tempUntil.Before(before.Add(relayUpstreamCooldown-time.Second)) ||
			repo.tempUntil.After(before.Add(relayUpstreamCooldown+time.Minute)) {
			t.Fatalf("unexpected cooldown until %s", repo.tempUntil.Format(time.RFC3339))
		}
	}
}

// 429 带可信重置时间时保留原有自动恢复逻辑（记录 rate_limit_reset_at + auto-recover tag），
// 不被中转站的固定 3 分钟冷却覆盖——上游已经明确告诉我们要等到几点。
func TestMaybeDisableAccountOnFailover_RelayUpstream_429WithResetTimeKeepsAutoRecover(t *testing.T) {
	repo := &failoverDisableRepoStub{acct: newRelayAccount(2)}
	cst := time.FixedZone("CST", 8*3600)
	resetAt := time.Now().In(cst).Add(2 * time.Hour)
	msg := "You have exceeded the 5-hour usage quota. It will reset at " +
		resetAt.Format("2006-01-02 15:04:05 -0700 MST") + ". Please wait."

	maybeDisableAccountOnFailover(context.Background(), newFailoverDisableSettingService(true), repo,
		repo.acct, http.StatusTooManyRequests, nil, alwaysFailover, msg, nil)

	if repo.tempCalled {
		t.Fatal("429 with a trusted reset time must keep auto-recover, not the fixed relay cooldown")
	}
	if repo.updated == nil || repo.updated.Status != StatusError {
		t.Fatal("expected account to be disabled with a recover time")
	}
	if repo.updated.RateLimitResetAt == nil || repo.updated.RateLimitResetAt.Unix() != resetAt.Unix() {
		t.Fatalf("expected rate_limit_reset_at = %d, got %v", resetAt.Unix(), repo.updated.RateLimitResetAt)
	}
	if !strings.Contains(repo.updated.ErrorMessage, FailoverAutoRecoverMessageTag) {
		t.Fatalf("expected auto-recover tag, got %q", repo.updated.ErrorMessage)
	}
}
