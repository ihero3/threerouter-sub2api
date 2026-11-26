package service

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// =========================
// 邀请注册风险标记（suspect 待复核 / abnormal 停用）
// =========================
//
// 与 enforceIPGuard 的区别：
//   - enforceIPGuard 会**拦截**注册（不绑定邀请人、不发奖励），是硬风控；
//   - 本文件的规则往 users.risk_level / risk_reasons 写标记，且标记会真正生效：
//     suspect / abnormal 拦住建 Key 与网关调用（见 User.RiskAccessBlocked）；
//     abnormal 打标时同步把 status 置 disabled（管理员账号除外，见 BumpUserRiskMarks）。
// abnormal 不能通过普通编辑或内容风控解封复活，唯一放行入口是管理员人工复核
// POST /api/v1/admin/users/:id/risk-review（见 AdminReviewUserRisk）。
//
// 规则清单（key 见 domain_constants.go 的 AffiliateRiskReason*）：
//  1. invite_burst_24h  邀请人 24h 内邀请数超阈值（按账号年龄分层：新号更敏感、老号放宽）
//  2. chain_depth       上溯/下溯链深超阈值
//  3. disposable_email  注册邮箱命中一次性邮箱域名
//  4. email_alias_stuffing 邮箱点号别名填充（Gmail 忽略点号，一个邮箱可派生无限地址）
//  5. chain_contagion   链路上/下 1 跳内存在已判异常的账号
//
// 另有两道独立于本引擎、在注册网关实时生效的规则（见 email_risk.go / auth_service.go）：
//  - temp_mail_mx_cluster      解析邮箱域 MX，命中已知一次性邮箱服务集群直接拒绝注册；
//  - registration_ip_correlation 同注册 IP 24h 关联账号数超阈值 → 软性存疑。
//
// 设计约束：
//   - 标记只升不降：扫描不会把已标记的用户改回正常，避免证据被后续扫描抹掉。
//   - 所有规则共享同一份「原始信号 + 判定函数」，实时路径与存量扫描走完全相同的代码，
//     不存在两套判定逻辑。
//   - 全量扫描跳过已人工复核放行的账号（user_risk_review_exemptions，见迁移 253）
//     与已付费用户（payment_orders 真实支付成功，见 ListUserIDsForRiskScan），
//     避免管理员的复核结论被同一批静态证据反复推翻；
//     实时邀请绑定不受此豁免影响，新证据仍会重新打标。

// AffiliateRiskSignal 单个用户的邀请注册风险原始信号，由仓储层一次 SQL 聚合得到。
type AffiliateRiskSignal struct {
	UserID       int64
	Email        string
	PasswordHash string // 保留字段：shared_password 规则下线后已不参与判定，仅仓储层顺带回传。
	// AncestorDepth 上溯深度：该用户上面有几级邀请人（1 = 直接邀请人）。
	AncestorDepth int
	// DescendantDepth 下溯深度：该用户下面有几级被邀请人（1 = 直接下级）。
	DescendantDepth int
	// ChainContagion 上下各 AffiliateRiskContagionRadius 跳内是否存在已判 abnormal 的账号。
	ChainContagion bool
	// InvitesIn24h 该用户作为邀请人，近 24 小时内的邀请数量。
	InvitesIn24h int
	// AccountAgeDays 账号年龄（天），用于邀请爆发分层的"新号/老号"判定。
	AccountAgeDays int
}

// UserRiskMark 待写入 users 表的一条风险标记。
type UserRiskMark struct {
	UserID  int64
	Level   string
	Reasons []string
}

// AffiliateRiskScanResult 存量扫描结果，仅用于回显给管理员。
type AffiliateRiskScanResult struct {
	Scanned  int `json:"scanned"`
	Suspect  int `json:"suspect"`
	Abnormal int `json:"abnormal"`
	Marked   int `json:"marked"`
}

// riskRank 把等级映射为可比较的序号，用于「只升不降」判断。
func riskRank(level string) int {
	switch level {
	case AffiliateRiskLevelAbnormal:
		return 2
	case AffiliateRiskLevelSuspect:
		return 1
	default:
		return 0
	}
}

