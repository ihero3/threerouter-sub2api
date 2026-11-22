package service

import (
	"context"
	"sync"
	"testing"
	"time"
)

// ---------------- 共享 IP 风险扫描单测（不依赖 DB / Redis） ----------------

// listCall 记录一次 ListUserIDsBySharedIP 的查询参数，用于断言双档口径。
type listCall struct {
	minUsers int
	window   time.Duration
}

// listResult 是 fake 按调用顺序吐出的结果（runOnce 固定先快速档后长期档）。
type listResult struct {
	ids []int64
	err error
}

type fakeSharedIPLister struct {
	queue []listResult
	calls []listCall
}

func (f *fakeSharedIPLister) ListUserIDsBySharedIP(_ context.Context, minUsers int, window time.Duration) ([]int64, error) {
	f.calls = append(f.calls, listCall{minUsers: minUsers, window: window})
	if len(f.queue) == 0 {
		return nil, nil
	}
	r := f.queue[0]
	f.queue = f.queue[1:]
	return r.ids, r.err
}

// TestSharedIPScan_DoubleTierQueryParams 锁死双档口径：先查 1 天 ≥3（快速档 suspect），
// 再查 10 天 ≥5（长期档 abnormal 自动禁用）。改口径必须有意为之并同步本测试。
func TestSharedIPScan_DoubleTierQueryParams(t *testing.T) {
	lister := &fakeSharedIPLister{queue: []listResult{{}, {}}}
	writer := &fakeSharedIPMarkWriter{}
	penalty := &fakeSharedIPPenalty{}
	svc := newScanServiceForTest(lister, writer, penalty, &fakeExemptionCheck{})

	svc.runOnce()

	if len(lister.calls) != 2 {
		t.Fatalf("expected 2 tier queries, got %d", len(lister.calls))
	}
	if lister.calls[0].minUsers != sharedIPRiskShortMinUsers || lister.calls[0].window != sharedIPRiskShortWindow {
		t.Fatalf("short tier = %d/%s, want %d/%s",
			lister.calls[0].minUsers, lister.calls[0].window, sharedIPRiskShortMinUsers, sharedIPRiskShortWindow)
	}
	if lister.calls[1].minUsers != sharedIPRiskMinUsers || lister.calls[1].window != sharedIPRiskWindow {
		t.Fatalf("long tier = %d/%s, want %d/%s",
			lister.calls[1].minUsers, lister.calls[1].window, sharedIPRiskMinUsers, sharedIPRiskWindow)
	}
}

// TestSharedIPScan_TierConstants 锁死双档常量（用户拍板 2026-11-20）。
func TestSharedIPScan_TierConstants(t *testing.T) {
	if sharedIPRiskShortWindow != 24*time.Hour {
		t.Fatalf("short window = %s, want 24h", sharedIPRiskShortWindow)
	}
	if sharedIPRiskShortMinUsers != 3 {
		t.Fatalf("short minUsers = %d, want 3", sharedIPRiskShortMinUsers)
	}
	if sharedIPRiskWindow != 10*24*time.Hour {
		t.Fatalf("long window = %s, want 10 days", sharedIPRiskWindow)
	}
	if sharedIPRiskMinUsers != 5 {
		t.Fatalf("long minUsers = %d, want 5", sharedIPRiskMinUsers)
	}
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

// TestSharedIPScan_ShortTierMarksSuspect 快速档命中 → suspect + 扣费。
func TestSharedIPScan_ShortTierMarksSuspect(t *testing.T) {
	lister := &fakeSharedIPLister{queue: []listResult{
		{ids: []int64{1, 2, 3}}, // 快速档命中
		{},                      // 长期档无命中
	}}
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

// TestSharedIPScan_LongTierMarksSuspect 长期档命中 → suspect（2026-11-21 起不再 abnormal 自动禁用）+ 扣费。
func TestSharedIPScan_LongTierMarksSuspect(t *testing.T) {
	lister := &fakeSharedIPLister{queue: []listResult{
		{},                   // 快速档无命中
		{ids: []int64{7, 8}}, // 长期档命中
	}}
	writer := &fakeSharedIPMarkWriter{}
	penalty := &fakeSharedIPPenalty{}
	svc := newScanServiceForTest(lister, writer, penalty, &fakeExemptionCheck{})

	svc.runOnce()

	marks := writer.snapshot()
	if len(marks) != 2 {
		t.Fatalf("expected 2 marks, got %d", len(marks))
	}
	for _, m := range marks {
		if m.Level != AffiliateRiskLevelSuspect {
			t.Fatalf("long tier must mark suspect, got %q", m.Level)
		}
		if len(m.Reasons) != 1 || m.Reasons[0] != sharedIPRiskReason {
			t.Fatalf("unexpected reasons: %v", m.Reasons)
		}
	}
	if calls := penalty.snapshot(); len(calls) != 2 {
		t.Fatalf("expected 2 penalties, got %d", len(calls))
	}
}

func TestSharedIPScan_SkipsExemptUsers(t *testing.T) {
	lister := &fakeSharedIPLister{queue: []listResult{
		{ids: []int64{1, 2, 3}}, // 快速档命中，用户 2 豁免
		{},                      // 长期档无命中
	}}
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
	lister := &fakeSharedIPLister{}
	writer := &fakeSharedIPMarkWriter{}
	penalty := &fakeSharedIPPenalty{}
	svc := newScanServiceForTest(lister, writer, penalty, &fakeExemptionCheck{})

	svc.runOnce()

	if len(writer.snapshot()) != 0 || len(penalty.snapshot()) != 0 {
		t.Fatal("no candidates must produce no marks and no penalties")
	}
}

func TestSharedIPScan_ListFailureIsNoop(t *testing.T) {
	lister := &fakeSharedIPLister{queue: []listResult{
		{err: context.DeadlineExceeded}, // 快速档查询失败
		{},                              // 长期档仍执行
	}}
	writer := &fakeSharedIPMarkWriter{}
	penalty := &fakeSharedIPPenalty{}
	svc := newScanServiceForTest(lister, writer, penalty, &fakeExemptionCheck{})

	svc.runOnce() // 不应 panic

	if len(writer.snapshot()) != 0 || len(penalty.snapshot()) != 0 {
		t.Fatal("list failure must not mark or penalize anyone")
	}
}
