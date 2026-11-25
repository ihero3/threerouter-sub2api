package service

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

// AccountStatusNotifyService 在上游账号状态发生变化的瞬间，向管理员配置的通知邮箱
// 发送变更邮件（携带账号标识与变更前后状态）。
//
// AccountStatusNotifyService sends an email to the configured admin notification
// emails the moment an upstream account status changes.
type AccountStatusNotifyService struct {
	settingRepo              SettingRepository
	accountRepo              AccountQuotaReader
	notificationEmailService *NotificationEmailService
}

func NewAccountStatusNotifyService(settingRepo SettingRepository, accountRepo AccountQuotaReader) *AccountStatusNotifyService {
	return &AccountStatusNotifyService{settingRepo: settingRepo, accountRepo: accountRepo}
}

func (s *AccountStatusNotifyService) SetNotificationEmailService(notificationEmailService *NotificationEmailService) {
	s.notificationEmailService = notificationEmailService
}

// AccountStatusChangeNotifier 是仓储层可感知的最小能力（避免 repository 依赖具体服务类型）。
type AccountStatusChangeNotifier interface {
	NotifyStatusChange(ctx context.Context, accountID int64, fromStatus, toStatus, errorMessage string)
}

// NotifyAccountStatusChanged 供 repository 在状态批量变更（如调度器自动恢复）后回调；
// notifier 为 nil 时静默跳过。
func NotifyAccountStatusChanged(ctx context.Context, notifier AccountStatusChangeNotifier, accountID int64, fromStatus, toStatus, errorMessage string) {
	if notifier == nil {
		return
	}
	notifier.NotifyStatusChange(ctx, accountID, fromStatus, toStatus, errorMessage)
}

// AccountAvailabilityState 返回账号对外的「可调度状态」，与管理后台账号列表展示口径一致：
//   - status 非 active（error/disabled/expired…）→ 直接返回 status
//   - status=active 且调度开关关闭 → 返回 "paused"（列表显示「暂停」）
//   - 其余 → active
//
// 状态变更通知以此为判定基准，因此关闭/开启调度开关同样会触发通知。
func AccountAvailabilityState(account *Account) string {
	if account == nil {
		return ""
	}
	return accountAvailabilityStateOf(account.Status, account.Schedulable)
}

// accountAvailabilityStateOf 是 AccountAvailabilityState 的拆解版本，供批量路径在
// 未持有完整账号对象时按 status + schedulable 组合计算。
func accountAvailabilityStateOf(status string, schedulable bool) string {
	status = strings.TrimSpace(status)
	if status != "" && status != StatusActive {
		return status
	}
	if !schedulable {
		return "paused"
	}
	if status == "" {
		return StatusActive
	}
	return status
}

// enabledNotifyEmails 返回启用中（未停用且非空）的收件人，去重保序。
// 与 filterVerifiedEmails 的区别：不要求 verified —— 管理后台配置的通知邮箱没有验证流程。
func enabledNotifyEmails(entries []NotifyEmailEntry) []string {
	recipients := make([]string, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if entry.Disabled {
			continue
		}
		email := strings.TrimSpace(entry.Email)
		if email == "" {
			continue
		}
		lower := strings.ToLower(email)
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}
		recipients = append(recipients, email)
	}
	return recipients
}

// NotifyStatusChange 在状态确实发生变化时发送通知。
// fromStatus 为空表示调用方未捕获变更前状态，此时无法证明发生了状态变化，直接跳过发送，
// 避免周期性任务把同一状态重复通知。
//
// 该方法可能在网关请求路径上被调用（上游故障自动禁用），因此只做轻量校验后异步发送，
// 绝不阻塞调用方请求。
func (s *AccountStatusNotifyService) NotifyStatusChange(parent context.Context, accountID int64, fromStatus, toStatus, errorMessage string) {
	if s == nil || s.settingRepo == nil || s.notificationEmailService == nil || accountID <= 0 {
		return
	}
	oldStatus := strings.TrimSpace(fromStatus)
	toStatus = strings.TrimSpace(toStatus)
	if toStatus == "" || oldStatus == "" || oldStatus == toStatus {
		return
	}

	// 断开父 ctx 的取消信号，避免请求结束后通知被中断。
	ctx := context.WithoutCancel(parent)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("account status change notification panic", "account_id", accountID, "panic", r)
			}
		}()
		s.notifyStatusChange(ctx, accountID, oldStatus, toStatus, errorMessage)
	}()
}

