package repository

import (
	"context"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const openAI403CounterPrefix = "openai_403_count:account:"

var openAI403CounterIncrScript = redis.NewScript(`
	local key = KEYS[1]
	local ttl = tonumber(ARGV[1])

	local count = redis.call('INCR', key)
	if count == 1 then
		redis.call('EXPIRE', key, ttl)
	end

	return count
`)

type openAI403CounterCache struct {
	rdb *redis.Client
}

func NewOpenAI403CounterCache(rdb *redis.Client) service.OpenAI403CounterCache {
	return &openAI403CounterCache{rdb: rdb}
}

func (c *openAI403CounterCache) IncrementOpenAI403Count(ctx context.Context, accountID int64, windowMinutes int) (int64, error) {
	key := fmt.Sprintf("%s%d", openAI403CounterPrefix, accountID)

	ttlSeconds := windowMinutes * 60
	if ttlSeconds < 60 {
		ttlSeconds = 60
	}

	result, err := openAI403CounterIncrScript.Run(ctx, c.rdb, []string{key}, ttlSeconds).Int64()
	if err != nil {
		return 0, fmt.Errorf("increment openai 403 count: %w", err)
	}
	return result, nil
}

func (c *openAI403CounterCache) ResetOpenAI403Count(ctx context.Context, accountID int64) error {
	key := fmt.Sprintf("%s%d", openAI403CounterPrefix, accountID)
	return c.rdb.Del(ctx, key).Err()
}

// 编译期确认：同一缓存同时实现 AuthErrorEscalationCache（独立 key 前缀），
// 供 ProvideRateLimitService 类型断言注入，避免 provider 签名变动牵连 wire_gen.go。
var _ service.AuthErrorEscalationCache = (*openAI403CounterCache)(nil)

// authErrorCounterPrefix 是 401/403「冷却+计数升级」计数的 key 前缀，
// 与 openAI403CounterPrefix（历史 403 计数）物理隔离，互不影响。
const authErrorCounterPrefix = "auth_error_count:account:"

func (c *openAI403CounterCache) authErrorCounterKey(accountID int64, statusCode int) string {
	return fmt.Sprintf("%s%d:status:%d", authErrorCounterPrefix, accountID, statusCode)
}

// IncrementAuthErrorCount 按（账号, 状态码）原子递增升级计数。
func (c *openAI403CounterCache) IncrementAuthErrorCount(ctx context.Context, accountID int64, statusCode int, windowMinutes int) (int64, error) {
	key := c.authErrorCounterKey(accountID, statusCode)
	ttlSeconds := windowMinutes * 60
	if ttlSeconds < 60 {
		ttlSeconds = 60
	}
	result, err := openAI403CounterIncrScript.Run(ctx, c.rdb, []string{key}, ttlSeconds).Int64()
	if err != nil {
		return 0, fmt.Errorf("increment auth error count: %w", err)
	}
	return result, nil
}

// ResetAuthErrorCounters 清空该账号全部状态码的升级计数（成功请求/人工恢复时调用）。
func (c *openAI403CounterCache) ResetAuthErrorCounters(ctx context.Context, accountID int64) error {
	pattern := fmt.Sprintf("%s%d:status:*", authErrorCounterPrefix, accountID)
	var keys []string
	iter := c.rdb.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("scan auth error counters: %w", err)
	}
	if len(keys) == 0 {
		return nil
	}
	return c.rdb.Del(ctx, keys...).Err()
}
