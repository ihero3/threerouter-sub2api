package service

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AccountStatusNotifyService 在上游账号状态发生变化的瞬间，向管理员配置的通知邮箱
// 发送变更邮件（携带账号标识与变更前后状态）。
//
// 覆盖面（2026-11-26 用户要求「账号有动作变化就发邮件」）：
//   - 账号状态位变化：active / error / disabled / expired / paused；
//   - 冷却与调度层变化：429 限流、529 过载、模型级冷却（含生图/参考图限流）、
//     临时不可调度、用量阈值暂停、配额耗尽；
//   - 结构性变化：分组绑定、调度优先级。
//
// AccountStatusNotifyService sends an email to the configured admin notification
// emails the moment an upstream account status changes.
type AccountStatusNotifyService struct {
	settingRepo              SettingRepository
	accountRepo              AccountQuotaReader
	notificationEmailService *NotificationEmailService

	// tracks 保存尚未结算的写操作（写前状态 + 结算截止时间），由后台 worker 复查。
	tracks   sync.Map
	dedup    sync.Map
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
	// claimed 记录每个账号最近一次真正投递的前后状态，用于折叠重复登记：
	// 同一次变更可能同时被仓储层与管理层登记（两条链路都合理），但对管理员只是一件事，
	// 这里以「与最近一次投递相同的 from->to」判定为重复，直接丢弃。
	claimed sync.Map
}

func NewAccountStatusNotifyService(settingRepo SettingRepository, accountRepo AccountQuotaReader) *AccountStatusNotifyService {
	svc := &AccountStatusNotifyService{
		settingRepo: settingRepo,
		accountRepo: accountRepo,
		stopCh:      make(chan struct{}),
	}
	// worker 常驻：没有待结算项时每轮只做一次空 map 遍历，开销可忽略；
	// 用常驻换取「冷却到期自动补发恢复邮件」这个必须的时钟兜底。
	svc.wg.Add(1)
	go svc.runTransitionWorker()
	return svc
}

func (s *AccountStatusNotifyService) SetNotificationEmailService(notificationEmailService *NotificationEmailService) {
	s.notificationEmailService = notificationEmailService
}

// 上游账号冷却/容量类事件（写进 to_status，与 active/paused/error 等真实状态区分）。
// 这类事件不是账号状态位变化，而是「现在能不能被调度」的变化，2026-11-26 用户要求
// 全部发邮件（含限流、冷却层、过载、配额/余额、调度暂停、分组/优先级变更）。
const (
	AccountEventRateLimited           = "rate-limited"      // 账号级限流（429 / 配额窗口耗尽）
	AccountEventOverloaded            = "overloaded"        // 529 过载冷却
	AccountEventModelLimited          = "model-limited"     // 模型级冷却（含生图/参考图限流）
	AccountEventTempCooled            = "temp-cooled"       // 临时不可调度（冷却层）
	AccountEventQuotaExhausted        = "quota-exhausted"   // 配额/额度耗尽
	AccountEventSchedulingThreshold   = "scheduling-paused" // 用量阈值触发的调度暂停
	AccountEventExpiredPaused         = "expired-paused"    // 到期自动暂停
	AccountEventGroupsChanged         = "groups-changed"    // 分组绑定变化
	AccountEventPriorityChanged       = "priority-changed"  // 调度优先级变化
	accountNotifyReasonPreviewMaxSize = 300

	// accountNotifyPollInterval / accountNotifyResolveDelay 控制写操作后的状态复查节奏：
	// 200ms 复查一次，未变化最多等 8s。延迟复查而非立即复查，是为了让同一请求里
	// 「先限流冷却、后因换号失败被禁用」这类连写收敛成最终状态，只发一封有意义的邮件。
	accountNotifyPollInterval = 200 * time.Millisecond
	accountNotifyResolveDelay = 8 * time.Second
	// accountNotifyDedupWindow 同账号、同事件、同原因在一个窗口内只投递一次，
	// 防止并发 429/超阈值判定刷屏；不同事件的切换照常投递。
	accountNotifyDedupWindow = time.Minute
	// accountNotifyClaimTTL 是「同一次变更只发一封」的折叠窗口。仓储层与管理层
	// 可能先后登记同一次状态变化，两次登记之间通常只差几十毫秒，10s 足够覆盖，
	// 又不会把用户真正的二次操作（先后两次关闭调度）误吞。
	accountNotifyClaimTTL = 10 * time.Second
)

