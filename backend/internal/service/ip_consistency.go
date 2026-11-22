package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// IP 一致性风控（IP consistency guard）。
//
// 处置统一为「suspect + 扣到 floor + 邮件」（用户拍板 2026-11-21，不再自动禁用）：
// 滚动窗口内（默认 10 分钟窗口 + 可选 1 小时长窗口，长窗口 0 = 关闭）出现
// ≥ipMarkThreshold(3) 个不同客户端 IP 时：写真实 suspect 标记（进入人工复核队列，
// 同时被待复核冻结闸门拦截网关调用与建 Key）+ 把余额降到只剩 $0.03 + 写调账备注 +
// 给用户发一封通知邮件，并中断本次请求。账号状态不再自动置为 disabled。
//
// 命中工单标记 / 人工复核豁免 / IP 白名单的用户整体跳过本风控（见 CheckUser）。
//
// 两个功能各有独立开关，默认都开启；存量 v1 配置在读取时一次性迁移到 v2，
// 保证升级后线上立即生效（管理员之后再进后台关闭才会写回 v2 配置）。
//
// 覆盖入口：
//   - 注册：同一邮箱在窗口内从不同 IP 反复提交注册 → 拒绝。
//   - 登录：登录成功后按用户维度校验，命中 → 打标扣费并拒绝本次登录。
//   - 控制台：JWT 鉴权后校验。
//   - 模型网关：API Key 鉴权后校验。
//
// 豁免：管理员账号、白名单用户、白名单 IP（支持精确 IP 与 CIDR）。
// 降级：Redis 不可用时放行（fail-open），风控不能反过来把整站打挂；
// 处罚依赖（余额服务/邮件服务）未注入或失败时只记日志，不影响请求。

const (
	// SettingKeyIPConsistencySettings IP 一致性风控配置键
	SettingKeyIPConsistencySettings = "ip_consistency_settings"

	defaultIPConsistencyWindowSeconds = 600 // 10 分钟
	minIPConsistencyWindowSeconds     = 60
	maxIPConsistencyWindowSeconds     = 24 * 3600

	// ipMarkThreshold IP 一致性阈值：窗口内达到该数量的不同客户端 IP 即触发统一处置
	// （suspect 标记 + 扣减 + 邮件 + 中断请求）。最近一次调整：标记 4→3、禁用 5→4，
	// 2026-11-21 起取消独立禁用档，3 个 IP 即统一处置。
	ipMarkThreshold = 3

	// defaultIPConsistencyLongWindowSeconds 长窗口默认 1 小时（与 10 分钟主窗口互补，
	// 抓「每 10 分钟窗口都不超标、但 1 小时内缓慢换 IP」的慢速滥用）。
	defaultIPConsistencyLongWindowSeconds = 3600
	// ipConsistencyUserLongKeyPrefix 长窗口的用户维度 Redis key 前缀。0 = 长窗口关闭。
	ipConsistencyUserLongKeyPrefix = "ipguard:user-long:"

	// ipConsistencySettingsVersion 配置结构版本。v1 只有「标记可疑 + 禁用」且默认关闭；
	// v2 拆成两个独立功能且默认都开。存量 v1 在读取时一次性迁移，保证升级后立即生效。
	ipConsistencySettingsVersion = 2

	// DefaultIPBalancePenaltyFloor 命中 IP 异常处罚后保留的余额（美元）。
	DefaultIPBalancePenaltyFloor = 0.03

	// IPAbnormalityMessage 调账备注与通知邮件正文共用的纯英语文案。
	IPAbnormalityMessage = "The system detected an abnormality in your IP."

	// IPAbnormalityEmailSubject 通知邮件标题（纯英语）。
	IPAbnormalityEmailSubject = "Security notice: unusual IP activity on your account"

	// IPPenaltyRefundType 复核放行时返还流水的 type。刻意不用 admin_balance：
	// SumPositiveBalanceByUser 会把 admin_balance 的正数计入「累计充值」，
	// 返还是归还扣减而非充值，用独立 type 避免污染统计；余额历史按 used_by
	// 查询不过滤 type，返还流水仍然可见。
	IPPenaltyRefundType = "ip_penalty_refund"
	// IPPenaltyRefundCodePrefix 返还流水 code 前缀，拼原扣减流水 id 后天然唯一，
	// 配合唯一约束 + ON CONFLICT DO NOTHING 保证同一条扣减只返还一次。
	IPPenaltyRefundCodePrefix = "RF-"
	// IPPenaltyRefundNotePrefix 返还流水备注前缀（纯英语，管理端与客户端可见）。
	IPPenaltyRefundNotePrefix = "Risk review refund for IP penalty #"

	// ipConsistencyPenaltyCooldown 同一用户的处罚冷却期：窗口滚动或用户充值后，
	// 短时间内不重复扣减、不重复发信。DB 层的条件 UPDATE 是第二重幂等保证。
	ipConsistencyPenaltyCooldown = 24 * time.Hour

	// ipConsistencyUserKeyPrefix 用户维度 Redis key 前缀
	ipConsistencyUserKeyPrefix = "ipguard:user:"
	// ipConsistencyRegisterKeyPrefix 注册邮箱维度 Redis key 前缀
	ipConsistencyRegisterKeyPrefix = "ipguard:register:"
	// ipConsistencyPenaltyKeyPrefix 处罚幂等锁 Redis key 前缀
	ipConsistencyPenaltyKeyPrefix = "ipguard:penalty:"
	// ipConsistencyNewUserPenaltyKeyPrefix 新用户严格规则的独立扣减幂等键。
	// 刻意与 3/4 IP 规则的 penalty key 分开，避免两条规则互相吞掉处罚。
	// 该键同时承担「窗口内只罚一次」的职责（24h 冷却），防止同一用户在窗口内
	// 多次换 IP 被重复扣钱 / 重复发信；与判定解耦，IP 一致的调用不占用它。
	ipConsistencyNewUserPenaltyKeyPrefix = "ipguard:newuser-penalty:"
	// ipConsistencyRegIPCorrelationPenaltyKeyPrefix 注册 IP 关联规则的独立扣减幂等键。
	// 与新用户规则、3/4 IP 规则分开，避免互相吞掉处罚；24h 冷却保证只扣一次。
	ipConsistencyRegIPCorrelationPenaltyKeyPrefix = "ipguard:regip-penalty:"
	// newUserIPMismatchWindow 新用户注册后 IP 严格校验窗口：注册后 3 分钟内。
	// 窗口内任意一次 API 调用的客户端 IP 与注册 IP 不一致即禁用 + 扣减 + 邮件；
	// 超过窗口则回落到 CheckUser 的 3/4 IP 滚动窗口规则。
	// 最近一次调整：10 分钟 → 3 分钟，且从「仅首调」改为「窗口内任意一次」（用户拍板 2026-11-20）。
	newUserIPMismatchWindow = 3 * time.Minute
	// newUserIPMismatchReason 真实 suspect 标记的原因码（进人工复核队列）。
	// 保留旧值 new_user_first_use_ip_mismatch 以兼容历史数据，语义已扩展为窗口内任意一次。
	newUserIPMismatchReason = "new_user_first_use_ip_mismatch"
)

