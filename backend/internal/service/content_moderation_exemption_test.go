package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type autoBanExemptionStub struct {
	exempt bool
	called []int64
}

func (s *autoBanExemptionStub) IsExempt(_ context.Context, id int64) bool {
	s.called = append(s.called, id)
	return s.exempt
}
func (s *autoBanExemptionStub) Invalidate(int64) {}

func autoBanTestService(exemption RiskExemptionCheck, userRepo UserRepository) *ContentModerationService {
	svc := &ContentModerationService{userRepo: userRepo}
	svc.SetRiskExemptionChecker(exemption)
	return svc
}

func autoBanTestConfig() *ContentModerationConfig {
	return &ContentModerationConfig{AutoBanEnabled: true, BanThreshold: 1}
}

// 提过工单 / 复核放行的用户命中豁免时，内容风控自动封号必须跳过。
func TestAutoBan_SkippedForExemptUser(t *testing.T) {
	userID := int64(7)
	userRepo := &contentModerationTestUserRepo{user: &User{ID: userID, Role: RoleUser, Status: StatusActive}}
	checker := &autoBanExemptionStub{exempt: true}
	svc := autoBanTestService(checker, userRepo)

	applied := svc.applyFlaggedAccountSideEffects(context.Background(), autoBanTestConfig(),
		&ContentModerationLog{Flagged: true, UserID: &userID})

	require.False(t, applied, "exempt user must not be auto-banned")
	require.Equal(t, StatusActive, userRepo.user.Status)
	require.Equal(t, []int64{userID}, checker.called, "exemption must actually be checked")
}

func TestAutoBan_AppliesForNonExemptUser(t *testing.T) {
	userID := int64(7)
	userRepo := &contentModerationTestUserRepo{user: &User{ID: userID, Role: RoleUser, Status: StatusActive}}
	checker := &autoBanExemptionStub{exempt: false}
	svc := autoBanTestService(checker, userRepo)

	applied := svc.applyFlaggedAccountSideEffects(context.Background(), autoBanTestConfig(),
		&ContentModerationLog{Flagged: true, UserID: &userID})

	require.True(t, applied, "non-exempt user must still be auto-banned")
	require.Equal(t, StatusDisabled, userRepo.user.Status)
}

// 检查器未注入时保持旧行为：豁免不生效，处罚照常（fail-open 为不豁免）。
func TestAutoBan_NoCheckerKeepsLegacyBehavior(t *testing.T) {
	userID := int64(7)
	userRepo := &contentModerationTestUserRepo{user: &User{ID: userID, Role: RoleUser, Status: StatusActive}}
	svc := &ContentModerationService{userRepo: userRepo}

	applied := svc.applyFlaggedAccountSideEffects(context.Background(), autoBanTestConfig(),
		&ContentModerationLog{Flagged: true, UserID: &userID})

	require.True(t, applied)
	require.Equal(t, StatusDisabled, userRepo.user.Status)
}