// accountTransitionClaim 记录一个账号最近一次投递的 (from->to) 及其折叠有效期。
type accountTransitionClaim struct {
	signature string
	expireAt  time.Time
}

// accountNotifyTrack 是一次待结算写操作的现场。
type accountNotifyTrack struct {
	// before 写操作前读到的对外状态。
	before string
	// after 写操作者已确认的新状态（admin 层在写后回读过）。非空时 worker 视其为准，
	// 不再要求「必然变化」才发信——否则「关调度 → 触发自动禁用 → 状态未变」这类
	// 列表上已确认的变化会被漏掉。
	after string
	// reason 写操作者给出的冷却原因（可能为空，回退读库里的 TempUnschedulableReason）。
	reason string
	// detail 已拼好的详情；为空时由 worker 从账号冷却字段反推。
	detail string
	// sourceType / sourceID 参与投递去重，避免同一变化的两次登记各发一封。
	sourceType string
	sourceID   string
	// deadline 结算截止时间，超过后不再跟踪。
	deadline time.Time
}

// AccountStatusChangeNotifier 是仓储层可感知的最小能力（避免 repository 依赖具体服务类型）。
type AccountStatusChangeNotifier interface {
	NotifyStatusChange(ctx context.Context, accountID int64, fromStatus, toStatus, errorMessage string)
}

// AccountAvailabilityState 返回账号对外的「可调度状态」，与管理后台账号列表展示口径一致：
//   - status 非 active（error/disabled/expired…）→ 直接返回 status
//   - status=active 且调度开关关闭 → 返回 "paused"（列表显示「暂停」）
//   - active 但正在限流/冷却/过载/超阈值 → 返回对应事件（rate-limited / temp-cooled /
//     model-limited / overloaded / scheduling-paused），与列表上的冷却标签一致
//   - 其余 → active
//
// 状态变更通知以此为判定基准，因此关闭/开启调度开关、进入/退出冷却层同样会触发通知。
// 冷却到期无需写库，这里按到期时间动态回落，判定自然从冷却态变回 active
// （配合 accountTransitionNotifier 的写后复查，到期那一刻会补发一封「恢复可用」）。
func AccountAvailabilityState(account *Account) string {
	if account == nil {
		return ""
	}
	state := accountAvailabilityStateOf(account.Status, account.Schedulable)
	if state != StatusActive {
		return state
	}
	return accountRuntimeCapacityState(account, time.Now())
}

// accountRuntimeCapacityState 在账号状态位为 active 时，进一步判定「现在能否被调度」。
// 优先级与 IsSchedulable 的检查顺序保持一致，保证邮件里的事件名就是列表上看得到的那个。
func accountRuntimeCapacityState(account *Account, now time.Time) string {
	if account.OverloadUntil != nil && now.Before(*account.OverloadUntil) {
		return AccountEventOverloaded
	}
	if account.RateLimitResetAt != nil && now.Before(*account.RateLimitResetAt) {
		return AccountEventRateLimited
	}
	if account.TempUnschedulableUntil != nil && now.Before(*account.TempUnschedulableUntil) {
		if isSchedulingThresholdTempUnschedReason(account.TempUnschedulableReason) {
			return AccountEventSchedulingThreshold
		}
		return AccountEventTempCooled
	}
	if account.SupportsAccountQuota() && account.IsQuotaExceeded() {
		return AccountEventQuotaExhausted
	}
	return StatusActive
}