// ErrIPConsistencyBlocked IP 一致性校验未通过（注册/登录返回 403）。
var ErrIPConsistencyBlocked = infraerrors.Forbidden(
	"USER_IP_MISMATCH",
	"account restricted: client IP changed within the monitoring window",
)

// IPConsistencySettings IP 一致性风控配置（v2）。
type IPConsistencySettings struct {
	// Enabled 功能A：≥ipMarkThreshold 个 IP 打真实 suspect 标记并中断请求（默认开启）。
	Enabled bool `json:"enabled"`
	// WindowSeconds 滚动窗口秒数（默认 600，取值区间 60 ~ 86400）
	WindowSeconds int `json:"window_seconds"`
	// LongWindowSeconds 长窗口秒数（默认 3600；0 = 关闭；开启时取值区间 60 ~ 86400）。
	// 与主窗口独立计数，任一窗口达到阈值即触发统一处置。
	LongWindowSeconds int `json:"long_window_seconds"`
	// WhitelistUserIDs 豁免的用户 ID 列表
	WhitelistUserIDs []int64 `json:"whitelist_user_ids"`
	// WhitelistIPs 豁免的 IP / CIDR 列表
	WhitelistIPs []string `json:"whitelist_ips"`

	// SchemaVersion 配置结构版本；<2 视为存量 v1，读取时迁移到 v2 默认值。
	SchemaVersion int `json:"schema_version"`
	// BalancePenaltyEnabled 功能B：≥ipMarkThreshold 个 IP 把余额降到 floor + 写备注 + 发通知邮件（默认开启）。
	BalancePenaltyEnabled bool `json:"balance_penalty_enabled"`
	// BalancePenaltyFloor 处罚后保留的余额（默认 0.03，只在当前余额高于它时扣减）。
	BalancePenaltyFloor float64 `json:"balance_penalty_floor"`
	// PenaltyNotifyEmail 处罚确实扣到钱时是否发通知邮件（默认开启）。
	PenaltyNotifyEmail bool `json:"penalty_notify_email"`
}

