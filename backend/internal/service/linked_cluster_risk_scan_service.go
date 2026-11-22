package service

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// 多维关联团伙风险周期扫描。
//
// 背景：现有规则各自只看单一维度——
//   - 注册 IP 关联：同一 register_ip 达阈值（4）才命中；
//   - 共享使用 IP 扫描：同一 usage_logs.ip_address 达阈值（5）才命中。
//
// 但截图实测的作弊团伙「单维度都不够、组合明显是一伙」：4 个新号里同注册 IP 只有 3 个、
// 同使用 IP 也只有 3 个，两条规则都差一口气温和放过。本服务把「共享注册 IP」和
// 「共享最后使用 IP」当作图的两类边，用并查集求连通分量：只要一个连通分量里有
// ≥3 个新号（且注册时间集中），就判为团伙重罚（suspect + 扣钱 + 邮件；
// 2026-11-21 起不再打 abnormal 自动禁用，由人工复核队列与冻结闸门承担拦截）。
//
// 阈值（用户 2026-11-20 拍板取严格档）：
//   - 连通分量最小成员数 AffiliateRiskLinkedClusterMinSize = 3；
//   - 只统计最近 AffiliateRiskLinkedClusterWindow(24h) 内注册的新号；
//   - 成员年龄上限 AffiliateRiskLinkedClusterMaxAccountDays = 7 天。
//
// 误伤防护：
//   - 复用 RiskExemptionChecker（工单 / 复核放行）在写标与扣钱前双重拦截；
//   - 打标走 BumpUserRiskMarksBatch 的「只升不降 + admin 排除 + SQL 层豁免守卫」；
//   - 扣钱复用 ApplyIPBalancePenalty 的条件 UPDATE（降到 floor 后不再扣），notes 沿用
//     IPAbnormalityMessage 保证复核放行时返还 SQL 能匹配；
//   - DB / 缓存异常一律 fail-open：只记日志，绝不阻断业务。
//
// 幂等：leader 锁选主避免多实例重复扫描；重复扫描对已 abnormal/floor 的用户是 no-op。

const (
	linkedClusterScanInterval = 10 * time.Minute
	// 单轮整体超时须小于 leader 锁 TTL。
	linkedClusterScanTimeout   = 4 * time.Minute
	linkedClusterLeaderLockKey = "linked_cluster:risk:scan:leader"
	linkedClusterLeaderLockTTL = 6 * time.Minute
)

// RecentRegistrationIP 是多维关联团伙检测的一个节点：窗口内注册的新号及其注册 IP。
// 定义在 service 层（接口所属层），由 repository 实现返回的映射。
type RecentRegistrationIP struct {
	UserID     int64
	RegisterIP string
	CreatedAt  time.Time
}

// LinkedClusterNodeLister 返回窗口内注册的新号节点（由 *userRepository 实现）。
type LinkedClusterNodeLister interface {
	ListRecentRegistrationIPs(ctx context.Context, since time.Time) ([]RecentRegistrationIP, error)
}

// LinkedClusterUsedIPLister 返回每个用户最后使用 IP（由 *userRepository 实现，复用现成方法）。
type LinkedClusterUsedIPLister interface {
	GetLatestUsedIPByUserIDs(ctx context.Context, userIDs []int64) (map[int64]string, error)
}

// LinkedClusterMarkWriter 批量写风险标记（由 *AffiliateService 实现）。
type LinkedClusterMarkWriter interface {
	BumpUserRiskMarksBatch(ctx context.Context, marks []UserRiskMark)
}

// LinkedClusterPenalty 扣减到 floor（由 *IPConsistencyPenaltyService 实现）。
type LinkedClusterPenalty interface {
	ApplyIPBalancePenalty(ctx context.Context, userID int64, floor float64, notes string, notify bool) error
}

// LinkedClusterSettings 提供扣减 floor 与豁免查询（由 settingService / riskExemptionChecker 实现）。
type LinkedClusterSettings interface {
	GetIPConsistencySettingsCached(ctx context.Context) IPConsistencySettings
}

// LinkedClusterRiskScanService 周期扫描服务。
type LinkedClusterRiskScanService struct {
	nodes       LinkedClusterNodeLister
	usedIPs     LinkedClusterUsedIPLister
	markWriter  LinkedClusterMarkWriter
	penalty     LinkedClusterPenalty
	exemptCheck RiskExemptionCheck
	settings    LinkedClusterSettings

	interval   time.Duration
	stopCh     chan struct{}
	stopOnce   sync.Once
	wg         sync.WaitGroup
	instanceID string

	lockCache LeaderLockCache
	db        *sql.DB
}

// NewLinkedClusterRiskScanService 创建扫描服务。依赖缺失时对应能力静默跳过。
func NewLinkedClusterRiskScanService(
	nodes LinkedClusterNodeLister,
	usedIPs LinkedClusterUsedIPLister,
	markWriter LinkedClusterMarkWriter,
	penalty LinkedClusterPenalty,
	exemptCheck RiskExemptionCheck,
	settings LinkedClusterSettings,
) *LinkedClusterRiskScanService {
	return &LinkedClusterRiskScanService{
		nodes:       nodes,
		usedIPs:     usedIPs,
		markWriter:  markWriter,
		penalty:     penalty,
		exemptCheck: exemptCheck,
		settings:    settings,
		interval:    linkedClusterScanInterval,
		stopCh:      make(chan struct{}),
		instanceID:  uuid.NewString(),
	}
}

// SetLeaderLock 注入多实例协调后端（cache 优先，db advisory 兜底）。
func (s *LinkedClusterRiskScanService) SetLeaderLock(cache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = cache
	s.db = db
}

