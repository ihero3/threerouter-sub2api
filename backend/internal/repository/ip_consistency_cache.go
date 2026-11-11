package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

// ipConsistencyRecordScript 原子地记录一次客户端 IP 访问，并返回窗口内出现过的
// 全部去重 IP。
//
// 语义：ZSET（score=毫秒时间戳，member=IP）。每次请求先删除窗口外的旧成员，
// 再写入当前 IP，因此是「滚动窗口」而不是固定时间桶。
// TTL 设为 2 倍窗口，避免长期无请求的 key 永久驻留。
var ipConsistencyRecordScript = redis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local ip = ARGV[3]
redis.call('ZREMRANGEBYSCORE', key, '-inf', now - window)
redis.call('ZADD', key, now, ip)
redis.call('EXPIRE', key, math.ceil(window / 1000) * 2)
return redis.call('ZRANGE', key, 0, -1)
`)

// IPConsistencyCacheImpl IP 一致性窗口缓存 Redis 实现。
type IPConsistencyCacheImpl struct {
	rdb *redis.Client
}

// NewIPConsistencyCache 创建 IP 一致性窗口缓存。
func NewIPConsistencyCache(rdb *redis.Client) service.IPConsistencyCache {
	return &IPConsistencyCacheImpl{rdb: rdb}
}

// RecordAndListIPs 记录本次 IP 并返回窗口内去重后的 IP 列表。
func (c *IPConsistencyCacheImpl) RecordAndListIPs(
	ctx context.Context,
	key, clientIP string,
	now time.Time,
	window time.Duration,
) ([]string, error) {
	if c == nil || c.rdb == nil || key == "" || clientIP == "" {
		return nil, nil
	}
	windowMs := window.Milliseconds()
	if windowMs <= 0 {
		return nil, fmt.Errorf("invalid ip consistency window: %s", window)
	}
	ips, err := ipConsistencyRecordScript.Run(ctx, c.rdb, []string{key},
		now.UnixMilli(), windowMs, clientIP).StringSlice()
	if err != nil {
		return nil, fmt.Errorf("ip consistency record: %w", err)
	}
	return ips, nil
}

// Clear 清除某个 key 的窗口记录。
func (c *IPConsistencyCacheImpl) Clear(ctx context.Context, key string) error {
	if c == nil || c.rdb == nil || key == "" {
		return nil
	}
	if err := c.rdb.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("ip consistency clear: %w", err)
	}
	return nil
}