// DefaultIPConsistencySettings 返回 v2 默认配置：两个功能都开启 + 10 分钟主窗口 + 1 小时长窗口。
func DefaultIPConsistencySettings() *IPConsistencySettings {
	return &IPConsistencySettings{
		Enabled:               true,
		WindowSeconds:         defaultIPConsistencyWindowSeconds,
		LongWindowSeconds:     defaultIPConsistencyLongWindowSeconds,
		SchemaVersion:         ipConsistencySettingsVersion,
		BalancePenaltyEnabled: true,
		BalancePenaltyFloor:   DefaultIPBalancePenaltyFloor,
		PenaltyNotifyEmail:    true,
	}
}

// normalizeIPConsistencySettings 修正非法取值、清理空白项，并把存量 v1 配置升级到 v2。
//
// 迁移语义：v1 的 Enabled 默认 false，很多线上库可能存过显式关闭值；用户已明确要求
// 升级后两个功能都打开且立即生效，因此读取路径直接把 v1 重置为 v2 默认值，只保留
// 窗口与白名单。写入路径（SetIPConsistencySettings）会先把 SchemaVersion 置为 2，
// 因此管理员之后保存的任何选择都不会被这里覆盖。
func normalizeIPConsistencySettings(s *IPConsistencySettings) {
	if s == nil {
		return
	}
	if s.SchemaVersion < ipConsistencySettingsVersion {
		defaults := DefaultIPConsistencySettings()
		defaults.WindowSeconds = s.WindowSeconds
		defaults.WhitelistUserIDs = s.WhitelistUserIDs
		defaults.WhitelistIPs = s.WhitelistIPs
		*s = *defaults
	}
	if s.WindowSeconds <= 0 {
		s.WindowSeconds = defaultIPConsistencyWindowSeconds
	}
	if s.WindowSeconds < minIPConsistencyWindowSeconds {
		s.WindowSeconds = minIPConsistencyWindowSeconds
	}
	if s.WindowSeconds > maxIPConsistencyWindowSeconds {
		s.WindowSeconds = maxIPConsistencyWindowSeconds
	}
	// 长窗口：0 表示关闭；负数视为未配置回落默认；开启时与主窗口同区间收敛。
	if s.LongWindowSeconds < 0 {
		s.LongWindowSeconds = defaultIPConsistencyLongWindowSeconds
	}
	if s.LongWindowSeconds > 0 {
		if s.LongWindowSeconds < minIPConsistencyWindowSeconds {
			s.LongWindowSeconds = minIPConsistencyWindowSeconds
		}
		if s.LongWindowSeconds > maxIPConsistencyWindowSeconds {
			s.LongWindowSeconds = maxIPConsistencyWindowSeconds
		}
	}
	if s.BalancePenaltyFloor <= 0 || s.BalancePenaltyFloor > 100 {
		s.BalancePenaltyFloor = DefaultIPBalancePenaltyFloor
	}
	if len(s.WhitelistIPs) > 0 {
		cleaned := s.WhitelistIPs[:0]
		for _, entry := range s.WhitelistIPs {
			if v := strings.TrimSpace(entry); v != "" {
				cleaned = append(cleaned, v)
			}
		}
		s.WhitelistIPs = cleaned
	}
	s.SchemaVersion = ipConsistencySettingsVersion
}

// validateIPConsistencyWhitelist 校验白名单条目必须是合法 IP 或 CIDR。
func validateIPConsistencyWhitelist(entries []string) error {
	for _, entry := range entries {
		v := strings.TrimSpace(entry)
		if v == "" {
			continue
		}
		if _, err := netip.ParseAddr(v); err == nil {
			continue
		}
		if _, err := netip.ParsePrefix(v); err == nil {
			continue
		}
		return fmt.Errorf("invalid whitelist entry %q: must be an IP address or CIDR", entry)
	}
	return nil
}

// GetIPConsistencySettings 直读 DB（管理端读写路径使用）。
func (s *SettingService) GetIPConsistencySettings(ctx context.Context) (*IPConsistencySettings, error) {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyIPConsistencySettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return DefaultIPConsistencySettings(), nil
		}
		return nil, fmt.Errorf("get ip consistency settings: %w", err)
	}
	if strings.TrimSpace(value) == "" {
		return DefaultIPConsistencySettings(), nil
	}

	settings := &IPConsistencySettings{}
	if err := json.Unmarshal([]byte(value), settings); err != nil {
		slog.Warn("failed to unmarshal ip consistency settings, falling back to defaults",
			"error", err, "key", SettingKeyIPConsistencySettings)
		return DefaultIPConsistencySettings(), nil
	}
	normalizeIPConsistencySettings(settings)
	return settings, nil
}

