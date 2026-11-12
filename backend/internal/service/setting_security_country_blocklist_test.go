package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"

	"github.com/stretchr/testify/require"
)

// countryBlocklistErrRepo 模拟设置表读取故障，验证缓存路径 fail-open。
type countryBlocklistErrRepo struct {
	*ipBlacklistStubRepo
}

func (r *countryBlocklistErrRepo) GetValue(_ context.Context, _ string) (string, error) {
	return "", errors.New("settings db unavailable")
}

func newCountryBlocklistSettingService(repo SettingRepository) *SettingService {
	return NewSettingService(repo, &config.Config{})
}

func TestSecurityCountryBlocklistDefaults(t *testing.T) {
	s := newCountryBlocklistSettingService(&ipBlacklistStubRepo{})

	// 键缺失：默认关闭、空名单；升级后线上行为完全不变
	settings, err := s.GetSecurityCountryBlocklist(context.Background())
	require.NoError(t, err)
	require.False(t, settings.Enabled)
	require.Empty(t, settings.Countries)

	cached := s.GetSecurityCountryBlocklistCached(context.Background())
	require.False(t, cached.Enabled)
	require.Empty(t, cached.Countries)
}

func TestSecurityCountryBlocklistNormalizeAndPersist(t *testing.T) {
	repo := &ipBlacklistStubRepo{}
	s := newCountryBlocklistSettingService(repo)

	err := s.SetSecurityCountryBlocklist(context.Background(), &SecurityCountryBlocklistSettings{
		Enabled:   true,
		Countries: []string{" cn ", "US", "cn", "  ", "de"},
	})
	require.NoError(t, err)

	settings, err := s.GetSecurityCountryBlocklist(context.Background())
	require.NoError(t, err)
	require.True(t, settings.Enabled)
	require.Equal(t, []string{"CN", "US", "DE"}, settings.Countries)

	// 写入必须立即刷新进程内缓存（多节点最多 60s）
	cached := s.GetSecurityCountryBlocklistCached(context.Background())
	require.True(t, cached.Enabled)
	require.Equal(t, []string{"CN", "US", "DE"}, cached.Countries)

	// 关闭时名单保留不丢
	require.NoError(t, s.SetSecurityCountryBlocklist(context.Background(), &SecurityCountryBlocklistSettings{
		Enabled:   false,
		Countries: settings.Countries,
	}))
	off := s.GetSecurityCountryBlocklistCached(context.Background())
	require.False(t, off.Enabled)
	require.Equal(t, []string{"CN", "US", "DE"}, off.Countries)
}

func TestSecurityCountryBlocklistCachedReadsAndNormalizes(t *testing.T) {
	repo := &ipBlacklistStubRepo{values: map[string]string{
		SettingKeySecurityCountryBlocklist: `{"enabled":true,"countries":["cn","cn","us"," tw "]}`,
	}}
	s := newCountryBlocklistSettingService(repo)

	cached := s.GetSecurityCountryBlocklistCached(context.Background())
	require.True(t, cached.Enabled)
	require.Equal(t, []string{"CN", "US", "TW"}, cached.Countries)
}

func TestSecurityCountryBlocklistValidation(t *testing.T) {
	for _, code := range []string{"CHN", "C1", "中", "A", "1A"} {
		repo := &ipBlacklistStubRepo{}
		s := newCountryBlocklistSettingService(repo)
		err := s.SetSecurityCountryBlocklist(context.Background(), &SecurityCountryBlocklistSettings{
			Enabled:   true,
			Countries: []string{code},
		})
		require.Error(t, err, "code %q must be rejected", code)
		// 校验失败绝不能写设置表
		_, getErr := repo.GetValue(context.Background(), SettingKeySecurityCountryBlocklist)
		require.ErrorIs(t, getErr, ErrSettingNotFound)
	}

	// 超过条目上限：用 301 个互不相同的两位字母代码（去重后仍超限）
	codes := make([]string, maxSecurityCountryBlocklistEntries+1)
	for i := range codes {
		codes[i] = fmt.Sprintf("%c%c", 'A'+rune(i/26), 'A'+rune(i%26))
	}
	s := newCountryBlocklistSettingService(&ipBlacklistStubRepo{})
	err := s.SetSecurityCountryBlocklist(context.Background(), &SecurityCountryBlocklistSettings{
		Enabled:   true,
		Countries: codes,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "too many")

	// 空项会被规范化掉，得到空名单，属于合法输入
	s2 := newCountryBlocklistSettingService(&ipBlacklistStubRepo{})
	require.NoError(t, s2.SetSecurityCountryBlocklist(context.Background(), &SecurityCountryBlocklistSettings{
		Enabled:   true,
		Countries: []string{"", "  "},
	}))
	got, err := s2.GetSecurityCountryBlocklist(context.Background())
	require.NoError(t, err)
	require.Empty(t, got.Countries)
}

func TestSecurityCountryBlocklistInvalidJSONFallsBack(t *testing.T) {
	repo := &ipBlacklistStubRepo{values: map[string]string{
		SettingKeySecurityCountryBlocklist: "not-json",
	}}
	s := newCountryBlocklistSettingService(repo)

	settings, err := s.GetSecurityCountryBlocklist(context.Background())
	require.NoError(t, err)
	require.False(t, settings.Enabled)
	require.Empty(t, settings.Countries)

	cached := s.GetSecurityCountryBlocklistCached(context.Background())
	require.False(t, cached.Enabled)
}

func TestSecurityCountryBlocklistDBErrorFailOpen(t *testing.T) {
	s := newCountryBlocklistSettingService(&countryBlocklistErrRepo{&ipBlacklistStubRepo{}})

	_, err := s.GetSecurityCountryBlocklist(context.Background())
	require.Error(t, err)

	// 中间件读的是缓存路径：DB 挂了必须退回默认关闭，绝不拦截全站
	cached := s.GetSecurityCountryBlocklistCached(context.Background())
	require.False(t, cached.Enabled)
	require.Empty(t, cached.Countries)
}

func TestSecurityCountryBlocklistNilSettingsRejected(t *testing.T) {
	s := newCountryBlocklistSettingService(&ipBlacklistStubRepo{})
	err := s.SetSecurityCountryBlocklist(context.Background(), nil)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "nil"))
}
