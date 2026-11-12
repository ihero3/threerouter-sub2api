package service

import (
	"context"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// riskEnforceUserRepo 用嵌入接口的方式满足方法众多的 UserRepository，
// 只覆盖本测试实际会调用的方法，避免被迫实现全部方法。
type riskEnforceUserRepo struct {
	UserRepository
	user         *User
	getErr       error
	updateCalled bool
	clearCalled  bool
	clearUserID  int64
}

func (m *riskEnforceUserRepo) GetByID(_ context.Context, _ int64) (*User, error) {
	return m.user, m.getErr
}

func (m *riskEnforceUserRepo) Update(_ context.Context, _ *User, _ UserUpdateFields) error {
	m.updateCalled = true
	return nil
}

// ClearUserRiskReview 模拟仓储 SQL 的效果，便于断言「清标记 + 恢复 active」。
func (m *riskEnforceUserRepo) ClearUserRiskReview(_ context.Context, id int64) error {
	m.clearCalled = true
	m.clearUserID = id
	if m.user != nil {
		m.user.RiskLevel = AffiliateRiskLevelNone
		m.user.RiskReasons = ""
		m.user.RiskUpdatedAt = nil
		m.user.Status = StatusActive
	}
	return nil
}

type riskEnforceAuthCache struct {
	APIKeyAuthCacheInvalidator
	userIDs []int64
}

func (m *riskEnforceAuthCache) InvalidateAuthCacheByUserID(_ context.Context, id int64) {
	m.userIDs = append(m.userIDs, id)
}

// TestUpdateUser_AbnormalCannotBeReactivated 覆盖「abnormal 未清标记时禁止改回 active」。
func TestUpdateUser_AbnormalCannotBeReactivated(t *testing.T) {
	repo := &riskEnforceUserRepo{
		user: &User{ID: 7, Role: RoleUser, Status: StatusDisabled, RiskLevel: AffiliateRiskLevelAbnormal},
	}
	s := &adminServiceImpl{userRepo: repo}

	_, err := s.UpdateUser(context.Background(), 7, &UpdateUserInput{Status: StatusActive})
	if err == nil {
		t.Fatal("expected an error when reactivating an abnormal user, got nil")
	}
	if infraerrors.Reason(err) != riskReviewRequiredCode {
		t.Fatalf("reason = %q, want %q", infraerrors.Reason(err), riskReviewRequiredCode)
	}
	if infraerrors.Message(err) != riskReviewRequiredMessage {
		t.Fatalf("message = %q, want pure-English %q", infraerrors.Message(err), riskReviewRequiredMessage)
	}
	if repo.updateCalled {
		t.Fatal("userRepo.Update must not be called for a blocked reactivation")
	}
}

// TestUpdateUser_NonAbnormalReactivationStillAllowed 对照：普通 disabled 用户仍可改回 active，
// 证明保护是最小条件、不误伤正常用户。
func TestUpdateUser_NonAbnormalReactivationStillAllowed(t *testing.T) {
	repo := &riskEnforceUserRepo{
		user: &User{ID: 8, Role: RoleUser, Status: StatusDisabled, RiskLevel: AffiliateRiskLevelNone},
	}
	s := &adminServiceImpl{userRepo: repo}

	got, err := s.UpdateUser(context.Background(), 8, &UpdateUserInput{Status: StatusActive})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.Status != StatusActive {
		t.Fatalf("status = %v, want %q", got, StatusActive)
	}
	if !repo.updateCalled {
		t.Fatal("userRepo.Update should be called for a normal reactivation")
	}
}

// TestAdminReviewUserRisk_ClearsMarkAndReactivates 覆盖「复核放行清标记 + 恢复 active」，
// 并断言放行后失效了认证缓存。
func TestAdminReviewUserRisk_ClearsMarkAndReactivates(t *testing.T) {
	repo := &riskEnforceUserRepo{
		user: &User{ID: 42, Role: RoleUser, Status: StatusDisabled, RiskLevel: AffiliateRiskLevelAbnormal},
	}
	cache := &riskEnforceAuthCache{}
	s := &adminServiceImpl{userRepo: repo, authCacheInvalidator: cache}

	if err := s.AdminReviewUserRisk(context.Background(), 42); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.clearCalled || repo.clearUserID != 42 {
		t.Fatalf("ClearUserRiskReview not called with 42: called=%v id=%d", repo.clearCalled, repo.clearUserID)
	}
	if repo.user.RiskLevel != AffiliateRiskLevelNone || repo.user.Status != StatusActive {
		t.Fatalf("after review: risk_level=%q status=%q, want cleared + active", repo.user.RiskLevel, repo.user.Status)
	}
	if len(cache.userIDs) != 1 || cache.userIDs[0] != 42 {
		t.Fatalf("auth cache invalidation = %v, want [42]", cache.userIDs)
	}
}

// TestAdminReviewUserRisk_RejectsInvalidUserID 非法 id 必须在触碰仓储前被拒绝。
func TestAdminReviewUserRisk_RejectsInvalidUserID(t *testing.T) {
	repo := &riskEnforceUserRepo{}
	s := &adminServiceImpl{userRepo: repo}

	if err := s.AdminReviewUserRisk(context.Background(), 0); err == nil {
		t.Fatal("expected an error for userID=0")
	}
	if repo.clearCalled {
		t.Fatal("ClearUserRiskReview must not be called for an invalid user id")
	}
}