// SetIPConsistencySettings 保存配置并让进程内缓存立即失效。
func (s *SettingService) SetIPConsistencySettings(ctx context.Context, settings *IPConsistencySettings) error {
	if settings == nil {
		return fmt.Errorf("settings cannot be nil")
	}
	if err := validateIPConsistencyWhitelist(settings.WhitelistIPs); err != nil {
		return err
	}
	// 写入路径按 v2 处理：不能让 v1→v2 的强制迁移覆盖管理员本次的显式选择
	// （尤其是主动关闭某个功能）。读取路径才负责把存量 v1 升级成默认全开。
	settings.SchemaVersion = ipConsistencySettingsVersion
	normalizeIPConsistencySettings(settings)

	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal ip consistency settings: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyIPConsistencySettings, string(data)); err != nil {
		return err
	}
	s.invalidateCachedSettingValues(SettingKeyIPConsistencySettings)
	return nil
}

// GetIPConsistencySettingsCached 读取配置（进程内缓存，热路径安全）。
// DB 不可用时返回 v2 默认值（两个功能都开启）——与线上默认口径保持一致；
// 真正的 fail-open 由 Redis 窗口读取失败时的降级分支保证。
func (s *SettingService) GetIPConsistencySettingsCached(ctx context.Context) IPConsistencySettings {
	value, found := s.getCachedSettingValue(ctx, SettingKeyIPConsistencySettings)
	if !found || strings.TrimSpace(value) == "" {
		return *DefaultIPConsistencySettings()
	}
	var settings IPConsistencySettings
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return *DefaultIPConsistencySettings()
	}
	normalizeIPConsistencySettings(&settings)
	return settings
}

// IPConsistencySettingsProvider 提供 IP 一致性配置（*SettingService 实现）。
type IPConsistencySettingsProvider interface {
	GetIPConsistencySettingsCached(ctx context.Context) IPConsistencySettings
}

// IPConsistencyCache IP 一致性窗口存储（Redis ZSET 实现，见 repository 层）。
type IPConsistencyCache interface {
	// RecordAndListIPs 记录本次 IP 访问并返回窗口内出现过的去重 IP 列表。
	RecordAndListIPs(ctx context.Context, key, clientIP string, now time.Time, window time.Duration) ([]string, error)
	// Clear 清除某个 key 的窗口记录。
	Clear(ctx context.Context, key string) error
}

// RiskExemptionCheck 全局豁免判断能力（由 *RiskExemptionChecker 实现）。
// 独立成窄接口便于测试桩，且同样是可选依赖：未注入时豁免不生效，处罚照常。
type RiskExemptionCheck interface {
	IsExempt(ctx context.Context, userID int64) bool
	Invalidate(userID int64)
}

// NewUserRiskMarker 写入「真实 suspect」标记的能力（由 *AffiliateService 实现）。
// suspect 会触发 RiskAccessBlocked，并在管理端复核队列中可见；与旧 ip_suspect
// 轻标记完全不同的处置档位。独立成窄接口便于测试与可选注入。
type NewUserRiskMarker interface {
	MarkUserRisk(ctx context.Context, userID int64, level string, reasons []string)
}

// IPConsistencyPenaltyLocker 处罚幂等锁（由 repository 的 IP 窗口缓存实现）。
// 独立成可选接口，避免逼所有 IPConsistencyCache 测试桩实现新方法。
type IPConsistencyPenaltyLocker interface {
	TryClaimPenalty(ctx context.Context, key string, ttl time.Duration) (bool, error)
	ReleasePenalty(ctx context.Context, key string) error
}

// IPConsistencyPenalty 处罚执行器（余额 + 调账流水 + 通知邮件）。
type IPConsistencyPenalty interface {
	// ApplyIPBalancePenalty 把余额原子降到 floor、按 notes 写调账流水；
	// notify=true 且确实发生扣减时给用户发通知邮件。未发生扣减则不发信。
	ApplyIPBalancePenalty(ctx context.Context, userID int64, floor float64, notes string, notify bool) error
}

// ipInconsistencyReason 可疑标记写入 risk_reasons 的原因码（逗号追加、幂等）。
const ipInconsistencyReason = "ip_inconsistent"

