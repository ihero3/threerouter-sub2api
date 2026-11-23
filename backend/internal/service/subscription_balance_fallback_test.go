package service

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

// ── 订阅额度耗尽自动回退余额（subscription_balance_fallback_enabled）单测 ──

// fallbackBillingCacheStub 余额与订阅缓存可配的最小 BillingCache 桩。
type fallbackBillingCacheStub struct {
	BillingCache
	balance  float64
	subCache *SubscriptionCacheData
}

func (f *fallbackBillingCacheStub) GetUserBalance(context.Context, int64) (float64, error) {
	return f.balance, nil
}

func (f *fallbackBillingCacheStub) GetSubscriptionCache(context.Context, int64, int64) (*SubscriptionCacheData, error) {
	if f.subCache == nil {
		return nil, errors.New("cache miss")
	}
	return f.subCache, nil
}

// fallbackSettingRepoStub SettingRepository 最小桩，只支持 GetValue。
type fallbackSettingRepoStub struct {
	values map[string]string
}

func (s *fallbackSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if v, ok := s.values[key]; ok {
		return v, nil
	}
	return "", errors.New("missing setting: " + key)
}

func (s *fallbackSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, errors.New("not implemented")
}
func (s *fallbackSettingRepoStub) Set(context.Context, string, string) error {
	return errors.New("not implemented")
}
func (s *fallbackSettingRepoStub) GetMultiple(_ context.Context, _ []string) (map[string]string, error) {
	return map[string]string{}, nil
}
func (s *fallbackSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	return errors.New("not implemented")
}
func (s *fallbackSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}
func (s *fallbackSettingRepoStub) Delete(context.Context, string) error {
	return errors.New("not implemented")
}

func newFallbackBillingService(t *testing.T, cache *fallbackBillingCacheStub, fallbackSwitchOn bool) *BillingCacheService {
	t.Helper()
	settingSvc := &SettingService{settingRepo: &fallbackSettingRepoStub{values: map[string]string{
		SettingKeySubscriptionBalanceFallbackEnabled: strconv.FormatBool(fallbackSwitchOn),
	}}}
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{}, nil, settingSvc)
	t.Cleanup(svc.Stop)
	return svc
}

func fallbackExhaustedSubCache() *SubscriptionCacheData {
	return &SubscriptionCacheData{
		Status:     SubscriptionStatusActive,
		ExpiresAt:  time.Now().Add(time.Hour),
		DailyUsage: 5,
	}
}

func newFallbackGroup() *Group {
	daily := 5.0
	return &Group{ID: 2, SubscriptionType: SubscriptionTypeSubscription, DailyLimitUSD: &daily}
}

// 开关关闭（回归）：订阅档位耗尽保持原档位错误，不回退。
func TestSubscriptionBalanceFallbackSwitchOffKeepsTierError(t *testing.T) {
	svc := newFallbackBillingService(t, &fallbackBillingCacheStub{balance: 10, subCache: fallbackExhaustedSubCache()}, false)

	sub, err := svc.ResolveEffectiveSubscription(context.Background(), &User{ID: 1}, &APIKey{ID: 3}, newFallbackGroup(), &UserSubscription{ID: 1}, "")
	require.Nil(t, sub)
	require.ErrorIs(t, err, ErrDailyLimitExceeded)
}

// 开关开启 + 余额充足：任一档耗尽回退余额，返回 (nil, nil) 表示本次按余额计费。
func TestSubscriptionBalanceFallbackFallsBackToBalance(t *testing.T) {
	svc := newFallbackBillingService(t, &fallbackBillingCacheStub{balance: 10, subCache: fallbackExhaustedSubCache()}, true)

	sub, err := svc.ResolveEffectiveSubscription(context.Background(), &User{ID: 1}, &APIKey{ID: 3}, newFallbackGroup(), &UserSubscription{ID: 1}, "")
	require.NoError(t, err)
	require.Nil(t, sub, "回退成功必须返回 nil 订阅，使下游自然走余额分支")
}

// 开关开启 + 余额充足：周档耗尽同样回退（任一档口径）。
func TestSubscriptionBalanceFallbackWeeklyTier(t *testing.T) {
	weekly := 5.0
	group := &Group{ID: 2, SubscriptionType: SubscriptionTypeSubscription, WeeklyLimitUSD: &weekly}
	cache := &fallbackBillingCacheStub{balance: 1, subCache: &SubscriptionCacheData{
		Status:      SubscriptionStatusActive,
		ExpiresAt:   time.Now().Add(time.Hour),
		WeeklyUsage: 5,
	}}
	svc := newFallbackBillingService(t, cache, true)

	sub, err := svc.ResolveEffectiveSubscription(context.Background(), &User{ID: 1}, &APIKey{ID: 3}, group, &UserSubscription{ID: 1}, "")
	require.NoError(t, err)
	require.Nil(t, sub)
}

