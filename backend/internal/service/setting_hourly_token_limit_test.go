//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// hourlyUsageStub 内存版小时桶计费用量缓存。
// hourlyUsageKey 小时桶维度的用量 key（与 Redis 的「用户 + 整点」组合一致）。
type hourlyUsageKey struct {
	userID int64
	bucket int64
}

type hourlyUsageStub struct {
	BillingCache
	usage     map[hourlyUsageKey]int64
	bonuses   map[int64]int64
	bonusUsed map[int64]int64
	err       error
}

func newHourlyUsageStub() *hourlyUsageStub {
	return &hourlyUsageStub{
		usage:     make(map[hourlyUsageKey]int64),
		bonuses:   make(map[int64]int64),
		bonusUsed: make(map[int64]int64),
	}
}

// setCurrentHourUsage 快捷设置「当前小时」的已用量。
func (s *hourlyUsageStub) setCurrentHourUsage(userID int64, tokens int64) {
	s.usage[hourlyUsageKey{userID, hourlyTokenBucket(hourlyTokenNow())}] = tokens
}

// currentHourUsage 快捷读取「当前小时」的已用量。
func (s *hourlyUsageStub) currentHourUsage(userID int64) int64 {
	return s.usage[hourlyUsageKey{userID, hourlyTokenBucket(hourlyTokenNow())}]
}

func (s *hourlyUsageStub) GetUserHourlyTokenUsage(_ context.Context, userID int64, hourBucket int64) (int64, error) {
	if s.err != nil {
		return 0, s.err
	}
	return s.usage[hourlyUsageKey{userID, hourBucket}], nil
}

func (s *hourlyUsageStub) IncrUserHourlyTokenUsage(_ context.Context, userID int64, hourBucket int64, tokens int64) error {
	if s.err != nil {
		return s.err
	}
	s.usage[hourlyUsageKey{userID, hourBucket}] += tokens
	return nil
}

func (s *hourlyUsageStub) ListUserHourlyTokenUsage(_ context.Context, userID int64, hourBuckets []int64) (map[int64]int64, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make(map[int64]int64, len(hourBuckets))
	for _, bucket := range hourBuckets {
		if used, ok := s.usage[hourlyUsageKey{userID, bucket}]; ok {
			out[bucket] = used
		}
	}
	return out, nil
}

func (s *hourlyUsageStub) GetUserHourlyBonusSnapshot(_ context.Context, _ int64, hourBucket int64) (int64, error) {
	if s.err != nil {
		return 0, s.err
	}
	return s.bonuses[hourBucket], nil
}

func (s *hourlyUsageStub) RecordUserHourlyBonusSnapshot(_ context.Context, _ int64, hourBucket int64, bonusTokens int64) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	// 与 Redis HSetNX 一致：同一小时只写第一次。
	if _, ok := s.bonuses[hourBucket]; ok {
		return false, nil
	}
	s.bonuses[hourBucket] = bonusTokens
	return true, nil
}

func (s *hourlyUsageStub) ListUserHourlyBonusSnapshots(_ context.Context, _ int64, hourBuckets []int64) (map[int64]int64, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make(map[int64]int64, len(hourBuckets))
	for _, bucket := range hourBuckets {
		if bonus, ok := s.bonuses[bucket]; ok {
			out[bucket] = bonus
		}
	}
	return out, nil
}

func (s *hourlyUsageStub) GetUserHourlyBonusUsedTotal(_ context.Context, userID int64) (int64, error) {
	if s.err != nil {
		return 0, s.err
	}
	return s.bonusUsed[userID], nil
}

func (s *hourlyUsageStub) IncrUserHourlyBonusUsedTotal(_ context.Context, userID int64, tokens int64) error {
	if s.err != nil {
		return s.err
	}
	if tokens > 0 {
		s.bonusUsed[userID] += tokens
	}
	return nil
}

type rechargeStub struct {
	recharged bool
	amountUSD float64
	err       error
	calls     int
}

func (s *rechargeStub) HasPaidRecharge(_ context.Context, _ int64) (bool, error) {
	s.calls++
	return s.recharged, s.err
}

func (s *rechargeStub) SumPaidRechargeAmount(_ context.Context, _ int64) (float64, error) {
	if s.err != nil {
		return 0, s.err
	}
	return s.amountUSD, nil
}

