package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// HourlyTokenLimitSettings 每小时 token 流量控制配置（双开关）。
//
// 两个开关彼此独立：
//   - NonRechargedEnabled：未充值用户每小时 token 上限是否生效；
//   - RechargedBonusEnabled：已充值用户每小时赠送 token 额度是否生效。
//
// 「已充值」口径 = 存在支付成功的余额充值订单（兑换码与订阅订单不算）。
// 未充值用户累计用量超出上限 → 429 拒绝，下一整点自动重置；
// 已充值用户每小时获得一份赠送额度，先用赠送额度抵扣，用完后再正常扣余额，
// 未用完的部分随整点作废，不累积、不补发。
//
// 「赠送不超充值额」：每小时赠送额度同时受「累计充值金额 × BonusTokensPerUSD」封顶，
// 即充 $1 最多解锁 BonusTokensPerUSD 个赠送 token，避免小额充值刷走大额免费额度。
type HourlyTokenLimitSettings struct {
	NonRechargedEnabled   bool  `json:"non_recharged_enabled"`
	NonRechargedLimit     int64 `json:"non_recharged_limit"`
	RechargedBonusEnabled bool  `json:"recharged_bonus_enabled"`
	RechargedBonus        int64 `json:"recharged_bonus"`
	// BonusTokensPerUSD 每 1 美元累计充值可解锁的每小时赠送 token 数（0 = 不按充值额封顶）。
	BonusTokensPerUSD int64 `json:"bonus_tokens_per_usd"`
}

const (
	// hourlyTokenLimitMaxValue 单个额度的取值上限（10 亿 token/小时），防止异常配置失去意义。
	hourlyTokenLimitMaxValue = int64(1_000_000_000)

	hourlyTokenLimitCacheTTL  = 60 * time.Second
	hourlyTokenLimitErrorTTL  = 5 * time.Second
	hourlyTokenLimitDBTimeout = 5 * time.Second
)

// cachedHourlyTokenLimitSettings 进程内缓存条目（60s TTL）。
type cachedHourlyTokenLimitSettings struct {
	settings  HourlyTokenLimitSettings
	expiresAt int64 // unix nano
}

// DefaultHourlyTokenLimitSettings 返回默认配置：未充值用户 100 万 token/小时，
// 已充值用户每小时赠送 300 万 token。与其它限流配置一致，开关默认关闭，需管理员显式开启。
func DefaultHourlyTokenLimitSettings() *HourlyTokenLimitSettings {
	return &HourlyTokenLimitSettings{
		NonRechargedEnabled:   false,
		NonRechargedLimit:     1_000_000,
		RechargedBonusEnabled: false,
		RechargedBonus:        3_000_000,
		BonusTokensPerUSD:     3_000_000,
	}
}

// normalizeHourlyTokenLimitSettings 修正非法取值（负数归零、超上限截断）。
func normalizeHourlyTokenLimitSettings(s *HourlyTokenLimitSettings) {
	if s == nil {
		return
	}
	if s.NonRechargedLimit < 0 {
		s.NonRechargedLimit = 0
	}
	if s.NonRechargedLimit > hourlyTokenLimitMaxValue {
		s.NonRechargedLimit = hourlyTokenLimitMaxValue
	}
	if s.RechargedBonus < 0 {
		s.RechargedBonus = 0
	}
	if s.RechargedBonus > hourlyTokenLimitMaxValue {
		s.RechargedBonus = hourlyTokenLimitMaxValue
	}
	if s.BonusTokensPerUSD < 0 {
		s.BonusTokensPerUSD = 0
	}
	if s.BonusTokensPerUSD > hourlyTokenLimitMaxValue {
		s.BonusTokensPerUSD = hourlyTokenLimitMaxValue
	}
}

// Normalize 把「开关开启但额度 <= 0」视为该开关无实际作用。
func (s *HourlyTokenLimitSettings) Normalize() {
	if s == nil {
		return
	}
	if s.NonRechargedLimit <= 0 {
		s.NonRechargedEnabled = false
	}
	if s.RechargedBonus <= 0 {
		s.RechargedBonusEnabled = false
	}
}

