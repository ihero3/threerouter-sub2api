package service

import (
	"context"
	"fmt"
	"log/slog"
)

// ipBalancePenaltyAdmin 是 adminServiceImpl 提供的可选能力。
// 用窄接口 + 类型断言，不往 AdminService 大接口加方法，避免所有 handler 测试桩被迫实现。
type ipBalancePenaltyAdmin interface {
	ApplyIPBalancePenalty(ctx context.Context, userID int64, floor float64, notes string) (email string, applied bool, err error)
}

// ipPenaltyEmailSender 统一出站邮件入口（*EmailService 实现）。
// 抽成窄接口便于在单测里注入假发信器，验证「邮件失败不回滚处罚」等分支。
type ipPenaltyEmailSender interface {
	SendEmail(ctx context.Context, to, subject, body string) error
}

// IPConsistencyPenaltyService 把「IP 异常」处罚封装成可注入 IP 一致性风控的执行器。
//
// 余额与调账流水走 AdminService 的原子能力（与后台「- 退款」同款），通知邮件走
// 统一出站入口 EmailService。整个执行器由 CheckUser 起后台 goroutine 调用，
// 因此这里的同步 DB / SMTP 操作都不会阻塞登录或模型网关鉴权热路径。
type IPConsistencyPenaltyService struct {
	admin        ipBalancePenaltyAdmin
	emailService ipPenaltyEmailSender
}

var _ IPConsistencyPenalty = (*IPConsistencyPenaltyService)(nil)

// NewIPConsistencyPenaltyService 创建处罚执行器。
// adminService 是 AdminService 接口；取其可选能力失败时 admin 为 nil，
// ApplyIPBalancePenalty 会返回错误让上层只记日志、绝不阻断请求（fail-open）。
func NewIPConsistencyPenaltyService(adminService AdminService, emailService *EmailService) *IPConsistencyPenaltyService {
	var admin ipBalancePenaltyAdmin
	if candidate, ok := adminService.(ipBalancePenaltyAdmin); ok {
		admin = candidate
	}
	return &IPConsistencyPenaltyService{admin: admin, emailService: emailService}
}

// ApplyIPBalancePenalty 实现 IPConsistencyPenalty。
//
//   - 只有确实扣到钱（applied=true）且 notify=true 时才发信，避免余额已达标时重复骚扰用户；
//   - 邮件失败只告警、不返回错误：余额与流水已落库，返回错误会让上层释放幂等锁，
//     后续请求可能重复处罚。
func (p *IPConsistencyPenaltyService) ApplyIPBalancePenalty(ctx context.Context, userID int64, floor float64, notes string, notify bool) error {
	if p == nil || p.admin == nil {
		return fmt.Errorf("ip balance penalty admin capability unavailable")
	}
	email, applied, err := p.admin.ApplyIPBalancePenalty(ctx, userID, floor, notes)
	if err != nil {
		return fmt.Errorf("apply ip balance penalty: %w", err)
	}
	if !applied {
		slog.Info("ip balance penalty skipped: balance already at or below floor",
			"audit", true, "user_id", userID, "floor", floor)
		return nil
	}
	slog.Warn("ip abnormality penalty applied",
		"audit", true, "user_id", userID, "floor", floor, "notes", notes)

	if !notify || p.emailService == nil {
		return nil
	}
	if email == "" {
		slog.Warn("ip balance penalty email skipped: user email unavailable",
			"audit", true, "user_id", userID)
		return nil
	}
	// 正文就是对外承诺的纯英语文案，与调账备注保持一致。
	if err := p.emailService.SendEmail(ctx, email, IPAbnormalityEmailSubject, notes); err != nil {
		slog.Warn("ip balance penalty email failed",
			"audit", true, "user_id", userID, "error", err)
	}
	return nil
}

// ProvideIPConsistencyPenaltyLocker 从 IP 窗口缓存取出可选的处罚幂等锁。
// NewIPConsistencyCache 返回的是接口，具体实现的 SETNX 能力需要类型断言取得。
func ProvideIPConsistencyPenaltyLocker(cache IPConsistencyCache) IPConsistencyPenaltyLocker {
	locker, _ := cache.(IPConsistencyPenaltyLocker)
	return locker
}
