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

// TestClearUserRiskReviewSQL_RecordsScanExemption 覆盖「放行即登记全量扫描豁免」。
// 清标记与写豁免必须在同一条 CTE 语句里：拆成两次写会出现用户已恢复正常、
// 但豁免没落库，下一次扫描又把账号标回待复核。
func TestClearUserRiskReviewSQL_RecordsScanExemption(t *testing.T) {
	sql := clearUserRiskReviewSQL

	for _, want := range []string{
		"WITH cleared AS (",
		"RETURNING id",
		"INSERT INTO user_risk_review_exemptions (user_id, reviewed_at)",
		"SELECT id, NOW() FROM cleared",
		"ON CONFLICT (user_id) DO UPDATE SET reviewed_at = NOW()",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("clear SQL missing scan-exemption clause %q", want)
		}
	}
	// 只对本次实际放行的用户写豁免，绝不能按条件批量给所有用户加豁免。
	if !strings.Contains(sql, "WHERE id = $1") {
		t.Fatal("exemption must be scoped to the cleared user id")
	}
}

// TestListUserIDsForRiskScanSQL_SkipsReviewedUsers 覆盖「已人工放行的账号不再被全量扫描标回」。
func TestListUserIDsForRiskScanSQL_SkipsReviewedUsers(t *testing.T) {
	sql := listUserIDsForRiskScanSQL

	for _, want := range []string{
		"user_risk_review_exemptions",
		"NOT EXISTS",
		"e.user_id = u.id",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("risk scan SQL missing exemption clause %q", want)
		}
	}
	// 分批游标、软删除过滤、LIMIT 一个都不能被豁免条件挤掉。
	for _, want := range []string{
		"u.deleted_at IS NULL",
		"u.id > $1",
		"ORDER BY u.id",
		"LIMIT $2",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("risk scan SQL missing batch contract %q", want)
		}
	}
	// 扫描只读取数据，绝不在这里写标记。
	if strings.Contains(sql, "UPDATE ") || strings.Contains(sql, "INSERT ") {
		t.Fatal("risk scan SQL must stay read-only")
	}
}
