//go:build unit

package repository

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

func TestScanUserDashboardUsageWindowCombinesTotals(t *testing.T) {
	db, mock := newSQLMock(t)
	today := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
	stats := &usagestats.UserDashboardStats{}

	mock.ExpectQuery(regexp.QuoteMeta("COUNT(*) FILTER (WHERE created_at >= $2)")).
		WithArgs(int64(42), today).
		WillReturnRows(sqlmock.NewRows([]string{
			"total_requests", "total_input_tokens", "total_output_tokens",
			"total_cache_creation_tokens", "total_cache_read_tokens", "total_cost",
			"total_actual_cost", "avg_duration_ms", "today_requests",
			"today_input_tokens", "today_output_tokens", "today_cache_creation_tokens",
			"today_cache_read_tokens", "today_cost", "today_actual_cost",
		}).AddRow(5, 10, 20, 3, 7, 1.5, 1.2, 50.0, 2, 5, 8, 1, 2, 0.5, 0.4))

	require.NoError(t, scanUserDashboardUsageWindow(t.Context(), db, "user_id", 42, today, stats))
	require.Equal(t, int64(5), stats.TotalRequests)
	require.Equal(t, int64(2), stats.TodayRequests)
	require.Equal(t, 1.2, stats.TotalActualCost)
	require.Equal(t, 0.4, stats.TodayActualCost)
	require.NoError(t, mock.ExpectationsWereMet())
}
