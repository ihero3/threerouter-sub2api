package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeExemptionRepo struct {
	result map[int64]bool
	err    error
	calls  int
}

func (f *fakeExemptionRepo) IsRiskExemptByID(_ context.Context, id int64) (bool, error) {
	f.calls++
	if f.err != nil {
		return false, f.err
	}
	return f.result[id], nil
}

func TestRiskExemptionChecker_CachesWithinTTL(t *testing.T) {
	repo := &fakeExemptionRepo{result: map[int64]bool{7: true}}
	checker := NewRiskExemptionChecker(repo, time.Minute)

	if !checker.IsExempt(context.Background(), 7) {
		t.Fatal("expected exempt")
	}
	if !checker.IsExempt(context.Background(), 7) {
		t.Fatal("expected exempt on cache hit")
	}
	if repo.calls != 1 {
		t.Fatalf("expected exactly one DB call within TTL, got %d", repo.calls)
	}
}

func TestRiskExemptionChecker_InvalidateForcesReload(t *testing.T) {
	repo := &fakeExemptionRepo{result: map[int64]bool{7: false}}
	checker := NewRiskExemptionChecker(repo, time.Minute)

	if checker.IsExempt(context.Background(), 7) {
		t.Fatal("expected not exempt initially")
	}
	repo.result[7] = true // 管理端刚设了工单标记
	checker.Invalidate(7)
	if !checker.IsExempt(context.Background(), 7) {
		t.Fatal("expected exempt after invalidate + reload")
	}
	if repo.calls != 2 {
		t.Fatalf("expected two DB calls around invalidate, got %d", repo.calls)
	}
}

func TestRiskExemptionChecker_DBErrorFailsOpenToNotExempt(t *testing.T) {
	repo := &fakeExemptionRepo{err: errors.New("db down")}
	checker := NewRiskExemptionChecker(repo, time.Minute)

	// DB 异常时视作「不豁免」，回到原有处罚逻辑，而不是让风控整体短路。
	if checker.IsExempt(context.Background(), 7) {
		t.Fatal("DB error must fail-open to not exempt")
	}
}

func TestRiskExemptionChecker_ExpiresAfterTTL(t *testing.T) {
	repo := &fakeExemptionRepo{result: map[int64]bool{7: true}}
	checker := NewRiskExemptionChecker(repo, 10*time.Millisecond)

	if !checker.IsExempt(context.Background(), 7) {
		t.Fatal("expected exempt")
	}
	time.Sleep(20 * time.Millisecond)
	if !checker.IsExempt(context.Background(), 7) {
		t.Fatal("expected exempt after TTL reload")
	}
	if repo.calls != 2 {
		t.Fatalf("expected a reload after TTL, got %d calls", repo.calls)
	}
}
