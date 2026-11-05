package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	verifyCodeKeyPrefix          = "verify_code:"
	notifyVerifyKeyPrefix        = "notify_verify:"
	passwordResetKeyPrefix       = "password_reset:"
	passwordResetSentAtKeyPrefix = "password_reset_sent:"
	notifyCodeUserRateKeyPrefix  = "notify_code_user_rate:"
	emailDailySendKeyPrefix      = "email_daily_send:"
	smtpAccountCooldownKeyPrefix = "email_smtp_account_cooldown:"
)

// verifyCodeKey generates the Redis key for email verification code.
// Email is lowercased for case-insensitive consistency.
func verifyCodeKey(email string) string {
	return verifyCodeKeyPrefix + strings.ToLower(email)
}

// notifyVerifyKey generates the Redis key for notify email verification code.
// Email is lowercased to prevent case-sensitive key mismatch (the business layer
// uses strings.EqualFold for comparison).
func notifyVerifyKey(email string) string {
	return notifyVerifyKeyPrefix + strings.ToLower(email)
}

// passwordResetKey generates the Redis key for password reset token.
func passwordResetKey(email string) string {
	return passwordResetKeyPrefix + strings.ToLower(email)
}

// passwordResetSentAtKey generates the Redis key for password reset email sent timestamp.
func passwordResetSentAtKey(email string) string {
	return passwordResetSentAtKeyPrefix + strings.ToLower(email)
}

type emailCache struct {
	rdb *redis.Client
}

func NewEmailCache(rdb *redis.Client) service.EmailCache {
	return &emailCache{rdb: rdb}
}

func (c *emailCache) GetVerificationCode(ctx context.Context, email string) (*service.VerificationCodeData, error) {
	key := verifyCodeKey(email)
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	var data service.VerificationCodeData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (c *emailCache) SetVerificationCode(ctx context.Context, email string, data *service.VerificationCodeData, ttl time.Duration) error {
	key := verifyCodeKey(email)
	val, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, val, ttl).Err()
}

func (c *emailCache) DeleteVerificationCode(ctx context.Context, email string) error {
	key := verifyCodeKey(email)
	return c.rdb.Del(ctx, key).Err()
}

// Password reset token methods

func (c *emailCache) GetPasswordResetToken(ctx context.Context, email string) (*service.PasswordResetTokenData, error) {
	key := passwordResetKey(email)
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	var data service.PasswordResetTokenData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (c *emailCache) SetPasswordResetToken(ctx context.Context, email string, data *service.PasswordResetTokenData, ttl time.Duration) error {
	key := passwordResetKey(email)
	val, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, val, ttl).Err()
}

func (c *emailCache) DeletePasswordResetToken(ctx context.Context, email string) error {
	key := passwordResetKey(email)
	return c.rdb.Del(ctx, key).Err()
}

// Password reset email cooldown methods

func (c *emailCache) IsPasswordResetEmailInCooldown(ctx context.Context, email string) bool {
	key := passwordResetSentAtKey(email)
	exists, err := c.rdb.Exists(ctx, key).Result()
	return err == nil && exists > 0
}

func (c *emailCache) SetPasswordResetEmailCooldown(ctx context.Context, email string, ttl time.Duration) error {
	key := passwordResetSentAtKey(email)
	return c.rdb.Set(ctx, key, "1", ttl).Err()
}

// Notify email verification code methods

func (c *emailCache) GetNotifyVerifyCode(ctx context.Context, email string) (*service.VerificationCodeData, error) {
	key := notifyVerifyKey(email)
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	var data service.VerificationCodeData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (c *emailCache) SetNotifyVerifyCode(ctx context.Context, email string, data *service.VerificationCodeData, ttl time.Duration) error {
	key := notifyVerifyKey(email)
	val, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, val, ttl).Err()
}

func (c *emailCache) DeleteNotifyVerifyCode(ctx context.Context, email string) error {
	key := notifyVerifyKey(email)
	return c.rdb.Del(ctx, key).Err()
}

// User-level rate limiting for notify email verification codes

func notifyCodeUserRateKey(userID int64) string {
	return notifyCodeUserRateKeyPrefix + fmt.Sprintf("%d", userID)
}

func (c *emailCache) IncrNotifyCodeUserRate(ctx context.Context, userID int64, window time.Duration) (int64, error) {
	key := notifyCodeUserRateKey(userID)
	count, err := c.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	// Always set TTL (idempotent) to avoid orphan keys if process crashes between INCR and EXPIRE.
	if err := c.rdb.Expire(ctx, key, window).Err(); err != nil {
		return count, fmt.Errorf("expire notify code rate key: %w", err)
	}
	return count, nil
}

func (c *emailCache) GetNotifyCodeUserRate(ctx context.Context, userID int64) (int64, error) {
	key := notifyCodeUserRateKey(userID)
	count, err := c.rdb.Get(ctx, key).Int64()
	if err != nil {
		return 0, err
	}
	return count, nil
}

// IncrEmailDailySendCount 递增当日发信计数（scopeKey 为 global 或收件人维度），返回累计值。
// 首次计数时把过期时间设到次日凌晨后 1 小时，避免跨天时计数提前失效。
func (c *emailCache) IncrEmailDailySendCount(ctx context.Context, scopeKey string) (int64, error) {
	now := time.Now()
	key := fmt.Sprintf("%s%s:%s", emailDailySendKeyPrefix, scopeKey, now.Format("20060102"))
	count, err := c.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if count == 1 {
		expireAt := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).
			Add(24*time.Hour + time.Hour)
		if err := c.rdb.ExpireAt(ctx, key, expireAt).Err(); err != nil {
			return count, fmt.Errorf("expire email daily send key: %w", err)
		}
	}
	return count, nil
}

// IsSMTPAccountInCooldown 判断某个 SMTP 账号是否处于失败冷却期。
func (c *emailCache) IsSMTPAccountInCooldown(ctx context.Context, accountKey string) bool {
	ok, err := c.rdb.Exists(ctx, smtpAccountCooldownKeyPrefix+accountKey).Result()
	// Redis 故障时返回 false（视为可用），与发信计数的可用性优先策略一致。
	return err == nil && ok > 0
}

// SetSMTPAccountCooldown 给某个 SMTP 账号设置失败冷却。
func (c *emailCache) SetSMTPAccountCooldown(ctx context.Context, accountKey string, ttl time.Duration) error {
	return c.rdb.Set(ctx, smtpAccountCooldownKeyPrefix+accountKey, "1", ttl).Err()
}