// newHourlyTestService 构造一个使用给定开关配置的 BillingCacheService。
func newHourlyTestService(t *testing.T, settings HourlyTokenLimitSettings, usage *hourlyUsageStub, recharge *rechargeStub) *BillingCacheService {
	t.Helper()
	svc := &BillingCacheService{cache: usage}
	svc.settingService = newHourlyTestSettingService(t, settings)
	svc.SetRechargeStatusProvider(recharge)
	return svc
}

// hourlySettingRepo 内存版 settings 仓储，仅实现读取/写入单个 key 所需方法。
type hourlySettingRepo struct {
	SettingRepository
	values map[string]string
}

func (r *hourlySettingRepo) GetValue(_ context.Context, key string) (string, error) {
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (r *hourlySettingRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

// newHourlyTestSettingService 用内存仓储构造 SettingService 并写入给定配置。
func newHourlyTestSettingService(t *testing.T, settings HourlyTokenLimitSettings) *SettingService {
	t.Helper()
	repo := &hourlySettingRepo{values: map[string]string{}}
	svc := &SettingService{settingRepo: repo}
	require.NoError(t, svc.SetHourlyTokenLimitSettings(context.Background(), &settings))
	return svc
}

func TestHourlyTokenBucket(t *testing.T) {
	base := time.Date(2026, 11, 25, 10, 37, 12, 0, time.UTC)
	require.Equal(t, time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC).Unix(), hourlyTokenBucket(base))
	require.Equal(t, 60*22+48, HourlyTokenBucketRemainSeconds(base))
	// 刚好整点时至少返回 1 秒，避免 Retry-After: 0。
	require.Equal(t, 3600, HourlyTokenBucketRemainSeconds(time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)))
}

// 未充值用户：未到上限放行，达到上限返回 429 错误。
func TestHourlyTokenGate_NonRecharged(t *testing.T) {
	usage := newHourlyUsageStub()
	svc := newHourlyTestService(t, HourlyTokenLimitSettings{
		NonRechargedEnabled: true,
		NonRechargedLimit:   100,
	}, usage, &rechargeStub{recharged: false})

	usage.setCurrentHourUsage(1, 99)
	gate, err := svc.HourlyTokenGate(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, gate)
	require.Zero(t, gate.BonusRemaining)

	usage.setCurrentHourUsage(1, 100)
	_, err = svc.HourlyTokenGate(context.Background(), 1)
	require.ErrorIs(t, err, ErrHourlyTokenLimitExceeded)
}

// 开关关闭时不限制未充值用户。
func TestHourlyTokenGate_NonRechargedDisabled(t *testing.T) {
	usage := newHourlyUsageStub()
	svc := newHourlyTestService(t, HourlyTokenLimitSettings{
		NonRechargedEnabled: false,
		NonRechargedLimit:   100,
	}, usage, &rechargeStub{})
	usage.setCurrentHourUsage(1, 10_000)

	gate, err := svc.HourlyTokenGate(context.Background(), 1)
	require.NoError(t, err)
	require.Nil(t, gate)
}

// 已充值用户：赠送额度内免限制，额度用完后仍放行（改为正常扣费）。
func TestHourlyTokenGate_RechargedBonus(t *testing.T) {
	usage := newHourlyUsageStub()
	svc := newHourlyTestService(t, HourlyTokenLimitSettings{
		NonRechargedEnabled:   true,
		NonRechargedLimit:     100,
		RechargedBonusEnabled: true,
		RechargedBonus:        1000,
	}, usage, &rechargeStub{recharged: true})

	usage.setCurrentHourUsage(1, 300)
	gate, err := svc.HourlyTokenGate(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, gate)
	require.EqualValues(t, 700, gate.BonusRemaining)

	// 已用超过赠送额度：不受未充值上限约束，返回 nil 表示全额正常扣费。
	usage.setCurrentHourUsage(1, 1200)
	gate, err = svc.HourlyTokenGate(context.Background(), 1)
	require.NoError(t, err)
	require.Nil(t, gate)
}

