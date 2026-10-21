//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

// IP 每日注册限额默认关闭：即使历史上该 IP 注册过大量账户也不拦截不改变现有行为。
func TestAuthService_Register_IPQuotaDisabledByDefault(t *testing.T) {
	repo := &userRepoStub{nextID: 7, ipCounts: map[string]int{"203.0.113.9": 99}}
	svc := newAuthService(repo, map[string]string{
		SettingKeyRegistrationEnabled: "true",
	}, nil, nil)

	_, user, err := svc.Register(context.Background(), "anyone@example.com", "password", "203.0.113.9")
	require.NoError(t, err)
	require.Equal(t, int64(7), user.ID)
	require.Zero(t, repo.ipLimitedCreates)
}

// 达到当日上限时拒绝，返回统一 reason 与“明天再试”的用户可见文案。
func TestAuthService_Register_IPQuotaRejectsOverDailyLimit(t *testing.T) {
	repo := &userRepoStub{ipCounts: map[string]int{"203.0.113.9": 2}}
	svc := newAuthService(repo, map[string]string{
		SettingKeyRegistrationEnabled:        "true",
		SettingKeyRegistrationIPQuotaEnabled: "true",
		SettingKeyRegistrationIPQuotaLimit:   "2",
	}, nil, nil)

	_, _, err := svc.Register(context.Background(), "blocked@example.com", "password", "203.0.113.9")
	require.ErrorIs(t, err, ErrRegistrationIPLimit)
	appErr := infraerrors.FromError(err)
	require.Equal(t, "REGISTRATION_IP_LIMIT", appErr.Reason)
	require.Contains(t, appErr.Message, "today")
	require.Contains(t, appErr.Message, "tomorrow")
}

// 未达上限时放行，并确认建号走的是带事务内复查的守卫路径，register_ip 已落库。
func TestAuthService_Register_IPQuotaAllowsBelowLimitAndGuardsCreate(t *testing.T) {
	repo := &userRepoStub{nextID: 11, ipCounts: map[string]int{"203.0.113.9": 1}}
	svc := newAuthService(repo, map[string]string{
		SettingKeyRegistrationEnabled:        "true",
		SettingKeyRegistrationIPQuotaEnabled: "true",
		SettingKeyRegistrationIPQuotaLimit:   "2",
	}, nil, nil)

	_, user, err := svc.Register(context.Background(), "second@example.com", "password", "203.0.113.9")
	require.NoError(t, err)
	require.Equal(t, int64(11), user.ID)
	require.Equal(t, 1, repo.ipLimitedCreates)
	require.Equal(t, "203.0.113.9", user.RegisterIP)
}

// 拿不到客户端 IP 时不参与限额统计，也不进入 IP 守卫创建路径。
func TestAuthService_Register_IPQuotaDoesNotApplyWithoutClientIP(t *testing.T) {
	repo := &userRepoStub{nextID: 12, ipCounts: map[string]int{"": 99}}
	svc := newAuthService(repo, map[string]string{
		SettingKeyRegistrationEnabled:        "true",
		SettingKeyRegistrationIPQuotaEnabled: "true",
		SettingKeyRegistrationIPQuotaLimit:   "1",
	}, nil, nil)

	_, user, err := svc.Register(context.Background(), "noproxy@example.com", "password", "")
	require.NoError(t, err)
	require.Equal(t, int64(12), user.ID)
	require.Zero(t, repo.ipLimitedCreates)
}

// 统计查询失败时按服务不可用处理，避免限额被静默绕过。
func TestAuthService_Register_IPQuotaCountErrorReturnsServiceUnavailable(t *testing.T) {
	repo := &userRepoStub{ipCountErr: errors.New("db down")}
	svc := newAuthService(repo, map[string]string{
		SettingKeyRegistrationEnabled:        "true",
		SettingKeyRegistrationIPQuotaEnabled: "true",
		SettingKeyRegistrationIPQuotaLimit:   "2",
	}, nil, nil)

	_, _, err := svc.Register(context.Background(), "later@example.com", "password", "203.0.113.9")
	require.ErrorIs(t, err, ErrServiceUnavailable)
}

// 预检通过后仍可能被并发注册挤掉配额：建号事务内复查必须同样拒绝。
func TestAuthService_Register_IPQuotaRecheckedInsideCreate(t *testing.T) {
	repo := &userRepoStub{
		nextID:      13,
		ipCounts:    map[string]int{"203.0.113.9": 0},
		ipCreateErr: BuildRegistrationIPLimitError(1),
	}
	svc := newAuthService(repo, map[string]string{
		SettingKeyRegistrationEnabled:        "true",
		SettingKeyRegistrationIPQuotaEnabled: "true",
		SettingKeyRegistrationIPQuotaLimit:   "1",
	}, nil, nil)

	_, _, err := svc.Register(context.Background(), "race@example.com", "password", "203.0.113.9")
	require.ErrorIs(t, err, ErrRegistrationIPLimit)
}

func TestClampRegistrationIPQuotaLimit(t *testing.T) {
	require.Equal(t, RegistrationIPQuotaLimitMin, clampRegistrationIPQuotaLimit(0))
	require.Equal(t, RegistrationIPQuotaLimitMin, clampRegistrationIPQuotaLimit(-5))
	require.Equal(t, 5, clampRegistrationIPQuotaLimit(5))
	require.Equal(t, RegistrationIPQuotaLimitMax, clampRegistrationIPQuotaLimit(RegistrationIPQuotaLimitMax+1))
}

// 设置缺失/非法时回退默认值，越界值被钳制到区间内。
func TestSettingService_GetRegistrationIPQuotaLimit(t *testing.T) {
	tests := []struct {
		name    string
		values  map[string]string
		want    int
		enabled bool
	}{
		{name: "missing falls back to default", values: map[string]string{}, want: RegistrationIPQuotaLimitDefault, enabled: false},
		{name: "invalid falls back to default", values: map[string]string{SettingKeyRegistrationIPQuotaLimit: "abc"}, want: RegistrationIPQuotaLimitDefault, enabled: false},
		{name: "zero clamped to min", values: map[string]string{SettingKeyRegistrationIPQuotaLimit: "0"}, want: RegistrationIPQuotaLimitMin, enabled: false},
		{name: "too large clamped to max", values: map[string]string{SettingKeyRegistrationIPQuotaLimit: "5000"}, want: RegistrationIPQuotaLimitMax, enabled: false},
		{name: "explicit enabled and limit", values: map[string]string{
			SettingKeyRegistrationIPQuotaEnabled: "true",
			SettingKeyRegistrationIPQuotaLimit:   "5",
		}, want: 5, enabled: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewSettingService(&settingRepoStub{values: tt.values}, &config.Config{})
			ctx := context.Background()
			require.Equal(t, tt.want, svc.GetRegistrationIPQuotaLimit(ctx))
			require.Equal(t, tt.enabled, svc.IsRegistrationIPQuotaEnabled(ctx))
		})
	}
}