// GetHourlyTokenLimitSettings 直读 DB 获取配置（供管理端读写路径使用）。
// 缺失/空/解析失败 → 返回默认配置。
func (s *SettingService) GetHourlyTokenLimitSettings(ctx context.Context) (*HourlyTokenLimitSettings, error) {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyHourlyTokenLimitSettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return DefaultHourlyTokenLimitSettings(), nil
		}
		return nil, fmt.Errorf("get hourly token limit settings: %w", err)
	}
	if strings.TrimSpace(value) == "" {
		return DefaultHourlyTokenLimitSettings(), nil
	}
	settings := &HourlyTokenLimitSettings{}
	if err := json.Unmarshal([]byte(value), settings); err != nil {
		slog.Warn("failed to unmarshal hourly token limit settings, falling back to defaults",
			"error", err, "key", SettingKeyHourlyTokenLimitSettings)
		return DefaultHourlyTokenLimitSettings(), nil
	}
	normalizeHourlyTokenLimitSettings(settings)
	return settings, nil
}

// SetHourlyTokenLimitSettings 保存配置并立即刷新进程内缓存，
// 使当前节点的下一个请求即生效（多节点部署最迟 60s 内生效）。
func (s *SettingService) SetHourlyTokenLimitSettings(ctx context.Context, settings *HourlyTokenLimitSettings) error {
	if settings == nil {
		return fmt.Errorf("settings cannot be nil")
	}
	if settings.RechargedBonus < 0 {
		return fmt.Errorf("token limits cannot be negative")
	}
	if settings.BonusTokensPerUSD < 0 {
		return fmt.Errorf("bonus tokens per usd cannot be negative")
	}
	if settings.NonRechargedLimit > hourlyTokenLimitMaxValue {
		return fmt.Errorf("non recharged limit must be at most %d", hourlyTokenLimitMaxValue)
	}
	if settings.RechargedBonus > hourlyTokenLimitMaxValue {
		return fmt.Errorf("recharged bonus must be at most %d", hourlyTokenLimitMaxValue)
	}
	if settings.BonusTokensPerUSD > hourlyTokenLimitMaxValue {
		return fmt.Errorf("bonus tokens per usd must be at most %d", hourlyTokenLimitMaxValue)
	}
	normalizeHourlyTokenLimitSettings(settings)

	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal hourly token limit settings: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyHourlyTokenLimitSettings, string(data)); err != nil {
		return err
	}
	s.storeHourlyTokenLimitCache(*settings, hourlyTokenLimitCacheTTL)
	return nil
}

// GetHourlyTokenLimitSettingsCached 返回配置（进程内缓存，60s TTL）。
// 网关每个请求的热路径都会调用，绝不能每次访问 DB；DB 错误时沿用最近一次已知值。
func (s *SettingService) GetHourlyTokenLimitSettingsCached(ctx context.Context) HourlyTokenLimitSettings {
	if s == nil || s.settingRepo == nil {
		return *DefaultHourlyTokenLimitSettings()
	}
	if cached, ok := s.hourlyTokenLimitCache.Load().(*cachedHourlyTokenLimitSettings); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached.settings
		}
	}

	result, _, _ := s.hourlyTokenLimitSF.Do("hourly_token_limit_settings", func() (any, error) {
		if cached, ok := s.hourlyTokenLimitCache.Load().(*cachedHourlyTokenLimitSettings); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return cached.settings, nil
			}
		}
		if ctx == nil {
			ctx = context.Background()
		}
		// 独立 context：断开请求取消链，避免客户端断连污染缓存。
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), hourlyTokenLimitDBTimeout)
		defer cancel()

		settings, err := s.GetHourlyTokenLimitSettings(dbCtx)
		if err != nil {
			slog.Warn("failed to get hourly token limit settings", "error", err)
			fallback := *DefaultHourlyTokenLimitSettings()
			if prior, ok := s.hourlyTokenLimitCache.Load().(*cachedHourlyTokenLimitSettings); ok && prior != nil {
				fallback = prior.settings
			}
			s.storeHourlyTokenLimitCache(fallback, hourlyTokenLimitErrorTTL)
			return fallback, nil
		}
		s.storeHourlyTokenLimitCache(*settings, hourlyTokenLimitCacheTTL)
		return *settings, nil
	})
	if settings, ok := result.(HourlyTokenLimitSettings); ok {
		return settings
	}
	return *DefaultHourlyTokenLimitSettings()
}

