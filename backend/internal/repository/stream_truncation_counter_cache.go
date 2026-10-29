package repository

import (
	"context"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// streamTruncationCounterPrefix 与 timeout_count:account: 分开命名空间。
// 截断降权阈值（2 次/5 分钟 → 降权 2 分钟）远低于流超时阈值，混用同一计数器
// 会让两类事件互相污染。
const streamTruncationCounterPrefix = "stream_truncation_count:account:"

// 复用 timeoutCounterCache 的底层 Lua 脚本与 Redis 连接，仅换 key 前缀。
// 在 service 层通过类型断言把同一个实例同时用作 TimeoutCounterCache 和
// StreamTruncationCounterCache，无需改 ProvideRateLimitService 的函数签名。
var _ service.StreamTruncationCounterCache = (*timeoutCounterCache)(nil)

// IncrementStreamTruncationCount 增加账号的截断计数，返回当前窗口内累计值。
func (c *timeoutCounterCache) IncrementStreamTruncationCount(ctx context.Context, accountID int64, windowMinutes int) (int64, error) {
	key := fmt.Sprintf("%s%d", streamTruncationCounterPrefix, accountID)

	ttlSeconds := windowMinutes * 60
	if ttlSeconds < 60 {
		ttlSeconds = 60
	}

	result, err := timeoutCounterIncrScript.Run(ctx, c.rdb, []string{key}, ttlSeconds).Int64()
	if err != nil {
		return 0, fmt.Errorf("increment stream truncation count: %w", err)
	}
	return result, nil
}

// ResetStreamTruncationCount 降权生效后清零计数。
func (c *timeoutCounterCache) ResetStreamTruncationCount(ctx context.Context, accountID int64) error {
	key := fmt.Sprintf("%s%d", streamTruncationCounterPrefix, accountID)
	return c.rdb.Del(ctx, key).Err()
}
