package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type accountStatusNotifyTestRepo struct {
	accounts map[int64]*Account
}

func (r *accountStatusNotifyTestRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	account, ok := r.accounts[id]
	if !ok {
		return nil, errors.New("account not found")
	}
	return account, nil
}

// newAccountStatusNotifyTestService 构造带本地 SMTP 服务器的通知服务与设置仓库。
func newAccountStatusNotifyTestService(t *testing.T) (*AccountStatusNotifyService, *notificationEmailMemorySettingRepo, *notificationEmailTestSMTPServer) {
	t.Helper()
	ctx := context.Background()
	settingRepo := newNotificationEmailMemorySettingRepo()
	smtpServer := startNotificationEmailTestSMTPServer(t)
	require.NoError(t, settingRepo.SetMultiple(ctx, smtpServer.settings()))
	_ = settingRepo.Set(ctx, SettingKeyAccountStatusNotifyEnabled, "true")
	_ = settingRepo.Set(ctx, SettingKeyAccountStatusNotifyEmails, `[{"email":"ops@example.com","verified":true}]`)

	emailService := NewEmailService(settingRepo, nil)
	svc := NewAccountStatusNotifyService(settingRepo, &accountStatusNotifyTestRepo{
		accounts: map[int64]*Account{7: {ID: 7, Name: "upstream-main", Platform: "openai"}},
	})
	svc.SetNotificationEmailService(NewNotificationEmailService(settingRepo, emailService))
	return svc, settingRepo, smtpServer
}

func TestAccountStatusNotifySkipsWhenStatusUnchanged(t *testing.T) {
	svc, _, smtpServer := newAccountStatusNotifyTestService(t)

	svc.NotifyStatusChange(context.Background(), 7, StatusActive, StatusActive, "")
	require.Equal(t, int64(0), smtpServer.messageCount(), "status unchanged should not send")
}

func TestAccountStatusNotifySkipsUnknownPreviousStatus(t *testing.T) {
	svc, _, smtpServer := newAccountStatusNotifyTestService(t)

	// 未捕获变更前状态时无法证明状态变化，不应发送。
	svc.NotifyStatusChange(context.Background(), 7, "", StatusError, "boom")
	require.Equal(t, int64(0), smtpServer.messageCount())
}

func TestAccountStatusNotifySkipsWhenDisabled(t *testing.T) {
	svc, settingRepo, smtpServer := newAccountStatusNotifyTestService(t)
	require.NoError(t, settingRepo.Set(context.Background(), SettingKeyAccountStatusNotifyEnabled, "false"))

	svc.NotifyStatusChange(context.Background(), 7, StatusActive, StatusError, "boom")
	require.Equal(t, int64(0), smtpServer.messageCount())
}

func TestAccountStatusNotifySkipsWhenNoRecipients(t *testing.T) {
	svc, settingRepo, smtpServer := newAccountStatusNotifyTestService(t)
	require.NoError(t, settingRepo.Set(context.Background(), SettingKeyAccountStatusNotifyEmails, "[]"))

	svc.NotifyStatusChange(context.Background(), 7, StatusActive, StatusError, "boom")
	require.Equal(t, int64(0), smtpServer.messageCount())
}

func TestAccountStatusNotifySendsOnStatusChange(t *testing.T) {
	svc, _, smtpServer := newAccountStatusNotifyTestService(t)

	svc.NotifyStatusChange(context.Background(), 7, StatusActive, StatusError, "upstream 401")
	require.Equal(t, int64(1), smtpServer.messageCount())

	body := smtpServer.lastMessageBody(t)
	require.Contains(t, body, "upstream-main")
	require.Contains(t, body, "active")
	require.Contains(t, body, "error")
	require.Contains(t, body, "upstream 401")
}