// evaluateAffiliateRisk 根据原始信号算出风险等级与命中原因。
// 返回的 Level 为 AffiliateRiskLevelNone 时表示无风险，调用方不应写标记。
func evaluateAffiliateRisk(sig AffiliateRiskSignal) (string, []string) {
	level := AffiliateRiskLevelNone
	reasons := make([]string, 0, 6)
	raise := func(l string) {
		if riskRank(l) > riskRank(level) {
			level = l
		}
	}

	// 规则 3：一次性邮箱域名 —— 强特征，直接判异常。
	if isDisposableEmail(sig.Email) {
		reasons = append(reasons, AffiliateRiskReasonDisposableEmail)
		raise(AffiliateRiskLevelAbnormal)
	}

	// 规则 4（原 random_local_part）已下线：对 AI 脚本生成的邮箱无效，且会误伤拼音邮箱。

	// 规则 5：点号别名填充 —— Gmail 忽略点号，同一邮箱可派生无限地址，只判存疑。
	if isStuffedEmailAlias(sig.Email) {
		reasons = append(reasons, AffiliateRiskReasonEmailAliasStuffing)
		raise(AffiliateRiskLevelSuspect)
	}

	// 规则 2：邀请链深度。上溯、下溯任一方向超标即命中。
	maxDepth := sig.AncestorDepth
	if sig.DescendantDepth > maxDepth {
		maxDepth = sig.DescendantDepth
	}
	if maxDepth >= AffiliateRiskChainDepthAbnormal {
		reasons = append(reasons, AffiliateRiskReasonChainDepth)
		raise(AffiliateRiskLevelAbnormal)
	} else if maxDepth >= AffiliateRiskChainDepthSuspect {
		reasons = append(reasons, AffiliateRiskReasonChainDepth)
		raise(AffiliateRiskLevelSuspect)
	}

	// 规则 1：24h 邀请爆发，按账号年龄分层（新号更敏感，老号放宽）。
	// 阈值语义是「超过 N 个」，故用严格大于。
	if sig.AccountAgeDays <= AffiliateRiskNewAccountDays {
		if sig.InvitesIn24h > AffiliateRiskInviteBurstNewAbnormal {
			reasons = append(reasons, AffiliateRiskReasonInviteBurst24h)
			raise(AffiliateRiskLevelAbnormal)
		} else if sig.InvitesIn24h > AffiliateRiskInviteBurstNewSuspect {
			reasons = append(reasons, AffiliateRiskReasonInviteBurst24h)
			raise(AffiliateRiskLevelSuspect)
		}
	} else {
		if sig.InvitesIn24h > AffiliateRiskInviteBurstOldAbnormal {
			reasons = append(reasons, AffiliateRiskReasonInviteBurst24h)
			raise(AffiliateRiskLevelAbnormal)
		} else if sig.InvitesIn24h > AffiliateRiskInviteBurstOldSuspect {
			reasons = append(reasons, AffiliateRiskReasonInviteBurst24h)
			raise(AffiliateRiskLevelSuspect)
		}
	}

	// 规则 10：链路传染 —— 上下 1 跳内已有异常账号，本账号至少存疑。
	if sig.ChainContagion {
		reasons = append(reasons, AffiliateRiskReasonChainContagion)
		raise(AffiliateRiskLevelSuspect)
	}

	return level, reasons
}

// isDisposableEmail 判断邮箱是否属于已知一次性/临时邮箱域名（含子域名）。
func isDisposableEmail(email string) bool {
	domain := emailDomain(email)
	if domain == "" {
		return false
	}
	for _, d := range AffiliateRiskDisposableEmailDomains {
		if domain == d || strings.HasSuffix(domain, "."+d) {
			return true
		}
	}
	return false
}

// isRandomLocalPart 规则已于风控优化中下线：对 AI 脚本生成的邮箱无效（AI 可生成带元音、
// 看似正常的本地部分），且会误伤拼音邮箱，故不再作为判定信号。
// 保留本注释以便追溯，函数实现已删除。

// isStuffedEmailAlias 判断邮箱是否为点号别名填充。
//
// Gmail / Googlemail 会丢弃本地部分的点号，因此
// "a.b.c.d@gmail.com" 与 "abcd@gmail.com" 是同一个邮箱，
// 一个邮箱可派生无限个"新"地址，是小号刷邀请的典型手法。
//
// 判定口径（四项同时满足，尽量压低误报）：
//   - 域名属于忽略点号的服务商（AffiliateRiskDotAliasingDomains）；
//   - 本地部分的点号数 >= AffiliateRiskAliasStuffingMinDots；
//   - 存在一个点号分隔段长度为 1（如 ".i."、".e." 这类单字符填充）；
//   - 本地部分总长 >= AffiliateRiskAliasStuffingMinLength。
//
// 这样 firstname.lastname（1 个点）与 john.q.public（总长 13）这类真实姓名不受影响。
func isStuffedEmailAlias(email string) bool {
	domain := emailDomain(email)
	if domain == "" {
		return false
	}
	matched := false
	for _, d := range AffiliateRiskDotAliasingDomains {
		if domain == d {
			matched = true
			break
		}
	}
	if !matched {
		return false
	}

	local := strings.ToLower(strings.TrimSpace(email[:strings.IndexByte(email, '@')]))
	if len(local) < AffiliateRiskAliasStuffingMinLength {
		return false
	}
	segments := strings.Split(local, ".")
	if len(segments)-1 < AffiliateRiskAliasStuffingMinDots {
		return false
	}
	for _, seg := range segments {
		if len(seg) == 1 {
			return true
		}
	}
	return false
}

