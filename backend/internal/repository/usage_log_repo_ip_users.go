package repository

import (
	"context"
	"database/sql"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ListUsersByIP 列出使用过指定 IP 的用户（每个用户一行），按该用户的最后调用时间倒序。
//
// "最后使用时间"与用户列表 / 邀请关系保持同一口径：取该用户在 usage_logs 中
// 最后一次调用的时间（不限当前 IP），而不是 users.last_active_at（登录即刷新）。
// 若某用户只有历史记录、之后再无调用，仍返回其真实的最后调用时间。
func (r *usageLogRepository) ListUsersByIP(ctx context.Context, ip string, params pagination.PaginationParams) ([]service.UsageIPUser, *pagination.PaginationResult, error) {
	const where = `EXISTS (SELECT 1 FROM usage_logs x WHERE x.user_id = u.id AND x.ip_address = $1)`

	var total int64
	rows, err := r.sql.QueryContext(ctx, `SELECT COUNT(*) FROM users u WHERE `+where, ip)
	if err != nil {
		return nil, nil, err
	}
	if rows.Next() {
		if scanErr := rows.Scan(&total); scanErr != nil {
			_ = rows.Close()
			return nil, nil, scanErr
		}
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, nil, closeErr
	}
	if total == 0 {
		return []service.UsageIPUser{}, paginationResultFromTotal(0, params), nil
	}

	query := `
SELECT u.id,
       COALESCE(u.email, ''),
       COALESCE(u.username, ''),
       u.created_at,
       (SELECT MAX(l.created_at) FROM usage_logs l WHERE l.user_id = u.id) AS last_used_at
FROM users u
WHERE ` + where + `
ORDER BY last_used_at DESC NULLS LAST, u.id DESC
LIMIT $2 OFFSET $3`

	rows, err = r.sql.QueryContext(ctx, query, ip, params.Limit(), params.Offset())
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()

	users := make([]service.UsageIPUser, 0, params.Limit())
	for rows.Next() {
		var (
			item       service.UsageIPUser
			lastUsedAt sql.NullTime
		)
		if err := rows.Scan(&item.UserID, &item.Email, &item.Username, &item.CreatedAt, &lastUsedAt); err != nil {
			return nil, nil, err
		}
		if lastUsedAt.Valid {
			t := lastUsedAt.Time
			item.LastUsedAt = &t
		}
		users = append(users, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return users, paginationResultFromTotal(total, params), nil
}
