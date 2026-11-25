//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// autoUnbanUserRepo 用嵌入接口的方式满足方法众多的 UserRepository，
// 只覆盖自动解封实际会调用的方法。
type autoUnbanUserRepo struct {
	UserRepository
	user    *User
	getErr  error
	clearOK bool

	clearCalled bool
	clearUserID int64
	clearErr    error
	updateCalls int
	updateField UserUpdateFields
}

func (m *autoUnbanUserRepo) GetByID(_ context.Context, _ int64) (*User, error) {
	return m.user, m.getErr
}

func (m *autoUnbanUserRepo) Update(_ context.Context, user *User, fields UserUpdateFields) error {
	m.updateCalls++
	m.updateField = fields
	if m.user != nil {
		m.user.Status = user.Status
	}
	return nil
}

// ClearUserRiskReview 模拟仓储 SQL：清风险标记 + 恢复 active + 登记豁免。
func (m *autoUnbanUserRepo) ClearUserRiskReview(_ context.Context, id int64) error {
	m.clearCalled = true
	m.clearUserID = id
	if m.clearErr != nil {
		return m.clearErr
	}
	if m.user != nil {
		m.user.RiskLevel = AffiliateRiskLevelNone
		m.user.RiskReasons = ""
		m.user.RiskUpdatedAt = nil
		m.user.Status = StatusActive
	}
	return nil
}

// plainUserRepo 故意不实现 UserRiskReviewRepository，验证降级分支。
type plainAutoUnbanUserRepo struct {
	UserRepository
	user        *User
	updateCalls int
	updateField UserUpdateFields
}

func (m *plainAutoUnbanUserRepo) GetByID(_ context.Context, _ int64) (*User, error) {
	return m.user, nil
}

func (m *plainAutoUnbanUserRepo) Update(_ context.Context, user *User, fields UserUpdateFields) error {
	m.updateCalls++
	m.updateField = fields
	m.user.Status = user.Status
	return nil
}

type autoUnbanExemption struct {
	invalidated []int64
}

func (m *autoUnbanExemption) Invalidate(userID int64) {
	m.invalidated = append(m.invalidated, userID)
}

type autoUnbanAuthCache struct {
	APIKeyAuthCacheInvalidator
	userIDs []int64
}

func (m *autoUnbanAuthCache) InvalidateAuthCacheByUserID(_ context.Context, id int64) {
	m.userIDs = append(m.userIDs, id)
}

// TestReactivateUserAfterRecharge_ClearsRiskLikeReview 覆盖禁用账号充值后按
// 「复核放行」口径解封：清标记、恢复 active、登记豁免，并立即失效两侧缓存。
func TestReactivateUserAfterRecharge_ClearsRiskLikeReview(t *testing.T) {
	repo := &autoUnbanUserRepo{
		user: &User{ID: 42, Status: StatusDisabled, RiskLevel: AffiliateRiskLevelAbnormal, RiskReasons: "bot"},
	}
	exemption := &autoUnbanExemption{}
	authCache := &autoUnbanAuthCache{}
	s := &RedeemService{
		userRepo:                 repo,
		riskExemptionInvalidator: exemption,
		authCacheInvalidator:     authCache,
	}

	s.reactivateUserAfterRecharge(context.Background(), 42)

	require.True(t, repo.clearCalled)
	require.Equal(t, int64(42), repo.clearUserID)
	require.Zero(t, repo.updateCalls, "走 ClearUserRiskReview 时不应再裸回写整行")
	require.Equal(t, StatusActive, repo.user.Status)
	require.Equal(t, AffiliateRiskLevelNone, repo.user.RiskLevel)
	require.Empty(t, repo.user.RiskReasons)
	require.Equal(t, []int64{42}, exemption.invalidated)
	require.Equal(t, []int64{42}, authCache.userIDs)
}

// TestReactivateUserAfterRecharge_SkipsActiveUser 正常账号不重复解封，也不失效缓存。
func TestReactivateUserAfterRecharge_SkipsActiveUser(t *testing.T) {
	repo := &autoUnbanUserRepo{user: &User{ID: 7, Status: StatusActive}}
	exemption := &autoUnbanExemption{}
	authCache := &autoUnbanAuthCache{}
	s := &RedeemService{
		userRepo:                 repo,
		riskExemptionInvalidator: exemption,
		authCacheInvalidator:     authCache,
	}

	s.reactivateUserAfterRecharge(context.Background(), 7)

	require.False(t, repo.clearCalled)
	require.Zero(t, repo.updateCalls)
	require.Empty(t, exemption.invalidated)
	require.Empty(t, authCache.userIDs)
}

// TestReactivateUserAfterRecharge_ClearFailureKeepsCaches 解封失败只记日志，
// 不失效缓存（否则会用一个仍是 disabled 的用户刷新豁免缓存）。
func TestReactivateUserAfterRecharge_ClearFailureKeepsCaches(t *testing.T) {
	repo := &autoUnbanUserRepo{
		user:     &User{ID: 9, Status: StatusDisabled, RiskLevel: AffiliateRiskLevelAbnormal},
		clearErr: errors.New("db down"),
	}
	exemption := &autoUnbanExemption{}
	authCache := &autoUnbanAuthCache{}
	s := &RedeemService{
		userRepo:                 repo,
		riskExemptionInvalidator: exemption,
		authCacheInvalidator:     authCache,
	}

	require.NotPanics(t, func() {
		s.reactivateUserAfterRecharge(context.Background(), 9)
	})

	require.True(t, repo.clearCalled)
	require.Empty(t, exemption.invalidated)
	require.Empty(t, authCache.userIDs)
}

// TestReactivateUserAfterRecharge_FallbackWithoutReviewRepo 仓储不支持复核接口时
// 降级为直接改 status。
func TestReactivateUserAfterRecharge_FallbackWithoutReviewRepo(t *testing.T) {
	repo := &plainAutoUnbanUserRepo{user: &User{ID: 11, Status: StatusDisabled}}
	s := &RedeemService{userRepo: repo}

	s.reactivateUserAfterRecharge(context.Background(), 11)

	require.Equal(t, 1, repo.updateCalls)
	require.True(t, repo.updateField.Status)
	require.Equal(t, StatusActive, repo.user.Status)
}

// TestReactivateUserAfterRecharge_IgnoresInvalidInput 非法入参与查询失败都不应触碰仓储。
func TestReactivateUserAfterRecharge_IgnoresInvalidInput(t *testing.T) {
	repo := &autoUnbanUserRepo{
		user:   &User{ID: 5, Status: StatusDisabled},
		getErr: errors.New("not found"),
	}
	s := &RedeemService{userRepo: repo}

	require.NotPanics(t, func() {
		s.reactivateUserAfterRecharge(context.Background(), 0)
		s.reactivateUserAfterRecharge(context.Background(), -1)
		s.reactivateUserAfterRecharge(context.Background(), 5)
		(*RedeemService)(nil).reactivateUserAfterRecharge(context.Background(), 5)
	})

	require.False(t, repo.clearCalled)
	require.Zero(t, repo.updateCalls)
}