// Start 启动周期扫描。首轮立即执行一次，随后按 interval 触发。
func (s *LinkedClusterRiskScanService) Start() {
	if s == nil || s.nodes == nil || s.interval <= 0 {
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
func (s *LinkedClusterRiskScanService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() { close(s.stopCh) })
	s.wg.Wait()
}

func (s *LinkedClusterRiskScanService) runOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), linkedClusterScanTimeout)
	defer cancel()

	release, acquired := tryAcquireSingletonLeaderLock(ctx, s.lockCache, s.db,
		linkedClusterLeaderLockKey, s.instanceID, linkedClusterLeaderLockTTL)
	if !acquired {
		return
	}
	defer release()

	ids := s.detectClusterUserIDs(ctx)
	if len(ids) == 0 {
		return
	}

	floor := DefaultIPBalancePenaltyFloor
	if s.settings != nil {
		floor = s.settings.GetIPConsistencySettingsCached(ctx).BalancePenaltyFloor
	}
	// notes 必须与复核返还 SQL 匹配条件一致，否则团伙扣款在人工复核放行时无法返还。
	notes := IPAbnormalityMessage

	marked, penalized := 0, 0
	for _, userID := range ids {
		if userID <= 0 {
			continue
		}
		if s.exemptCheck != nil && s.exemptCheck.IsExempt(ctx, userID) {
			continue
		}
		if s.markWriter != nil {
			s.markWriter.BumpUserRiskMarksBatch(ctx, []UserRiskMark{{
				UserID:  userID,
				Level:   AffiliateRiskLevelSuspect,
				Reasons: []string{AffiliateRiskReasonLinkedClusterAbuse},
			}})
			marked++
		}
		if s.penalty != nil {
			if err := s.penalty.ApplyIPBalancePenalty(ctx, userID, floor, notes, true); err != nil {
				slog.Warn("linked cluster risk scan: penalty failed",
					"audit", true, "user_id", userID, "error", err)
				continue
			}
			penalized++
		}
	}
	slog.Info("linked cluster risk scan completed",
		"audit", true,
		"candidates", len(ids),
		"marked", marked,
		"penalized", penalized,
		"min_size", AffiliateRiskLinkedClusterMinSize,
	)
}

// detectClusterUserIDs 建图并返回所有属于「达阈值连通分量」的用户 ID。
//
// 节点 = 窗口内注册的新号；边 = 共享 register_ip 或共享最后使用 IP。
// 用并查集求连通分量：分量大小 ≥ AffiliateRiskLinkedClusterMinSize 即全量返回。
// 判定逻辑抽成纯函数 buildLinkedClusters 便于单测。
func (s *LinkedClusterRiskScanService) detectClusterUserIDs(ctx context.Context) []int64 {
	since := time.Now().Add(-AffiliateRiskLinkedClusterWindow)
	nodes, err := s.nodes.ListRecentRegistrationIPs(ctx, since)
	if err != nil {
		slog.Warn("linked cluster risk scan: list nodes failed", "audit", true, "error", err)
		return nil
	}
	if len(nodes) < AffiliateRiskLinkedClusterMinSize {
		return nil
	}

	userIDs := make([]int64, 0, len(nodes))
	for _, n := range nodes {
		userIDs = append(userIDs, n.UserID)
	}
	usedIP := map[int64]string{}
	if s.usedIPs != nil {
		var err error
		usedIP, err = s.usedIPs.GetLatestUsedIPByUserIDs(ctx, userIDs)
		if err != nil {
			slog.Warn("linked cluster risk scan: list used IPs failed", "audit", true, "error", err)
			// 少一个维度仍可继续（只用注册 IP 建图），不直接放弃。
			usedIP = map[int64]string{}
		}
	}

	clusters := buildLinkedClusters(nodes, usedIP, AffiliateRiskLinkedClusterMinSize)
	return clusters
}

// buildLinkedClusters 纯函数：把节点按两类 IP 建并查集，返回达阈值分量的全部 userID。
//
// 参数 nodes 已由调用方按时间窗过滤；minSize 为分量最小成员数。
// 空 IP 不参与建边（避免所有空值被连成一片）。
func buildLinkedClusters(nodes []RecentRegistrationIP, usedIP map[int64]string, minSize int) []int64 {
	if minSize <= 1 || len(nodes) < minSize {
		return nil
	}

	// 并查集：下标与 nodes 对应。
	parent := make([]int, len(nodes))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]] // 路径压缩
			x = parent[x]
		}
		return x
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[rb] = ra
		}
	}

	// 按 IP 值聚合下标：同一 IP 的节点两两相连。
	// 用「IP 值 → 首个下标」，后续同值节点都与首个合并，形成星型即可保证连通。
	regFirst := map[string]int{}
	useFirst := map[string]int{}
	for i, n := range nodes {
		if ip := n.RegisterIP; ip != "" {
			if j, ok := regFirst[ip]; ok {
				union(i, j)
			} else {
				regFirst[ip] = i
			}
		}
		if ip := usedIP[n.UserID]; ip != "" {
			if j, ok := useFirst[ip]; ok {
				union(i, j)
			} else {
				useFirst[ip] = i
			}
		}
	}

	// 统计每个根的成员数。
	size := map[int]int{}
	for i := range nodes {
		size[find(i)]++
	}
	out := make([]int64, 0)
	for i := range nodes {
		if size[find(i)] >= minSize {
			out = append(out, nodes[i].UserID)
		}
	}
	return out
}
