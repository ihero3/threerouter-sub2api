package repository

import (
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// 翻页稳定性回归：ORDER BY 只按业务列排序时，等值排序键内部的行序由 Postgres 决定，
// 大表走并行计划（Gather Merge + Sort）后每次执行都可能不同，LIMIT/OFFSET 会出现
// 「同一行出现在两页、另一行永远取不到」。
//
// 实测（5000 行 created_at 完全相同，100 行/页取 50 页）：
//   不带 tiebreaker → 只取到 4977 个不同 id（丢 23 行）
//   带 ual.id DESC → 5000 个 id 完整
//
// 所以每处列表排序都必须带一个唯一的次级排序键。新增列表查询时如果漏传 tiebreaker，
// 这个测试会立刻变红。
func TestBuildAffiliateRecordOrderByRequiresUniqueTiebreaker(t *testing.T) {
	sortColumns := map[string]string{"created_at": "ual.created_at"}

	tiebreaker := "ual.id DESC"
	for _, tc := range []struct {
		name   string
		filter service.AffiliateRecordFilter
	}{
		{name: "default desc", filter: service.AffiliateRecordFilter{}},
		{name: "asc", filter: service.AffiliateRecordFilter{SortDesc: false}},
		{name: "unknown sort key falls back", filter: service.AffiliateRecordFilter{SortBy: "nope", SortDesc: true}},
	} {
		got := buildAffiliateRecordOrderBy(tc.filter, sortColumns, "ual.created_at", tiebreaker)
		if !strings.Contains(got, "NULLS LAST") {
			t.Fatalf("%s: missing NULLS LAST: %q", tc.name, got)
		}
		if !strings.HasSuffix(got, ", "+tiebreaker) {
			t.Fatalf("%s: ORDER BY must end with the unique tiebreaker %q, got %q", tc.name, tiebreaker, got)
		}
	}
}

// 每个调用点都必须真的传了 tiebreaker：空字符串会被静默忽略，退化成不稳定排序。
func TestBuildAffiliateRecordOrderByEmptyTiebreakerIsIgnored(t *testing.T) {
	// 注意：SortDesc 的零值是 false，所以默认方向是 ASC（沿用既有行为，不是本次改动引入的）。
	got := buildAffiliateRecordOrderBy(service.AffiliateRecordFilter{}, map[string]string{}, "ual.created_at", "   ")
	if got != "ORDER BY ual.created_at ASC NULLS LAST" {
		t.Fatalf("blank tiebreaker should be ignored, got %q", got)
	}
}