// IPConsistencyViolation 一次 IP 不一致命中的结果。
type IPConsistencyViolation struct {
	UserID    int64
	CurrentIP string
	KnownIPs  []string
}

// IPConsistencyService IP 一致性风控服务。
type IPConsistencyService struct {
	cache             IPConsistencyCache
	settingsProvider  IPConsistencySettingsProvider
	penalty           IPConsistencyPenalty
	penaltyLocker     IPConsistencyPenaltyLocker
	exemptionChecker  RiskExemptionCheck
	newUserRiskMarker NewUserRiskMarker
}

// NewIPConsistencyService 创建 IP 一致性风控服务。
func NewIPConsistencyService(
	cache IPConsistencyCache,
	settingsProvider IPConsistencySettingsProvider,
) *IPConsistencyService {
	return &IPConsistencyService{
		cache:            cache,
		settingsProvider: settingsProvider,
	}
}

// SetPenaltyService 注入减额度处罚执行器与幂等锁（可选依赖）。
// 未注入时功能B 静默跳过、只记录告警，功能A（标记/禁用）照常工作，绝不阻断鉴权。
func (s *IPConsistencyService) SetPenaltyService(penalty IPConsistencyPenalty, locker IPConsistencyPenaltyLocker) {
	if s == nil {
		return
	}
	s.penalty = penalty
	s.penaltyLocker = locker
}

// SetRiskExemptionChecker 注入全局豁免检查器（可选依赖）。
// 未注入时豁免不生效、处罚照常，与注入失败时的行为一致。
func (s *IPConsistencyService) SetRiskExemptionChecker(checker RiskExemptionCheck) {
	if s == nil {
		return
	}
	s.exemptionChecker = checker
}

// SetNewUserRiskMarker 注入真实 suspect 打标器（可选依赖）。
// 未注入时仍会禁用与扣钱，只是复核队列里看不到标记，绝不阻断鉴权。
func (s *IPConsistencyService) SetNewUserRiskMarker(marker NewUserRiskMarker) {
	if s == nil {
		return
	}
	s.newUserRiskMarker = marker
}

// CheckNewUserFirstUseIP 新用户注册后 3 分钟内「客户端 IP 与注册 IP 不一致」的严格校验。
//
// 与按窗口统计 IP 数量的 CheckUser 是两条独立规则：这里只比对注册 IP 与当前 IP，
// 窗口内任意一次不一致即 suspect + 扣减 + 邮件（2026-11-21 起不再自动禁用），并中断本次请求。返回非 nil 表示命中。
//
// 与旧版「仅首调」语义的关键差异（用户拍板 2026-11-20）：
//   - 旧版用 Redis SETNX 首调令牌，首调 IP 一致就消费令牌，之后换 IP 不追罚；
//   - 新版不消费任何令牌，窗口内每次调用都比对，第一次出现不一致就命中。
//
// 「只罚一次」由独立的扣减幂等键（ipConsistencyNewUserPenaltyKeyPrefix + 24h 冷却）保证：
// 禁用本身是幂等的，扣减/发信由该键去重，因此窗口内多次换 IP 不会重复扣钱 / 重复发信。
// IP 一致的调用直接放行，不占用该键，保证窗口内后续一旦换 IP 仍能被抓到。
//
// Redis 不可用、字段缺失、超过窗口、管理员、白名单与工单/复核豁免均 fail-open。
func (s *IPConsistencyService) CheckNewUserFirstUseIP(
	ctx context.Context,
	userID int64,
	role, clientIP, registerIP string,
	createdAt time.Time,
) *IPConsistencyViolation {
	if s == nil || s.penaltyLocker == nil || userID <= 0 || role == RoleAdmin || createdAt.IsZero() {
		return nil
	}
	age := time.Since(createdAt)
	if age < 0 || age >= newUserIPMismatchWindow {
		return nil
	}

	clientIP = normalizeClientIP(clientIP)
	registerIP = normalizeClientIP(registerIP)
	if clientIP == "" || registerIP == "" {
		return nil
	}

	settings := *DefaultIPConsistencySettings()
	if s.settingsProvider != nil {
		settings = s.settingsProvider.GetIPConsistencySettingsCached(ctx)
	}
	// 与主 IP 规则同口径：两个功能都关时整条规则不生效；白名单、工单与复核豁免照旧。
	if !settings.Enabled && !settings.BalancePenaltyEnabled {
		return nil
	}
	if containsInt64(settings.WhitelistUserIDs, userID) || ipWhitelisted(clientIP, settings.WhitelistIPs) {
		return nil
	}
	if s.exemptionChecker != nil && s.exemptionChecker.IsExempt(ctx, userID) {
		return nil
	}

	// 窗口内 IP 一致：直接放行，绝不消费扣减幂等键——否则后续换 IP 就罚不到了。
	if clientIP == registerIP {
		return nil
	}

	if s.newUserRiskMarker != nil {
		s.newUserRiskMarker.MarkUserRisk(ctx, userID, AffiliateRiskLevelSuspect, []string{newUserIPMismatchReason})
	}
	// 统一处置（用户拍板 2026-11-21）：suspect + 扣到 floor + 邮件，不再自动禁用账号；
	// suspect 触发的待复核冻结闸门（禁建 Key + 网关 403）承担后续拦截。
	// 严格规则无条件扣钱，不受「3 IP」规则的功能B 开关影响；使用独立幂等键，
	// 该键的 24h 冷却保证窗口内多次命中只扣一次钱、只发一封邮件。
	s.scheduleBalancePenaltyWithKey(ctx, userID, settings.BalancePenaltyFloor,
		settings.PenaltyNotifyEmail, ipConsistencyNewUserPenaltyKeyPrefix+fmt.Sprintf("%d", userID))

	slog.Warn("account marked suspect: new-user client ip differs from registration ip within guard window",
		"audit", true,
		"user_id", userID,
		"current_ip", clientIP,
		"register_ip", registerIP,
	)
	return &IPConsistencyViolation{
		UserID:    userID,
		CurrentIP: clientIP,
		KnownIPs:  []string{registerIP},
	}
}

