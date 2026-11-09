package admin

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func newIPRankingContext(t *testing.T, query string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/v1/admin/usage/ip-ranking"+query, nil)
	return c
}

// 默认窗口：不给日期时按最近 24 小时统计，避免一次请求扫全表。
func TestParseIPRankingWindow_DefaultWindow(t *testing.T) {
	before := time.Now()
	start, end, err := parseIPRankingWindow(newIPRankingContext(t, ""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := end.Sub(start); got != ipRankingDefaultWindow {
		t.Fatalf("window = %v, want %v", got, ipRankingDefaultWindow)
	}
	if end.Before(before) {
		t.Fatalf("end = %v, want not before %v", end, before)
	}
}

// 小于上限的范围按调用方设定执行：[start, end) 半开区间，end_date 顺延到次日 0 点。
// 显式指定 UTC，避免测试机时区带 DST 时算出差 1 小时。
func TestParseIPRankingWindow_KeepsShortRange(t *testing.T) {
	start, end, err := parseIPRankingWindow(newIPRankingContext(t, "?start_date=2026-10-20&end_date=2026-10-22&timezone=UTC"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := end.Sub(start); got != 72*time.Hour {
		t.Fatalf("window = %v, want 72h", got)
	}
	if end.Hour() != 0 || end.Minute() != 0 || end.Second() != 0 {
		t.Fatalf("end = %v, want midnight", end)
	}
}

// 超过 7 天的范围必须被收紧到 7 天：这是防止聚合查询拖垮数据库的硬闸门。
func TestParseIPRankingWindow_ClampsLongRange(t *testing.T) {
	start, end, err := parseIPRankingWindow(newIPRankingContext(t, "?start_date=2026-09-01&end_date=2026-10-26"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := end.Sub(start); got != ipRankingMaxWindow {
		t.Fatalf("window = %v, want %v", got, ipRankingMaxWindow)
	}
}

// 反向范围直接 400，不做“帮你调换”的猜测。
func TestParseIPRankingWindow_RejectsReversedRange(t *testing.T) {
	if _, _, err := parseIPRankingWindow(newIPRankingContext(t, "?start_date=2026-10-26&end_date=2026-10-20")); err == nil {
		t.Fatal("expected error for reversed range")
	}
}

// 非法格式必须报错，而不是静默回落到默认窗口。
func TestParseIPRankingWindow_RejectsInvalidFormat(t *testing.T) {
	if _, _, err := parseIPRankingWindow(newIPRankingContext(t, "?start_date=2026/10/20")); err == nil {
		t.Fatal("expected error for invalid start_date")
	}
}
