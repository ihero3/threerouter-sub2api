package service

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestApplyInviterRechargeGate 覆盖「邀请人自己也充值」两个开关的组合语义：
//   - 两个开关都关闭：历史行为，返利原样保留；
//   - requireRecharge：邀请人零实充时返 0，有实充时不受影响；
//   - capByRecharge：邀请人累计充值返利不超过其累计实充，超出部分截断；
//   - 两个开关同时开启：先卡门槛再卡总额。
func TestApplyInviterRechargeGate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		rebate          float64
		inviterRecharge float64
		alreadyAccrued  float64
		requireRecharge bool
		capByRecharge   bool
		want            float64
	}{
		{
			name:            "both switches off keeps historical rebate",
			rebate:          30,
			inviterRecharge: 0,
			want:            30,
		},
		{
			name:            "require on and inviter never recharged yields nothing",
			rebate:          30,
			inviterRecharge: 0,
			requireRecharge: true,
			want:            0,
		},
		{
			name:            "require on and inviter recharged keeps rebate",
			rebate:          3,
			inviterRecharge: 10,
			requireRecharge: true,
			want:            3,
		},
		{
			name:            "cap trims rebate to inviter total recharge",
			rebate:          30,
			inviterRecharge: 10,
			capByRecharge:   true,
			want:            10,
		},
		{
			name:            "cap trims to remaining headroom after previous accruals",
			rebate:          30,
			inviterRecharge: 10,
			alreadyAccrued:  4,
			capByRecharge:   true,
			want:            6,
		},
		{
			name:            "cap exhausted yields nothing",
			rebate:          5,
			inviterRecharge: 10,
			alreadyAccrued:  10,
			capByRecharge:   true,
			want:            0,
		},
		{
			name:            "cap on with no inviter recharge yields nothing",
			rebate:          5,
			inviterRecharge: 0,
			capByRecharge:   true,
			want:            0,
		},
		{
			name:            "cap leaves smaller rebate untouched",
			rebate:          3,
			inviterRecharge: 100,
			capByRecharge:   true,
			want:            3,
		},
		{
			name:            "both switches on: require passes then cap applies",
			rebate:          30,
			inviterRecharge: 10,
			requireRecharge: true,
			capByRecharge:   true,
			want:            10,
		},
		{
			name:            "both switches on and inviter never recharged",
			rebate:          3,
			inviterRecharge: 0,
			requireRecharge: true,
			capByRecharge:   true,
			want:            0,
		},
		{
			name:            "zero rebate stays zero",
			rebate:          0,
			inviterRecharge: 100,
			want:            0,
		},
		{
			name:            "NaN rebate is rejected",
			rebate:          math.NaN(),
			inviterRecharge: 100,
			want:            0,
		},
		{
			name:            "Inf rebate is rejected",
			rebate:          math.Inf(1),
			inviterRecharge: 100,
			want:            0,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := applyInviterRechargeGate(tc.rebate, tc.inviterRecharge, tc.alreadyAccrued, tc.requireRecharge, tc.capByRecharge)
			require.InDelta(t, tc.want, got, 1e-9)
		})
	}
}