// CheckUser 记录并校验已认证用户的客户端 IP 一致性（主窗口 + 可选长窗口）。
// 返回 nil 表示放行；返回非 nil 表示任一窗口内命中 ≥ipMarkThreshold 个不同 IP，
// 已写真实 suspect 标记、触发扣减并中断本次请求。
func (s *IPConsistencyService) CheckUser(ctx context.Context, userID int64, role, clientIP string) *IPConsistencyViolation {
	if s == nil || s.cache == nil || s.settingsProvider == nil || userID <= 0 {
		return nil
	}
	settings := s.settingsProvider.GetIPConsistencySettingsCached(ctx)
	// 两个功能都关时完全跳过，连 Redis 往返都省掉。
	if !settings.Enabled && !settings.BalancePenaltyEnabled {
		return nil
	}
	// 管理员账号豁免，避免把管理员自己锁在后台之外。
	if role == RoleAdmin {
		return nil
	}
	if containsInt64(settings.WhitelistUserIDs, userID) {
		return nil
	}

	clientIP = normalizeClientIP(clientIP)
	if clientIP == "" || ipWhitelisted(clientIP, settings.WhitelistIPs) {
		return nil
	}
	// 全局豁免：提过工单 / 人工复核放行 / IP 白名单命中者，跳过一切自动处罚
	// （suspect 标记 + 扣减）。白名单已在上面判断；这里查工单标记与复核豁免。
	if s.exemptionChecker != nil && s.exemptionChecker.IsExempt(ctx, userID) {
		return nil
	}

	// 主窗口（默认 10 分钟）与长窗口（默认 1 小时，0 = 关闭）独立计数；
	// 任一窗口达到阈值即触发统一处置。两个窗口共用同一处罚幂等键（24h 冷却），
	// 保证同一次滥用只扣一次钱、只发一封邮件。
	mainKey := ipConsistencyUserKeyPrefix + fmt.Sprintf("%d", userID)
	ips, err := s.cache.RecordAndListIPs(ctx, mainKey, clientIP, time.Now(), time.Duration(settings.WindowSeconds)*time.Second)
	if err != nil {
		// 降级放行：Redis 故障不应导致整站 403。
		slog.Warn("ip consistency check skipped: cache unavailable", "user_id", userID, "error", err)
		return nil
	}
	var longIPs []string
	if settings.LongWindowSeconds > 0 {
		longKey := ipConsistencyUserLongKeyPrefix + fmt.Sprintf("%d", userID)
		recorded, longErr := s.cache.RecordAndListIPs(ctx, longKey, clientIP, time.Now(), time.Duration(settings.LongWindowSeconds)*time.Second)
		if longErr != nil {
			// 长窗口降级不影响主窗口判定（fail-open）。
			slog.Warn("ip consistency long-window check skipped: cache unavailable", "user_id", userID, "error", longErr)
		} else {
			longIPs = recorded
		}
	}

	mainHit := len(ips) >= ipMarkThreshold
	longHit := len(longIPs) >= ipMarkThreshold
	if !mainHit && !longHit {
		return nil
	}

	hitIPs := ips
	hitWindow := settings.WindowSeconds
	if !mainHit {
		hitIPs = longIPs
		hitWindow = settings.LongWindowSeconds
	}

	// 功能B：扣到 floor + 通知邮件。幂等锁保证一个冷却期内只扣一次、只发一封信。
	if settings.BalancePenaltyEnabled {
		s.scheduleBalancePenalty(ctx, userID, settings)
	}

	// 功能A：写真实 suspect 标记（进人工复核队列，并由待复核冻结闸门拦截后续调用），
	// 中断本次请求。不再自动禁用账号（用户拍板 2026-11-21）。
	if settings.Enabled {
		if s.newUserRiskMarker != nil {
			s.newUserRiskMarker.MarkUserRisk(ctx, userID, AffiliateRiskLevelSuspect, []string{ipInconsistencyReason})
		}
		slog.Warn("user marked suspect: threshold client ips within monitoring window",
			"audit", true,
			"user_id", userID,
			"current_ip", clientIP,
			"known_ips", hitIPs,
			"window_seconds", hitWindow,
			"long_window_hit", !mainHit,
		)
		return &IPConsistencyViolation{
			UserID:    userID,
			CurrentIP: clientIP,
			KnownIPs:  hitIPs,
		}
	}
	return nil
}

