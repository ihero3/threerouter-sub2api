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

// GatewayIPRateLimitSettings 网关 API 按客户端 IP 限流配置。
// 作用于模型 API 转发链路（/v1/* 等网关路由组），按安全客户端 IP 维度做
// 固定窗口计数，用于限制同一出口 IP 下多个用户/脚本的聚合请求速率。
type GatewayIPRateLimitSettings struct {
	// Enabled 总开关（默认关闭，开启前请确认阈值与 Nginx 单 IP 并发限制匹配）
	Enabled bool `json:"enabled"`
	// RPM 每 IP 每分钟请求数上限（0 = 不限制）
	RPM int `json:"rpm"`
	// Whitelist 豁免 IP/CIDR 列表（精确 IP 或 CIDR 网段，命中即跳过计数）
	Whitelist []string `json:"whitelist"`
}

// 网关 IP 限流 RPM 的取值上限，防止配置异常大的值失去意义。
const gatewayIPRateLimitRPMMax = 100000

const (
	gatewayIPRateLimitCacheTTL  = 60 * time.Second
	gatewayIPRateLimitErrorTTL  = 5 * time.Second
	gatewayIPRateLimitDBTimeout = 5 * time.Second
)

// cachedGatewayIPRateLimitSettings 进程内缓存条目（60s TTL）。
type cachedGatewayIPRateLimitSettings struct {
	settings  GatewayIPRateLimitSettings
	expiresAt int64 // unix nano
}

// DefaultGatewayIPRateLimitSettings 返回默认网关 IP 限流配置。
// 默认关闭：开启前应先观察现网单 IP 请求分布，避免误伤企业 NAT 出口场景。
func DefaultGatewayIPRateLimitSettings() *GatewayIPRateLimitSettings {
	return &GatewayIPRateLimitSettings{
		Enabled:   false,
		RPM:       600,
		Whitelist: nil,
	}
}

// normalizeGatewayIPRateLimitSettings 修正非法取值（负数归零、超上限截断、白名单去空白）。
func normalizeGatewayIPRateLimitSettings(s *GatewayIPRateLimitSettings) {
	if s == nil {
		return
	}
	if s.RPM < 0 {
		s.RPM = 0
	}
	if s.RPM > gatewayIPRateLimitRPMMax {
		s.RPM = gatewayIPRateLimitRPMMax
	}
	if len(s.Whitelist) > 0 {
		cleaned := s.Whitelist[:0]
		for _, entry := range s.Whitelist {
			if v := strings.TrimSpace(entry); v != "" {
				cleaned = append(cleaned, v)
			}
		}
		s.Whitelist = cleaned
	}
}

// validateGatewayIPRateLimitWhitelist 校验白名单条目必须是合法 IP 或 CIDR。
func validateGatewayIPRateLimitWhitelist(whitelist []string) error {
	for _, entry := range whitelist {
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

// GetGatewayIPRateLimitSettings 获取网关 IP 限流配置（直读 DB，供管理端读写路径使用）。
// 缺失/空/解析失败 → 返回默认配置。
func (s *SettingService) GetGatewayIPRateLimitSettings(ctx context.Context) (*GatewayIPRateLimitSettings, error) {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyGatewayIPRateLimitSettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return DefaultGatewayIPRateLimitSettings(), nil
		}
		return nil, fmt.Errorf("get gateway ip rate limit settings: %w", err)
	}
	if strings.TrimSpace(value) == "" {
		return DefaultGatewayIPRateLimitSettings(), nil
	}

	settings := &GatewayIPRateLimitSettings{}
	if err := json.Unmarshal([]byte(value), settings); err != nil {
		slog.Warn("failed to unmarshal gateway ip rate limit settings, falling back to defaults",
			"error", err, "key", SettingKeyGatewayIPRateLimitSettings)
		return DefaultGatewayIPRateLimitSettings(), nil
	}
	normalizeGatewayIPRateLimitSettings(settings)
	return settings, nil
}

// SetGatewayIPRateLimitSettings 保存网关 IP 限流配置，并立即刷新进程内缓存，
// 使当前节点的下一个请求即生效（多节点部署最迟 60s 内生效）。
func (s *SettingService) SetGatewayIPRateLimitSettings(ctx context.Context, settings *GatewayIPRateLimitSettings) error {
	if settings == nil {
		return fmt.Errorf("settings cannot be nil")
	}
	if settings.RPM < 0 {
		return fmt.Errorf("rpm cannot be negative")
	}
	if settings.RPM > gatewayIPRateLimitRPMMax {
		return fmt.Errorf("rpm must be at most %d", gatewayIPRateLimitRPMMax)
	}
	if err := validateGatewayIPRateLimitWhitelist(settings.Whitelist); err != nil {
		return err
	}
	normalizeGatewayIPRateLimitSettings(settings)

	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal gateway ip rate limit settings: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyGatewayIPRateLimitSettings, string(data)); err != nil {
		return err
	}

	s.storeGatewayIPRateLimitCache(*settings, gatewayIPRateLimitCacheTTL)
	return nil
}

// GetGatewayIPRateLimitSettingsCached 返回网关 IP 限流配置（进程内缓存，60s TTL）。
// 网关每个 API 请求的热路径都会调用，绝不能每次访问 DB；
// DB 错误时返回最近一次已知值（无缓存则返回默认值），并以短 TTL 快速重试。
func (s *SettingService) GetGatewayIPRateLimitSettingsCached(ctx context.Context) GatewayIPRateLimitSettings {
	if s == nil || s.settingRepo == nil {
		return *DefaultGatewayIPRateLimitSettings()
	}
	if cached, ok := s.gatewayIPRateLimitCache.Load().(*cachedGatewayIPRateLimitSettings); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached.settings
		}
	}

	result, _, _ := s.gatewayIPRateLimitSF.Do("gateway_ip_rate_limit_settings", func() (any, error) {
		// 二次检查，避免排队的 goroutine 重复查询
		if cached, ok := s.gatewayIPRateLimitCache.Load().(*cachedGatewayIPRateLimitSettings); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return cached.settings, nil
			}
		}
		if ctx == nil {
			ctx = context.Background()
		}
		// 独立 context：断开请求取消链，避免客户端断连污染缓存
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), gatewayIPRateLimitDBTimeout)
		defer cancel()

		settings, err := s.GetGatewayIPRateLimitSettings(dbCtx)
		if err != nil {
			slog.Warn("failed to get gateway ip rate limit settings", "error", err)
			// 保留最近一次已知值，短 TTL 快速重试
			fallback := *DefaultGatewayIPRateLimitSettings()
			if prior, ok := s.gatewayIPRateLimitCache.Load().(*cachedGatewayIPRateLimitSettings); ok && prior != nil {
				fallback = prior.settings
			}
			s.storeGatewayIPRateLimitCache(fallback, gatewayIPRateLimitErrorTTL)
			return fallback, nil
		}

		s.storeGatewayIPRateLimitCache(*settings, gatewayIPRateLimitCacheTTL)
		return *settings, nil
	})
	if settings, ok := result.(GatewayIPRateLimitSettings); ok {
		return settings
	}
	return *DefaultGatewayIPRateLimitSettings()
}

func (s *SettingService) storeGatewayIPRateLimitCache(settings GatewayIPRateLimitSettings, ttl time.Duration) {
	s.gatewayIPRateLimitCache.Store(&cachedGatewayIPRateLimitSettings{
		settings:  settings,
		expiresAt: time.Now().Add(ttl).UnixNano(),
	})
}