// emailDomain 提取小写域名；无 @ 或 @ 在末尾时返回空串。
func emailDomain(email string) string {
	email = strings.TrimSpace(email)
	at := strings.LastIndexByte(email, '@')
	if at < 0 || at == len(email)-1 {
		return ""
	}
	return strings.ToLower(email[at+1:])
}

// emailLocalPart 提取邮箱本地部分（@ 前，不改写大小写）；无 @ 或 @ 在首位时返回空串。
func emailLocalPart(email string) string {
	email = strings.TrimSpace(email)
	at := strings.LastIndexByte(email, '@')
	if at <= 0 {
		return ""
	}
	return email[:at]
}

// evaluateInviterFirstUseRisk 首用时「邀请人」风险判定（用户拍板 2026-11-23）。
//
// 与 evaluateAffiliateRisk 的区别：
//   - 评估对象是邀请人（拉新方），不是被邀请人；被邀请人可能是正常用户，绝不连带处罚；
//   - 新增两条规则：累计邀请数 >10（AffCount，与 24h 窗口无关）与邮箱本地部分 >25；
//   - 命中一律 suspect 档（进复核队列 + 异步扣减 + 邮件），不判 abnormal、不自动停用。
//
// 规则（OR，任一命中即返回）：
//  1. 24h 邀请数 >5 或 累计邀请数 >10
//  2. 上溯/下溯链深 ≥4
//  3. 邮箱本地部分长度 >25
//  4. 保留的旧规则：一次性邮箱 / 别名填充 / 链路传染
func evaluateInviterFirstUseRisk(sig AffiliateRiskSignal, totalInvites int) []string {
	reasons := make([]string, 0, 6)
	if sig.InvitesIn24h > AffiliateRiskInviteBurstNewSuspect || totalInvites > AffiliateRiskInviterTotalInvitesSuspect {
		reasons = append(reasons, AffiliateRiskReasonInviteBurst24h)
	}
	maxDepth := sig.AncestorDepth
	if sig.DescendantDepth > maxDepth {
		maxDepth = sig.DescendantDepth
	}
	if maxDepth >= AffiliateRiskInviterFirstUseChainDepth {
		reasons = append(reasons, AffiliateRiskReasonChainDepth)
	}
	if len(emailLocalPart(sig.Email)) > AffiliateRiskInviterEmailLocalPartMax {
		reasons = append(reasons, AffiliateRiskReasonEmailLocalPartLong)
	}
	if isDisposableEmail(sig.Email) {
		reasons = append(reasons, AffiliateRiskReasonDisposableEmail)
	}
	if isStuffedEmailAlias(sig.Email) {
		reasons = append(reasons, AffiliateRiskReasonEmailAliasStuffing)
	}
	if sig.ChainContagion {
		reasons = append(reasons, AffiliateRiskReasonChainContagion)
	}
	return reasons
}

// EvaluateInviterFirstUseRisk 被邀请人首用时评估其邀请人的风险。
// 返回 (inviterID, reasons)：无邀请人 / 充值豁免 / 未命中 / 采集失败时返回 (0, nil)。
// 全程 fail-open：本函数是鉴权热路径上的旁路评估，任何失败都不影响被邀请人的请求。
func (s *AffiliateService) EvaluateInviterFirstUseRisk(ctx context.Context, inviteeID int64) (int64, []string) {
	if s == nil || s.repo == nil || inviteeID <= 0 {
		return 0, nil
	}
	self, err := s.repo.EnsureUserAffiliate(ctx, inviteeID)
	if err != nil || self == nil || self.InviterID == nil || *self.InviterID <= 0 {
		return 0, nil
	}
	inviterID := *self.InviterID

	// 充值豁免：邀请人有任一成功充值订单即不处罚（用户拍板 2026-11-23）。
	// 查询失败按「不豁免」继续（与 RiskExemptionChecker 同向的 fail-open）。
	if total, err := s.repo.GetCompletedRechargeTotal(ctx, inviterID); err == nil && total > 0 {
		return 0, nil
	}

	signals, err := s.repo.CollectAffiliateRiskSignals(ctx, []int64{inviterID})
	if err != nil || len(signals) == 0 {
		return 0, nil
	}
	inviter, err := s.repo.EnsureUserAffiliate(ctx, inviterID)
	if err != nil || inviter == nil {
		return 0, nil
	}
	reasons := evaluateInviterFirstUseRisk(signals[0], inviter.AffCount)
	if len(reasons) == 0 {
		return 0, nil
	}
	return inviterID, reasons
}

