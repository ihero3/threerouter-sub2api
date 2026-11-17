package repository

import (
	"strings"
	"testing"
)

// 这些测试不依赖真实数据库：直接对 SQL 字符串做契约断言，锁住安全约束。

func TestSharedIPRiskUserIDsSQL_Contracts(t *testing.T) {
	sql := sharedIPRiskUserIDsSQL
	for _, want := range []string{
		"SELECT DISTINCT l.user_id",
		"HAVING COUNT(DISTINCT user_id) >= $1",
		"created_at >= $2",
		"u.deleted_at IS NULL",
		"ip_address <> ''",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("shared IP scan SQL missing %q", want)
		}
	}
	// 阈值必须是参数而非硬编码，便于运营调整。
	if !strings.Contains(sql, ">= $1") {
		t.Fatal("min-users threshold must be parameterized ($1)")
	}
	// 必须是只读查询：绝不在这里写标记或扣钱。
	upper := strings.ToUpper(sql)
	if strings.Contains(upper, "UPDATE ") || strings.Contains(upper, "INSERT ") || strings.Contains(upper, "DELETE ") {
		t.Fatal("shared IP scan SQL must stay read-only")
	}
}

func TestListUserIDsBySharedIP_NoExecutorReturnsError(t *testing.T) {
	// 无 sql 执行器时返回错误而不是 panic。
	r := &usageLogRepository{}
	if _, err := r.ListUserIDsBySharedIP(t.Context(), 0, 0); err == nil {
		t.Fatal("expected error when sql executor is not configured")
	}
}
