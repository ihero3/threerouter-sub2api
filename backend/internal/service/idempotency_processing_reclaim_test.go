package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// #6 回归：processing 记录超过「处理锁 + ProcessingHardCap」后必须可被回收重跑，
// 否则原请求进程崩溃会让幂等键被死锁到 24h 过期，调用方拿同键重试永远被挡。
func TestIdempotencyExecute_ProcessingRecordReclaimedAfterHardCap(t *testing.T) {
	repo := newInMemoryIdempotencyRepo()
	cfg := DefaultIdempotencyConfig()
	cfg.ProcessingHardCap = 1 * time.Second
	cfg.ObserveOnly = false
	coord := NewIdempotencyCoordinator(repo, cfg)

	scope := "media_create"
	key := "k-reclaim"
	payload := map[string]any{"prompt": "a cat"}
	fp, err := BuildIdempotencyFingerprint("POST", "/v1/media", "user:1", payload)
	require.NoError(t, err)

	now := time.Now()
	lockedUntil := now.Add(-2 * time.Second) // 处理锁早已过期
	expiresAt := now.Add(24 * time.Hour)
	repo.data[repo.key(scope, HashIdempotencyKey(key))] = &IdempotencyRecord{
		ID:                 1,
		Scope:              scope,
		IdempotencyKeyHash: HashIdempotencyKey(key),
		RequestFingerprint: fp,
		Status:             IdempotencyStatusProcessing,
		LockedUntil:        &lockedUntil,
		ExpiresAt:          expiresAt,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	var execCount int32
	result, execErr := coord.Execute(context.Background(), IdempotencyExecuteOptions{
		Scope:          scope,
		ActorScope:     "user:1",
		Method:         "POST",
		Route:          "/v1/media",
		IdempotencyKey: key,
		Payload:        payload,
	}, func(ctx context.Context) (any, error) {
		atomic.AddInt32(&execCount, 1)
		return map[string]any{"rerun": true}, nil
	})
	require.NoError(t, execErr)
	require.NotNil(t, result)
	require.False(t, result.Replayed, "卡死的 processing 应回收重跑而非重放")
	require.Equal(t, int32(1), atomic.LoadInt32(&execCount), "超时 processing 必须重跑一次")
	require.Equal(t, map[string]any{"rerun": true}, result.Data)
	// 重跑后记录应被标终态（succeeded），不再卡 processing。
	stored, getErr := repo.GetByScopeAndKeyHash(context.Background(), scope, HashIdempotencyKey(key))
	require.NoError(t, getErr)
	require.NotNil(t, stored)
	require.Equal(t, IdempotencyStatusSucceeded, stored.Status)
}

// #6 反向护栏：未超过宽限的 processing 仍按"进行中"挡住并发重试，禁止双创建/双扣费。
func TestIdempotencyExecute_ProcessingRecordStillBlocksBeforeHardCap(t *testing.T) {
	repo := newInMemoryIdempotencyRepo()
	cfg := DefaultIdempotencyConfig()
	cfg.ProcessingHardCap = 1 * time.Second
	cfg.ObserveOnly = false
	coord := NewIdempotencyCoordinator(repo, cfg)

	scope := "media_create"
	key := "k-block"
	payload := map[string]any{"prompt": "a dog"}
	fp, err := BuildIdempotencyFingerprint("POST", "/v1/media", "user:1", payload)
	require.NoError(t, err)

	now := time.Now()
	lockedUntil := now.Add(10 * time.Second) // 处理锁仍在未来
	expiresAt := now.Add(24 * time.Hour)
	repo.data[repo.key(scope, HashIdempotencyKey(key))] = &IdempotencyRecord{
		ID:                 1,
		Scope:              scope,
		IdempotencyKeyHash: HashIdempotencyKey(key),
		RequestFingerprint: fp,
		Status:             IdempotencyStatusProcessing,
		LockedUntil:        &lockedUntil,
		ExpiresAt:          expiresAt,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	var execCount int32
	_, execErr := coord.Execute(context.Background(), IdempotencyExecuteOptions{
		Scope:          scope,
		ActorScope:     "user:1",
		Method:         "POST",
		Route:          "/v1/media",
		IdempotencyKey: key,
		Payload:        payload,
	}, func(ctx context.Context) (any, error) {
		atomic.AddInt32(&execCount, 1)
		return map[string]any{"x": 1}, nil
	})
	require.Error(t, execErr)
	require.Equal(t, int32(0), atomic.LoadInt32(&execCount), "宽限内的 processing 不得重跑（防双扣费）")
}
