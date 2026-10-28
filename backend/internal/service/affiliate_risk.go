package service

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// =========================
// 邀请注册风险标记（只标记，不拦截）
// =========================
//
// 与 enforceIPGuard 的区别：
//   - enforceIPGuard 会**拦截**注册（不绑定邀请人、不发奖励），是硬风控；
//   - 本文件的规则只往 users.risk_level / risk_reasons 写一个标记，
//     不改变登录、调用、返利任何行为，仅供管理端在用户列表展示并人工研判。
//
// 规则清单（key 见 domain_constants.go 的 AffiliateRiskReason*）：
//  1. invite_burst_24h  邀请人 24h 内邀请数超阈值
//  2. chain_depth       上溯/下溯链深超阈值
//  3. disposable_email  注册邮箱命中一次性邮箱域名
//  4. random_local_part 邮箱本地部分为无意义随机串
//  6. shared_password   多个账号使用完全相同的密码
// 10. chain_contagion   链路上下两跳内存在已判异常的账号
//
// 设计约束：
//   - 标记只升不降：扫描不会把已标记的用户改回正常，避免证据被后续扫描抹掉。
//   - 所有规则共享同一份「原始信号 + 判定函数」，实时路径与存量扫描走完全相同的代码，
//     不存在两套判定逻辑。

// AffiliateRiskSignal 单个用户的邀请注册风险原始信号，由仓储层一次 SQL 聚合得到。
type AffiliateRiskSignal struct {
	UserID         int64
	Email          string
	PasswordHash   string
	// AncestorDepth 上溯深度：该用户上面有几级邀请人（1 = 直接邀请人）。
	AncestorDepth int
	// DescendantDepth 下溯深度：该用户下面有几级被邀请人（1 = 直接下级）。
	DescendantDepth int
	// ChainContagion 上下各 AffiliateRiskContagionRadius 跳内是否存在已判 abnormal 的账号。
	ChainContagion bool
	// InvitesIn24h 该用户作为邀请人，近 24 小时内的邀请数量。
	InvitesIn24h int
	// SharedPasswordCount 使用与该用户完全相同密码的账号数（含自己）。
	SharedPasswordCount int
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

	// 规则 4：邮箱本地部分为随机串 —— 弱特征（也可能是无意义用户名），只判存疑。
	if isRandomLocalPart(sig.Email) {
		reasons = append(reasons, AffiliateRiskReasonRandomLocalPart)
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

	// 规则 1：24h 邀请爆发。注意阈值语义是「超过 N 个」，故用严格大于。
	if sig.InvitesIn24h > AffiliateRiskInviteBurstAbnormal {
		reasons = append(reasons, AffiliateRiskReasonInviteBurst24h)
		raise(AffiliateRiskLevelAbnormal)
	} else if sig.InvitesIn24h > AffiliateRiskInviteBurstSuspect {
		reasons = append(reasons, AffiliateRiskReasonInviteBurst24h)
		raise(AffiliateRiskLevelSuspect)
	}

	// 规则 6：密码哈希复用 —— 批量注册脚本的典型特征。
	if sig.SharedPasswordCount >= AffiliateRiskSharedPasswordAbnormal {
		reasons = append(reasons, AffiliateRiskReasonSharedPassword)
		raise(AffiliateRiskLevelAbnormal)
	} else if sig.SharedPasswordCount >= AffiliateRiskSharedPasswordSuspect {
		reasons = append(reasons, AffiliateRiskReasonSharedPassword)
		raise(AffiliateRiskLevelSuspect)
	}

	// 规则 10：链路传染 —— 上下两跳内已有异常账号，本账号至少存疑。
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

// isRandomLocalPart 判断邮箱本地部分是否像机器生成的随机串。
//
// 判定口径（三项同时满足才命中，尽量压低误报）：
//   - 长度为 10-24（覆盖机器随机串与「长人名+数字」批量命名形态）；
//   - 只含小写字母与数字，且不含任何分隔符；
//   - 数字至少 2 个；
//   - 元音占比 (a/e/i/o/u) 不超过 20% —— 正常姓名/拼音通常在 25% 以上，
//     随机串通常在 10% 以下。
//
// 例：trfajvw9xplr（0.09，命中）、zhangsan2024（0.25，不命中）。
func isRandomLocalPart(email string) bool {
	at := strings.IndexByte(email, '@')
	if at <= 0 {
		return false
	}
	local := strings.ToLower(strings.TrimSpace(email[:at]))
	if len(local) < 10 || len(local) > 24 {
		return false
	}
	letters, digits, vowels := 0, 0, 0
	for _, r := range local {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r >= 'a' && r <= 'z':
			letters++
			switch r {
			case 'a', 'e', 'i', 'o', 'u':
				vowels++
			}
		default:
			// 出现分隔符（. _ - +）说明是有意构造的邮箱，不算随机串。
			return false
		}
	}
	if digits < 2 || letters == 0 {
		return false
	}
	return float64(vowels)/float64(letters) <= 0.2
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
	}
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
	}
}
