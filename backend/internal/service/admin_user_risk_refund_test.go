package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// riskRefundUserRepo 用嵌入接口满足方法众多的 UserRepository，只实现复核放行
// 链路会用到的方法，同时实现可选的 IPPenaltyRefundRepository。
type riskRefundUserRepo struct {
	UserRepository
	user           *User
	refundAmount   float64
	refundErr      error
	refundCalls    int
	refundedUserID int64
	clearCalls     int
	events         []string
}

func (m *riskRefundUserRepo) GetByID(_ context.Context, _ int64) (*User, error) {
	return m.user, nil
}

func (m *riskRefundUserRepo) RefundIPBalancePenalties(_ context.Context, id int64) (float64, error) {
	m.refundCalls++
	m.refundedUserID = id
	m.events = append(m.events, "refund")
	return m.refundAmount, m.refundErr
}

func (m *riskRefundUserRepo) ClearUserRiskReview(_ context.Context, _ int64) error {
	m.clearCalls++
	m.events = append(m.events, "clear")
	return nil
}

func newRiskRefundService(repo *riskRefundUserRepo) *adminServiceImpl {
	return &adminServiceImpl{userRepo: repo}
}

// TestAdminReviewUserRisk_RefundsBeforeClearing 覆盖「先返还、后清标记」的顺序：
// 反序会在返还失败时留下「标记清了、钱没退、按钮消失」的死角。
func TestAdminReviewUserRisk_RefundsBeforeClearing(t *testing.T) {
	repo := &riskRefundUserRepo{
		user:         &User{ID: 42, Status: StatusDisabled, RiskLevel: AffiliateRiskLevelAbnormal},
		refundAmount: 4.97,
	}
	if err := newRiskRefundService(repo).AdminReviewUserRisk(context.Background(), 42); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(repo.events, []string{"refund", "clear"}) {
		t.Fatalf("call order = %v, want [refund clear]", repo.events)
	}
	if repo.refundedUserID != 42 || repo.clearCalls != 1 {
		t.Fatalf("refund user=%d clear calls=%d, want 42 / 1", repo.refundedUserID, repo.clearCalls)
	}
}

// TestAdminReviewUserRisk_NoDeductionStillApproves 覆盖「之前没扣过就不返还」：
// 返还仓储返回 0 时复核照常通过，不能因为没扣过就卡住放行。
func TestAdminReviewUserRisk_NoDeductionStillApproves(t *testing.T) {
	repo := &riskRefundUserRepo{
		user:         &User{ID: 43, Status: StatusActive, RiskLevel: AffiliateRiskLevelIPSuspect},
		refundAmount: 0,
	}
	if err := newRiskRefundService(repo).AdminReviewUserRisk(context.Background(), 43); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.refundCalls != 1 || repo.refundAmount != 0 {
		t.Fatal("refund must be attempted once and be a no-op when nothing was deducted")
	}
	if !reflect.DeepEqual(repo.events, []string{"refund", "clear"}) {
		t.Fatalf("call order = %v, want [refund clear]", repo.events)
	}
}

// TestAdminReviewUserRisk_RefundFailureKeepsReviewPending 覆盖返还失败时不清除标记，
// 让管理员可以重试而不是把入口一起关掉。
func TestAdminReviewUserRisk_RefundFailureKeepsReviewPending(t *testing.T) {
	repo := &riskRefundUserRepo{
		user:      &User{ID: 44, Status: StatusActive, RiskLevel: AffiliateRiskLevelSuspect},
		refundErr: errors.New("db unavailable"),
	}
	if err := newRiskRefundService(repo).AdminReviewUserRisk(context.Background(), 44); err == nil {
		t.Fatal("expected refund failure to abort the review")
	}
	if repo.clearCalls != 0 {
		t.Fatal("risk marks must not be cleared when the refund failed")
	}
}

// TestAdminReviewUserRisk_WorksWithoutRefundCapability 覆盖可选接口的向后兼容：
// 未实现返还能力的仓储（如测试桩）仍可正常复核放行。
func TestAdminReviewUserRisk_WorksWithoutRefundCapability(t *testing.T) {
	repo := &riskEnforceUserRepo{user: &User{ID: 45, Status: StatusActive, RiskLevel: AffiliateRiskLevelSuspect}}
	s := &adminServiceImpl{userRepo: repo}
	if err := s.AdminReviewUserRisk(context.Background(), 45); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.clearCalled || repo.clearUserID != 45 {
		t.Fatalf("review must still clear marks, clearCalled=%v id=%d", repo.clearCalled, repo.clearUserID)
	}
}
