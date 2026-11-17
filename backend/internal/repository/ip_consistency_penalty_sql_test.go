package repository

import (
	"strings"
	"testing"
)

// IP 一致性「3 个 IP 轻标记」的 SQL 契约。
// 绝不能写回 risk_level=suspect：那会触发待复核冻结闸门（禁建 Key + 网关 403），
// 比「≥4 个 IP 才禁用」还早生效，梯度倒挂且误伤真实用户。
func TestBumpIPInconsistencyMarkSQL_UsesNonBlockingTag(t *testing.T) {
	if !strings.Contains(bumpIPInconsistencyMarkSQL, "'ip_suspect'") {
		t.Fatal("IP inconsistency must write the non-blocking ip_suspect tag")
	}
	if strings.Contains(bumpIPInconsistencyMarkSQL, "ELSE 'suspect'") {
		t.Fatal("IP inconsistency must never write risk_level=suspect")
	}
	if !strings.Contains(bumpIPInconsistencyMarkSQL, "IN ('abnormal', 'suspect')") {
		t.Fatal("existing frozen accounts (suspect/abnormal) must never be downgraded to a light tag")
	}
	if !strings.Contains(bumpIPInconsistencyMarkSQL, "deleted_at IS NULL") {
		t.Fatal("soft-deleted users must not be touched")
	}
}

// 「减额度」处罚的 SQL 契约：条件写在 WHERE 里，由数据库行锁保证幂等。
func TestReduceBalanceToFloorSQL_IsConditionalAndAtomic(t *testing.T) {
	sql := reduceBalanceToFloorSQL
	for _, want := range []string{
		"prev.balance > $1",                 // 只在余额高于 floor 时扣，绝不反向充值
		"RETURNING prev.balance, u.balance", // 供上层写调账流水
		"deleted_at IS NULL",
		"updated_at = NOW()",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("reduceBalanceToFloorSQL missing %q", want)
		}
	}
	if strings.Contains(sql, "GREATEST") {
		t.Fatal("must not use GREATEST: this is a set-to-floor operation, not a subtract")
	}
}
