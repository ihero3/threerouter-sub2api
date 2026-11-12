package repository

import (
	"strings"
	"testing"
)

// 这些测试不依赖真实数据库：SQL 语句本身是权威，这里把它抽成常量后
// 直接对字符串做契约断言，锁住安全约束，防止后续改动误删关键条件。

// TestBumpUserRiskMarksSQL_AdminNeverDisabled 覆盖「admin 账号不被批量 SQL 停用」。
func TestBumpUserRiskMarksSQL_AdminNeverDisabled(t *testing.T) {
	sql := bumpUserRiskMarksSQL

	// abnormal 才停用，且仅限非 admin。
	if !strings.Contains(sql, "WHEN v.level = 'abnormal' AND COALESCE(u.role, '') <> 'admin' THEN 'disabled'") {
		t.Fatal("SQL must disable only abnormal non-admin users")
	}
	// suspect 不动 status：CASE 的 ELSE 必须保留原值。
	if !strings.Contains(sql, "ELSE u.status") {
		t.Fatal("SQL must preserve status for non-abnormal marks")
	}
	// admin 排除必须同时出现在 status CASE 与 WHERE（整体不参与打标）两处，双保险。
	if strings.Count(sql, "COALESCE(u.role, '') <> 'admin'") < 2 {
		t.Fatal("SQL must exclude admin both in the status CASE and in WHERE")
	}
	// 只升不降：abnormal 不能被后续 suspect 扫描抹掉。
	if !strings.Contains(sql, ">= CASE COALESCE(u.risk_level, '')") {
		t.Fatal("SQL must keep the monotonic (only-raise) rank guard")
	}
	// status 的写入必须走 CASE，不能是无条件覆盖。
	if !strings.Contains(sql, "status = CASE") {
		t.Fatal("SQL must set status via a CASE expression")
	}
}

// TestClearUserRiskReviewSQL_ClearsMarksAndReactivates 覆盖「复核放行清标记 + 恢复 active」的 SQL 契约。
func TestClearUserRiskReviewSQL_ClearsMarksAndReactivates(t *testing.T) {
	sql := clearUserRiskReviewSQL

	for _, want := range []string{
		"risk_level = ''",
		"risk_reasons = ''",
		"risk_updated_at = NULL",
		"status = 'active'",
		"deleted_at IS NULL",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("clear SQL missing %q", want)
		}
	}
	if !strings.Contains(sql, "WHERE id = $1") {
		t.Fatal("clear SQL must target a single user id")
	}
	// 放行 SQL 绝不能触碰其它用户或把所有用户复活。
	if strings.Contains(sql, "UPDATE users u") {
		t.Fatal("clear SQL must not be a bulk multi-table update")
	}
}
