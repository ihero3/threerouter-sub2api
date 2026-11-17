package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"

	"github.com/stretchr/testify/require"
)

// ipBlacklistStubRepo 内存版 SettingRepository（bare 测试专用，避免依赖 unit 标签文件）。
type ipBlacklistStubRepo struct {
	mu     sync.Mutex
	values map[string]string
}

func (r *ipBlacklistStubRepo) Get(_ context.Context, key string) (*Setting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.values[key]
	if !ok {
		return nil, ErrSettingNotFound
	}
	return &Setting{Key: key, Value: value}, nil
}

func (r *ipBlacklistStubRepo) GetValue(_ context.Context, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (r *ipBlacklistStubRepo) Set(_ context.Context, key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.values == nil {
		r.values = make(map[string]string)
	}
	r.values[key] = value
	return nil
}

func (r *ipBlacklistStubRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (r *ipBlacklistStubRepo) SetMultiple(_ context.Context, settings map[string]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.values == nil {
		r.values = make(map[string]string)
	}
	for key, value := range settings {
		r.values[key] = value
	}
	return nil
}

func (r *ipBlacklistStubRepo) GetAll(_ context.Context) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]string, len(r.values))
	for key, value := range r.values {
		out[key] = value
	}
	return out, nil
}

func (r *ipBlacklistStubRepo) Delete(_ context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.values, key)
	return nil
}

func newIPBlacklistSettingService(repo SettingRepository) *SettingService {
	return NewSettingService(repo, &config.Config{})
}

func TestGetSecurityIPBlacklistDefaults(t *testing.T) {
	s := newIPBlacklistSettingService(&ipBlacklistStubRepo{})

	// 键缺失 → 默认（关闭、空名单）
	settings, err := s.GetSecurityIPBlacklist(context.Background())
	require.NoError(t, err)
	require.False(t, settings.Enabled)
	require.Empty(t, settings.Blacklist)

	// 缓存读路径同样返回默认（不拦截）
	cached := s.GetSecurityIPBlacklistCached(context.Background())
	require.False(t, cached.Enabled)
	require.Empty(t, cached.Blacklist)
}

func TestGetSecurityIPBlacklistInvalidJSONFallsBack(t *testing.T) {
	repo := &ipBlacklistStubRepo{values: map[string]string{
		SettingKeySecurityIPBlacklist: "not-json",
	}}
	s := newIPBlacklistSettingService(repo)

	settings, err := s.GetSecurityIPBlacklist(context.Background())
	require.NoError(t, err)
	require.False(t, settings.Enabled)
	require.Empty(t, settings.Blacklist)
}

func TestSetSecurityIPBlacklistValidationAndRoundtrip(t *testing.T) {
	repo := &ipBlacklistStubRepo{}
	s := newIPBlacklistSettingService(repo)
	ctx := context.Background()

	// 非法条目 → 拒绝写入
	err := s.SetSecurityIPBlacklist(ctx, &SecurityIPBlacklistSettings{
		Enabled:   true,
		Blacklist: []string{"203.0.113.9", "not-an-ip"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not-an-ip")
	_, getErr := s.GetSecurityIPBlacklist(ctx)
	require.NoError(t, getErr)

	// 合法条目（IPv4/IPv6 精确 + CIDR）→ 写入成功，读回一致
	err = s.SetSecurityIPBlacklist(ctx, &SecurityIPBlacklistSettings{
		Enabled:   true,
		Blacklist: []string{" 203.0.113.9 ", "2a02:4780:5e:4f48::1", "198.51.100.0/24", "2001:db8::/32"},
	})
	require.NoError(t, err)

	// 写后立即生效：直读与缓存读都返回新值（条目已去空白）
	settings, err := s.GetSecurityIPBlacklist(ctx)
	require.NoError(t, err)
	require.True(t, settings.Enabled)
	require.Equal(t, []string{"203.0.113.9", "2a02:4780:5e:4f48::1", "198.51.100.0/24", "2001:db8::/32"}, settings.Blacklist)

	cached := s.GetSecurityIPBlacklistCached(ctx)
	require.True(t, cached.Enabled)
	require.Equal(t, settings.Blacklist, cached.Blacklist)

	// nil 配置 → 报错
	require.Error(t, s.SetSecurityIPBlacklist(ctx, nil))
}

func TestSetSecurityIPBlacklistEmptyListDisables(t *testing.T) {
	repo := &ipBlacklistStubRepo{values: map[string]string{
		SettingKeySecurityIPBlacklist: `{"enabled":true,"blacklist":["203.0.113.9"]}`,
	}}
	s := newIPBlacklistSettingService(repo)
	ctx := context.Background()

	// 保存前：缓存路径拦截生效
	cached := s.GetSecurityIPBlacklistCached(ctx)
	require.True(t, cached.Enabled)
	require.Equal(t, []string{"203.0.113.9"}, cached.Blacklist)

	// 清空名单（开关保留）→ 写入后缓存立即不再有名单
	err := s.SetSecurityIPBlacklist(ctx, &SecurityIPBlacklistSettings{Enabled: true})
	require.NoError(t, err)
	cached = s.GetSecurityIPBlacklistCached(ctx)
	require.True(t, cached.Enabled)
	require.Empty(t, cached.Blacklist)
}

func TestGetSecurityIPBlacklistRepoError(t *testing.T) {
	// repo 报非 NotFound 错误 → Get 返回错误
	s := newIPBlacklistSettingService(&ipBlacklistErrorRepo{})
	_, err := s.GetSecurityIPBlacklist(context.Background())
	require.Error(t, err)

	// 缓存路径 fail-open：默认值（不拦截）
	cached := s.GetSecurityIPBlacklistCached(context.Background())
	require.False(t, cached.Enabled)
}

type ipBlacklistErrorRepo struct{ ipBlacklistStubRepo }

func (r *ipBlacklistErrorRepo) GetValue(_ context.Context, _ string) (string, error) {
	return "", errors.New("db down")
}
