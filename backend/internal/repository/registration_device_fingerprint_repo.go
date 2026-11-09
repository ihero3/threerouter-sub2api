package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.RegistrationDeviceFingerprintRepository = (*userRepository)(nil)

// CountRegistrationDeviceFingerprints 统计同一服务端指纹在窗口内的成功注册数。
//
// 走原生 SQL 而非 ent，是为了避免改动 users 的 ent schema 并重新生成代码：
// 指纹记录是纯风控流水，与用户主数据生命周期解耦，独立表更合适。
func (r *userRepository) CountRegistrationDeviceFingerprints(ctx context.Context, fingerprint string, since time.Time) (int, error) {
	rows, err := r.sql.QueryContext(ctx,
		`SELECT count(*) FROM registration_device_fingerprints
		 WHERE fingerprint = $1 AND created_at >= $2`,
		fingerprint, since)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var count int
	if rows.Next() {
		if err := rows.Scan(&count); err != nil {
			return 0, err
		}
	}
	return count, rows.Err()
}

// RecordRegistrationDeviceFingerprint 记录一次成功注册。
//
// 该写入在建号事务之外执行：建号事务回滚时可能留下一条孤儿记录，只会让
// 风控略微偏保守（少数一次），不会造成账号与记录不一致的安全问题。换来的是
// 不必把风控流水硬塞进用户创建事务，降低注册主链路的耦合与锁竞争。
func (r *userRepository) RecordRegistrationDeviceFingerprint(ctx context.Context, fingerprint string, userID int64, registerIP string) error {
	_, err := r.sql.ExecContext(ctx,
		`INSERT INTO registration_device_fingerprints (fingerprint, user_id, register_ip)
		 VALUES ($1, $2, $3)`,
		fingerprint, userID, registerIP)
	return err
}