// scheduleBalancePenalty 抢到幂等锁后异步执行处罚。
// CheckUser 位于登录 / JWT / API Key 鉴权热路径，DB 写入与 SMTP 绝不能同步执行。
func (s *IPConsistencyService) scheduleBalancePenalty(ctx context.Context, userID int64, settings IPConsistencySettings) {
	s.scheduleBalancePenaltyWithKey(ctx, userID, settings.BalancePenaltyFloor, settings.PenaltyNotifyEmail,
		ipConsistencyPenaltyKeyPrefix+fmt.Sprintf("%d", userID))
}

// ScheduleRegistrationIPCorrelationPenalty 注册 IP 关联规则命中后的自动处罚：
// 扣到 floor + 通知邮件（确实发生扣减才发信，notes 沿用 IPAbnormalityMessage
// 保证人工复核放行时可返还）。异步执行，独立幂等键 24h 冷却内只扣一次。
// 供注册流程（auth_service）在 suspect 打标后调用；依赖未注入时静默跳过不阻断注册。
func (s *IPConsistencyService) ScheduleRegistrationIPCorrelationPenalty(ctx context.Context, userID int64) {
	if s == nil || userID <= 0 {
		return
	}
	floor := DefaultIPBalancePenaltyFloor
	notify := true
	if s.settingsProvider != nil {
		settings := s.settingsProvider.GetIPConsistencySettingsCached(ctx)
		floor = settings.BalancePenaltyFloor
		notify = settings.PenaltyNotifyEmail
	}
	s.scheduleBalancePenaltyWithKey(ctx, userID, floor, notify,
		ipConsistencyRegIPCorrelationPenaltyKeyPrefix+fmt.Sprintf("%d", userID))
}

// scheduleBalancePenaltyWithKey 是处罚调度的唯一实现；不同规则通过不同 key 隔离幂等。
func (s *IPConsistencyService) scheduleBalancePenaltyWithKey(ctx context.Context, userID int64, floor float64, notify bool, key string) {
	if s.penalty == nil || s.penaltyLocker == nil {
		slog.Warn("ip balance penalty skipped: penalty dependencies not wired",
			"audit", true, "user_id", userID)
		return
	}
	claimed, err := s.penaltyLocker.TryClaimPenalty(ctx, key, ipConsistencyPenaltyCooldown)
	if err != nil {
		// 锁不可用时不处罚，也不阻断请求（fail-open）；下次请求仍有机会重试。
		slog.Warn("ip balance penalty skipped: lock unavailable",
			"user_id", userID, "error", err)
		return
	}
	if !claimed {
		return // 冷却期内已处罚过
	}
	// 邮件失败不算失败（见处罚服务），不释放锁。
	penalty := s.penalty
	locker := s.penaltyLocker
	go func() {
		bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cancel()
		if err := penalty.ApplyIPBalancePenalty(bgCtx, userID, floor, IPAbnormalityMessage, notify); err != nil {
			slog.Error("ip balance penalty failed",
				"audit", true, "user_id", userID, "error", err)
			// 余额/流水失败时释放锁，让后续请求可以重试；邮件失败不算失败（见处罚服务）。
			// 释放用独立 context：bgCtx 可能刚好因 15s 超时被取消，导致锁白留 24 小时。
			releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer releaseCancel()
			if releaseErr := locker.ReleasePenalty(releaseCtx, key); releaseErr != nil {
				slog.Warn("ip balance penalty lock release failed",
					"user_id", userID, "error", releaseErr)
			}
		}
	}()
}

