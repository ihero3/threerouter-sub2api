package service

import (
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/oschwald/maxminddb-golang"
)

// GeoIPService 基于 MaxMind GeoLite2-Country 数据库解析客户端 IP 的司法管辖区（国家 ISO 代码）。
//
// 设计要点（见 docs/合规方案.md 4.1.2 GeoIP 降级策略）：
//   - 数据库不可用（未启用 / 文件缺失 / 加载失败）时，按 fallbackEnabled 决定是否降级；
//   - 降级模式下 Lookup 返回空字符串且不报错（fail-open），user_jurisdiction 留空，不阻断请求；
//   - 非降级模式下返回错误，由调用方决定处理策略。
type GeoIPService struct {
	reader          *maxminddb.Reader
	fallbackEnabled bool
	// enabled 记录初始化时的启用意图，用于对外暴露可用性状态。
	enabled bool
	// mu 保护 reader 的并发访问与关闭。
	mu sync.RWMutex
}

// geoIPCountryRecord 是 GeoLite2-Country 数据库的最小映射结构，仅取国家 ISO 代码。
type geoIPCountryRecord struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
}

// NewGeoIPService 根据合规配置创建 GeoIP 服务。
//
// 无论加载成功与否都返回一个非 nil 的 *GeoIPService，以保证 wire 注入始终成功：
//   - 未启用或加载失败时，reader 为 nil，行为退化为降级策略；
//   - 加载失败仅记录日志，不返回错误（避免 GeoIP 数据缺失导致整个服务无法启动）。
func NewGeoIPService(cfg *config.Config) *GeoIPService {
	geoCfg := cfg.Compliance.GeoIP
	svc := &GeoIPService{
		fallbackEnabled: geoCfg.FallbackEnabled,
		enabled:         geoCfg.Enabled,
	}

	if !geoCfg.Enabled {
		slog.Info("geoip.disabled", "reason", "compliance.geoip.enabled=false")
		return svc
	}

	path := strings.TrimSpace(geoCfg.DatabasePath)
	if path == "" {
		slog.Warn("geoip.init_skipped", "reason", "empty database_path", "fallback_enabled", geoCfg.FallbackEnabled)
		return svc
	}

	reader, err := maxminddb.Open(path)
	if err != nil {
		slog.Warn("geoip.open_failed",
			"path", path,
			"error", err.Error(),
			"fallback_enabled", geoCfg.FallbackEnabled,
		)
		return svc
	}

	svc.reader = reader
	slog.Info("geoip.ready",
		"path", path,
		"build_epoch", reader.Metadata.BuildEpoch,
		"database_type", reader.Metadata.DatabaseType,
	)
	return svc
}

// Available 报告 GeoIP 数据库是否已就绪（可执行真实查询）。
func (g *GeoIPService) Available() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.reader != nil
}

// FallbackEnabled 报告是否启用降级策略。
func (g *GeoIPService) FallbackEnabled() bool {
	return g.fallbackEnabled
}

// Lookup 解析给定 IP 的国家 ISO 代码（如 "US"、"CN"、"DE"）。
//
// 返回约定：
//   - 成功：返回大写 ISO 3166-1 alpha-2 代码；私有/回环/无法识别 IP 返回空字符串。
//   - reader 不可用或查询失败：fallbackEnabled=true 时返回 ("", nil)，否则返回错误。
func (g *GeoIPService) Lookup(ip string) (string, error) {
	g.mu.RLock()
	reader := g.reader
	g.mu.RUnlock()

	if reader == nil {
		if g.fallbackEnabled {
			return "", nil
		}
		return "", fmt.Errorf("geoip not available")
	}

	ip = strings.TrimSpace(ip)
	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		// 无法解析的 IP 视为未知司法管辖区，遵循 fail-open 不阻断请求。
		if g.fallbackEnabled {
			return "", nil
		}
		return "", fmt.Errorf("invalid ip: %q", ip)
	}

	// 私有/回环/链路本地地址不参与司法管辖区判定，直接返回空。
	if ipAddr.IsPrivate() || ipAddr.IsLoopback() || ipAddr.IsLinkLocalUnicast() {
		return "", nil
	}

	var record geoIPCountryRecord
	if err := reader.Lookup(ipAddr, &record); err != nil {
		if g.fallbackEnabled {
			return "", nil
		}
		return "", fmt.Errorf("geoip lookup %q: %w", ip, err)
	}

	return strings.ToUpper(record.Country.ISOCode), nil
}

// Close 释放底层数据库句柄。多次调用安全。
func (g *GeoIPService) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.reader == nil {
		return nil
	}
	err := g.reader.Close()
	g.reader = nil
	return err
}