func (s *SettingService) storeHourlyTokenLimitCache(settings HourlyTokenLimitSettings, ttl time.Duration) {
	s.hourlyTokenLimitCache.Store(&cachedHourlyTokenLimitSettings{
		settings:  settings,
		expiresAt: time.Now().Add(ttl).UnixNano(),
	})
}

// ============================================================================
// 判定与结算
// ============================================================================

// ErrHourlyTokenLimitExceeded 未充值用户超出每小时 token 上限（下一整点自动恢复）。
var ErrHourlyTokenLimitExceeded = infraerrors.TooManyRequests(
	"HOURLY_TOKEN_LIMIT_EXCEEDED",
	"未充值账号每小时 token 额度已用完，下一整点自动恢复；充值后可解除该限制并每小时获得额外赠送额度",
)

// HourlyTokenUsageCache 提供「用户 + 小时桶」的 token 用量计数（Redis 实现）。
type HourlyTokenUsageCache interface {
	GetUserHourlyTokenUsage(ctx context.Context, userID int64, hourBucket int64) (int64, error)
	IncrUserHourlyTokenUsage(ctx context.Context, userID int64, hourBucket int64, tokens int64) error
	// ListUserHourlyTokenUsage 批量读取若干小时桶的已用 token（缺失按 0）。
	ListUserHourlyTokenUsage(ctx context.Context, userID int64, hourBuckets []int64) (map[int64]int64, error)
	// RecordUserHourlyBonusSnapshot 记录某小时的赠送额度快照（Usage 页展示用）。
	RecordUserHourlyBonusSnapshot(ctx context.Context, userID int64, hourBucket int64, bonusTokens int64) error
	// ListUserHourlyBonusSnapshots 读取近若干小时的赠送额度快照（缺失按 0）。
	ListUserHourlyBonusSnapshots(ctx context.Context, userID int64, hourBuckets []int64) (map[int64]int64, error)
}

// RechargeStatusProvider 判断用户的余额充值情况。
// 口径与订单管理的「已充值」一致（paid_at 非空 + PAID/RECHARGING/COMPLETED），
// 但只认余额充值单，兑换码与订阅订单不算。
type RechargeStatusProvider interface {
	HasBalanceRecharge(ctx context.Context, userID int64) (bool, error)
	// SumBalanceRechargeAmount 返回用户累计成功充值的金额（美元），用于封顶每小时赠送额度。
	SumBalanceRechargeAmount(ctx context.Context, userID int64) (float64, error)
}

const (
	// hourlyTokenRechargeCacheTTL 已充值标记的进程内缓存时长。
	hourlyTokenRechargeCacheTTL = 2 * time.Minute
	// hourlyTokenRechargeNegativeTTL 查询失败时对「未充值」结论的短 TTL，快速重试。
	hourlyTokenRechargeNegativeTTL = 10 * time.Second
)

// hourlyTokenNow 便于测试替换时钟。
var hourlyTokenNow = time.Now

// hourlyTokenBucket 返回时间所属的小时桶（整点 unix 秒）。
// 用整点做 key 天然实现「下一小时清零」：新整点换 key，旧 key 自然过期。
func hourlyTokenBucket(t time.Time) int64 {
	return t.Truncate(time.Hour).Unix()
}

// HourlyTokenBucketRemainSeconds 返回距离下一整点自动重置的秒数（至少 1）。
func HourlyTokenBucketRemainSeconds(t time.Time) int {
	next := t.Truncate(time.Hour).Add(time.Hour)
	remain := int(next.Sub(t).Seconds())
	if remain <= 0 {
		return 1
	}
	return remain
}

