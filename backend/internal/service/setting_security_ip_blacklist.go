package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
	"time"
)

// SecurityIPBlacklistSettings 全站 IP 黑名单配置。
// 作用于所有入站流量（面板 /api/v1/*、模型网关 /v1 /v1beta /backend-api/codex
// /antigravity /models 等），按安全客户端 IP 维度匹配，命中即 403。
// 豁免：管理端 /api/v1/admin（防管理员自锁）、支付 webhook 回调、静态前端。
type SecurityIPBlacklistSettings struct {
	// Enabled 总开关（默认关闭）。关闭时不拦截任何请求，名单保留不丢。
	Enabled bool `json:"enabled"`
	// Blacklist IP/CIDR 列表（支持 IPv4/IPv6 精确地址与网段前缀）。
	Blacklist []string `json:"blacklist"`
}

const (
	securityIPBlacklistCacheTTL  = 60 * time.Second
	securityIPBlacklistErrorTTL  = 5 * time.Second
	securityIPBlacklistDBTimeout = 5 * time.Second
)

// cachedSecurityIPBlacklistSettings 进程内缓存条目（60s TTL）。
type cachedSecurityIPBlacklistSettings struct {
	settings  SecurityIPBlacklistSettings
	expiresAt int64 // unix nano
}

// DefaultSecurityIPBlacklistSettings 返回默认全站 IP 黑名单配置（关闭、空名单）。
func DefaultSecurityIPBlacklistSettings() *SecurityIPBlacklistSettings {
	return &SecurityIPBlacklistSettings{
		Enabled:   false,
		Blacklist: nil,
	}
}

// normalizeSecurityIPBlacklistSettings 修正非法取值（黑名单条目去空白、去空项）。
func normalizeSecurityIPBlacklistSettings(s *SecurityIPBlacklistSettings) {
	if s == nil {
		return
	}
	if len(s.Blacklist) > 0 {
		cleaned := s.Blacklist[:0]
		for _, entry := range s.Blacklist {
			if v := strings.TrimSpace(entry); v != "" {
				cleaned = append(cleaned, v)
			}
		}
		s.Blacklist = cleaned
	}
}

// validateSecurityIPBlacklist 校验黑名单条目必须是合法 IP 或 CIDR。
// 与网关限流黑名单同一校验口径，但独立实现以免牵动既有已测代码。
func validateSecurityIPBlacklist(blacklist []string) error {
	for _, entry := range blacklist {
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
		return fmt.Errorf("invalid blacklist entry %q: must be an IP address or CIDR", entry)
	}
	return nil
}

// GetSecurityIPBlacklist 获取全站 IP 黑名单配置（直读 DB，供管理端读写路径使用）。
// 缺失/空/解析失败 → 返回默认配置。
func (s *SettingService) GetSecurityIPBlacklist(ctx context.Context) (*SecurityIPBlacklistSettings, error) {
	value, err := s.settingRepo.GetValue(ctx, SettingKeySecurityIPBlacklist)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return DefaultSecurityIPBlacklistSettings(), nil
		}
		return nil, fmt.Errorf("get security ip blacklist settings: %w", err)
	}
	if strings.TrimSpace(value) == "" {
		return DefaultSecurityIPBlacklistSettings(), nil
	}

	settings := &SecurityIPBlacklistSettings{}
	if err := json.Unmarshal([]byte(value), settings); err != nil {
		slog.Warn("failed to unmarshal security ip blacklist settings, falling back to defaults",
			"error", err, "key", SettingKeySecurityIPBlacklist)
		return DefaultSecurityIPBlacklistSettings(), nil
	}
	normalizeSecurityIPBlacklistSettings(settings)
	return settings, nil
}

// SetSecurityIPBlacklist 保存全站 IP 黑名单配置，并立即刷新进程内缓存，
// 使当前节点的下一个请求即生效（多节点部署最迟 60s 内生效）。
func (s *SettingService) SetSecurityIPBlacklist(ctx context.Context, settings *SecurityIPBlacklistSettings) error {
	if settings == nil {
		return fmt.Errorf("settings cannot be nil")
	}
	if err := validateSecurityIPBlacklist(settings.Blacklist); err != nil {
		return err
	}
	normalizeSecurityIPBlacklistSettings(settings)

	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal security ip blacklist settings: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeySecurityIPBlacklist, string(data)); err != nil {
		return err
	}

	s.storeSecurityIPBlacklistCache(*settings, securityIPBlacklistCacheTTL)
	return nil
}

// GetSecurityIPBlacklistCached 返回全站 IP 黑名单配置（进程内缓存，60s TTL）。
// 全局中间件每个请求都会调用，绝不能每次访问 DB；
// DB 错误时返回最近一次已知值（无缓存则返回默认值=不拦截），并以短 TTL 快速重试。
// fail-open 是刻意取舍：黑名单依赖 DB 可用性时不能反过来把整站打挂。
func (s *SettingService) GetSecurityIPBlacklistCached(ctx context.Context) SecurityIPBlacklistSettings {
	if s == nil || s.settingRepo == nil {
		return *DefaultSecurityIPBlacklistSettings()
	}
	if cached, ok := s.securityIPBlacklistCache.Load().(*cachedSecurityIPBlacklistSettings); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached.settings
		}
	}

	result, _, _ := s.securityIPBlacklistSF.Do("security_ip_blacklist", func() (any, error) {
		// 二次检查，避免排队的 goroutine 重复查询
		if cached, ok := s.securityIPBlacklistCache.Load().(*cachedSecurityIPBlacklistSettings); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return cached.settings, nil
			}
		}
		if ctx == nil {
			ctx = context.Background()
		}
		// 独立 context：断开请求取消链，避免客户端断连污染缓存
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), securityIPBlacklistDBTimeout)
		defer cancel()

		settings, err := s.GetSecurityIPBlacklist(dbCtx)
		if err != nil {
			slog.Warn("failed to get security ip blacklist settings", "error", err)
			// 保留最近一次已知值，短 TTL 快速重试
			fallback := *DefaultSecurityIPBlacklistSettings()
			if prior, ok := s.securityIPBlacklistCache.Load().(*cachedSecurityIPBlacklistSettings); ok && prior != nil {
				fallback = prior.settings
			}
			s.storeSecurityIPBlacklistCache(fallback, securityIPBlacklistErrorTTL)
			return fallback, nil
		}

		s.storeSecurityIPBlacklistCache(*settings, securityIPBlacklistCacheTTL)
		return *settings, nil
	})
	if settings, ok := result.(SecurityIPBlacklistSettings); ok {
		return settings
	}
	return *DefaultSecurityIPBlacklistSettings()
}

func (s *SettingService) storeSecurityIPBlacklistCache(settings SecurityIPBlacklistSettings, ttl time.Duration) {
	s.securityIPBlacklistCache.Store(&cachedSecurityIPBlacklistSettings{
		settings:  settings,
		expiresAt: time.Now().Add(ttl).UnixNano(),
	})
}