// CheckRegistration 校验同一注册邮箱在窗口内的 IP 一致性。
// 返回 nil 放行；返回 ErrIPConsistencyBlocked 表示拒绝注册。
func (s *IPConsistencyService) CheckRegistration(ctx context.Context, email, clientIP string) error {
	if s == nil || s.cache == nil || s.settingsProvider == nil {
		return nil
	}
	settings := s.settingsProvider.GetIPConsistencySettingsCached(ctx)
	if !settings.Enabled {
		return nil
	}
	clientIP = normalizeClientIP(clientIP)
	if clientIP == "" || ipWhitelisted(clientIP, settings.WhitelistIPs) {
		return nil
	}

	key := ipConsistencyRegisterKey(email)
	ips, err := s.cache.RecordAndListIPs(ctx, key, clientIP, time.Now(), time.Duration(settings.WindowSeconds)*time.Second)
	if err != nil {
		slog.Warn("registration ip consistency check skipped: cache unavailable", "error", err)
		return nil
	}
	if len(ips) <= 1 {
		return nil
	}
	slog.Warn("registration rejected: client ip changed within monitoring window",
		"audit", true,
		"current_ip", clientIP,
		"known_ips", ips,
		"window_seconds", settings.WindowSeconds,
	)
	return ErrIPConsistencyBlocked
}

// RecordUserIP 把账号的首次 IP（如注册 IP）写入用户维度主窗口与长窗口，
// 让后续登录/控制台/API 访问能与之比对。
func (s *IPConsistencyService) RecordUserIP(ctx context.Context, userID int64, clientIP string) {
	if s == nil || s.cache == nil || s.settingsProvider == nil || userID <= 0 {
		return
	}
	settings := s.settingsProvider.GetIPConsistencySettingsCached(ctx)
	// 两个功能任一开启都要记录首次 IP，否则功能B 单独开启时窗口会少算一个 IP。
	if !settings.Enabled && !settings.BalancePenaltyEnabled {
		return
	}
	clientIP = normalizeClientIP(clientIP)
	if clientIP == "" {
		return
	}
	now := time.Now()
	windows := []struct {
		key    string
		window time.Duration
	}{
		{ipConsistencyUserKeyPrefix + fmt.Sprintf("%d", userID), time.Duration(settings.WindowSeconds) * time.Second},
	}
	if settings.LongWindowSeconds > 0 {
		windows = append(windows, struct {
			key    string
			window time.Duration
		}{ipConsistencyUserLongKeyPrefix + fmt.Sprintf("%d", userID), time.Duration(settings.LongWindowSeconds) * time.Second})
	}
	for _, w := range windows {
		if _, err := s.cache.RecordAndListIPs(ctx, w.key, clientIP, now, w.window); err != nil {
			slog.Warn("failed to record registration ip", "user_id", userID, "error", err)
		}
	}
}

// ipConsistencyRegisterKey 生成注册邮箱维度 key（对邮箱做哈希，避免明文落 Redis）。
func ipConsistencyRegisterKey(email string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	return ipConsistencyRegisterKeyPrefix + hex.EncodeToString(sum[:16])
}

// clientIPFromContext 读取 SessionBindingContext 注入的客户端 IP。
func clientIPFromContext(ctx context.Context) string {
	if binding := SessionBindingFromContext(ctx); binding != nil {
		return binding.IP
	}
	return ""
}

// normalizeClientIP 规范化客户端 IP：去空白、IPv4-mapped IPv6 还原为 IPv4。
func normalizeClientIP(clientIP string) string {
	v := strings.TrimSpace(clientIP)
	if v == "" {
		return ""
	}
	if addr, err := netip.ParseAddr(v); err == nil {
		return addr.Unmap().String()
	}
	return v
}

// ipWhitelisted 判断客户端 IP 是否命中白名单（精确 IP 或 CIDR）。
func ipWhitelisted(clientIP string, whitelist []string) bool {
	if len(whitelist) == 0 {
		return false
	}
	addr, err := netip.ParseAddr(clientIP)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, entry := range whitelist {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if prefix, err := netip.ParsePrefix(entry); err == nil {
			if prefix.Contains(addr) {
				return true
			}
			continue
		}
		if allowed, err := netip.ParseAddr(entry); err == nil && allowed.Unmap() == addr {
			return true
		}
	}
	return false
}
