//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func TestGrantRechargePerkOnFulfillmentUsesLifetimeOnceSemantics(t *testing.T) {
	ctx := context.Background()

	var calls int
	var gotConcurrency, gotRPM int
	grantedOnce := false
	repo := &mockUserRepo{
		grantRechargePerkFn: func(_ context.Context, userID, sourceOrderID int64, concurrencyBonus, rpmBonus int) (bool, error) {
			calls++
			require.EqualValues(t, 42, userID)
			gotConcurrency, gotRPM = concurrencyBonus, rpmBonus
			if grantedOnce {
				return false, nil
			}
			grantedOnce = true
			return true, nil
		},
	}
	svc := &PaymentService{userRepo: repo}
	order := &dbent.PaymentOrder{ID: 1001, UserID: 42}

	svc.grantRechargePerkOnFulfillment(ctx, order)
	svc.grantRechargePerkOnFulfillment(ctx, order)

	require.Equal(t, 2, calls, "每笔履约都会尝试，但幂等由仓储层保证")
	require.Equal(t, rechargePerkConcurrencyBonus, gotConcurrency)
	require.Equal(t, 3, gotConcurrency)
	require.Equal(t, rechargePerkRPMBonus, gotRPM)
	require.Equal(t, 30, gotRPM)
}

func TestGrantRechargePerkOnFulfillmentSwallowsErrors(t *testing.T) {
	repo := &mockUserRepo{
		grantRechargePerkFn: func(context.Context, int64, int64, int, int) (bool, error) {
			return false, errors.New("db down")
		},
	}
	svc := &PaymentService{userRepo: repo}

	// 发放失败不得让充值履约失败。
	require.NotPanics(t, func() {
		svc.grantRechargePerkOnFulfillment(context.Background(), &dbent.PaymentOrder{ID: 1, UserID: 2})
	})
}

func TestGrantRechargePerkOnFulfillmentSkipsMissingOrder(t *testing.T) {
	repo := &mockUserRepo{
		grantRechargePerkFn: func(context.Context, int64, int64, int, int) (bool, error) {
			t.Fatal("should not be called")
			return false, nil
		},
	}
	svc := &PaymentService{userRepo: repo}

	svc.grantRechargePerkOnFulfillment(context.Background(), nil)
	svc.grantRechargePerkOnFulfillment(context.Background(), &dbent.PaymentOrder{ID: 1, UserID: 0})
}
