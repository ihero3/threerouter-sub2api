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
// 目标：同一个账号在「滚动窗口」（默认 10 分钟）内出现两个及以上不同的客户端 IP，
// 判定为账号共享/盗用，直接禁用账号（Status=disabled，管理员可在后台解封）。
//
// 覆盖入口：
//   - 注册：同一邮箱在窗口内从不同 IP 反复提交注册 → 拒绝。
//   - 登录：登录成功后按用户维度校验，命中 → 禁用并拒绝本次登录。
//   - 控制台：JWT 鉴权后校验。
//   - 模型网关：API Key 鉴权后校验。
//
// 豁免：管理员账号、白名单用户、白名单 IP（支持精确 IP 与 CIDR）。
// 降级：Redis 不可用时放行（fail-open），风控不能反过来把整站打挂。

const (
	// SettingKeyIPConsistencySettings IP 一致性风控配置键
	SettingKeyIPConsistencySettings = "ip_consistency_settings"

	defaultIPConsistencyWindowSeconds = 600 // 10 分钟
	minIPConsistencyWindowSeconds     = 60
	maxIPConsistencyWindowSeconds     = 24 * 3600

	// ipConsistencyUserKeyPrefix 用户维度 Redis key 前缀
	ipConsistencyUserKeyPrefix = "ipguard:user:"
	// ipConsistencyRegisterKeyPrefix 注册邮箱维度 Redis key 前缀
	ipConsistencyRegisterKeyPrefix = "ipguard:register:"
)

// ErrIPConsistencyBlocked IP 一致性校验未通过（注册/登录返回 403）。
var ErrIPConsistencyBlocked = infraerrors.Forbidden(
	"USER_IP_MISMATCH",
	"account disabled: client IP changed within the monitoring window",
)

// IPConsistencySettings IP 一致性风控配置。
type IPConsistencySettings struct {
	// Enabled 总开关（默认关闭）
	Enabled bool `json:"enabled"`
	// WindowSeconds 滚动窗口秒数（默认 600，取值区间 60 ~ 86400）
	WindowSeconds int `json:"window_seconds"`
	// WhitelistUserIDs 豁免的用户 ID 列表
	WhitelistUserIDs []int64 `json:"whitelist_user_ids"`
	// WhitelistIPs 豁免的 IP / CIDR 列表
	WhitelistIPs []string `json:"whitelist_ips"`
}

// DefaultIPConsistencySettings 返回默认配置：关闭 + 10 分钟窗口。
func DefaultIPConsistencySettings() *IPConsistencySettings {
	return &IPConsistencySettings{
		Enabled:       false,
		WindowSeconds: defaultIPConsistencyWindowSeconds,
	}
}

// normalizeIPConsistencySettings 修正非法取值并清理空白项。
func normalizeIPConsistencySettings(s *IPConsistencySettings) {
	if s == nil {
		return
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
	if len(s.WhitelistIPs) > 0 {
		cleaned := s.WhitelistIPs[:0]
		for _, entry := range s.WhitelistIPs {
			if v := strings.TrimSpace(entry); v != "" {
				cleaned = append(cleaned, v)
			}
		}
		s.WhitelistIPs = cleaned
	}
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
// DB 不可用时返回默认值（关闭）——风控降级为不拦截。
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

// UserStatusUpdater IP 一致性风控对用户状态/标记的能力要求（*UserService 实现）。
type UserStatusUpdater interface {
	UpdateStatus(ctx context.Context, userID int64, status string) error
	// MarkIPInconsistent 给用户打「窗口内 IP 不一致」可疑标记（risk_level=suspect）。
	MarkIPInconsistent(ctx context.Context, userID int64, reason string)
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
	userStatusUpdater UserStatusUpdater
}

// NewIPConsistencyService 创建 IP 一致性风控服务。
func NewIPConsistencyService(
	cache IPConsistencyCache,
	settingsProvider IPConsistencySettingsProvider,
	userStatusUpdater UserStatusUpdater,
) *IPConsistencyService {
	return &IPConsistencyService{
		cache:             cache,
		settingsProvider:  settingsProvider,
		userStatusUpdater: userStatusUpdater,
	}
}

// CheckUser 记录并校验已认证用户的客户端 IP 一致性。
// 返回 nil 表示放行；返回非 nil 表示命中：账号已被自动禁用。
func (s *IPConsistencyService) CheckUser(ctx context.Context, userID int64, role, clientIP string) *IPConsistencyViolation {
	if s == nil || s.cache == nil || s.settingsProvider == nil || userID <= 0 {
		return nil
	}
	settings := s.settingsProvider.GetIPConsistencySettingsCached(ctx)
	if !settings.Enabled {
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

	key := ipConsistencyUserKeyPrefix + fmt.Sprintf("%d", userID)
	ips, err := s.cache.RecordAndListIPs(ctx, key, clientIP, time.Now(), time.Duration(settings.WindowSeconds)*time.Second)
	if err != nil {
		// 降级放行：Redis 故障不应导致整站 403。
		slog.Warn("ip consistency check skipped: cache unavailable", "user_id", userID, "error", err)
		return nil
	}
	// 判定阈值：窗口内 ≥3 个不同客户端 IP → 自动禁用；
	// 恰好 2 个 → 仅打可疑标记（管理端用户列表「最后使用 IP」旁展示），不封禁。
	switch {
	case len(ips) >= 3:
		if s.userStatusUpdater != nil {
			if err := s.userStatusUpdater.UpdateStatus(ctx, userID, StatusDisabled); err != nil {
				slog.Error("failed to disable user after ip consistency violation",
					"user_id", userID, "error", err)
			}
		}
		_ = s.cache.Clear(ctx, key)

		slog.Warn("account disabled: multiple client ips within monitoring window",
			"audit", true,
			"user_id", userID,
			"current_ip", clientIP,
			"known_ips", ips,
			"window_seconds", settings.WindowSeconds,
		)

		return &IPConsistencyViolation{
			UserID:    userID,
			CurrentIP: clientIP,
			KnownIPs:  ips,
		}
	case len(ips) == 2:
		if s.userStatusUpdater != nil {
			s.userStatusUpdater.MarkIPInconsistent(ctx, userID, ipInconsistencyReason)
		}
		slog.Warn("user marked suspect: two client ips within monitoring window",
			"audit", true,
			"user_id", userID,
			"current_ip", clientIP,
			"known_ips", ips,
			"window_seconds", settings.WindowSeconds,
		)
		return nil
	}
	return nil
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

// RecordUserIP 把账号的首次 IP（如注册 IP）写入用户维度窗口，
// 让后续登录/控制台/API 访问能与之比对。
func (s *IPConsistencyService) RecordUserIP(ctx context.Context, userID int64, clientIP string) {
	if s == nil || s.cache == nil || s.settingsProvider == nil || userID <= 0 {
		return
	}
	settings := s.settingsProvider.GetIPConsistencySettingsCached(ctx)
	if !settings.Enabled {
		return
	}
	clientIP = normalizeClientIP(clientIP)
	if clientIP == "" {
		return
	}
	key := ipConsistencyUserKeyPrefix + fmt.Sprintf("%d", userID)
	if _, err := s.cache.RecordAndListIPs(ctx, key, clientIP, time.Now(), time.Duration(settings.WindowSeconds)*time.Second); err != nil {
		slog.Warn("failed to record registration ip", "user_id", userID, "error", err)
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

