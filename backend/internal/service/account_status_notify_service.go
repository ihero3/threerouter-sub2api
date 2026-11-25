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

// NotifyStatusChange 在状态确实发生变化时发送通知。
// fromStatus 为空表示调用方未捕获变更前状态，此时无法证明发生了状态变化，直接跳过发送，
// 避免周期性任务把同一状态重复通知。
//
// NotifyStatusChange sends a notification only when the status actually changed.
func (s *AccountStatusNotifyService) NotifyStatusChange(parent context.Context, accountID int64, fromStatus, toStatus, errorMessage string) {
	if s == nil || s.settingRepo == nil || s.notificationEmailService == nil || accountID <= 0 {
		return
	}
	oldStatus := strings.TrimSpace(fromStatus)
	toStatus = strings.TrimSpace(toStatus)
	if toStatus == "" || oldStatus == "" || oldStatus == toStatus {
		return
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), emailSendTimeout)
	defer cancel()

	enabled, recipients := s.getNotifyConfig(ctx)
	if !enabled || len(recipients) == 0 {
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
	return true, filterVerifiedEmails(ParseNotifyEmails(raw))
}
