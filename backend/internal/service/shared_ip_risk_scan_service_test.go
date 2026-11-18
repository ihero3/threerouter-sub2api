package service

import (
	"context"
	"sync"
	"testing"
	"time"
)

// ---------------- 共享 IP 风险扫描单测（不依赖 DB / Redis） ----------------

type fakeSharedIPLister struct {
	ids []int64
	err error
}

func (f *fakeSharedIPLister) ListUserIDsBySharedIP(_ context.Context, _ int, _ time.Duration) ([]int64, error) {
	return f.ids, f.err
}

type fakeSharedIPMarkWriter struct {
	mu    sync.Mutex
	marks []UserRiskMark
}

func (f *fakeSharedIPMarkWriter) BumpUserRiskMarksBatch(_ context.Context, marks []UserRiskMark) {
	f.mu.Lock()
	f.marks = append(f.marks, marks...)
	f.mu.Unlock()
}

func (f *fakeSharedIPMarkWriter) snapshot() []UserRiskMark {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]UserRiskMark, len(f.marks))
	copy(out, f.marks)
	return out
}

type fakeSharedIPPenalty struct {
	mu    sync.Mutex
	calls []int64
}

func (f *fakeSharedIPPenalty) ApplyIPBalancePenalty(_ context.Context, userID int64, _ float64, _ string, _ bool) error {
	f.mu.Lock()
	f.calls = append(f.calls, userID)
	f.mu.Unlock()
	return nil
}

func (f *fakeSharedIPPenalty) snapshot() []int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]int64, len(f.calls))
	copy(out, f.calls)
	return out
}

type fakeExemptionCheck struct {
	exempt map[int64]bool
}

func (f *fakeExemptionCheck) IsExempt(_ context.Context, userID int64) bool {
	return f.exempt[userID]
}
func (f *fakeExemptionCheck) Invalidate(_ int64) {}

func newScanServiceForTest(lister *fakeSharedIPLister, writer *fakeSharedIPMarkWriter, penalty *fakeSharedIPPenalty, exempt RiskExemptionCheck) *SharedIPRiskScanService {
	return NewSharedIPRiskScanService(lister, writer, penalty, exempt, nil)
}

func TestSharedIPScan_MarksAndPenalizesNonExempt(t *testing.T) {
	lister := &fakeSharedIPLister{ids: []int64{1, 2, 3}}
	writer := &fakeSharedIPMarkWriter{}
	penalty := &fakeSharedIPPenalty{}
	svc := newScanServiceForTest(lister, writer, penalty, &fakeExemptionCheck{})

	svc.runOnce()

	marks := writer.snapshot()
	if len(marks) != 3 {
		t.Fatalf("expected 3 marks, got %d", len(marks))
	}
	for _, m := range marks {
		if m.Level != AffiliateRiskLevelSuspect {
			t.Fatalf("expected suspect mark, got %q", m.Level)
		}
		if len(m.Reasons) != 1 || m.Reasons[0] != sharedIPRiskReason {
			t.Fatalf("unexpected reasons: %v", m.Reasons)
		}
	}
	if calls := penalty.snapshot(); len(calls) != 3 {
		t.Fatalf("expected 3 penalties, got %d", len(calls))
	}
}

func TestSharedIPScan_SkipsExemptUsers(t *testing.T) {
	lister := &fakeSharedIPLister{ids: []int64{1, 2, 3}}
	writer := &fakeSharedIPMarkWriter{}
	penalty := &fakeSharedIPPenalty{}
	exempt := &fakeExemptionCheck{exempt: map[int64]bool{2: true}} // 用户 2 提过工单/复核豁免
	svc := newScanServiceForTest(lister, writer, penalty, exempt)

	svc.runOnce()

	marks := writer.snapshot()
	if len(marks) != 2 {
		t.Fatalf("expected 2 marks (user 2 skipped), got %d", len(marks))
	}
	for _, m := range marks {
		if m.UserID == 2 {
			t.Fatal("exempt user 2 must not be marked")
		}
	}
	for _, id := range penalty.snapshot() {
		if id == 2 {
			t.Fatal("exempt user 2 must not be penalized")
		}
	}
}

func TestSharedIPScan_NoCandidatesIsNoop(t *testing.T) {
	lister := &fakeSharedIPLister{ids: nil}
	writer := &fakeSharedIPMarkWriter{}
	penalty := &fakeSharedIPPenalty{}
	svc := newScanServiceForTest(lister, writer, penalty, &fakeExemptionCheck{})

	svc.runOnce()

	if len(writer.snapshot()) != 0 || len(penalty.snapshot()) != 0 {
		t.Fatal("no candidates must produce no marks and no penalties")
	}
}

func TestSharedIPScan_ListFailureIsNoop(t *testing.T) {
	lister := &fakeSharedIPLister{err: context.DeadlineExceeded}
	writer := &fakeSharedIPMarkWriter{}
	penalty := &fakeSharedIPPenalty{}
	svc := newScanServiceForTest(lister, writer, penalty, &fakeExemptionCheck{})

	svc.runOnce() // 不应 panic，也不写任何东西

	if len(writer.snapshot()) != 0 || len(penalty.snapshot()) != 0 {
		t.Fatal("list failure must not mark or penalize anyone")
	}
}
