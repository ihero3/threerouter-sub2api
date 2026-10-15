//go:build unit

package service

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// TestResolveRebateRatePercent_PerUserOverride verifies that per-inviter
// AffRebateRatePercent overrides the global rate, that NULL falls back to the
// global rate, and that out-of-range exclusive rates are clamped silently.
//
// SettingService is left nil here so globalRebateRatePercent returns the
// documented default (AffiliateRebateRateDefault = 10%) — this exercises the
// fallback path without spinning up a settings stub.
func TestResolveRebateRatePercent_PerUserOverride(t *testing.T) {
	t.Parallel()
	svc := &AffiliateService{}

	// nil exclusive rate → falls back to global default (10%)
	require.InDelta(t, AffiliateRebateRateDefault,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{}), 1e-9)

	// exclusive rate set → overrides global
	rate := 50.0
	require.InDelta(t, 50.0,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &rate}), 1e-9)

	// exclusive rate 0 → returns 0 (no rebate, intentional)
	zero := 0.0
	require.InDelta(t, 0.0,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &zero}), 1e-9)

	// exclusive rate above max → clamped to Max
	tooHigh := 250.0
	require.InDelta(t, AffiliateRebateRateMax,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &tooHigh}), 1e-9)

	// exclusive rate below min → clamped to Min
	tooLow := -5.0
	require.InDelta(t, AffiliateRebateRateMin,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &tooLow}), 1e-9)
}

// TestIsEnabled_NilSettingServiceReturnsDefault verifies that IsEnabled
// safely handles a nil settingService dependency by returning the default
// (off). This protects callers from nil-pointer crashes in misconfigured
// environments.
func TestIsEnabled_NilSettingServiceReturnsDefault(t *testing.T) {
	t.Parallel()
	svc := &AffiliateService{}
	require.False(t, svc.IsEnabled(context.Background()))
	require.Equal(t, AffiliateEnabledDefault, svc.IsEnabled(context.Background()))
}

// TestValidateExclusiveRate_BoundaryAndInvalid covers the validator used by
// admin-facing rate setters: nil is always valid (clear), in-range values
// are accepted, NaN/Inf and out-of-range values produce a typed BadRequest.
func TestValidateExclusiveRate_BoundaryAndInvalid(t *testing.T) {
	t.Parallel()
	require.NoError(t, validateExclusiveRate(nil))

	for _, v := range []float64{0, 0.01, 50, 99.99, 100} {
		v := v
		require.NoError(t, validateExclusiveRate(&v), "value %v should be valid", v)
	}

	for _, v := range []float64{-0.01, 100.01, -100, 200} {
		v := v
		require.Error(t, validateExclusiveRate(&v), "value %v should be rejected", v)
	}

	nan := math.NaN()
	require.Error(t, validateExclusiveRate(&nan))
	posInf := math.Inf(1)
	require.Error(t, validateExclusiveRate(&posInf))
	negInf := math.Inf(-1)
	require.Error(t, validateExclusiveRate(&negInf))
}

func TestMaskEmail(t *testing.T) {
	t.Parallel()
	require.Equal(t, "a***@g***.com", maskEmail("alice@gmail.com"))
	require.Equal(t, "x***@d***", maskEmail("x@domain"))
	require.Equal(t, "", maskEmail(""))
}

func TestIsValidAffiliateCodeFormat(t *testing.T) {
	t.Parallel()

	// 邀请码格式校验同时服务于：
	// 1) 系统自动生成的 12 位随机码（A-Z 去 I/O，2-9 去 0/1）
	// 2) 管理员设置的自定义专属码（如 "VIP2026"、"NEW_USER-1"）
	// 因此校验放宽到 [A-Z0-9_-]{4,32}（要求调用方先 ToUpper）。
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"valid canonical 12-char", "ABCDEFGHJKLM", true},
		{"valid all digits 2-9", "234567892345", true},
		{"valid mixed", "A2B3C4D5E6F7", true},
		{"valid admin custom short", "VIP1", true},
		{"valid admin custom with hyphen", "NEW-USER", true},
		{"valid admin custom with underscore", "VIP_2026", true},
		{"valid 32-char max", "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345", true},
		// Previously-excluded chars (I/O/0/1) are now allowed since admins may use them.
		{"letter I now allowed", "IBCDEFGHJKLM", true},
		{"letter O now allowed", "OBCDEFGHJKLM", true},
		{"digit 0 now allowed", "0BCDEFGHJKLM", true},
		{"digit 1 now allowed", "1BCDEFGHJKLM", true},
		{"too short (3 chars)", "ABC", false},
		{"too long (33 chars)", "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456", false},
		{"lowercase rejected (caller must ToUpper first)", "abcdefghjklm", false},
		{"empty", "", false},
		{"utf8 non-ascii", "ÄÄÄÄÄÄ", false}, // bytes out of charset
		{"ascii punctuation .", "ABCDEFGHJK.M", false},
		{"whitespace", "ABCDEFGHJK M", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, isValidAffiliateCodeFormat(tc.in))
		})
	}
}

// ipGuardRepoStub 只覆写 enforceIPGuard 依赖的两个方法，
// 其余 AffiliateRepository 方法沿用内嵌接口（未覆写即为 nil，调用会 panic）。
type ipGuardRepoStub struct {
	AffiliateRepository

	count    int
	countErr error

	countCalls     int
	lastInviterID  int64
	lastIP         string
	createCalls    int
	createdInvitee int64
	createdInviter int64
	createdIP      string
	createdReason  string
	createErr      error
}

func (r *ipGuardRepoStub) CountInviteesByInviterAndIP(_ context.Context, inviterID int64, registerIP string) (int, error) {
	r.countCalls++
	r.lastInviterID = inviterID
	r.lastIP = registerIP
	return r.count, r.countErr
}