// 双耗尽：档位错误保留并附 window_resets_at + balance_fallback_rejected metadata。
func TestSubscriptionBalanceFallbackDoubleExhaustedMetadata(t *testing.T) {
	svc := newFallbackBillingService(t, &fallbackBillingCacheStub{balance: 0, subCache: fallbackExhaustedSubCache()}, true)

	sub, err := svc.ResolveEffectiveSubscription(context.Background(), &User{ID: 1}, &APIKey{ID: 3}, newFallbackGroup(), &UserSubscription{ID: 1}, "")
	require.Nil(t, sub)
	require.ErrorIs(t, err, ErrDailyLimitExceeded)

	var appErr *infraerrors.ApplicationError
	require.True(t, errors.As(err, &appErr), "应为 ApplicationError 以便 handler 读取 metadata")
	resetAt, ok := appErr.Metadata[metadataKeyWindowResetsAt]
	require.True(t, ok, "双耗尽必须带窗口重置时间")
	parsed, perr := time.Parse(time.RFC3339, resetAt)
	require.NoError(t, perr)
	require.True(t, parsed.After(time.Now()), "重置时间应在未来")
	require.Equal(t, "true", appErr.Metadata[metadataKeyBalanceFallbackRejected])
}

// 非档位错误（订阅失效）不回退，原样透传。
func TestSubscriptionBalanceFallbackSkipsNonTierError(t *testing.T) {
	cache := &fallbackBillingCacheStub{balance: 10, subCache: &SubscriptionCacheData{
		Status:    SubscriptionStatusSuspended,
		ExpiresAt: time.Now().Add(time.Hour),
	}}
	svc := newFallbackBillingService(t, cache, true)

	sub, err := svc.ResolveEffectiveSubscription(context.Background(), &User{ID: 1}, &APIKey{ID: 3}, newFallbackGroup(), &UserSubscription{ID: 1}, "")
	require.Nil(t, sub)
	require.ErrorIs(t, err, ErrSubscriptionInvalid)
}

// 订阅还有额度：任何情况下都订阅优先，不扣余额。
func TestSubscriptionBalanceFallbackNotTriggeredWithQuota(t *testing.T) {
	subCache := fallbackExhaustedSubCache()
	subCache.DailyUsage = 1
	svc := newFallbackBillingService(t, &fallbackBillingCacheStub{balance: 10, subCache: subCache}, true)

	sub, err := svc.ResolveEffectiveSubscription(context.Background(), &User{ID: 1}, &APIKey{ID: 3}, newFallbackGroup(), &UserSubscription{ID: 1}, "")
	require.NoError(t, err)
	require.NotNil(t, sub, "订阅有额度时保持订阅计费")
}

// 余额模式 key：不参与回退逻辑，余额充足直接通过。
func TestSubscriptionBalanceFallbackBalanceModeUnaffected(t *testing.T) {
	svc := newFallbackBillingService(t, &fallbackBillingCacheStub{balance: 10}, true)

	group := &Group{ID: 4, SubscriptionType: SubscriptionTypeStandard}
	sub, err := svc.ResolveEffectiveSubscription(context.Background(), &User{ID: 1}, &APIKey{ID: 3}, group, nil, "")
	require.NoError(t, err)
	require.Nil(t, sub)
}

// 订阅缺窗口锚点时，重置时间回退到自然窗口边界（Retry-After 永有值）。
func TestSubscriptionTierResetTimeNaturalWindowFallback(t *testing.T) {
	now := time.Date(2026, 11, 22, 15, 30, 0, 0, time.UTC)
	bare := &UserSubscription{ID: 1}

	daily := subscriptionTierResetTime(bare, ErrDailyLimitExceeded, now)
	require.Equal(t, now.AddDate(0, 0, 1).Format("2006-01-02"), daily.Format("2006-01-02"))

	weekly := subscriptionTierResetTime(bare, ErrWeeklyLimitExceeded, now)
	require.True(t, weekly.After(now), "周窗口回退应指向未来边界（周一 0 点起算）")
	require.Equal(t, time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC), weekly)

	monthly := subscriptionTierResetTime(bare, ErrMonthlyLimitExceeded, now)
	require.Equal(t, 30*24*time.Hour, monthly.Sub(now))
}

// 档位错误判定只认日/周/月档位耗尽。
func TestIsSubscriptionTierLimitError(t *testing.T) {
	require.True(t, isSubscriptionTierLimitError(ErrDailyLimitExceeded))
	require.True(t, isSubscriptionTierLimitError(ErrWeeklyLimitExceeded))
	require.True(t, isSubscriptionTierLimitError(ErrMonthlyLimitExceeded))
	require.False(t, isSubscriptionTierLimitError(ErrSubscriptionInvalid))
	require.False(t, isSubscriptionTierLimitError(ErrInsufficientBalance))
	require.False(t, isSubscriptionTierLimitError(nil))
}