// 赠送额度按累计充值金额封顶：充 $0.5 只能解锁 1.5M（每 $1 解锁 3M）。
func TestEffectiveHourlyBonusCappedByRechargeAmount(t *testing.T) {
	settings := HourlyTokenLimitSettings{
		RechargedBonusEnabled: true,
		RechargedBonus:        3_000_000,
		BonusTokensPerUSD:     3_000_000,
	}

	capped := newHourlyTestService(t, settings, newHourlyUsageStub(), &rechargeStub{recharged: true, amountUSD: 0.5})
	bonus, ok := capped.effectiveHourlyBonus(context.Background(), 1)
	require.True(t, ok)
	require.EqualValues(t, 1_500_000, bonus)

	// 充值额足够 → 取配置额度上限。
	enough := newHourlyTestService(t, settings, newHourlyUsageStub(), &rechargeStub{recharged: true, amountUSD: 10})
	bonus, ok = enough.effectiveHourlyBonus(context.Background(), 1)
	require.True(t, ok)
	require.EqualValues(t, 3_000_000, bonus)

	// BonusTokensPerUSD = 0 表示不封顶。
	uncappedSettings := settings
	uncappedSettings.BonusTokensPerUSD = 0
	uncapped := newHourlyTestService(t, uncappedSettings, newHourlyUsageStub(), &rechargeStub{recharged: true, amountUSD: 0.1})
	bonus, ok = uncapped.effectiveHourlyBonus(context.Background(), 1)
	require.True(t, ok)
	require.EqualValues(t, 3_000_000, bonus)

	// 充值额为 0（异常数据）→ 无赠送额度。
	zero := newHourlyTestService(t, settings, newHourlyUsageStub(), &rechargeStub{recharged: true, amountUSD: 0})
	_, ok = zero.effectiveHourlyBonus(context.Background(), 1)
	require.False(t, ok)
}

// 累计封顶：充 $2（换算 6M）可连送两小时，共 6M；第三小时不再赠送。
func TestHourlyBonusCumulativeCap(t *testing.T) {
	usage := newHourlyUsageStub()
	svc := newHourlyTestService(t, HourlyTokenLimitSettings{
		RechargedBonusEnabled: true,
		RechargedBonus:        3_000_000,
		BonusTokensPerUSD:     3_000_000,
	}, usage, &rechargeStub{recharged: true, amountUSD: 2})

	original := hourlyTokenNow
	now := time.Date(2026, 11, 25, 10, 10, 0, 0, time.UTC)
	hourlyTokenNow = func() time.Time { return now }
	t.Cleanup(func() { hourlyTokenNow = original })

	// 第 1 小时用满 3M，全部由赠送抵扣。
	settling := svc.BeginHourlyTokenSettling(context.Background(), 1)
	require.NotNil(t, settling)
	svc.SettleHourlyTokenUsage(context.Background(), 1, 3_000_000, settling)
	require.EqualValues(t, 3_000_000, usage.bonusUsed[1])

	// 第 2 小时仍可再送 3M（累计到 6M，恰好等于充值额换算上限）。
	now = now.Add(time.Hour)
	settling = svc.BeginHourlyTokenSettling(context.Background(), 1)
	require.NotNil(t, settling)
	svc.SettleHourlyTokenUsage(context.Background(), 1, 3_000_000, settling)
	require.EqualValues(t, 6_000_000, usage.bonusUsed[1])

	// 第 3 小时累计已达上限：不再赠送，仅记录普通用量。
	now = now.Add(time.Hour)
	require.Nil(t, svc.BeginHourlyTokenSettling(context.Background(), 1))
	svc.SettleHourlyTokenUsage(context.Background(), 1, 1_000_000, nil)
	require.EqualValues(t, 6_000_000, usage.bonusUsed[1])

	// 再次充值后上限抬高，恢复赠送。
	svc.SetRechargeStatusProvider(&rechargeStub{recharged: true, amountUSD: 3})
	svc.InvalidateHourlyTokenLimitUser(1)
	require.NotNil(t, svc.BeginHourlyTokenSettling(context.Background(), 1))
}

// 同一小时内：赠送抵扣量按「累计上限剩余」封顶，不会超发。
func TestHourlyBonusCumulativeCapWithinHour(t *testing.T) {
	usage := newHourlyUsageStub()
	svc := newHourlyTestService(t, HourlyTokenLimitSettings{
		RechargedBonusEnabled: true,
		RechargedBonus:        3_000_000,
		BonusTokensPerUSD:     3_000_000,
	}, usage, &rechargeStub{recharged: true, amountUSD: 1})

	// 第 1 次：2M 全部抵扣。
	settling := svc.BeginHourlyTokenSettling(context.Background(), 1)
	require.NotNil(t, settling)
	require.Equal(t, 2_000_000, settling.CoveredTokens(2_000_000))
	svc.SettleHourlyTokenUsage(context.Background(), 1, 2_000_000, settling)
	// 发放即扣池：本小时发放 3M，即使只用了 2M，累计池也整份扣 3M。
	require.EqualValues(t, 3_000_000, usage.bonusUsed[1])

	// 第 2 次：本小时已发放 3M，已用 2M，只剩 1M 可抵扣；且不会重复扣池。
	settling = svc.BeginHourlyTokenSettling(context.Background(), 1)
	require.NotNil(t, settling)
	require.Equal(t, 1_000_000, settling.CoveredTokens(2_000_000))
	svc.SettleHourlyTokenUsage(context.Background(), 1, 2_000_000, settling)
	require.EqualValues(t, 3_000_000, usage.bonusUsed[1])

	// 第 3 次：本小时 3M 已用完，无赠送额度；nil 快照也不会虚增累计赠送量。
	require.Nil(t, svc.BeginHourlyTokenSettling(context.Background(), 1))
	svc.SettleHourlyTokenUsage(context.Background(), 1, 2_000_000, nil)
	require.EqualValues(t, 3_000_000, usage.bonusUsed[1])
}