// isSchedulingThresholdTempUnschedReason 区分「用量阈值暂停」与一般临时冷却：
// 两者都写 temp_unschedulable_until，但前者是主动让位，邮件里应显示为调度暂停。
// 阈值原因以 JSON payload 落库（source=account_scheduling_threshold），故用同一个
// 结构化判定，避免字符串包含判断漏掉转义差异。
func isSchedulingThresholdTempUnschedReason(reason string) bool {
	if strings.TrimSpace(reason) == "" {
		return false
	}
	if IsAccountSchedulingThresholdReason(reason) {
		return true
	}
	return strings.Contains(reason, AccountSchedulingThresholdReasonSource)
}

// AccountTransitionNotifier 是仓储层可感知的「变化即通知」能力。
// 一次写操作通过它登记一份 AccountNotifyTransition，由独立 worker 在写后复查
// 对外状态并投递邮件——不阻塞请求路径，也不会因为写前读到了陈旧状态而漏报/误报。
type AccountTransitionNotifier interface {
	QueueAccountTransition(ctx context.Context, id int64, tr AccountNotifyTransition)
}

// AccountNotifyTransition 描述一次可能改变账号对外状态的写操作。
//
// 字段全部导出：repository 需要构造它，而通知服务需要读它；两边的语义由注释锁定。
type AccountNotifyTransition struct {
	// Before 写操作前读到的对外状态（空表示未能捕获，交给写后复查判定）。
	Before string
	// After 写操作者已知的新状态（仅当 After 非空且与 Before 不同才直接投递）。
	After string
	// Detail 邮件正文里的详情（原因 / 冷却到期时间 / HTTP 状态 / 原始上游措辞）。
	Detail string
	// Reason 可选的冷却原因，worker 写后复查时用来识别「调度阈值暂停」。
	Reason string
	// SourceType / SourceID 用于通知服务的投递去重（同一次变化只发一封）。
	SourceType string
	SourceID   string
}

// howShortUntil 用于把「冷却多久」写进邮件详情，空时间表示不适用。
func howShortUntil(until time.Time) string {
	if until.IsZero() {
		return ""
	}
	return until.UTC().Format(time.RFC3339)
}

// NotifyAccountStatusChanged 供 repository 在状态变更后回调；notifier 为 nil 时静默跳过。
func NotifyAccountStatusChanged(ctx context.Context, notifier AccountStatusChangeNotifier, accountID int64, fromStatus, toStatus, errorMessage string) {
	if notifier == nil {
		return
	}
	notifier.NotifyStatusChange(ctx, accountID, fromStatus, toStatus, errorMessage)
}

