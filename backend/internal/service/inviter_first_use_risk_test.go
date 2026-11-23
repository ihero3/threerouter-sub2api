package service

import (
	"context"
	"testing"
)

type fakeInviterRiskEvaluator struct {
	inviterID int64
	reasons   []string
	calls     int
}

func (f *fakeInviterRiskEvaluator) EvaluateInviterFirstUseRisk(_ context.Context, inviteeID int64) (int64, []string) {
	f.calls++
	if inviteeID <= 0 {
		return 0, nil
	}
	return f.inviterID, f.reasons
}

type fakeRiskExemption struct {
	exempt bool
}

func (f *fakeRiskExemption) IsExempt(_ context.Context, _ int64) bool { return f.exempt }
func (f *fakeRiskExemption) Invalidate(_ int64)                       {}

func newInviterFirstUseFixture() (*IPConsistencyService, *fakePenaltyLocker, *fakePenalty, *fakeNewUserRiskMarker, *fakeInviterRiskEvaluator) {
	locker := &fakePenaltyLocker{}
	penalty := &fakePenalty{started: make(chan struct{}, 4)}
	marker := &fakeNewUserRiskMarker{}
	evaluator := &fakeInviterRiskEvaluator{}
	svc := NewIPConsistencyService(&fakeIPConsistencyCache{}, penaltySettings())
	svc.SetPenaltyService(penalty, locker)
	svc.SetNewUserRiskMarker(marker)
	svc.SetInviterRiskEvaluator(evaluator)
	return svc, locker, penalty, marker, evaluator
}

// TestInviterFirstUse_PenalizesOnlyInviter 锁死核心口径（2026-11-23 用户拍板）：
// 被邀请人首用时只罚邀请人；被邀请人自己不被打标、不扣款、请求不被阻断。
func TestInviterFirstUse_PenalizesOnlyInviter(t *testing.T) {
	svc, locker, penalty, marker, evaluator := newInviterFirstUseFixture()
	evaluator.inviterID = 99
	evaluator.reasons = []string{AffiliateRiskReasonInviteBurst24h}

	svc.EvaluateInviterRiskOnFirstUse(context.Background(), 42)

	pc := waitPenaltyCall(t, penalty)
	if pc.userID != 99 {
		t.Fatalf("penalty must hit inviter 99, got %+v", pc)
	}
	if len(marker.calls) != 1 || marker.calls[0].userID != 99 || marker.calls[0].level != AffiliateRiskLevelSuspect {
		t.Fatalf("must mark inviter suspect exactly once, got %+v", marker.calls)
	}
	_, _, lastKey, _ := locker.snapshot()
	if want := ipConsistencyInviterFirstUsePenaltyKeyPrefix + "99"; lastKey != want {
		t.Fatalf("penalty key = %q, want %q", lastKey, want)
	}
	// 被邀请人 42 绝不出现在处罚/打标对象里（上面已断言都是 99）。
}

// TestInviterFirstUse_NoHitDoesNothing 未命中时不打标、不扣款。
func TestInviterFirstUse_NoHitDoesNothing(t *testing.T) {
	svc, _, penalty, marker, evaluator := newInviterFirstUseFixture()
	evaluator.inviterID = 0
	evaluator.reasons = nil

	svc.EvaluateInviterRiskOnFirstUse(context.Background(), 42)

	if len(marker.calls) != 0 || len(penalty.snapshot()) != 0 {
		t.Fatalf("no hit must not penalize: marker=%+v penalty=%+v", marker.calls, penalty.snapshot())
	}
}

// TestInviterFirstUse_ExemptionSkipsPenalty 邀请人工单/复核豁免时不处罚。
func TestInviterFirstUse_ExemptionSkipsPenalty(t *testing.T) {
	svc, _, penalty, marker, evaluator := newInviterFirstUseFixture()
	evaluator.inviterID = 99
	evaluator.reasons = []string{AffiliateRiskReasonChainDepth}
	svc.SetRiskExemptionChecker(&fakeRiskExemption{exempt: true})

	svc.EvaluateInviterRiskOnFirstUse(context.Background(), 42)

	if len(marker.calls) != 0 || len(penalty.snapshot()) != 0 {
		t.Fatalf("exempt inviter must not be penalized: marker=%+v penalty=%+v", marker.calls, penalty.snapshot())
	}
}

// TestInviterFirstUse_OneTimeGate 首用评估一次性门控：已被占用时不再查库/处罚。
func TestInviterFirstUse_OneTimeGate(t *testing.T) {
	svc, locker, _, marker, evaluator := newInviterFirstUseFixture()
	evaluator.inviterID = 99
	evaluator.reasons = []string{AffiliateRiskReasonInviteBurst24h}
	locker.claimSet = true
	locker.claimResult = false

	svc.EvaluateInviterRiskOnFirstUse(context.Background(), 42)

	if evaluator.calls != 0 || len(marker.calls) != 0 {
		t.Fatalf("gate claimed must skip evaluation: calls=%d marker=%+v", evaluator.calls, marker.calls)
	}
	_, _, lastKey, lastTTL := locker.snapshot()
	if want := ipInviterFirstUseEvalGatePrefix + "42"; lastKey != want {
		t.Fatalf("gate key = %q, want %q", lastKey, want)
	}
	if lastTTL != inviterFirstUseEvalGateTTL {
		t.Fatalf("gate ttl = %v, want %v", lastTTL, inviterFirstUseEvalGateTTL)
	}
}