// 只要某小时用了赠送（不管用多少），就扣掉整小时赠送额度；下一小时池子已空则不再赠送。
func TestHourlyBonusGrantChargesWholeHour(t *testing.T) {
	usage := newHourlyUsageStub()
	svc := newHourlyTestService(t, HourlyTokenLimitSettings{
		RechargedBonusEnabled: true,
		RechargedBonus:        3_000_000,
		BonusTokensPerUSD:     3_000_000,
	}, usage, &rechargeStub{recharged: true, amountUSD: 1})

	original := hourlyTokenNow
	now := time.Date(2026, 11, 25, 10, 10, 0, 0, time.UTC)
	hourlyTokenNow = func() time.Time { return now }
	t.Cleanup(func() { hourlyTokenNow = original })

	// 本小时只用了 1000 个 token，也按整小时 3M 扣累计池。
	settling := svc.BeginHourlyTokenSettling(context.Background(), 1)
	require.NotNil(t, settling)
	require.Equal(t, 1000, settling.CoveredTokens(1000))
	svc.SettleHourlyTokenUsage(context.Background(), 1, 1000, settling)
	require.EqualValues(t, 3_000_000, usage.bonusUsed[1])
	require.EqualValues(t, 3_000_000, usage.bonuses[hourlyTokenBucket(now)])

	// 同一小时后续请求仍可用剩余赠送额度，且不重复扣池。
	settling = svc.BeginHourlyTokenSettling(context.Background(), 1)
	require.NotNil(t, settling)
	require.Equal(t, 500_000, settling.CoveredTokens(500_000))
	svc.SettleHourlyTokenUsage(context.Background(), 1, 500_000, settling)
	require.EqualValues(t, 3_000_000, usage.bonusUsed[1])

	// 下一小时：累计池已扣完，不再赠送。
	now = now.Add(time.Hour)
	require.Nil(t, svc.BeginHourlyTokenSettling(context.Background(), 1))
}

// 赠送额度使用记录：按整点倒序，跳过空桶但保留当前小时。
func TestGetHourlyTokenUsageHistory(t *testing.T) {
	usage := newHourlyUsageStub()
	svc := newHourlyTestService(t, HourlyTokenLimitSettings{}, usage, &rechargeStub{})

	original := hourlyTokenNow
	now := time.Date(2026, 11, 25, 10, 20, 0, 0, time.UTC)
	hourlyTokenNow = func() time.Time { return now }
	t.Cleanup(func() { hourlyTokenNow = original })

	current := hourlyTokenBucket(now)
	usage.setCurrentHourUsage(1, 400)
	_, err := usage.RecordUserHourlyBonusSnapshot(context.Background(), 1, current, 3_000_000)
	require.NoError(t, err)
	_, err = usage.RecordUserHourlyBonusSnapshot(context.Background(), 1, current-3600*2, 2_000_000)
	require.NoError(t, err)

	records := svc.GetHourlyTokenUsageHistory(context.Background(), 1, 24)
	require.Len(t, records, 2)
	require.Equal(t, current, records[0].HourBucket)
	require.True(t, records[0].IsCurrent)
	require.EqualValues(t, 3_000_000, records[0].BonusTokens)
	require.EqualValues(t, 400, records[0].UsedTokens)
	require.EqualValues(t, 2_999_600, records[0].RemainingTokens)
	require.Equal(t, current-3600*2, records[1].HourBucket)
	require.False(t, records[1].IsCurrent)
	require.EqualValues(t, 2_000_000, records[1].RemainingTokens)
	// 缓存故障：只保留当前小时的空占位，不报错。
	usage.err = errors.New("redis down")
	broken := svc.GetHourlyTokenUsageHistory(context.Background(), 1, 24)
	require.Len(t, broken, 1)
	require.True(t, broken[0].IsCurrent)
	require.Zero(t, broken[0].BonusTokens)
}

