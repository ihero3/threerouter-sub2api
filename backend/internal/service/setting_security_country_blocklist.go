package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// SecurityCountryBlocklistSettings 全站国家/地区封禁配置。
//
// 作用范围与全站 IP 黑名单一致：面板 /api/*（管理端与支付 webhook 豁免）与
// 模型网关 /v1 /v1beta /backend-api /antigravity /models，命中即 403。
// 国家/地区代码取 GeoLite2-Country 的 ISO 3166-1 alpha-2（如 CN、US、DE），
// GeoLite2 对 CN、HK、MO、TW 分别返回不同代码，需要一并限制时请分别填写。
//
// 默认关闭且名单为空：未配置 GeoIP 数据库的部署升级后行为完全不变。
type SecurityCountryBlocklistSettings struct {
	// Enabled 总开关（默认关闭）。关闭时不解析 GeoIP、不拦截任何请求，名单保留不丢。
	Enabled bool `json:"enabled"`
	// Countries 被封禁的国家/地区代码列表，统一按大写保存。
	Countries []string `json:"countries"`
}

const (
	securityCountryBlocklistCacheTTL  = 60 * time.Second
	securityCountryBlocklistErrorTTL  = 5 * time.Second
	securityCountryBlocklistDBTimeout = 5 * time.Second
	// maxSecurityCountryBlocklistEntries ISO 3166-1 共 249 个代码，留少量余量，
	// 防止误粘贴超大文本拖慢全局中间件的逐项匹配。
	maxSecurityCountryBlocklistEntries = 300
)

// cachedSecurityCountryBlocklistSettings 进程内缓存条目（60s TTL）。
type cachedSecurityCountryBlocklistSettings struct {
	settings  SecurityCountryBlocklistSettings
	expiresAt int64 // unix nano
}

// DefaultSecurityCountryBlocklistSettings 返回默认配置（关闭、空名单）。
func DefaultSecurityCountryBlocklistSettings() *SecurityCountryBlocklistSettings {
	return &SecurityCountryBlocklistSettings{
		Enabled:   false,
		Countries: nil,
	}
}

// normalizeSecurityCountryBlocklistSettings 规范化名单：
// 去空白、转大写、去重、去空项，空结果统一为 nil（JSON 输出为 null）。
func normalizeSecurityCountryBlocklistSettings(s *SecurityCountryBlocklistSettings) {
	if s == nil || len(s.Countries) == 0 {
		if s != nil {
			s.Countries = nil
		}
		return
	}
	cleaned := make([]string, 0, len(s.Countries))
	seen := make(map[string]struct{}, len(s.Countries))
	for _, entry := range s.Countries {
		code := strings.ToUpper(strings.TrimSpace(entry))
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		cleaned = append(cleaned, code)
	}
	if len(cleaned) == 0 {
		cleaned = nil
	}
	s.Countries = cleaned
}

// isISO3166Alpha2 判断是否为两位 ASCII 字母的国家/地区代码。
func isISO3166Alpha2(v string) bool {
	if len(v) != 2 {
		return false
	}
	for i := 0; i < len(v); i++ {
		c := v[i]
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')) {
			return false
		}
	}
	return true
}

// validateSecurityCountryBlocklist 校验名单条目为合法 ISO 3166-1 alpha-2 代码。
// 必须在 normalize 之后调用（此时已去重、转大写）。
func validateSecurityCountryBlocklist(countries []string) error {
	if len(countries) > maxSecurityCountryBlocklistEntries {
		return fmt.Errorf("too many country/region codes: %d (max %d)", len(countries), maxSecurityCountryBlocklistEntries)
	}
	for _, code := range countries {
		if !isISO3166Alpha2(code) {
			return fmt.Errorf("invalid country/region code %q: expected a 2-letter ISO 3166-1 alpha-2 code", code)
		}
	}
	return nil
}