// hourlyTokenLimitSettings 读取配置；settingService 缺失时按全关闭处理（不改变既有行为）。
func (s *BillingCacheService) hourlyTokenLimitSettings(ctx context.Context) HourlyTokenLimitSettings {
	if s == nil || s.settingService == nil {
		return HourlyTokenLimitSettings{}
	}
	settings := s.settingService.GetHourlyTokenLimitSettingsCached(ctx)
	if !settings.NonRechargedEnabled && !settings.RechargedBonusEnabled {
		return HourlyTokenLimitSettings{}
	}
	settings.Normalize()
	return settings
}

type hourlyTokenRechargeEntry struct {
	recharged bool
	// amountUSD 累计成功充值金额，用于按充值额封顶赠送额度。
	amountUSD float64
	expiresAt time.Time
}

// rechargeStatusCache 带 TTL 的进程内「已充值」标记缓存，支持并发读写。
type rechargeStatusCache struct {
	mu      sync.RWMutex
	entries map[int64]hourlyTokenRechargeEntry
}

func (c *rechargeStatusCache) Load(userID int64) (hourlyTokenRechargeEntry, bool) {
	if c == nil {
		return hourlyTokenRechargeEntry{}, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[userID]
	return entry, ok
}

func (c *rechargeStatusCache) Store(userID int64, entry hourlyTokenRechargeEntry) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = make(map[int64]hourlyTokenRechargeEntry)
	}
	c.entries[userID] = entry
}

func (c *rechargeStatusCache) Delete(userID int64) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, userID)
}

// rechargeInfoCached 返回用户充值情况（进程内缓存 + singleflight 回源）。
// 查询失败按「未充值、0 金额」处理并短 TTL 缓存，避免 DB 抖动把已充值用户永久降级。
func (s *BillingCacheService) rechargeInfoCached(ctx context.Context, userID int64) hourlyTokenRechargeEntry {
	if s == nil || s.rechargeStatus == nil || userID <= 0 {
		return hourlyTokenRechargeEntry{}
	}
	now := hourlyTokenNow()
	if entry, ok := s.rechargeCache.Load(userID); ok && now.Before(entry.expiresAt) {
		return entry
	}
	key := "hourly_recharge_" + strconv.FormatInt(userID, 10)
	value, err, _ := s.rechargeSF.Do(key, func() (any, error) {
		if entry, ok := s.rechargeCache.Load(userID); ok && now.Before(entry.expiresAt) {
			return entry, nil
		}
		loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), hourlyTokenLimitDBTimeout)
		defer cancel()
		recharged, err := s.rechargeStatus.HasBalanceRecharge(loadCtx, userID)
		if err != nil {
			return hourlyTokenRechargeEntry{}, err
		}
		entry := hourlyTokenRechargeEntry{recharged: recharged}
		if recharged {
			amount, amountErr := s.rechargeStatus.SumBalanceRechargeAmount(loadCtx, userID)
			if amountErr != nil {
				// 金额查询失败按 0 处理：宁可少发赠送，也不放宽风控。
				logger.LegacyPrintf("service.billing_cache",
					"Warning: hourly token recharge amount lookup failed for user %d: %v", userID, amountErr)
			} else if amount > 0 {
				entry.amountUSD = amount
			}
		}
		entry.expiresAt = now.Add(hourlyTokenRechargeCacheTTL)
		s.rechargeCache.Store(userID, entry)
		return entry, nil
	})
	if err != nil {
		logger.LegacyPrintf("service.billing_cache",
			"Warning: hourly token recharge status lookup failed for user %d: %v", userID, err)
		entry := hourlyTokenRechargeEntry{expiresAt: now.Add(hourlyTokenRechargeNegativeTTL)}
		s.rechargeCache.Store(userID, entry)
		return entry
	}
	entry, _ := value.(hourlyTokenRechargeEntry)
	return entry
}

// isUserRechargedCached 判断用户是否已充值。
func (s *BillingCacheService) isUserRechargedCached(ctx context.Context, userID int64) bool {
	return s.rechargeInfoCached(ctx, userID).recharged
}

