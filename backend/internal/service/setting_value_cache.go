package service

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// 通用单键设置缓存（进程内）。
//
// 背景：面板 JWT 鉴权、网关调度、注册/验证码等请求热路径上存在若干
// 「读单个 setting key」的开关查询（如 session_binding_enabled、beta_policy_settings），
// 直查 DB 会让每个请求都多一次数据库往返。这里提供统一的缓存读取入口，
// 与项目既有的 panelRateLimitCache / gatewayForwardingCache 同一范式：
// 60s TTL + singleflight 防击穿 + 错误短 TTL + 写后失效（refreshCachedSettings）。
//
// 多节点部署下配置变更最迟 60s 内生效，与既有缓存口径一致。

const (
	settingValueCacheTTL  = 60 * time.Second
	settingValueErrorTTL  = 5 * time.Second
	settingValueDBTimeout = 5 * time.Second
)

// cachedSettingValue 单个设置键的缓存条目。
type cachedSettingValue struct {
	value     string
	found     bool // key 是否存在于 DB（区分「不存在」与「存在但为空」）
	expiresAt int64
}

// getCachedSettingValue 读取单个设置键（进程内缓存，热路径安全）。
// 返回 (value, found)：found=false 表示键不存在或 DB 暂不可用（调用方按各自默认值回落）。
// DB 错误时有旧值继续兜底（短 TTL 快速重试），无旧值按 not-found 缓存 5s。
func (s *SettingService) getCachedSettingValue(ctx context.Context, key string) (string, bool) {
	if s == nil || s.settingRepo == nil {
		return "", false
	}
	if v, ok := s.settingValueCache.Load(key); ok {
		if entry, ok := v.(*cachedSettingValue); ok && entry != nil && time.Now().UnixNano() < entry.expiresAt {
			return entry.value, entry.found
		}
	}

	result, _, _ := s.settingValueSF.Do(key, func() (any, error) {
		// 二次检查，避免排队的 goroutine 重复查询
		if v, ok := s.settingValueCache.Load(key); ok {
			if entry, ok := v.(*cachedSettingValue); ok && entry != nil && time.Now().UnixNano() < entry.expiresAt {
				return entry, nil
			}
		}
		if ctx == nil {
			ctx = context.Background()
		}
		// 独立 context：断开请求取消链，避免客户端断连污染缓存
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), settingValueDBTimeout)
		defer cancel()

		value, err := s.settingRepo.GetValue(dbCtx, key)
		switch {
		case err == nil:
			entry := &cachedSettingValue{value: value, found: true, expiresAt: time.Now().Add(settingValueCacheTTL).UnixNano()}
			s.settingValueCache.Store(key, entry)
			return entry, nil
		case errors.Is(err, ErrSettingNotFound):
			entry := &cachedSettingValue{found: false, expiresAt: time.Now().Add(settingValueCacheTTL).UnixNano()}
			s.settingValueCache.Store(key, entry)
			return entry, nil
		default:
			slog.Warn("failed to read setting, serving stale/default value", "key", key, "error", err)
			// 有旧值继续兜底，短 TTL 快速重试
			if v, ok := s.settingValueCache.Load(key); ok {
				if stale, ok := v.(*cachedSettingValue); ok && stale != nil {
					entry := &cachedSettingValue{value: stale.value, found: stale.found, expiresAt: time.Now().Add(settingValueErrorTTL).UnixNano()}
					s.settingValueCache.Store(key, entry)
					return entry, nil
				}
			}
			entry := &cachedSettingValue{found: false, expiresAt: time.Now().Add(settingValueErrorTTL).UnixNano()}
			s.settingValueCache.Store(key, entry)
			return entry, nil
		}
	})
	if entry, ok := result.(*cachedSettingValue); ok && entry != nil {
		return entry.value, entry.found
	}
	return "", false
}

// invalidateCachedSettingValues 使指定键的缓存失效（直写 setter 路径调用）。
func (s *SettingService) invalidateCachedSettingValues(keys ...string) {
	if s == nil {
		return
	}
	for _, key := range keys {
		s.settingValueCache.Delete(key)
		s.settingValueSF.Forget(key)
	}
}

// invalidateAllCachedSettingValues 清空全部键缓存（UpdateSettings 全量写后调用）。
func (s *SettingService) invalidateAllCachedSettingValues() {
	if s == nil {
		return
	}
	s.settingValueCache.Clear()
}
