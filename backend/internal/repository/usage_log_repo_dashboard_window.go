package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

const userDashboardUsageWindowQuery = `
	SELECT
		COUNT(*) AS total_requests,
		COALESCE(SUM(input_tokens), 0) AS total_input_tokens,
		COALESCE(SUM(output_tokens), 0) AS total_output_tokens,
		COALESCE(SUM(cache_creation_tokens), 0) AS total_cache_creation_tokens,
		COALESCE(SUM(cache_read_tokens), 0) AS total_cache_read_tokens,
		COALESCE(SUM(total_cost), 0) AS total_cost,
		COALESCE(SUM(actual_cost), 0) AS total_actual_cost,
		COALESCE(AVG(duration_ms), 0) AS avg_duration_ms,
		COUNT(*) FILTER (WHERE created_at >= $2) AS today_requests,
		COALESCE(SUM(input_tokens) FILTER (WHERE created_at >= $2), 0) AS today_input_tokens,
		COALESCE(SUM(output_tokens) FILTER (WHERE created_at >= $2), 0) AS today_output_tokens,
		COALESCE(SUM(cache_creation_tokens) FILTER (WHERE created_at >= $2), 0) AS today_cache_creation_tokens,
		COALESCE(SUM(cache_read_tokens) FILTER (WHERE created_at >= $2), 0) AS today_cache_read_tokens,
		COALESCE(SUM(total_cost) FILTER (WHERE created_at >= $2), 0) AS today_cost,
		COALESCE(SUM(actual_cost) FILTER (WHERE created_at >= $2), 0) AS today_actual_cost
	FROM usage_logs
	WHERE %s = $1
`

// scanUserDashboardUsageWindow aggregates total and today token/cost fields in a
// single scan. column is always an internal whitelist: user_id or api_key_id.
func scanUserDashboardUsageWindow(
	ctx context.Context,
	sqlq sqlExecutor,
	column string,
	scopeID int64,
	today time.Time,
	stats *usagestats.UserDashboardStats,
) error {
	return scanSingleRow(
		ctx,
		sqlq,
		fmt.Sprintf(userDashboardUsageWindowQuery, column),
		[]any{scopeID, today},
		&stats.TotalRequests,
		&stats.TotalInputTokens,
		&stats.TotalOutputTokens,
		&stats.TotalCacheCreationTokens,
		&stats.TotalCacheReadTokens,
		&stats.TotalCost,
		&stats.TotalActualCost,
		&stats.AverageDurationMs,
		&stats.TodayRequests,
		&stats.TodayInputTokens,
		&stats.TodayOutputTokens,
		&stats.TodayCacheCreationTokens,
		&stats.TodayCacheReadTokens,
		&stats.TodayCost,
		&stats.TodayActualCost,
	)
}