// EvaluateInviteRisk 对给定用户批量评估风险并写入标记（只升不降）。
//
// 失败只记录日志、不返回错误：风险标记是旁路能力，不能影响注册主流程。
func (s *AffiliateService) EvaluateInviteRisk(ctx context.Context, userIDs ...int64) {
	if s == nil || s.repo == nil {
		return
	}
	ids := make([]int64, 0, len(userIDs))
	for _, id := range userIDs {
		if id > 0 {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return
	}
	signals, err := s.repo.CollectAffiliateRiskSignals(ctx, ids)
	if err != nil {
		logger.LegacyPrintf("service.affiliate", "[Affiliate] Collect risk signals failed for %v: %v", ids, err)
		return
	}
	marks := buildRiskMarks(signals)
	if len(marks) == 0 {
		return
	}
	if _, err := s.repo.BumpUserRiskMarks(ctx, marks); err != nil {
		logger.LegacyPrintf("service.affiliate", "[Affiliate] Bump risk marks failed: %v", err)
		return
	}
	// 打标成功后立即失效认证/余额缓存，让 suspect/abnormal 的冻结状态
	// 在下一个请求就生效，不必等 L2 缓存 TTL 自然过期。
	s.invalidateRiskMarkedCaches(ctx, marks)
}

// buildRiskMarks 把原始信号转成待写入的标记列表（过滤掉无风险的用户）。
func buildRiskMarks(signals []AffiliateRiskSignal) []UserRiskMark {
	marks := make([]UserRiskMark, 0, len(signals))
	for _, sig := range signals {
		level, reasons := evaluateAffiliateRisk(sig)
		if level == AffiliateRiskLevelNone || len(reasons) == 0 {
			continue
		}
		marks = append(marks, UserRiskMark{
			UserID:  sig.UserID,
			Level:   level,
			Reasons: reasons,
		})
	}
	return marks
}

// AdminScanAffiliateRisk 全量扫描存量用户并回填风险标记。
//
// 分批按 ID 游标推进，每批一次聚合查询 + 一次批量写入，避免单个超大事务。
// 只升不降：不会清除任何已有标记。
// 已人工复核放行 / 已付费的账号在取 ID 阶段就被排除（迁移 253 + payment_orders），
// 不会被标回待复核；实时邀请绑定仍会独立评估这些账号，属于新增证据的兜底。
func (s *AffiliateService) AdminScanAffiliateRisk(ctx context.Context) (*AffiliateRiskScanResult, error) {
	if s == nil || s.repo == nil {
		return nil, ErrAffiliateProfileNotFound
	}
	result := &AffiliateRiskScanResult{}
	lastID := int64(0)
	for {
		ids, err := s.repo.ListUserIDsForRiskScan(ctx, lastID, AffiliateRiskScanBatchSize)
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return result, nil
		}
		lastID = ids[len(ids)-1]
		result.Scanned += len(ids)

		signals, err := s.repo.CollectAffiliateRiskSignals(ctx, ids)
		if err != nil {
			return nil, err
		}
		marks := buildRiskMarks(signals)
		if len(marks) == 0 {
			continue
		}
		for _, m := range marks {
			switch m.Level {
			case AffiliateRiskLevelAbnormal:
				result.Abnormal++
			case AffiliateRiskLevelSuspect:
				result.Suspect++
			}
		}
		n, err := s.repo.BumpUserRiskMarks(ctx, marks)
		if err != nil {
			return nil, err
		}
		result.Marked += n
		// 只处理本批 marks（每批各自失效一次），不在分批循环里对历史批次重复失效，
		// 避免全量扫描退化成 O(N²) 的缓存 DB 工作。
		s.invalidateRiskMarkedCaches(ctx, marks)
	}
}

// invalidateRiskMarkedCaches 对一次批量打标涉及的用户失效认证缓存与余额缓存。
//
// 复用 AffiliateService.invalidateAffiliateCaches（同包同 struct，无需额外注入）。
// 风险标记是旁路能力：缓存失效失败只记日志、不返回错误，绝不中断打标/扫描主流程。
// 按 UserID 去重，避免全量扫描分批时同一用户被重复失效。
func (s *AffiliateService) invalidateRiskMarkedCaches(ctx context.Context, marks []UserRiskMark) {
	if s == nil {
		return
	}
	seen := make(map[int64]struct{}, len(marks))
	for _, m := range marks {
		if m.UserID <= 0 {
			continue
		}
		if _, ok := seen[m.UserID]; ok {
			continue
		}
		seen[m.UserID] = struct{}{}
		s.invalidateAffiliateCaches(ctx, m.UserID)
	}
}
