package repository

import (
	"testing"
	"time"
)

// 模型级冷却通知的去重判断：仅「新进入冷却」通知，冷却期内刷新不重复通知。
func TestModelLimitNotificationNeeded(t *testing.T) {
	now := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		name        string
		prevResetAt time.Time
		want        bool
	}{
		{"no previous record", time.Time{}, true},
		{"previous expired", now.Add(-time.Minute), true},
		{"previous boundary expired (re-entry)", now, true},
		{"still within cooldown", now.Add(30 * time.Minute), false},
	}
	for _, tc := range cases {
		if got := modelLimitNotificationNeeded(tc.prevResetAt, now); got != tc.want {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}