// 赠送开关关闭时已充值用户与普通用户走同一套判定。
func TestHourlyTokenGate_RechargedWithoutBonus(t *testing.T) {
	usage := newHourlyUsageStub()
	svc := newHourlyTestService(t, HourlyTokenLimitSettings{
		NonRechargedEnabled:   true,
		NonRechargedLimit:     100,
		RechargedBonusEnabled: false,
		RechargedBonus:        1000,
	}, usage, &rechargeStub{recharged: true})
	usage.setCurrentHourUsage(1, 100)

	_, err := svc.HourlyTokenGate(context.Background(), 1)
	require.ErrorIs(t, err, ErrHourlyTokenLimitExceeded)
}

// 缓存故障时 fail-open，不阻塞业务。
func TestHourlyTokenGate_CacheErrorFailsOpen(t *testing.T) {
	usage := newHourlyUsageStub()
	usage.err = errors.New("redis down")
	svc := newHourlyTestService(t, HourlyTokenLimitSettings{
		NonRechargedEnabled: true,
		NonRechargedLimit:   100,
	}, usage, &rechargeStub{})

	gate, err := svc.HourlyTokenGate(context.Background(), 1)
	require.NoError(t, err)
	require.Nil(t, gate)
}

func TestHourlyBonusCoveredTokens(t *testing.T) {
	require.Equal(t, 0, HourlyBonusCoveredTokens(0, 0, 100))
	require.Equal(t, 0, HourlyBonusCoveredTokens(500, 500, 100))
	require.Equal(t, 100, HourlyBonusCoveredTokens(400, 500, 100))
	require.Equal(t, 20, HourlyBonusCoveredTokens(480, 500, 100))
}

func TestApplyHourlyTokenBonus(t *testing.T) {
	cost := &CostBreakdown{TotalCost: 1.0, ActualCost: 0.8}

	// 无 settling / 无剩余额度：费用不变（同一指针）。
	require.Same(t, cost, applyHourlyTokenBonus(nil, 100, cost))
	require.Same(t, cost, applyHourlyTokenBonus(&HourlyTokenSettling{remaining: 0}, 100, cost))

	// 全部落在赠送额度内：不扣费，但原始费用保留在成本明细里。
	free := applyHourlyTokenBonus(&HourlyTokenSettling{remaining: 500}, 100, cost)
	require.Zero(t, free.TotalCost)
	require.Zero(t, free.ActualCost)
	require.Equal(t, 1.0, cost.TotalCost, "原始 cost 不应被修改")

	// 部分覆盖：按覆盖比例折算。
	half := applyHourlyTokenBonus(&HourlyTokenSettling{remaining: 50}, 100, cost)
	require.InDelta(t, 0.5, half.TotalCost, 1e-9)
	require.InDelta(t, 0.4, half.ActualCost, 1e-9)
}

func TestBeginHourlyTokenSettling(t *testing.T) {
	usage := newHourlyUsageStub()
	svc := newHourlyTestService(t, HourlyTokenLimitSettings{
		RechargedBonusEnabled: true,
		RechargedBonus:        1000,
	}, usage, &rechargeStub{recharged: true})

	usage.setCurrentHourUsage(7, 200)
	settling := svc.BeginHourlyTokenSettling(context.Background(), 7)
	require.NotNil(t, settling)
	require.Equal(t, 800, settling.CoveredTokens(1000))

	// 额度已用完 → 无需抵扣。
	usage.setCurrentHourUsage(7, 1000)
	require.Nil(t, svc.BeginHourlyTokenSettling(context.Background(), 7))

	// 未充值用户 → 无赠送额度。
	unrecharged := newHourlyTestService(t, HourlyTokenLimitSettings{
		RechargedBonusEnabled: true,
		RechargedBonus:        1000,
	}, newHourlyUsageStub(), &rechargeStub{recharged: false})
	require.Nil(t, unrecharged.BeginHourlyTokenSettling(context.Background(), 1))
}

