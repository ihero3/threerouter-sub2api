package service

import "testing"

// 返利明细是按 user_affiliate_ledger 的 action 分类型的：
// register_reward → 邀请返利，accrue → 充值返利，transfer 是提现不属于返利。
// 这里锁住映射本身，以及「清单 vs 映射表 vs 类型集合」三者数量一致，
// 避免以后新增 action 只改了一半（例如只写 SQL 忘了映射，或反过来）。
func TestAffiliateLedgerRebateTypeMapping(t *testing.T) {
	cases := map[string]string{
		AffiliateLedgerActionRegisterReward: AffiliateRebateTypeInvite,
		AffiliateLedgerActionAccrue:         AffiliateRebateTypeRecharge,
	}
	for action, want := range cases {
		if got := AffiliateLedgerRebateType(action); got != want {
			t.Errorf("AffiliateLedgerRebateType(%q) = %q, want %q", action, got, want)
		}
	}
}

func TestAffiliateLedgerRebateTypeNonRebateActions(t *testing.T) {
	// 非返利动作必须返回空串：明细查询用它来跳过不该展示的记录
	// （transfer 是额度转余额，source_user_id 为 NULL 且不是收益）。
	for _, action := range []string{AffiliateLedgerActionTransfer, "", "UNKNOWN_ACTION"} {
		if got := AffiliateLedgerRebateType(action); got != "" {
			t.Errorf("AffiliateLedgerRebateType(%q) = %q, want empty string", action, got)
		}
	}
}

func TestAffiliateRebateLedgerActionsStayConsistent(t *testing.T) {
	if len(AffiliateRebateLedgerActions) != len(affiliateLedgerActionRebateTypes) {
		t.Fatalf("AffiliateRebateLedgerActions=%d actions but mapping has=%d entries; they must cover the same actions",
			len(AffiliateRebateLedgerActions), len(affiliateLedgerActionRebateTypes))
	}

	seen := map[string]int{}
	for _, action := range AffiliateRebateLedgerActions {
		rebateType := AffiliateLedgerRebateType(action)
		if rebateType == "" {
			t.Fatalf("action %q is listed in AffiliateRebateLedgerActions but has no rebate type mapping", action)
		}
		seen[rebateType]++
	}
	for _, required := range []string{AffiliateRebateTypeInvite, AffiliateRebateTypeRecharge} {
		if seen[required] == 0 {
			t.Errorf("rebate type %q is not reachable from AffiliateRebateLedgerActions", required)
		}
	}
}
