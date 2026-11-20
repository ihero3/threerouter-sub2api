package repository

import (
	"context"
	"time"

	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ListRecentRegistrationIPs 返回 [since, now) 内注册、且注册 IP 非空的用户节点。
//
// 显式带 register_ip <> ” 谓词以命中部分索引 idx_users_register_ip_created_at
// (WHERE register_ip <> ”)：参数化查询下只有写出该谓词 PostgreSQL 才能推导出可用索引。
// 排除软删除用户；admin 由 service 层过滤（避免测试桩依赖 role 投影）。
func (r *userRepository) ListRecentRegistrationIPs(ctx context.Context, since time.Time) ([]service.RecentRegistrationIP, error) {
	client := clientFromContext(ctx, r.client)
	if client == nil {
		return nil, nil
	}
	users, err := client.User.Query().
		Where(
			dbuser.CreatedAtGTE(since),
			dbuser.RegisterIPNEQ(""),
		).
		Select(dbuser.FieldID, dbuser.FieldRegisterIP, dbuser.FieldCreatedAt).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.RecentRegistrationIP, 0, len(users))
	for _, u := range users {
		out = append(out, service.RecentRegistrationIP{
			UserID:     u.ID,
			RegisterIP: u.RegisterIP,
			CreatedAt:  u.CreatedAt,
		})
	}
	return out, nil
}
