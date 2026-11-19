// Package locallimit 提供进程内固定窗口限流，用作关键计数存储故障时的保守兜底。
// 它不追求多实例精确一致，只保证单个进程在 Redis/DB 短暂不可用时不会完全放行。
package locallimit

import (
	"sync"
	"time"
)

type window struct {
	count     int
	expiresAt time.Time
}

// Limiter 是带上限与惰性清理的本地固定窗口计数器。
type Limiter struct {
	mu      sync.Mutex
	windows map[string]*window
	maxKeys int
}

func New(maxKeys int) *Limiter {
	if maxKeys <= 0 {
		maxKeys = 4096
	}
	return &Limiter{windows: make(map[string]*window), maxKeys: maxKeys}
}

// Allow 在窗口内计数未达 limit 时返回 true并自增；超限返回 false。
func (l *Limiter) Allow(key string, limit int, duration time.Duration, now time.Time) bool {
	if l == nil || limit <= 0 || duration <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.windows == nil {
		l.windows = make(map[string]*window)
	}
	if len(l.windows) >= l.maxKeys {
		for k, w := range l.windows {
			if now.After(w.expiresAt) {
				delete(l.windows, k)
			}
		}
		if len(l.windows) >= l.maxKeys {
			return true
		}
	}
	w := l.windows[key]
	if w == nil || now.After(w.expiresAt) {
		w = &window{expiresAt: now.Add(duration)}
		l.windows[key] = w
	}
	if w.count >= limit {
		return false
	}
	w.count++
	return true
}
