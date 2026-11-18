package service

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// 全局自动风控豁免检查器。
//
// 用户命中以下任一条件时，跳过一切自动处罚（自动标注异常、扣减余额、自动禁用）：
//   - 管理员标记「提过工单」（users.ticket_flag）
//   - 已人工复核放行（user_risk_review_exemptions，迁移 253）
//   - IP 白名单（IPConsistencySettings.WhitelistUserIDs / WhitelistIPs）
//
// 白名单判断本就在 IPConsistencyService.CheckUser 内用进程内设置缓存完成、零 DB，
// 本检查器只负责需要查库的前两项（工单标记 / 复核豁免），并用进程内 TTL 缓存兜住
// 鉴权热路径：一个用户在 TTL 内最多查一次库，绝不让豁免判断拖慢每个 API 请求。
//
// 失效语义：
//   - 管理端 SetUserTicketFlag / AdminReviewUserRisk 成功后主动 Invalidate，立即生效；
//   - 缓存过期（TTL）自然淘汰；
//   - DB / 缓存异常一律 fail-open 视作「不豁免」——风控判定链路不能反过来把
//     扣减/禁用判断整条短路掉；豁免是「额外放行」，缺数据时回到原有处罚逻辑更安全。
//     （注意与 ip_consistency 的 Redis fail-open 不同：那里放行是为了不误伤，
//     这里若 fail-open 成「豁免」会让本该处罚的用户永久逃逸，方向相反。）

const riskExemptionCacheTTL = 30 * time.Second

// RiskExemptionRepository 供豁免检查器查询 DB 的窄接口。
type RiskExemptionRepository interface {
	IsRiskExemptByID(ctx context.Context, id int64) (bool, error)
}

// RiskExemptionChecker 全局自动风控豁免检查器（并发安全）。
type RiskExemptionChecker struct {
	repo RiskExemptionRepository
	ttl  time.Duration

	mu    sync.RWMutex
	cache map[int64]riskExemptionEntry
}

type riskExemptionEntry struct {
	exempt    bool
	expiresAt time.Time
}

// NewRiskExemptionChecker 创建豁免检查器。ttl<=0 时用默认值。
func NewRiskExemptionChecker(repo RiskExemptionRepository, ttl time.Duration) *RiskExemptionChecker {
	if ttl <= 0 {
		ttl = riskExemptionCacheTTL
	}
	return &RiskExemptionChecker{repo: repo, ttl: ttl, cache: make(map[int64]riskExemptionEntry)}
}

// IsExempt 判断用户是否命中工单标记 / 复核放行豁免。
// 命中缓存直接返回；未命中查一次库并写缓存。异常一律 fail-open 为「不豁免」。
func (c *RiskExemptionChecker) IsExempt(ctx context.Context, userID int64) bool {
	if c == nil || c.repo == nil || userID <= 0 {
		return false
	}
	c.mu.RLock()
	entry, ok := c.cache[userID]
	c.mu.RUnlock()
	if ok && time.Now().Before(entry.expiresAt) {
		return entry.exempt
	}

	exempt, err := c.repo.IsRiskExemptByID(ctx, userID)
	if err != nil {
		slog.Warn("risk exemption check failed, fail-open to not exempt",
			"audit", true, "user_id", userID, "error", err)
		return false
	}

	c.mu.Lock()
	// 只覆盖未命中或已过期的项；并发回源时后来者不抢占更新鲜的结果。
	if cur, exists := c.cache[userID]; !exists || !time.Now().Before(cur.expiresAt) {
		c.cache[userID] = riskExemptionEntry{exempt: exempt, expiresAt: time.Now().Add(c.ttl)}
	}
	c.mu.Unlock()
	return exempt
}

// Invalidate 主动失效某用户的豁免缓存。管理端改工单标记 / 复核放行后调用，立即生效。
func (c *RiskExemptionChecker) Invalidate(userID int64) {
	if c == nil {
		return
	}
	c.mu.Lock()
	delete(c.cache, userID)
	c.mu.Unlock()
}
