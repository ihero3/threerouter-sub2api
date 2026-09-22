package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %T: %v", v, err)
	}
	return string(raw)
}

func assertContainsKeys(t *testing.T, payload string, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if !strings.Contains(payload, `"`+key+`"`) {
			t.Errorf("payload %s is missing key %q (frontend reads it)", payload, key)
		}
	}
}

// 用户端「已邀请用户」列表的线上契约：粒度是每笔返利一行。
// 前端按 ledger_id 做行 key、按 rebate_type 渲染类型、按 amount 渲染金额，
// 任何一项改名都会让页面静默显示空白而不是报错，所以在这里锁住。
func TestAffiliateInviteeJSONContract(t *testing.T) {
	now := time.Unix(0, 0).UTC()
	payload := mustMarshal(t, AffiliateInvitee{
		UserID:     7,
		Email:      "a@example.com",
		Username:   "alice",
		LedgerID:   42,
		RebateType: AffiliateRebateTypeRecharge,
		Amount:     1.25,
		CreatedAt:  now,
		JoinedAt:   now,
	})
	assertContainsKeys(t, payload, "user_id", "email", "username", "ledger_id", "rebate_type", "amount", "created_at", "joined_at")
	// 旧字段已在改为「每笔一行」时移除，若被加回来说明有人按旧语义改代码，需要重新确认口径。
	if strings.Contains(payload, `"total_rebate"`) {
		t.Errorf("AffiliateInvitee must not expose total_rebate any more: %s", payload)
	}
}

// 管理端返利记录：注册奖励没有订单，订单相关字段必须可省略，且必须带 rebate_type，
// 否则前端会把「无订单」误渲染成 #0 / $0.00。
func TestAffiliateRebateRecordJSONContract(t *testing.T) {
	inviteRebate := mustMarshal(t, AffiliateRebateRecord{
		OutTradeNo:   "",
		InviterID:    1,
		InviteeID:    2,
		RebateType:   AffiliateRebateTypeInvite,
		RebateAmount: 1,
		CreatedAt:    time.Unix(0, 0).UTC(),
	})
	assertContainsKeys(t, inviteRebate, "inviter_id", "invitee_id", "rebate_type", "rebate_amount")
	for _, omitted := range []string{"order_id", "order_amount", "pay_amount"} {
		if strings.Contains(inviteRebate, `"`+omitted+`"`) {
			t.Errorf("nil %s must be omitted for invite rebates: %s", omitted, inviteRebate)
		}
	}

	orderID := int64(99)
	orderAmount := 10.0
	recharge := mustMarshal(t, AffiliateRebateRecord{
		OrderID:      &orderID,
		OrderAmount:  &orderAmount,
		RebateType:   AffiliateRebateTypeRecharge,
		RebateAmount: 2,
		CreatedAt:    time.Unix(0, 0).UTC(),
	})
	assertContainsKeys(t, recharge, "order_id", "order_amount", "rebate_type")
}

// 管理端邀请关系：空切片必须序列化成 [] 而不是 null，
// 否则前端 `relation.ancestors.length` 会抛异常、整页白屏。
func TestAffiliateInviteRelationJSONContract(t *testing.T) {
	empty := mustMarshal(t, AffiliateInviteRelation{
		User:            AffiliateRelationUser{UserID: 1},
		Ancestors:       []AffiliateRelationNode{},
		Descendants:     []AffiliateRelationNode{},
		DescendantCount: 0,
	})
	assertContainsKeys(t, empty, "user", "ancestors", "descendants", "descendant_count", "chain_truncated")
	if strings.Contains(empty, `"ancestors":null`) || strings.Contains(empty, `"descendants":null`) {
		t.Errorf("nil semantics leaked to JSON: %s", empty)
	}
	// inviter 为 nil 时必须整个省略，前端用它的有无来判断「来路不明」。
	if strings.Contains(empty, `"inviter"`) {
		t.Errorf("nil inviter must be omitted: %s", empty)
	}

	full := mustMarshal(t, AffiliateInviteRelation{
		User:      AffiliateRelationUser{UserID: 3, Email: "c@example.com"},
		Inviter:   &AffiliateRelationUser{UserID: 2},
		Ancestors: []AffiliateRelationNode{{UserID: 2, Depth: 1}},
		Descendants: []AffiliateRelationNode{
			{UserID: 4, Depth: 1, RebateAmount: 1.5},
			{UserID: 5, Depth: 2, RebateAmount: 0},
		},
		DescendantCount: 2,
	})
	assertContainsKeys(t, full, "inviter", "email", "depth", "rebate_amount")
}

// 无来源账号列表：前端展示余额/累计充值/档案状态，字段名同样需要锁住。
func TestAffiliateUnsourcedUserJSONContract(t *testing.T) {
	payload := mustMarshal(t, AffiliateUnsourcedUser{
		UserID:              9,
		Email:               "b@example.com",
		Username:            "bob",
		CreatedAt:           time.Unix(0, 0).UTC(),
		Balance:             1,
		TotalRecharged:      2,
		HasAffiliateProfile: false,
	})
	assertContainsKeys(t, payload, "user_id", "email", "username", "created_at", "balance", "total_recharged", "has_affiliate_profile")
}
