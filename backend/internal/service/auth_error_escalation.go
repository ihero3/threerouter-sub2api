package service

import (
	"context"
	"log/slog"
	"time"
)

// AuthErrorEscalationCache 是 401/403 账号级错误的通用升级计数器（可选依赖）。
// 按（账号, 状态码）维度在固定窗口内累计命中次数，驱动
// 「基础冷却 → 中档冷却 → 长冷却」的升级梯度，取代旧的「第 N 次永久禁用」语义：
// 疑似凭据失效的账号先短暂退出调度池，重复失败才逐级加长冷却；
// 正常账号成功请求或人工恢复时清零计数。实现方与 OpenAI403CounterCache
// 共用同一 Redis 实例（独立 key 前缀）。
type AuthErrorEscalationCache interface {
	// IncrementAuthErrorCount 原子递增（账号, 状态码）计数并返回当前值。
	IncrementAuthErrorCount(ctx context.Context, accountID int64, statusCode int, windowMinutes int) (int64, error)
	// ResetAuthErrorCounters 清空该账号全部状态码的升级计数。
	ResetAuthErrorCounters(ctx context.Context, accountID int64) error
}

const (
	// authErrorEscalationWindowMinutes 升级计数的统计窗口：24h。
	authErrorEscalationWindowMinutes = 24 * 60
	// authErrorEscalationBaseCooldown 首次命中的基础冷却。
	authErrorEscalationBaseCooldown = 30 * time.Minute
	// authErrorEscalationMidThreshold / MidCooldown：24h 内第 3 次起 → 6h 冷却。
	authErrorEscalationMidThreshold = 3
	authErrorEscalationMidCooldown  = 6 * time.Hour
	// authErrorEscalationHighThreshold / HighCooldown：24h 内第 5 次起 → 24h 冷却。
	authErrorEscalationHighThreshold = 5
	authErrorEscalationHighCooldown  = 24 * time.Hour
)

// authErrorEscalationCooldownForCount 按窗口内累计命中次数返回本次冷却时长。
func authErrorEscalationCooldownForCount(count int64) time.Duration {
	switch {
	case count >= authErrorEscalationHighThreshold:
		return authErrorEscalationHighCooldown
	case count >= authErrorEscalationMidThreshold:
		return authErrorEscalationMidCooldown
	default:
		return authErrorEscalationBaseCooldown
	}
}

// SetAuthErrorEscalationCache 设置 401/403 升级计数器（可选依赖）。
func (s *RateLimitService) SetAuthErrorEscalationCache(cache AuthErrorEscalationCache) {
	s.authErrorEscalationCache = cache
}

// handleAuthErrorEscalating 把「凭据疑似失效但可自愈」的账号级错误
// （401 API Key、401 OAuth 无 refresh_token、非能力类 403 等）从永久 SetError
// 降级为「临时冷却 + 计数升级」：首次 30min，24h 内第 3 次 6h、第 5 次 24h，
// 到期自动恢复调度，不再产生需要人工恢复的 status=error。
//
// 仍走永久 SetError 的语义性例外（调用点自行分流）：token 被上游永久作废
// （token_invalidated/token_revoked）、组织/工作区停用、身份验证要求、
// Antigravity 验证/违规封号、自定义错误码，以及余额不足
// （402，走 handlePaymentRequiredError 带自动复活探测）。
func (s *RateLimitService) handleAuthErrorEscalating(ctx context.Context, account *Account, statusCode int, errorMsg string) {
	if s == nil || s.accountRepo == nil || account == nil {
		return
	}
	// 中转站账号沿用 handleAuthError 的固定短冷却分支（上游内部账号状态透传，
	// 不代表本账号 Key 失效，冷却要短、计数不能消耗）。
	if account.IsRelayUpstream() {
		s.handleAuthError(ctx, account, errorMsg)
		return
	}
	count := int64(0)
	if s.authErrorEscalationCache != nil {
		if v, err := s.authErrorEscalationCache.IncrementAuthErrorCount(ctx, account.ID, statusCode, authErrorEscalationWindowMinutes); err != nil {
			slog.Warn("auth_error_escalation_increment_failed", "account_id", account.ID, "status_code", statusCode, "error", err)
		} else {
			count = v
		}
	}
	cooldown := authErrorEscalationCooldownForCount(count)
	until := time.Now().Add(cooldown)
	s.notifyAccountSchedulingBlocked(account, until, "auth_error_temp")
	if err := s.accountRepo.SetTempUnschedulable(ctx, account.ID, until, errorMsg); err != nil {
		slog.Warn("auth_error_escalation_temp_failed", "account_id", account.ID, "status_code", statusCode, "error", err)
		return
	}
	slog.Warn("auth_error_temp_cooldown_escalated",
		"account_id", account.ID,
		"status_code", statusCode,
		"count", count,
		"cooldown", cooldown.String(),
		"error", errorMsg)
}

// failover402RecoverProbeInterval 是 402 永久禁用账号的自动复活探测间隔：
// 每隔该时长由 AccountAutoRecoveryService 把账号放回调度池试一次，
// 上游充值后无需人工恢复。
const failover402RecoverProbeInterval = 24 * time.Hour

// handlePaymentRequiredError 处理 402（余额/计费不足）：这是唯一允许永久禁用
// status=error 的账号级错误（用户哲学：只有没费用才关号），但必须带上自动复活
// 探测——error_message 写入 [auto-recover at ...] 标记 + rate_limit_reset_at
// （= now + 24h），让 AccountAutoRecoveryService 每 24h 把账号放回调度池试一次，
// 充值后即可自动恢复，不再依赖管理员手动 recover-state。
//
// 2026-11-27：中转站账号不再享受「402 只做 3 分钟短冷却」的豁免。402 意味着这条
// 上游凭据没有余额，换号治不了本——池里每个中转站账号都会陆续 402，3 分钟冷却
// 结束后又被调度、又 402，用户只会持续收到上游错误。与其让它无限循环，不如停用
// 账号并靠 24h 探测在充值后自动恢复；401/403 等真正「可能误伤健康 Key」的透传
// 错误仍走 handleAuthError 的中转站短冷却分支（见 handleAuthErrorEscalating）。
func (s *RateLimitService) handlePaymentRequiredError(ctx context.Context, account *Account, errorMsg string) {
	if s == nil || s.accountRepo == nil || account == nil {
		return
	}
	probeAt := time.Now().Add(failover402RecoverProbeInterval)
	tagged := ensureFailoverAutoRecoverTag(errorMsg, probeAt)
	s.notifyAccountSchedulingBlocked(account, time.Time{}, "auth_error")
	if err := s.accountRepo.SetError(ctx, account.ID, tagged); err != nil {
		slog.Warn("account_set_error_failed", "account_id", account.ID, "error", err)
		return
	}
	// 自动恢复扫描的命中条件是 status=error AND rate_limit_reset_at 已到期 AND
	// error_message 带 tag；SetError 不写 rate_limit_reset_at，这里补一笔。
	if err := s.accountRepo.SetRateLimited(ctx, account.ID, probeAt); err != nil {
		slog.Warn("account_402_probe_reset_write_failed", "account_id", account.ID, "error", err)
	}
	slog.Warn("account_disabled_402_with_recover_probe",
		"account_id", account.ID,
		"probe_at", probeAt.Format(time.RFC3339),
		"error", errorMsg)
}
