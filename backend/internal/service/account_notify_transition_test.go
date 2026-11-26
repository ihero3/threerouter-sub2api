package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// accountNotifyTestRepo 可改写账号内容的仓，供写后复查用。
type accountNotifyTestRepo struct {
	mu   sync.Mutex
	acct *Account
}

func (r *accountNotifyTestRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.acct == nil || r.acct.ID != id {
		return nil, errors.New("account not found")
	}
	return r.acct, nil
}

func (r *accountNotifyTestRepo) setAccount(a *Account) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.acct = a
}

// newAccountNotifyTestService 构造带本地 SMTP 的通知服务，并暴露可改写的账号仓。
func newAccountNotifyTestService(t *testing.T) (*AccountStatusNotifyService, *accountNotifyTestRepo, *notificationEmailTestSMTPServer) {
	t.Helper()
	ctx := context.Background()
	settingRepo := newNotificationEmailMemorySettingRepo()
	smtpServer := startNotificationEmailTestSMTPServer(t)
	require.NoError(t, settingRepo.SetMultiple(ctx, smtpServer.settings()))
	_ = settingRepo.Set(ctx, SettingKeyAccountStatusNotifyEnabled, "true")
	_ = settingRepo.Set(ctx, SettingKeyAccountStatusNotifyEmails, `[{"email":"ops@example.com","verified":true}]`)

	repo := &accountNotifyTestRepo{}
	emailService := NewEmailService(settingRepo, nil)
	svc := NewAccountStatusNotifyService(settingRepo, repo)
	svc.SetNotificationEmailService(NewNotificationEmailService(settingRepo, emailService))
	return svc, repo, smtpServer
}

// 写操作侧登记一次 429 限流，worker 复查到「active -> rate-limited」后必须发邮件：
// 这条链路就是用户要的「限流也要发邮件」。
func TestQueueAccountTransitionSendsWhenCooldownAppears(t *testing.T) {
	svc, repo, smtpServer := newAccountNotifyTestService(t)
	defer svc.Stop()

	resetAt := time.Now().Add(30 * time.Minute)
	repo.setAccount(&Account{ID: 7, Name: "upstream-main", Platform: "openai", Status: StatusActive, Schedulable: true, RateLimitResetAt: &resetAt})

	// before 是写前的健康状态；after 留空 = 交给 worker 复查，这正是 SetRateLimited 的路径。
	svc.QueueAccountTransition(context.Background(), 7, AccountNotifyTransition{
		Before: StatusActive,
		Detail: "rate limit resets at " + resetAt.UTC().Format(time.RFC3339),
	})

	require.Eventually(t, func() bool {
		return smtpServer.messageCount() == 1
	}, 5*time.Second, 20*time.Millisecond)

	body := smtpServer.lastMessageBody(t)
	require.Contains(t, body, AccountEventRateLimited)
	require.Contains(t, body, "upstream-main")
	require.Contains(t, body, "rate limit resets at")
}

// 写后复查发现状态没变（例如冷却没写进去），不能凭空发一封假变更邮件。
func TestQueueAccountTransitionSkipsWhenStateUnchanged(t *testing.T) {
	svc, repo, smtpServer := newAccountNotifyTestService(t)
	defer svc.Stop()
	repo.setAccount(&Account{ID: 7, Name: "upstream-main", Platform: "openai", Status: StatusActive, Schedulable: true})

	svc.QueueAccountTransition(context.Background(), 7, AccountNotifyTransition{Before: StatusActive})
	// 复查一轮仍未变化，应不发信。
	svc.resolveTransitions()
	assertNeverSends(t, smtpServer)
}

// 写操作者已经知道目标状态（分组变更/优先级变更/调度开关）时按 After 投递。
// After 路径同样经过一次状态确认，因此账号需处于 After 描述的状态。
func TestQueueAccountTransitionDirectAfterValue(t *testing.T) {
	svc, repo, smtpServer := newAccountNotifyTestService(t)
	defer svc.Stop()
	// priority-changed 不是运行时容量态，复查回读得到的是 active，故这里以 active 为 target。
	repo.setAccount(&Account{ID: 7, Name: "upstream-main", Platform: "openai", Status: StatusActive, Schedulable: true})

	svc.QueueAccountTransition(context.Background(), 7, AccountNotifyTransition{
		Before:     AccountEventPriorityChanged,
		After:      StatusActive,
		Detail:     "priority changed from 5 to 1",
		SourceType: "account_scheduling",
	})
	require.Eventually(t, func() bool {
		return smtpServer.messageCount() == 1
	}, 5*time.Second, 20*time.Millisecond)
	require.Contains(t, smtpServer.lastMessageBody(t), "priority changed from 5 to 1")
}