// 充值额封顶同样作用于计费抵扣：充 $0.5 只能免费用 1.5M。
func TestBeginHourlyTokenSettlingCappedByRechargeAmount(t *testing.T) {
	usage := newHourlyUsageStub()
	svc := newHourlyTestService(t, HourlyTokenLimitSettings{
		RechargedBonusEnabled: true,
		RechargedBonus:        3_000_000,
		BonusTokensPerUSD:     3_000_000,
	}, usage, &rechargeStub{recharged: true, amountUSD: 0.5})

	settling := svc.BeginHourlyTokenSettling(context.Background(), 9)
	require.NotNil(t, settling)
	require.EqualValues(t, 1_500_000, settling.CoveredTokens(3_000_000))
	require.EqualValues(t, 1_500_000, settling.CoveredTokens(2_000_000))
}

// 已充值标记带缓存：同一用户多次判定只回源一次；充值成功后主动失效。
func TestIsUserRechargedCached(t *testing.T) {
	recharge := &rechargeStub{recharged: true}
	svc := newHourlyTestService(t, HourlyTokenLimitSettings{}, newHourlyUsageStub(), recharge)

	require.True(t, svc.isUserRechargedCached(context.Background(), 3))
	require.True(t, svc.isUserRechargedCached(context.Background(), 3))
	require.Equal(t, 1, recharge.calls)

	svc.InvalidateHourlyTokenLimitUser(3)
	require.True(t, svc.isUserRechargedCached(context.Background(), 3))
	require.Equal(t, 2, recharge.calls)
}

func TestSettleHourlyTokenUsage(t *testing.T) {
	usage := newHourlyUsageStub()
	svc := newHourlyTestService(t, HourlyTokenLimitSettings{NonRechargedEnabled: true, NonRechargedLimit: 100}, usage, &rechargeStub{})

	svc.SettleHourlyTokenUsage(context.Background(), 5, 40, nil)
	svc.SettleHourlyTokenUsage(context.Background(), 5, 60, nil)
	require.EqualValues(t, 100, usage.currentHourUsage(5))

	// 开关全部关闭 → 不写入。
	emptyUsage := newHourlyUsageStub()
	off := newHourlyTestService(t, HourlyTokenLimitSettings{}, emptyUsage, &rechargeStub{})
	off.SettleHourlyTokenUsage(context.Background(), 5, 40, nil)
	require.Empty(t, emptyUsage.usage)
}

func TestHourlyTokenLimitSettingsRoundTrip(t *testing.T) {
	repo := &hourlySettingRepo{values: map[string]string{}}
	svc := &SettingService{settingRepo: repo}

	defaults, err := svc.GetHourlyTokenLimitSettings(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 1_000_000, defaults.NonRechargedLimit)
	require.EqualValues(t, 3_000_000, defaults.RechargedBonus)

	saved := HourlyTokenLimitSettings{
		NonRechargedEnabled:   true,
		NonRechargedLimit:     12_345,
		RechargedBonusEnabled: true,
		RechargedBonus:        67_890,
	}
	require.NoError(t, svc.SetHourlyTokenLimitSettings(context.Background(), &saved))

	// 保存后缓存立即生效，不需要等 60s。
	cached := svc.GetHourlyTokenLimitSettingsCached(context.Background())
	require.EqualValues(t, 12_345, cached.NonRechargedLimit)
	require.True(t, cached.NonRechargedEnabled)

	// 未充值上限为负数时归零；其余负数与超上限被拒绝。
	require.NoError(t, svc.SetHourlyTokenLimitSettings(context.Background(), &HourlyTokenLimitSettings{NonRechargedLimit: -1}))
	zeroed, err := svc.GetHourlyTokenLimitSettings(context.Background())
	require.NoError(t, err)
	require.Zero(t, zeroed.NonRechargedLimit)
	require.Error(t, svc.SetHourlyTokenLimitSettings(context.Background(), &HourlyTokenLimitSettings{BonusTokensPerUSD: -1}))
	require.Error(t, svc.SetHourlyTokenLimitSettings(context.Background(), &HourlyTokenLimitSettings{BonusTokensPerUSD: hourlyTokenLimitMaxValue + 1}))
	require.Error(t, svc.SetHourlyTokenLimitSettings(context.Background(), &HourlyTokenLimitSettings{RechargedBonus: hourlyTokenLimitMaxValue + 1}))
}

// 开关开着但额度为 0 视为无实际作用。
func TestHourlyTokenLimitSettingsNormalize(t *testing.T) {
	settings := HourlyTokenLimitSettings{NonRechargedEnabled: true, NonRechargedLimit: 0, RechargedBonusEnabled: true}
	settings.Normalize()
	require.False(t, settings.NonRechargedEnabled)
	require.False(t, settings.RechargedBonusEnabled)
}
