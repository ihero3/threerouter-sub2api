package service

import infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"

// 邀请注册风险标记的「待复核冻结」语义。
//
// 历史上 users.risk_level 只用于管理端展示，不参与任何鉴权（见 affiliate_risk.go
// 头注释）。现在 suspect / abnormal 需要真正拦住建 Key 与网关调用，因此把判定和
// 客户端文案收敛到这一处，保证服务层、标准网关、Google 兼容网关三处完全一致。
//
// 放行方式：管理员复核通过后清空 risk_level（见 AdminReviewUserRisk），
// 认证缓存随之失效，下一次请求自然放行，不需要额外的开关字段。
const (
	UserRiskAccessBlockedCode    = "USER_RISK_REVIEW_REQUIRED"
	UserRiskAccessBlockedMessage = "This account is pending a security review. Please contact support."
)

// ErrUserRiskAccessBlocked 用于控制台创建 API Key 等面板侧入口。
// 文案必须是纯英语：客户端可见，不混汉语。
var ErrUserRiskAccessBlocked = infraerrors.Forbidden(UserRiskAccessBlockedCode, UserRiskAccessBlockedMessage)

// RiskAccessBlocked 报告用户是否因邀请注册风险标记被限制建 Key / 调用网关。
//   - suspect  : 存疑，等人工复核后放行；
//   - abnormal : 已判定异常，打标时同时会置 status=disabled；这里保留判定是为了
//     双保险，避免其它链路（如内容风控解封）把状态改回 active 时意外放开。
//
// 未知/空值一律放行，保证新增风险等级或脏数据不会把全站用户锁死。
func (u *User) RiskAccessBlocked() bool {
	if u == nil {
		return false
	}
	return u.RiskLevel == AffiliateRiskLevelSuspect || u.RiskLevel == AffiliateRiskLevelAbnormal
}