// 仓储层与管理层可能对同一次变更各登记一次（关调度、编辑账号、批量更新都会走两处），
// 折叠后管理员只应收到一封邮件——这是本次去重的核心不变量。
func TestAccountTransitionCollapsesDuplicateRegistrations(t *testing.T) {
	svc, repo, smtpServer := newAccountNotifyTestService(t)
	defer svc.Stop()
	repo.setAccount(&Account{ID: 7, Name: "upstream-main", Platform: "openai", Status: StatusActive, Schedulable: false})

	// 模拟两条链路先后登记同一次「active -> paused」。
	for i := 0; i < 2; i++ {
		svc.NotifyStatusChange(context.Background(), 7, StatusActive, "paused", "")
	}
	require.Eventually(t, func() bool {
		return smtpServer.messageCount() == 1
	}, 5*time.Second, 20*time.Millisecond)
	time.Sleep(400 * time.Millisecond)
	require.Equal(t, int64(1), smtpServer.messageCount())
}

// 同一账号先后两次真实变更（关调度再开调度）不能被折叠吞掉。
func TestAccountTransitionDoesNotCollapseDistinctChanges(t *testing.T) {
	svc, _, smtpServer := newAccountNotifyTestService(t)
	defer svc.Stop()

	svc.NotifyStatusChange(context.Background(), 7, StatusActive, "paused", "")
	require.Eventually(t, func() bool {
		return smtpServer.messageCount() == 1
	}, 5*time.Second, 20*time.Millisecond)
	svc.NotifyStatusChange(context.Background(), 7, "paused", StatusActive, "")
	require.Eventually(t, func() bool {
		return smtpServer.messageCount() == 2
	}, 5*time.Second, 20*time.Millisecond)
}

// After 已知但账号现状已不是它（期间又被改动）：以最新状态为准，不发过期的变更描述。
func TestQueueAccountTransitionPrefersLatestState(t *testing.T) {
	svc, repo, smtpServer := newAccountNotifyTestService(t)
	defer svc.Stop()
	// 关调度后被上游错误连带置为 error：列表上看到的是 error，不是 paused。
	repo.setAccount(&Account{ID: 7, Name: "upstream-main", Platform: "openai", Status: StatusError, Schedulable: false})

	svc.QueueAccountTransition(context.Background(), 7, AccountNotifyTransition{
		Before: StatusActive,
		After:  "paused",
		Detail: "schedulable set to false",
	})
	require.Eventually(t, func() bool {
		return smtpServer.messageCount() == 1
	}, 5*time.Second, 20*time.Millisecond)
	body := smtpServer.lastMessageBody(t)
	require.Contains(t, body, StatusError)
	require.NotContains(t, body, ">paused<")
}

// 同一账号 + 同一事件 + 同一原因在一个窗口内只发一封，防 429 并发刷屏；
// 换事件（active -> rate-limited -> active）照常逐次投递。
func TestAccountStatusNotifyDedupsSameEventInWindow(t *testing.T) {
	svc, _, smtpServer := newAccountNotifyTestService(t)
	defer svc.Stop()

	detail := "upstream 429"
	svc.notifyStatusChange(context.Background(), 7, StatusActive, AccountEventRateLimited, detail)
	svc.notifyStatusChange(context.Background(), 7, StatusActive, AccountEventRateLimited, detail)
	require.Eventually(t, func() bool {
		return smtpServer.messageCount() == 1
	}, 5*time.Second, 20*time.Millisecond)
	time.Sleep(300 * time.Millisecond)
	require.Equal(t, int64(1), smtpServer.messageCount())

	// 不同事件不被去重吞掉。
	svc.notifyStatusChange(context.Background(), 7, AccountEventRateLimited, StatusActive, "recovered")
	require.Eventually(t, func() bool {
		return smtpServer.messageCount() == 2
	}, 5*time.Second, 20*time.Millisecond)
}

// 模型不可用要按 404 model_not_found 回给客户端，而不是 502 upstream_error：
// 「上游不支持」就明说不支持，平台侧挡在前面而不是让终端用户面对上游原文。
func TestModelUnavailableBodyIsModelNotFound(t *testing.T) {
	body := ModelUnavailableBody()
	errObj, ok := body["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "model_not_found", errObj["type"])
	require.Equal(t, "The requested model is not available on this platform", errObj["message"])
	// 客户端错误文案必须是英语（红线）。
	require.NotContains(t, errObj["message"], "不")
	require.True(t, strings.HasPrefix(errObj["message"].(string), "The requested model"))
}
