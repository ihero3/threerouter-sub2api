package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ListIPAccountRanking 统计时间窗内每个 IP 上的账号数，按账号数倒序。
//
// 口径：只按 usage_logs 里真实产生过调用的记录聚合，COUNT(DISTINCT user_id)
// 即「用过该 IP 的账号数」。这里刻意不 join users：一是省掉 7 天窗口上的 join
// 成本，二是「按 IP 反查用户」页以 users 为驱动表，硬删除用户的存量日志在本页
// 仍会计数，两页在极小概率下可能出现 1 的差异，属于已知且可接受的取舍。
//
// 时间参数为半开区间 [$1, $2)，调用方负责把 end 落到「次日 0 点」。
func (r *usageLogRepository) ListIPAccountRanking(ctx context.Context, startTime, endTime time.Time, params pagination.PaginationParams) ([]service.UsageIPAccountRank, *pagination.PaginationResult, error) {
	// 一次聚合顺带用窗口函数取出分组总数，避免为分页再跑一遍 COUNT(DISTINCT ip_address)：
	// COUNT(*) OVER () 作用在 GROUP BY 之后，得到的就是不同 IP 的个数。
	const query = `
SELECT ip_address,
       COUNT(DISTINCT user_id) AS account_count,
       COUNT(*) OVER ()        AS total_ips
FROM usage_logs
WHERE created_at >= $1
  AND created_at <  $2
  AND ip_address IS NOT NULL
  AND ip_address <> ''
GROUP BY ip_address
ORDER BY account_count DESC, ip_address ASC
LIMIT $3 OFFSET $4`

	rows, err := r.sql.QueryContext(ctx, query, startTime, endTime, params.Limit(), params.Offset())
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.UsageIPAccountRank, 0, params.Limit())
	var total int64
	for rows.Next() {
		var (
			item       service.UsageIPAccountRank
			groupTotal int64
		)
		if err := rows.Scan(&item.IPAddress, &item.AccountCount, &groupTotal); err != nil {
			return nil, nil, err
		}
		// 每行携带的都是同一个分组总数，取任意一行即可；无行时保持 0。
		total = groupTotal
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return items, paginationResultFromTotal(total, params), nil
}
