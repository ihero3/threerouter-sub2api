package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 2026-11-26 用户要求「账号状态、调度、冷却等一切动作变化都发邮件」。
// 邮件是否发出，取决于 AccountAvailabilityState 能否把「现在不可调度」的各种形态
// 都表达出来——本文件锁死这个口径，防止后续有人往 IsSchedulable 加判断却忘了同步这里。

func TestAccountAvailabilityStateCoversRuntimeCapacity(t *testing.T) {
	future := time.Now().Add(10 * time.Minute)
	past := time.Now().Add(-10 * time.Minute)

	tests := []struct {
		name    string
		account *Account
		want    string
	}{
		{
			name:    "健康账号",
			account: &Account{Status: StatusActive, Schedulable: true},
			want:    StatusActive,
		},
		{
			name:    "调度开关关闭即暂停",
			account: &Account{Status: StatusActive, Schedulable: false},
			want:    "paused",
		},
		{
			name:    "状态位非 active 优先",
			account: &Account{Status: StatusError, Schedulable: true},
			want:    StatusError,
		},
		{
			// 参考图/生图限流这条：列表上是「限流至 09:52」，邮件里也必须是事件名。
			name:    "账号级限流进行中",
			account: &Account{Status: StatusActive, Schedulable: true, RateLimitResetAt: &future},
			want:    AccountEventRateLimited,
		},
		{
			name:    "限流已到期自动回落到 active",
			account: &Account{Status: StatusActive, Schedulable: true, RateLimitResetAt: &past},
			want:    StatusActive,
		},
		{
			name:    "过载冷却优先于限流",
			account: &Account{Status: StatusActive, Schedulable: true, OverloadUntil: &future, RateLimitResetAt: &future},
			want:    AccountEventOverloaded,
		},
		{
			name:    "临时不可调度",
			account: &Account{Status: StatusActive, Schedulable: true, TempUnschedulableUntil: &future, TempUnschedulableReason: "upstream 500"},
			want:    AccountEventTempCooled,
		},
		{
			name: "用量阈值暂停单独成事件",
			account: &Account{
				Status:                  StatusActive,
				Schedulable:             true,
				TempUnschedulableUntil:  &future,
				TempUnschedulableReason: BuildAccountSchedulingThresholdReason("5h window reached 92%"),
			},
			want: AccountEventSchedulingThreshold,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, AccountAvailabilityState(tt.account))
		})
	}
}

func TestAccountAvailabilityStateNilIsEmpty(t *testing.T) {
	require.Equal(t, "", AccountAvailabilityState(nil))
}

// 冷却到期不需要任何写库动作，只有「现在算出来的状态」发生变化，
// 定时复查才可能补发一封恢复邮件；这里验证同一对象在时间推进后状态会变。
func TestAccountAvailabilityStateFlipsAfterCooldownExpires(t *testing.T) {
	until := time.Now().Add(50 * time.Millisecond)
	account := &Account{Status: StatusActive, Schedulable: true, RateLimitResetAt: &until}

	require.Equal(t, AccountEventRateLimited, AccountAvailabilityState(account))
	require.Eventually(t, func() bool {
		return AccountAvailabilityState(account) == StatusActive
	}, time.Second, 20*time.Millisecond)
}