// NotifyAccountTransition 供 repository 登记一次「可能改变对外状态」的写操作。
// 只在没有直接状态通知（NotifyStatusChange）的写路径上使用，避免同一变化发两封。
func NotifyAccountTransition(ctx context.Context, notifier AccountTransitionNotifier, accountID int64, tr AccountNotifyTransition) {
	if notifier == nil || accountID <= 0 {
		return
	}
	notifier.QueueAccountTransition(ctx, accountID, tr)
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
	// 同一次变更可能被仓储层与管理层先后各报一次（例如账号编辑里改了状态：
	// repo.Update 报一次，admin 回读后又报一次）。折叠成一封，避免一次操作两封邮件。
	if !s.claimTransition(accountID, oldStatus, toStatus) {
		slog.Info("account status change notification collapsed",
			"account_id", accountID, "from", oldStatus, "to", toStatus)
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

// QueueAccountTransition 登记一次「可能改变账号对外状态」的写操作。
//
// 两种结算方式：
//   - After 已知（写操作方回读过账号）：延迟一个窗口后以 After 为准投递，但先确认
//     账号现状仍是 After，并且 After 与 Before 不同——这样既不会因为写前状态陈旧而
//     报出一次不存在的变更，也不会漏掉「关调度把账号带进 error」这类组合变化。
//   - After 未知（网关高频冷却路径）：由 worker 复查真实状态，变了才发信。
//
// 同一账号、同一 from->to 的重复登记会被折叠成一封邮件（见 claimed）。
func (s *AccountStatusNotifyService) QueueAccountTransition(_ context.Context, accountID int64, tr AccountNotifyTransition) {
	if s == nil || accountID <= 0 {
		return
	}
	before := strings.TrimSpace(tr.Before)
	after := strings.TrimSpace(tr.After)
	if after != "" && after == before {
		// 写操作方确认没有对外变化：不登记，避免延迟复查把随后的其它变化误归因给它。
		return
	}
	if s.notificationEmailService == nil {
		return
	}
	if s.accountRepo == nil && after == "" {
		// 没有回读能力时无法判定未知前态的变化，宁可不发也不误报。
		return
	}
	s.tracks.Store(accountID, &accountNotifyTrack{
		before:     before,
		after:      after,
		reason:     tr.Reason,
		detail:     tr.Detail,
		sourceType: tr.SourceType,
		sourceID:   tr.SourceID,
		deadline:   time.Now().Add(accountNotifyResolveDelay),
	})
}

// runTransitionWorker 周期复查待结算的写操作。时钟兜底是必须的：
// 冷却到期不需要任何写库动作，只有轮询才能发现「冷却结束、重新可调度」并补发恢复邮件；
// 同时也兜住「前端刚确认没进冷却」这类不产生新写入的变化。
func (s *AccountStatusNotifyService) runTransitionWorker() {
	defer s.wg.Done()
	ticker := time.NewTicker(accountNotifyPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.resolveTransitions()
		case <-s.stopCh:
			return
		}
	}
}

// Stop 停止 worker（测试与优雅退出用）。
func (s *AccountStatusNotifyService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() { close(s.stopCh) })
	s.wg.Wait()
}

func (s *AccountStatusNotifyService) resolveTransitions() {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("account transition notification panic", "panic", r)
		}
	}()
	now := time.Now()
	type diff struct {
		accountID  int64
		before     string
		after      string
		detail     string
		sourceType string
		sourceID   string
	}
	var pending []diff
	s.tracks.Range(func(key, _ any) bool {
		id, ok := key.(int64)
		if !ok {
			return true
		}
		value, exists := s.tracks.Load(id)
		if !exists {
			return true
		}
		track, ok := value.(*accountNotifyTrack)
		if !ok || track == nil {
			s.tracks.Delete(id)
			return true
		}
		account := s.loadAccount(context.Background(), id)
		current := AccountAvailabilityState(account)
		// After 已知：以写操作方回读到的状态为准，但要求账号现在仍是它，
		// 否则说明期间又变了（例如冷却中又被禁用），交给最后那次登记去描述。
		target := track.after
		if target != "" && current != "" && current != target {
			target = current
		}
		if target == "" {
			target = current
		}
		// After 未知时只有复查到真实变化才发信；After 已知时到结算点按 After 投递。
		if target != "" && target != track.before && (track.after != "" || current == target) {
			// 已被别的登记发过同一变更：直接丢弃，不再等窗口。
			if !s.claimTransition(id, track.before, target) {
				s.tracks.Delete(id)
				return true
			}
			pending = append(pending, diff{
				accountID:  id,
				before:     track.before,
				after:      target,
				detail:     track.detail,
				sourceType: track.sourceType,
				sourceID:   track.sourceID,
			})
			s.tracks.Delete(id)
			return true
		}
		// 状态没变（冷却没生效、仍停在冷却中）且还在结算窗口内：继续等，
		// 期间理由保持最新；超出窗口说明这次写操作没造成可见变化，不再跟踪。
		if now.Before(track.deadline) {
			if track.detail == "" {
				track.detail = transitionDetailFromAccount(account, track.reason)
			}
			return true
		}
		s.tracks.Delete(id)
		return true
	})
	for _, item := range pending {
		s.notifyStatusChange(context.Background(), item.accountID, item.before, item.after, item.detail)
	}
}