func (r *ipGuardRepoStub) CreateAffiliateGuardEvent(_ context.Context, inviteeID, inviterID int64, registerIP, reason string) (bool, error) {
	r.createCalls++
	r.createdInvitee = inviteeID
	r.createdInviter = inviterID
	r.createdIP = registerIP
	r.createdReason = reason
	return true, r.createErr
}

func newIPGuardService(values map[string]string, repo AffiliateRepository) *AffiliateService {
	return NewAffiliateService(repo, NewSettingService(&settingRepoStub{values: values}, &config.Config{}), nil, nil)
}

// TestEnforceIPGuard 覆盖注册 IP 关联风控的各分支：
// 关闭放行、空 IP 放行、未达阈值放行、block 拦截、review 落事件、
// 查询失败 fail-open、repo 缺失 fail-open。
func TestEnforceIPGuard(t *testing.T) {
	t.Parallel()

	const (
		inviteeID = int64(200)
		inviterID = int64(100)
		ip        = "1.2.3.4"
	)

	t.Run("guard off allows", func(t *testing.T) {
		t.Parallel()
		repo := &ipGuardRepoStub{count: 99}
		svc := newIPGuardService(map[string]string{SettingKeyAffiliateIPGuardMode: AffiliateIPGuardModeOff}, repo)
		require.NoError(t, svc.enforceIPGuard(context.Background(), inviteeID, inviterID, ip))
		require.Equal(t, 0, repo.countCalls, "guard off should short-circuit before counting")
	})

	t.Run("empty register ip allows", func(t *testing.T) {
		t.Parallel()
		repo := &ipGuardRepoStub{count: 99}
		svc := newIPGuardService(map[string]string{SettingKeyAffiliateIPGuardMode: AffiliateIPGuardModeBlock}, repo)
		require.NoError(t, svc.enforceIPGuard(context.Background(), inviteeID, inviterID, "   "))
		require.Equal(t, 0, repo.countCalls, "empty IP should short-circuit before counting")
	})

	t.Run("below threshold allows", func(t *testing.T) {
		t.Parallel()
		repo := &ipGuardRepoStub{count: 1}
		svc := newIPGuardService(map[string]string{
			SettingKeyAffiliateIPGuardMode:        AffiliateIPGuardModeBlock,
			SettingKeyAffiliateIPGuardMaxAccounts: "2",
		}, repo)
		require.NoError(t, svc.enforceIPGuard(context.Background(), inviteeID, inviterID, ip))
		require.Equal(t, 1, repo.countCalls)
		require.Equal(t, 0, repo.createCalls)
	})

	t.Run("count error fails open", func(t *testing.T) {
		t.Parallel()
		repo := &ipGuardRepoStub{countErr: errors.New("boom")}
		svc := newIPGuardService(map[string]string{SettingKeyAffiliateIPGuardMode: AffiliateIPGuardModeBlock}, repo)
		require.NoError(t, svc.enforceIPGuard(context.Background(), inviteeID, inviterID, ip))
		require.Equal(t, 0, repo.createCalls)
	})

	t.Run("block mode rejects at threshold", func(t *testing.T) {
		t.Parallel()
		repo := &ipGuardRepoStub{count: 1}
		svc := newIPGuardService(map[string]string{
			SettingKeyAffiliateIPGuardMode:        AffiliateIPGuardModeBlock,
			SettingKeyAffiliateIPGuardMaxAccounts: "1",
		}, repo)
		err := svc.enforceIPGuard(context.Background(), inviteeID, inviterID, ip)
		require.ErrorIs(t, err, ErrAffiliateIPGuardBlocked)
		require.Equal(t, 0, repo.createCalls, "block mode must not record a guard event")
	})

	t.Run("review mode records event and rejects", func(t *testing.T) {
		t.Parallel()
		repo := &ipGuardRepoStub{count: 1}
		svc := newIPGuardService(map[string]string{
			SettingKeyAffiliateIPGuardMode:        AffiliateIPGuardModeReview,
			SettingKeyAffiliateIPGuardMaxAccounts: "1",
		}, repo)
		err := svc.enforceIPGuard(context.Background(), inviteeID, inviterID, ip)
		require.ErrorIs(t, err, ErrAffiliateIPGuardReview)
		require.Equal(t, 1, repo.createCalls)
		require.Equal(t, inviteeID, repo.createdInvitee)
		require.Equal(t, inviterID, repo.createdInviter)
		require.Equal(t, ip, repo.createdIP)
		require.Equal(t, AffiliateGuardReasonIPShare, repo.createdReason)
	})

	t.Run("review event write error still rejects", func(t *testing.T) {
		t.Parallel()
		repo := &ipGuardRepoStub{count: 1, createErr: errors.New("insert failed")}
		svc := newIPGuardService(map[string]string{
			SettingKeyAffiliateIPGuardMode:        AffiliateIPGuardModeReview,
			SettingKeyAffiliateIPGuardMaxAccounts: "1",
		}, repo)
		require.ErrorIs(t, svc.enforceIPGuard(context.Background(), inviteeID, inviterID, ip), ErrAffiliateIPGuardReview)
	})

	t.Run("nil repo fails open", func(t *testing.T) {
		t.Parallel()
		svc := NewAffiliateService(nil, NewSettingService(&settingRepoStub{values: map[string]string{
			SettingKeyAffiliateIPGuardMode: AffiliateIPGuardModeBlock,
		}}, &config.Config{}), nil, nil)
		require.NoError(t, svc.enforceIPGuard(context.Background(), inviteeID, inviterID, ip))
	})
}
