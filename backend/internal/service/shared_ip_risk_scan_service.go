package service

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// 共享 IP 风险周期扫描（单 IP 关联过多用户 → 批量打 suspect + 扣减）。
//
// 与 IP 一致性风控（一个用户用了几个 IP）是两个正交维度：
//   - IP 一致性：按用户维度统计窗口内客户端 IP 数，命中打 ip_suspect / 禁用 / 扣钱；
//   - 本扫描：按 IP 维度统计近 10 天关联的不同用户数，命中则把这些用户批量打
//     suspect（进人工复核队列）+ 扣减到 floor。
//
// 阈值与豁免：
//   - 触发阈值 sharedIPRiskMinUsers = 5（原只读展示阈值为 2，放宽以兼容公司/NAT/代理）。
//   - 命中 ticket_flag / 复核豁免表 / IP 白名单的用户跳过（前两者由 BumpUserRiskMarksSQL
//     的豁免守卫 + 本服务的 IsExempt 双重拦截，白名单由设置缓存判断）。
//   - 打标走「只升不降」，不会把 abnormal 降级成 suspect。
//
// 幂等与安全：
//   - 多实例用 leader 锁选主，避免 N 份全表扫描和重复扣减；
//   - 扣减复用 AdminService.ApplyIPBalancePenalty 的条件 UPDATE（降到 floor 后不再扣），
//     即使重复扫描也不会重复扣钱；打标 SQL 只升不降且标记幂等追加；
//   -  DB / 缓存异常一律 fail-open：只记日志，绝不阻断业务。

const (
	sharedIPRiskScanInterval = 10 * time.Minute
	// sharedIPRiskScanTimeout 单轮扫描的整体超时，须小于 leader 锁 TTL。
	sharedIPRiskScanTimeout = 4 * time.Minute
	// sharedIPRiskLeaderLockKey / TTL 保证多实例只有一个跑全表扫描。
	sharedIPRiskLeaderLockKey = "shared_ip:risk:scan:leader"
	sharedIPRiskLeaderLockTTL = 6 * time.Minute
	// sharedIPRiskMinUsers 触发处罚的「同一 IP 关联最少不同用户数」。
	sharedIPRiskMinUsers = 5
	// sharedIPRiskWindow 统计窗口：与 GetSharedUsageIPs 的展示口径一致（10 天）。
	// 最近一次调整：180→10 天，缩短窗口以减少公司/NAT 长期累积命中。
	sharedIPRiskWindow = 10 * 24 * time.Hour
	// sharedIPRiskReason 写入 risk_reasons 的原因码。
	sharedIPRiskReason = "shared_ip_abuse"
)

// SharedIPUserLister 查出命中共享 IP 风险的用户 ID 集合（由 *usageLogRepository 实现）。
type SharedIPUserLister interface {
	ListUserIDsBySharedIP(ctx context.Context, minUsers int, window time.Duration) ([]int64, error)
}

// SharedIPRiskMarkWriter 批量写风险标记（由 *AffiliateService 实现）。
type SharedIPRiskMarkWriter interface {
	BumpUserRiskMarksBatch(ctx context.Context, marks []UserRiskMark)
}

// SharedIPRiskPenalty 扣减到 floor（由 *IPConsistencyPenaltyService 实现，复用 IP 处罚同款）。
type SharedIPRiskPenalty interface {
	ApplyIPBalancePenalty(ctx context.Context, userID int64, floor float64, notes string, notify bool) error
}

// SharedIPRiskScanService 周期扫描服务。
type SharedIPRiskScanService struct {
	userLister  SharedIPUserLister
	markWriter  SharedIPRiskMarkWriter
	penalty     SharedIPRiskPenalty
	exemptCheck RiskExemptionCheck
	settings    IPConsistencySettingsProvider

	interval   time.Duration
	stopCh     chan struct{}
	stopOnce   sync.Once
	wg         sync.WaitGroup
	instanceID string

	lockCache LeaderLockCache
	db        *sql.DB
}

// NewSharedIPRiskScanService 创建扫描服务。依赖缺失时对应能力静默跳过、不影响主循环。
func NewSharedIPRiskScanService(
	lister SharedIPUserLister,
	markWriter SharedIPRiskMarkWriter,
	penalty SharedIPRiskPenalty,
	exemptCheck RiskExemptionCheck,
	settings IPConsistencySettingsProvider,
) *SharedIPRiskScanService {
	return &SharedIPRiskScanService{
		userLister:  lister,
		markWriter:  markWriter,
		penalty:     penalty,
		exemptCheck: exemptCheck,
		settings:    settings,
		interval:    sharedIPRiskScanInterval,
		stopCh:      make(chan struct{}),
		instanceID:  uuid.NewString(),
	}
}

// SetLeaderLock 注入多实例协调后端（cache 优先，db advisory 兜底）。
func (s *SharedIPRiskScanService) SetLeaderLock(cache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = cache
	s.db = db
}

// Start 启动周期扫描。首轮立即执行一次，随后按 interval 触发。
func (s *SharedIPRiskScanService) Start() {
	if s == nil || s.userLister == nil || s.interval <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		s.runOnce()
		for {
			select {
			case <-ticker.C:
				s.runOnce()
			case <-s.stopCh:
				return
			}
		}
	}()
}

// Stop 停止扫描并等待在途的一轮结束。
func (s *SharedIPRiskScanService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() { close(s.stopCh) })
	s.wg.Wait()
}

func (s *SharedIPRiskScanService) runOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), sharedIPRiskScanTimeout)
	defer cancel()

	release, acquired := tryAcquireSingletonLeaderLock(ctx, s.lockCache, s.db, sharedIPRiskLeaderLockKey, s.instanceID, sharedIPRiskLeaderLockTTL)
	if !acquired {
		return
	}
	defer release()

	// minUsers 与窗口暂用常量（运营要后台可调再接设置项）；floor 复用 IP 一致性设置。
	minUsers := sharedIPRiskMinUsers
	ids, err := s.userLister.ListUserIDsBySharedIP(ctx, minUsers, sharedIPRiskWindow)
	if err != nil {
		slog.Warn("shared ip risk scan: list users failed", "audit", true, "error", err)
		return
	}
	if len(ids) == 0 {
		return
	}

	floor := DefaultIPBalancePenaltyFloor
	if s.settings != nil {
		floor = s.settings.GetIPConsistencySettingsCached(ctx).BalancePenaltyFloor
	}

	notes := "The system detected a shared IP address associated with multiple accounts."
	marked := 0
	penalized := 0
	for _, userID := range ids {
		if userID <= 0 {
			continue
		}
		// 全局豁免：工单 / 复核放行 / 白名单命中者跳过，不打标不扣钱。
		if s.exemptCheck != nil && s.exemptCheck.IsExempt(ctx, userID) {
			continue
		}
		if s.markWriter != nil {
			s.markWriter.BumpUserRiskMarksBatch(ctx, []UserRiskMark{{
				UserID:  userID,
				Level:   AffiliateRiskLevelSuspect,
				Reasons: []string{sharedIPRiskReason},
			}})
			marked++
		}
		if s.penalty != nil {
			if err := s.penalty.ApplyIPBalancePenalty(ctx, userID, floor, notes, true); err != nil {
				slog.Warn("shared ip risk scan: penalty failed",
					"audit", true, "user_id", userID, "error", err)
				continue
			}
			penalized++
		}
	}
	slog.Info("shared ip risk scan completed",
		"audit", true,
		"candidates", len(ids),
		"marked", marked,
		"penalized", penalized,
		"min_users", minUsers,
	)
}