// effectiveHourlyBonus 返回已充值用户本小时的赠送额度（受累计充值金额封顶）。
// ok=false 表示该用户当前不享受赠送额度（未充值 / 开关关闭 / 封顶后为 0）。
func (s *BillingCacheService) effectiveHourlyBonus(ctx context.Context, userID int64) (int64, bool) {
	settings := s.hourlyTokenLimitSettings(ctx)
	if !settings.RechargedBonusEnabled {
		return 0, false
	}
	info := s.rechargeInfoCached(ctx, userID)
	if !info.recharged {
		return 0, false
	}
	bonus := settings.RechargedBonus
	// 赠送不超充值额：充 $1 最多解锁 BonusTokensPerUSD 个赠送 token。
	if settings.BonusTokensPerUSD > 0 {
		capped := int64(info.amountUSD * float64(settings.BonusTokensPerUSD))
		if capped < bonus {
			bonus = capped
		}
	}
	if bonus <= 0 {
		return 0, false
	}
	return bonus, true
}

// InvalidateHourlyTokenLimitUser 充值成功后调用：立即失效「已充值」标记，
// 让用户无需等待缓存过期即可获得每小时赠送额度。
func (s *BillingCacheService) InvalidateHourlyTokenLimitUser(userID int64) {
	if s == nil || userID <= 0 {
		return
	}
	s.rechargeCache.Delete(userID)
}

// HourlyTokenGateResult 请求准入阶段的小时级 token 判定结果。
// 返回 nil 表示本次请求不受流量控制影响（开关关闭 / 已充值且赠送额度已用完）。
type HourlyTokenGateResult struct {
	// BonusRemaining 判定时本小时剩余的赠送额度 token 数（已充值且赠送开关开启时 > 0）。
	BonusRemaining int64
}

// HourlyTokenGate 请求准入判定：未充值用户超出上限返回 ErrHourlyTokenLimitExceeded。
func (s *BillingCacheService) HourlyTokenGate(ctx context.Context, userID int64) (*HourlyTokenGateResult, error) {
	if s == nil || s.cache == nil || userID <= 0 {
		return nil, nil
	}
	settings := s.hourlyTokenLimitSettings(ctx)
	if !settings.NonRechargedEnabled && !settings.RechargedBonusEnabled {
		return nil, nil
	}
	bucket := hourlyTokenBucket(hourlyTokenNow())
	used, err := s.cache.GetUserHourlyTokenUsage(ctx, userID, bucket)
	if err != nil {
		// Redis 抖动不阻塞业务：本次请求不参与流量控制。
		logger.LegacyPrintf("service.billing_cache",
			"Warning: hourly token usage read failed for user %d: %v", userID, err)
		return nil, nil
	}

	// 已充值用户走赠送额度逻辑，永远不受「未充值上限」约束：
	// 即使赠送额度（可能被充值额封顶）为 0 或用完，也只是恢复正常扣费。
	if settings.RechargedBonusEnabled && s.isUserRechargedCached(ctx, userID) {
		bonus, ok := s.effectiveHourlyBonus(ctx, userID)
		if !ok {
			return nil, nil
		}
		remaining := bonus - used
		if remaining <= 0 {
			// 赠送额度已用完：后续请求正常扣余额，不做任何限制。
			return nil, nil
		}
		return &HourlyTokenGateResult{BonusRemaining: remaining}, nil
	}

	if !settings.NonRechargedEnabled {
		return nil, nil
	}
	if used >= settings.NonRechargedLimit {
		return nil, ErrHourlyTokenLimitExceeded
	}
	return &HourlyTokenGateResult{}, nil
}

