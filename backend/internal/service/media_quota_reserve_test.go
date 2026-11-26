package service

import (
	"context"
	"errors"
	"testing"
)

// quotaFailRepo embeds the full interface and overrides only the increment
// method, letting us drive UpdateQuotaUsed success/failure without implementing
// every repository method.
type quotaFailRepo struct {
	APIKeyRepository
	err error
}

func (r *quotaFailRepo) IncrementQuotaUsed(context.Context, int64, float64) (float64, error) {
	if r.err != nil {
		return 0, r.err
	}
	return 1, nil
}

func (r *quotaFailRepo) GetByID(context.Context, int64) (*APIKey, error) {
	// Quota=0 means "no quota configured", so UpdateQuotaUsed returns nil without
	// needing cache or status updates.
	return &APIKey{ID: 42, Quota: 0}, nil
}

func newReserveTestKeyService(repo APIKeyRepository) *APIKeyService {
	return &APIKeyService{apiKeyRepo: repo}
}

// TestReserveAndTrackQuotaFailureNotRecorded is the regression guard for the
// double-accounting bug: a failed pre-deduction must not yield a ReservedCost
// pointer, otherwise a later refund (Create failure) or settlement diff returns
// quota that was never deducted.
func TestReserveAndTrackQuotaFailureNotRecorded(t *testing.T) {
	ctx := context.Background()
	const keyID = int64(42)
	const cost = 3.5

	t.Run("deduction succeeds -> cost recorded", func(t *testing.T) {
		svc := newReserveTestKeyService(&quotaFailRepo{})
		got, err := reserveAndTrackQuota(svc, ctx, keyID, cost)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil || *got != cost {
			t.Fatalf("expected recorded cost %v, got %#v", cost, got)
		}
	})

	t.Run("deduction fails -> nothing recorded", func(t *testing.T) {
		svc := newReserveTestKeyService(&quotaFailRepo{err: errors.New("deduction failed")})
		got, err := reserveAndTrackQuota(svc, ctx, keyID, cost)
		if err == nil {
			t.Fatal("expected deduction error")
		}
		if got != nil {
			t.Fatalf("expected nil ReservedCost on failed deduction, got %#v", *got)
		}
	})

	t.Run("nil service -> nothing recorded", func(t *testing.T) {
		got, err := reserveAndTrackQuota(nil, ctx, keyID, cost)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Fatalf("expected nil ReservedCost with no billing service, got %#v", *got)
		}
	})

	t.Run("non-positive cost -> nothing recorded", func(t *testing.T) {
		svc := newReserveTestKeyService(&quotaFailRepo{})
		got, err := reserveAndTrackQuota(svc, ctx, keyID, 0)
		if err != nil || got != nil {
			t.Fatalf("expected (nil, nil) for zero cost, got (%#v, %v)", got, err)
		}
	})
}