// claimTransition 折叠「同一次变更被多处登记」：同账号同 from->to 在
// accountNotifyClaimTTL 内只放行一次。仓储层与管理层都会登记调度/优先级变化，
// 这是必须的一层，否则管理员会一次操作收到两封邮件。
func (s *AccountStatusNotifyService) claimTransition(accountID int64, from, to string) bool {
	now := time.Now()
	key := strconv.FormatInt(accountID, 10)
	signature := from + "->" + to
	if value, ok := s.claimed.Load(key); ok {
		prev, isClaim := value.(accountTransitionClaim)
		if isClaim && prev.signature == signature && now.Before(prev.expireAt) {
			return false
		}
	}
	s.claimed.Store(key, accountTransitionClaim{signature: signature, expireAt: now.Add(accountNotifyClaimTTL)})
	s.claimed.Range(func(k, v any) bool {
		if prev, ok := v.(accountTransitionClaim); ok && now.After(prev.expireAt) {
			s.claimed.Delete(k)
		}
		return true
	})
	return true
}

// transitionDetailFromAccount 从账号当前冷却字段拼出「为什么 / 到什么时候」，
// 用于写操作方没有显式传 detail 的路径（例如 429 限流只写了 resetAt）。
func transitionDetailFromAccount(account *Account, reason string) string {
	if account == nil {
		return reason
	}
	now := time.Now()
	parts := make([]string, 0, 3)
	if account.TempUnschedulableUntil != nil && now.Before(*account.TempUnschedulableUntil) {
		parts = append(parts, "temp unschedulable until "+howShortUntil(*account.TempUnschedulableUntil))
	}
	if account.RateLimitResetAt != nil && now.Before(*account.RateLimitResetAt) {
		parts = append(parts, "rate limit resets at "+howShortUntil(*account.RateLimitResetAt))
	}
	if account.OverloadUntil != nil && now.Before(*account.OverloadUntil) {
		parts = append(parts, "overload cooldown until "+howShortUntil(*account.OverloadUntil))
	}
	if trimmed := strings.TrimSpace(reason); trimmed != "" {
		parts = append(parts, "reason: "+truncateString(trimmed, accountNotifyReasonPreviewMaxSize))
	} else if stored := strings.TrimSpace(account.TempUnschedulableReason); stored != "" {
		parts = append(parts, "reason: "+truncateString(stored, accountNotifyReasonPreviewMaxSize))
	}
	return strings.Join(parts, "; ")
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
	// 同一账号、同一事件、同一原因在 accountNotifyDedupWindow 内只发一封：
	// 网关高频冷却（并发请求连续 429、连续阈值判定）不应该刷屏；不同事件照常投递。
	if !s.claimDelivery(accountID, toStatus, errorMessage) {
		slog.Info("account status change notification deduplicated",
			"account_id", accountID, "from", oldStatus, "to", toStatus)
		return
	}
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

// claimDelivery 做进程内投递去重：同账号 + 同事件 + 同原因窗口期内只发一封。
// 只在单实例内生效（多实例各自持有一份），够用且不引入额外存储依赖；
// 真正的幂等保障仍由 NotificationEmailService 的 deliveryKey 承担。
func (s *AccountStatusNotifyService) claimDelivery(accountID int64, event, detail string) bool {
	now := time.Now()
	key := strconv.FormatInt(accountID, 10) + "|" + event + "|" + truncateString(strings.TrimSpace(detail), accountNotifyReasonPreviewMaxSize)
	if _, loaded := s.dedup.LoadOrStore(key, now.Add(accountNotifyDedupWindow)); loaded {
		value, ok := s.dedup.Load(key)
		expireAt, isTime := value.(time.Time)
		if ok && isTime && now.Before(expireAt) {
			return false
		}
		s.dedup.Store(key, now.Add(accountNotifyDedupWindow))
	}
	// 顺带清理过期键，避免长期运行 map 无界增长。
	s.dedup.Range(func(k, v any) bool {
		if expireAt, ok := v.(time.Time); ok && now.After(expireAt) {
			s.dedup.Delete(k)
		}
		return true
	})
	return true
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
