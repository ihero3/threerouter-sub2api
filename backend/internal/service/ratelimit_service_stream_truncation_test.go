//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// streamTruncationCounterCacheStub 按顺序返回预设计数。
type streamTruncationCounterCacheStub struct {
	counts     []int64
	resetCalls []int64
	err        error
}

func (s *streamTruncationCounterCacheStub) IncrementStreamTruncationCount(_ context.Context, _ int64, _ int) (int64, error) {
	if s.err != nil {
		return 0, s.err
	}
	if len(s.counts) == 0 {
		return 1, nil
	}
	count := s.counts[0]
	s.counts = s.counts[1:]
	return count, nil
}

func (s *streamTruncationCounterCacheStub) ResetStreamTruncationCount(_ context.Context, accountID int64) error {
	s.resetCalls = append(s.resetCalls, accountID)
	return nil
}

// 首次截断只累计计数，不降权。
func TestRateLimitService_HandleStreamTruncation_FirstHitNoPenalty(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	counter := &streamTruncationCounterCacheStub{counts: []int64{1}}
	blocker := &runtimeBlockRecorder{}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc.SetStreamTruncationCounterCache(counter)
	svc.SetAccountRuntimeBlocker(blocker)
	account := &Account{ID: 701, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	penalized := svc.HandleStreamTruncation(context.Background(), account, "gpt-5.5")

	require.False(t, penalized)
	require.Equal(t, 0, repo.tempCalls)
	require.Empty(t, blocker.accounts)
	require.Empty(t, counter.resetCalls)
}

// 窗口内第二次截断触发降权：写库 + 写运行时阻塞 + 清零计数。
func TestRateLimitService_HandleStreamTruncation_ThresholdTriggersPenalty(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	counter := &streamTruncationCounterCacheStub{counts: []int64{1, 2}}
	blocker := &runtimeBlockRecorder{}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc.SetStreamTruncationCounterCache(counter)
	svc.SetAccountRuntimeBlocker(blocker)
	account := &Account{ID: 702, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	require.False(t, svc.HandleStreamTruncation(context.Background(), account, "gpt-5.5"))
	require.True(t, svc.HandleStreamTruncation(context.Background(), account, "gpt-5.5"))

	require.Equal(t, 1, repo.tempCalls)
	require.Equal(t, account.ID, repo.lastTempID)
	require.Contains(t, repo.lastTempReason, "upstream_stream_truncated")
	require.Contains(t, repo.lastTempReason, "gpt-5.5")

	require.Len(t, blocker.accounts, 1)
	require.Equal(t, account.ID, blocker.accounts[0].ID)
	require.Equal(t, "upstream_stream_truncated_temp_unschedulable", blocker.reasons[0])
	require.True(t, blocker.until[0].After(time.Now()))

	require.Equal(t, []int64{account.ID}, counter.resetCalls)
}

// 计数器未注入时不降权（安全降级）。
func TestRateLimitService_HandleStreamTruncation_NoCounterNoPenalty(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	blocker := &runtimeBlockRecorder{}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc.SetAccountRuntimeBlocker(blocker)
	account := &Account{ID: 703, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	require.False(t, svc.HandleStreamTruncation(context.Background(), account, "gpt-5.5"))
	require.Equal(t, 0, repo.tempCalls)
	require.Empty(t, blocker.accounts)
}

// 计数器报错时不降权（避免把一次截断当阈值命中）。
func TestRateLimitService_HandleStreamTruncation_CounterErrorNoPenalty(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	counter := &streamTruncationCounterCacheStub{err: errors.New("redis down")}
	blocker := &runtimeBlockRecorder{}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc.SetStreamTruncationCounterCache(counter)
	svc.SetAccountRuntimeBlocker(blocker)
	account := &Account{ID: 704, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	require.False(t, svc.HandleStreamTruncation(context.Background(), account, "gpt-5.5"))
	require.Equal(t, 0, repo.tempCalls)
	require.Empty(t, blocker.accounts)
}