// SettleHourlyTokenUsage 后扣阶段记录本次实际 token 用量（用户维度累计，含订阅用户）。
// 累计值既支撑未充值用户的上限判定，也决定已充值用户本小时还剩多少赠送额度。
func (s *BillingCacheService) SettleHourlyTokenUsage(ctx context.Context, userID int64, actualTokens int) {
	if s == nil || s.cache == nil || userID <= 0 || actualTokens <= 0 {
		return
	}
	settings := s.hourlyTokenLimitSettings(ctx)
	if !settings.NonRechargedEnabled && !settings.RechargedBonusEnabled {
		return
	}
	bucket := hourlyTokenBucket(hourlyTokenNow())
	// 记录本小时的赠送额度快照，供 Usage 页展示「赠送了多少 / 用了多少 / 剩多少」。
	bonus := int64(0)
	if b, ok := s.effectiveHourlyBonus(ctx, userID); ok {
		bonus = b
	}
	if err := s.cache.IncrUserHourlyTokenUsage(ctx, userID, bucket, int64(actualTokens)); err != nil {
		logger.LegacyPrintf("service.billing_cache",
			"Warning: hourly token usage incr failed for user %d: %v", userID, err)
	}
	if bonus > 0 {
		s.recordHourlyBonusSnapshot(ctx, userID, bucket, bonus)
	}
}

// HourlyTokenSettling 已充值用户本次请求的赠送额度抵扣快照（后扣开始时取一次）。
type HourlyTokenSettling struct {
	// used 本次请求开始前本小时已用 token 数。
	used int64
	// bonus 每小时赠送额度。
	bonus int64
}

// BeginHourlyTokenSettling 读取用户本小时已用 token 与赠送额度，供后扣计算抵扣比例。
// 返回 nil 表示无需抵扣：开关关闭、用户未充值、赠送额度（含充值额封顶后）为 0、
// 已用完或读取失败。
func (s *BillingCacheService) BeginHourlyTokenSettling(ctx context.Context, userID int64) *HourlyTokenSettling {
	if s == nil || s.cache == nil || userID <= 0 {
		return nil
	}
	settings := s.hourlyTokenLimitSettings(ctx)
	if !settings.RechargedBonusEnabled {
		return nil
	}
	if !s.isUserRechargedCached(ctx, userID) {
		return nil
	}
	bonus, ok := s.effectiveHourlyBonus(ctx, userID)
	if !ok {
		return nil
	}
	used, err := s.cache.GetUserHourlyTokenUsage(ctx, userID, hourlyTokenBucket(hourlyTokenNow()))
	if err != nil {
		logger.LegacyPrintf("service.billing_cache",
			"Warning: hourly token usage read failed for user %d: %v", userID, err)
		return nil
	}
	if used >= bonus {
		return nil
	}
	return &HourlyTokenSettling{used: used, bonus: bonus}
}

// CoveredTokens 返回本次实际 token 中可由赠送额度覆盖的数量（按剩余额度封顶）。
func (h *HourlyTokenSettling) CoveredTokens(actualTokens int) int {
	if h == nil || actualTokens <= 0 {
		return 0
	}
	return HourlyBonusCoveredTokens(h.used, h.bonus, actualTokens)
}

// applyHourlyTokenBonus 按赠送额度覆盖比例扣减本次请求费用。
// 覆盖部分不扣余额/订阅额度，超出部分照常计费；token 单价在整单内一致，
// 因此按 token 比例折算金额即可。CacheStorage 等非 token 附加费不参与抵扣。
func applyHourlyTokenBonus(settling *HourlyTokenSettling, actualTokens int, cost *CostBreakdown) *CostBreakdown {
	if cost == nil || settling == nil || actualTokens <= 0 {
		return cost
	}
	covered := settling.CoveredTokens(actualTokens)
	if covered <= 0 {
		return cost
	}
	if covered >= actualTokens {
		bonus := *cost
		bonus.TotalCost = 0
		bonus.ActualCost = 0
		return &bonus
	}
	billed := float64(actualTokens-covered) / float64(actualTokens)
	bonus := *cost
	bonus.TotalCost = cost.TotalCost * billed
	bonus.ActualCost = cost.ActualCost * billed
	return &bonus
}

// usageLogTokens 返回一次请求消耗的原始 token 总数（输入 + 输出 + 缓存读写与创建 + 图片输出），
// 不含图片张数与搜索次数——流量控制只按原始 token 计量。
func usageLogTokens(result *ForwardResult) int {
	if result == nil {
		return 0
	}
	u := result.Usage
	return u.InputTokens + u.OutputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens + u.ImageOutputTokens
}

