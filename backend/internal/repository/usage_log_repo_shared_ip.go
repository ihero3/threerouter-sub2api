package repository

import (
	"context"
	"fmt"
	"time"
)

// sharedIPRiskUserIDsSQL 查出「最近窗口内被 ≥minUsers 个不同用户使用过的 IP」下的所有用户 ID。
//
// 与 GetSharedUsageIPs 同源（同一张 usage_logs、同一窗口），但直接返回用户 ID 集合
// 供周期扫描批量打标 + 扣减，而不是给管理端做展示标记。分层设计：
//   - $1: 最少不同用户数（运营尺度，当前 5）
//   - $2: 统计时间窗口起点（如 now()-180d）
//
// 刻意 DISTINCT：同一用户可能用多个命中 IP，也只处理一次。
// 软删除用户排除，避免给已注销账号写标记/扣减。
// 抽成常量便于在无真库的单测里对 SQL 契约做断言。
const sharedIPRiskUserIDsSQL = `
SELECT DISTINCT l.user_id
FROM usage_logs l
WHERE l.ip_address IS NOT NULL
  AND l.ip_address <> ''
  AND l.created_at >= $2
  AND l.user_id IN (
      SELECT l2.user_id
      FROM usage_logs l2
      WHERE l2.ip_address = l.ip_address
        AND l2.created_at >= $2
  )
  AND l.ip_address IN (
      SELECT ip_address
      FROM usage_logs
      WHERE ip_address IS NOT NULL
        AND ip_address <> ''
        AND created_at >= $2
      GROUP BY ip_address
      HAVING COUNT(DISTINCT user_id) >= $1
  )
  AND EXISTS (SELECT 1 FROM users u WHERE u.id = l.user_id AND u.deleted_at IS NULL)
`

// ListUserIDsBySharedIP 返回命中共享 IP 风险的用户 ID 集合。
// minUsers 是「同一 IP 关联的最少不同用户数」阈值，window 是统计时间窗口长度。
func (r *usageLogRepository) ListUserIDsBySharedIP(ctx context.Context, minUsers int, window time.Duration) ([]int64, error) {
	if r == nil || r.sql == nil {
		return nil, fmt.Errorf("sql executor is not configured")
	}
	if minUsers < 2 {
		minUsers = 2
	}
	if window <= 0 {
		window = 180 * 24 * time.Hour
	}
	since := time.Now().Add(-window)
	rows, err := r.sql.QueryContext(ctx, sharedIPRiskUserIDsSQL, minUsers, since)
	if err != nil {
		return nil, fmt.Errorf("list user ids by shared ip: %w", err)
	}
	defer func() { _ = rows.Close() }()

	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
