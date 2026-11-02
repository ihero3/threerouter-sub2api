package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 本文件守住"用户重试必须换源"这条跨请求不变量：
//
//  1. 客户端已经看到半截回答（可换号窗口已关闭）时，必须立刻把该账号移出调度，
//     而且是集群级（落库）而不只是本进程——否则重试打到另一个实例时仍会命中
//     同一个坏账号；
//  2. 请求失败后必须解除会话粘性绑定：会话哈希由请求内容派生，客户端重试通常
//     原样重发 body，不清绑定就会把重试直接送回刚刚失败的上游账号。

type streamFailureTempUnschedCall struct {
	accountID int64
	until     time.Time
	reason    string
}

type streamFailureAccountRepoStub struct {
	AccountRepository
	calls []streamFailureTempUnschedCall
}

func (r *streamFailureAccountRepoStub) SetTempUnschedulable(_ context.Context, id int64, until time.Time, reason string) error {
	r.calls = append(r.calls, streamFailureTempUnschedCall{accountID: id, until: until, reason: reason})
	return nil
}

type streamFailureSchedulingBlockerStub struct {
	reasons []string
	untils  []time.Time
}

func (b *streamFailureSchedulingBlockerStub) BlockAccountScheduling(_ *Account, until time.Time, reason string) {
	b.reasons = append(b.reasons, reason)
	b.untils = append(b.untils, until)
}

func (b *streamFailureSchedulingBlockerStub) ClearAccountSchedulingBlock(int64) {}

type streamFailureCounterStub struct {
	increments int
}

func (s *streamFailureCounterStub) IncrementStreamTruncationCount(_ context.Context, _ int64, _ int) (int64, error) {
	s.increments++
	return 1, nil
}

func (s *streamFailureCounterStub) ResetStreamTruncationCount(_ context.Context, _ int64) error {
	return nil
}

// TestPenalizeClientVisibleStreamTruncationUnschedulesClusterWide 验证"用户已经
// 看到截断回答"的降权会：立刻进程内冷却 + 立刻集群级（落库）短时不可调度 +
// 常规窗口计数。三者缺一，客户端重试都可能落回同一个坏账号。
func TestPenalizeClientVisibleStreamTruncationUnschedulesClusterWide(t *testing.T) {
	repo := &streamFailureAccountRepoStub{}
	blocker := &streamFailureSchedulingBlockerStub{}
	counter := &streamFailureCounterStub{}
	svc := NewRateLimitService(repo, nil, nil, nil, nil)
	svc.SetAccountRuntimeBlocker(blocker)
	svc.SetStreamTruncationCounterCache(counter)

	before := time.Now()
	svc.PenalizeClientVisibleStreamTruncation(context.Background(), &Account{ID: 42, Platform: PlatformOpenAI}, "deepseek-v4-pro")

	// 集群级：必须落库，且时长是"可见截断"的短冷却而非 2 分钟窗口降权。
	require.Len(t, repo.calls, 1, "可见截断必须立刻落库临时不可调度（集群级生效）")
	call := repo.calls[0]
	require.Equal(t, int64(42), call.accountID)
	require.Contains(t, call.reason, streamTruncationMatchedKeyword)
	require.Contains(t, call.reason, "deepseek-v4-pro")
	cooldown := call.until.Sub(before)
	require.GreaterOrEqual(t, cooldown, clientVisibleStreamTruncationCooldown-5*time.Second)
	require.LessOrEqual(t, cooldown, clientVisibleStreamTruncationCooldown+5*time.Second)

	// 进程内：不依赖 Redis/DB 的立即冷却（落库路径也会再通知一次运行时调度，
	// 两次都指向同一个账号与同一到期时间，幂等）。
	require.Contains(t, blocker.reasons, clientVisibleStreamTruncationReason)
	require.GreaterOrEqual(t, len(blocker.untils), 1)

	// 常规窗口计数照常累加（短时间内反复可见截断会升级成 2 分钟降权）。
	require.Equal(t, 1, counter.increments)
}

// TestPenalizeClientVisibleStreamTruncationSurvivesRepositoryFailure：落库失败时
// 进程内冷却仍然有效（不能因为 DB/Redis 抖动就完全不降权）。
func TestPenalizeClientVisibleStreamTruncationSurvivesRepositoryFailure(t *testing.T) {
	blocker := &streamFailureSchedulingBlockerStub{}
	svc := NewRateLimitService(nil, nil, nil, nil, nil) // accountRepo nil = 落库不可用
	svc.SetAccountRuntimeBlocker(blocker)

	svc.PenalizeClientVisibleStreamTruncation(context.Background(), &Account{ID: 7, Platform: PlatformOpenAI}, "m")

	require.Equal(t, []string{clientVisibleStreamTruncationReason}, blocker.reasons,
		"落库不可用时进程内冷却必须照常生效")
}

type streamFailureStickyCacheStub struct {
	GatewayCache
	deletedKeys []string
}

func (c *streamFailureStickyCacheStub) DeleteSessionAccountID(_ context.Context, _ int64, sessionHash string) error {
	c.deletedKeys = append(c.deletedKeys, sessionHash)
	return nil
}

// TestClearOpenAIStickySessionAfterStreamFailureDeletesBinding：请求失败后解除
// 会话绑定，客户端重试才能重新选号，而不是被 1 小时粘性绑回同一个坏源。
func TestClearOpenAIStickySessionAfterStreamFailureDeletesBinding(t *testing.T) {
	cache := &streamFailureStickyCacheStub{}
	svc := &OpenAIGatewayService{cache: cache}
	groupID := int64(11)

	svc.ClearOpenAIStickySessionAfterStreamFailure(context.Background(), &groupID, "session-abc")

	require.Equal(t, []string{"openai:session-abc"}, cache.deletedKeys)
}

// TestClearOpenAIStickySessionAfterStreamFailureIgnoresEmptyHash：没有会话哈希时
// 什么都不做（不能误删别人的绑定）。
func TestClearOpenAIStickySessionAfterStreamFailureIgnoresEmptyHash(t *testing.T) {
	cache := &streamFailureStickyCacheStub{}
	svc := &OpenAIGatewayService{cache: cache}
	groupID := int64(11)

	svc.ClearOpenAIStickySessionAfterStreamFailure(context.Background(), &groupID, "   ")

	require.Empty(t, cache.deletedKeys)
}