// notifyStatusChange 执行真正的配置读取与邮件投递（异步调用）。
func (s *AccountStatusNotifyService) notifyStatusChange(parent context.Context, accountID int64, oldStatus, toStatus, errorMessage string) {
	ctx, cancel := context.WithTimeout(parent, emailSendTimeout)
	defer cancel()

	enabled, recipients := s.getNotifyConfig(ctx)
	if !enabled || len(recipients) == 0 {
		// 打印一条可定位的日志，避免"配置了却没收到邮件"时无从排查。
		slog.Info("account status change notification skipped by config",
			"account_id", accountID, "from", oldStatus, "to", toStatus,
			"enabled", enabled, "recipient_count", len(recipients))
		return
	}

	account := s.loadAccount(ctx, accountID)
	accountName := strconv.FormatInt(accountID, 10)
	platform := "-"
	if account != nil {
		if strings.TrimSpace(account.Name) != "" {
			accountName = account.Name
		}
		if strings.TrimSpace(account.Platform) != "" {
			platform = account.Platform
		}
	}

	sourceID := strconv.FormatInt(accountID, 10)
	// 每次状态变更都必须独立投递：通知服务会用 (event, sourceType, sourceID, recipient, reminderKey)
	// 做投递去重，若 reminderKey 为空，则同一账号的首封邮件发出后，后续所有状态变更都会被当成
	// 重复投递而静默丢弃。这里用「变更前后状态 + 纳秒时间」保证每次变更唯一。
	reminderKey := oldStatus + "->" + toStatus + "@" + strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
	variables := map[string]string{
		"account_id":    sourceID,
		"account_name":  accountName,
		"platform":      platform,
		"from_status":   firstNonEmpty(oldStatus, "-"),
		"to_status":     firstNonEmpty(toStatus, "-"),
		"error_message": firstNonEmpty(strings.TrimSpace(errorMessage), "-"),
		"changed_at":    time.Now().UTC().Format(time.RFC3339),
	}
	for _, to := range recipients {
		if ctx.Err() != nil {
			return
		}
		sendCtx, sendCancel := context.WithTimeout(ctx, emailSendTimeout)
		err := s.notificationEmailService.Send(sendCtx, NotificationEmailSendInput{
			Event:          NotificationEmailEventAccountStatusChanged,
			RecipientEmail: to,
			RecipientName:  emailRecipientName(to),
			SourceType:     "account_status",
			SourceID:       sourceID,
			ReminderKey:    reminderKey,
			Variables:      variables,
		})
		sendCancel()
		if err != nil {
			slog.Warn("failed to send account status change notification",
				"account_id", accountID, "from", oldStatus, "to", toStatus, "to_email", to, "err", err.Error())
			continue
		}
		slog.Info("account status change notification sent",
			"account_id", accountID, "from", oldStatus, "to", toStatus)
	}
}

func (s *AccountStatusNotifyService) loadAccount(ctx context.Context, accountID int64) *Account {
	if s.accountRepo == nil {
		return nil
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		slog.Warn("failed to load account for status change notification", "account_id", accountID, "err", err.Error())
		return nil
	}
	return account
}

func (s *AccountStatusNotifyService) getNotifyConfig(ctx context.Context) (bool, []string) {
	enabledRaw, err := s.settingRepo.GetValue(ctx, SettingKeyAccountStatusNotifyEnabled)
	if err != nil || !isTrueSettingValue(enabledRaw) {
		return false, nil
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyAccountStatusNotifyEmails)
	if err != nil || strings.TrimSpace(raw) == "" || raw == "[]" {
		return true, nil
	}
	// 管理后台配置的通知邮箱无需走用户侧验证流程，仅跳过被显式停用/为空的条目。
	return true, enabledNotifyEmails(ParseNotifyEmails(raw))
}