// GetSecurityCountryBlocklist 获取配置（直读 DB，供管理端读写路径使用）。
// 缺失/空/解析失败 → 返回默认配置（关闭、空名单）。
func (s *SettingService) GetSecurityCountryBlocklist(ctx context.Context) (*SecurityCountryBlocklistSettings, error) {
	value, err := s.settingRepo.GetValue(ctx, SettingKeySecurityCountryBlocklist)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return DefaultSecurityCountryBlocklistSettings(), nil
		}
		return nil, fmt.Errorf("get security country blocklist settings: %w", err)
	}
	if strings.TrimSpace(value) == "" {
		return DefaultSecurityCountryBlocklistSettings(), nil
	}

	settings := &SecurityCountryBlocklistSettings{}
	if err := json.Unmarshal([]byte(value), settings); err != nil {
		slog.Warn("failed to unmarshal security country blocklist settings, falling back to defaults",
			"error", err, "key", SettingKeySecurityCountryBlocklist)
		return DefaultSecurityCountryBlocklistSettings(), nil
	}
	normalizeSecurityCountryBlocklistSettings(settings)
	return settings, nil
}

// SetSecurityCountryBlocklist 保存配置并立即刷新进程内缓存，
// 使当前节点下一个请求即生效（多节点部署最迟 60s 生效）。
func (s *SettingService) SetSecurityCountryBlocklist(ctx context.Context, settings *SecurityCountryBlocklistSettings) error {
	if settings == nil {
		return fmt.Errorf("settings cannot be nil")
	}
	normalizeSecurityCountryBlocklistSettings(settings)
	if err := validateSecurityCountryBlocklist(settings.Countries); err != nil {
		return err
	}

	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal security country blocklist settings: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeySecurityCountryBlocklist, string(data)); err != nil {
		return err
	}

	s.storeSecurityCountryBlocklistCache(*settings, securityCountryBlocklistCacheTTL)
	return nil
}

// GetSecurityCountryBlocklistCached 返回配置（进程内缓存，60s TTL）。
// 全局中间件每个请求都会调用，热路径零 DB 访问。
// DB 错误时返回最近一次已知值（无缓存则默认关闭），并以短 TTL 快速重试；
// fail-open 是刻意取舍：封禁依赖 DB 可用性时不能反过来把整站打挂。
func (s *SettingService) GetSecurityCountryBlocklistCached(ctx context.Context) SecurityCountryBlocklistSettings {
	if s == nil || s.settingRepo == nil {
		return *DefaultSecurityCountryBlocklistSettings()
	}
	if cached, ok := s.securityCountryBlocklistCache.Load().(*cachedSecurityCountryBlocklistSettings); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached.settings
		}
	}

	result, _, _ := s.securityCountryBlocklistSF.Do("security_country_blocklist", func() (any, error) {
		// 二次检查，避免排队的 goroutine 重复查询
		if cached, ok := s.securityCountryBlocklistCache.Load().(*cachedSecurityCountryBlocklistSettings); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return cached.settings, nil
			}
		}
		if ctx == nil {
			ctx = context.Background()
		}
		// 独立 context：断开请求取消链，避免客户端断连污染缓存
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), securityCountryBlocklistDBTimeout)
		defer cancel()

		settings, err := s.GetSecurityCountryBlocklist(dbCtx)
		if err != nil {
			slog.Warn("failed to get security country blocklist settings", "error", err)
			fallback := *DefaultSecurityCountryBlocklistSettings()
			if prior, ok := s.securityCountryBlocklistCache.Load().(*cachedSecurityCountryBlocklistSettings); ok && prior != nil {
				fallback = prior.settings
			}
			s.storeSecurityCountryBlocklistCache(fallback, securityCountryBlocklistErrorTTL)
			return fallback, nil
		}

		s.storeSecurityCountryBlocklistCache(*settings, securityCountryBlocklistCacheTTL)
		return *settings, nil
	})
	if settings, ok := result.(SecurityCountryBlocklistSettings); ok {
		return settings
	}
	return *DefaultSecurityCountryBlocklistSettings()
}

func (s *SettingService) storeSecurityCountryBlocklistCache(settings SecurityCountryBlocklistSettings, ttl time.Duration) {
	s.securityCountryBlocklistCache.Store(&cachedSecurityCountryBlocklistSettings{
		settings:  settings,
		expiresAt: time.Now().Add(ttl).UnixNano(),
	})
}
