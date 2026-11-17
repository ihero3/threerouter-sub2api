package repository

import (
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// 这些测试不依赖真实数据库：SQL 语句本身是权威，这里把它抽成常量后
// 直接对字符串做契约断言，锁住「只返还真正扣过的、且只返一次」的安全约束。

// TestRefundIPBalancePenaltiesSQL_OnlyPendingDeductions 覆盖「没扣过 / 已返还过都不写入」。
func TestRefundIPBalancePenaltiesSQL_OnlyPendingDeductions(t *testing.T) {
	sql := refundIPBalancePenaltiesSQL

	for _, want := range []string{
		"rc.used_by = $1",
		"rc.type = '" + service.AdjustmentTypeAdminBalance + "'",
		"rc.notes = '" + service.IPAbnormalityMessage + "'",
		"rc.value < 0",
		"NOT EXISTS",
		"FROM pending p",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("refund SQL missing pending-deduction clause %q", want)
		}
	}
	// 已返还过的流水靠固定 code 排除，前缀必须与生产常量一致。
	if !strings.Contains(sql, "'"+service.IPPenaltyRefundCodePrefix+"' || rc.id::text") {
		t.Fatal("refund SQL must exclude already-refunded deductions by their refund code")
	}
}

// TestRefundIPBalancePenaltiesSQL_IdempotentAndAdditive 覆盖幂等与「加法返还」。
func TestRefundIPBalancePenaltiesSQL_IdempotentAndAdditive(t *testing.T) {
	sql := refundIPBalancePenaltiesSQL

	for _, want := range []string{
		"ON CONFLICT (code) DO NOTHING",
		"'" + service.IPPenaltyRefundType + "'",
		"balance = balance + total.amount",
		"total.amount > 0",
		"users.id = $1 AND users.deleted_at IS NULL",
		"SELECT COALESCE((SELECT SUM(amount) FROM updated), 0)",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("refund SQL missing safety clause %q", want)
		}
	}
	// 返还绝不能把余额设回扣减前的旧值：那会抹掉之后的充值或消费。
	if strings.Contains(sql, "SET balance = $") || strings.Contains(sql, "SET balance = prev.") {
		t.Fatal("refund must add to the current balance, never restore a snapshot value")
	}
	// 没有待返还扣减时不允许凭空写流水或加钱。
	if !strings.Contains(sql, "FROM claimed") {
		t.Fatal("refund total must be derived only from successfully claimed rows")
	}
}