// HourlyBonusCoveredTokens 计算本次请求有多少 token 由每小时赠送额度覆盖（0 = 全额扣费）。
// used 为本小时已用用量，bonus 为赠送额度，剩余额度即两者之差。
func HourlyBonusCoveredTokens(used int64, bonus int64, actualTokens int) int {
	if bonus <= 0 || actualTokens <= 0 {
		return 0
	}
	remaining := bonus - used
	if remaining <= 0 {
		return 0
	}
	if int64(actualTokens) > remaining {
		return int(remaining)
	}
	return actualTokens
}

// HourlyTokenUsageRecord 单小时的赠送额度使用记录（Usage 页展示用）。
type HourlyTokenUsageRecord struct {
	// HourBucket 整点 unix 秒，前端按本地时区渲染。
	HourBucket int64 `json:"hour_bucket"`
	// BonusTokens 该小时发放的赠送额度（未充值或开关关闭时为 0）。
	BonusTokens int64 `json:"bonus_tokens"`
	// UsedTokens 该小时累计消耗的原始 token 数。
	UsedTokens int64 `json:"used_tokens"`
	// RemainingTokens 该小时剩余未用的赠送额度（作废后归零，不会累积到下一小时）。
	RemainingTokens int64 `json:"remaining_tokens"`
	// IsCurrent 是否为当前正在进行的小时。
	IsCurrent bool `json:"is_current"`
}

func (s *BillingCacheService) recordHourlyBonusSnapshot(ctx context.Context, userID int64, bucket int64, bonusTokens int64) {
	if s == nil || s.cache == nil || bonusTokens <= 0 {
		return
	}
	if err := s.cache.RecordUserHourlyBonusSnapshot(ctx, userID, bucket, bonusTokens); err != nil {
		logger.LegacyPrintf("service.billing_cache",
			"Warning: hourly bonus snapshot write failed for user %d: %v", userID, err)
	}
}

// GetHourlyTokenUsageHistory 返回最近 hours 个小时的赠送额度使用记录（按时间倒序）。
// 数据来源：Redis 小时桶用量 + 赠送额度快照；读取失败返回空列表而不报错。
func (s *BillingCacheService) GetHourlyTokenUsageHistory(ctx context.Context, userID int64, hours int) []HourlyTokenUsageRecord {
	if s == nil || s.cache == nil || userID <= 0 {
		return []HourlyTokenUsageRecord{}
	}
	if hours <= 0 || hours > 168 {
		hours = 24
	}
	now := hourlyTokenNow()
	currentBucket := hourlyTokenBucket(now)
	buckets := make([]int64, 0, hours)
	for i := 0; i < hours; i++ {
		buckets = append(buckets, currentBucket-int64(i)*3600)
	}

	usage, err := s.cache.ListUserHourlyTokenUsage(ctx, userID, buckets)
	if err != nil {
		logger.LegacyPrintf("service.billing_cache",
			"Warning: hourly token usage history read failed for user %d: %v", userID, err)
		usage = nil
	}
	bonuses, err := s.cache.ListUserHourlyBonusSnapshots(ctx, userID, buckets)
	if err != nil {
		logger.LegacyPrintf("service.billing_cache",
			"Warning: hourly bonus history read failed for user %d: %v", userID, err)
		bonuses = nil
	}

	records := make([]HourlyTokenUsageRecord, 0, hours)
	for _, bucket := range buckets {
		used := usage[bucket]
		bonus := bonuses[bucket]
		if used <= 0 && bonus <= 0 && bucket != currentBucket {
			continue
		}
		remaining := bonus - used
		if remaining < 0 {
			remaining = 0
		}
		records = append(records, HourlyTokenUsageRecord{
			HourBucket:      bucket,
			BonusTokens:     bonus,
			UsedTokens:      used,
			RemainingTokens: remaining,
			IsCurrent:       bucket == currentBucket,
		})
	}
	return records
}
