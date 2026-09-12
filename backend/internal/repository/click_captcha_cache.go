package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	clickCaptchaChallengeKeyPrefix = "captcha:click:challenge:"
	clickCaptchaTokenKeyPrefix     = "captcha:click:token:"
)

// ClickCaptchaCache 自建顺序点击验证码的 Redis 缓存。
type ClickCaptchaCache struct {
	rdb *redis.Client
}

// NewClickCaptchaCache 创建缓存实例。
func NewClickCaptchaCache(rdb *redis.Client) *ClickCaptchaCache {
	return &ClickCaptchaCache{rdb: rdb}
}

// SetChallenge 写入一道题。
func (c *ClickCaptchaCache) SetChallenge(ctx context.Context, challengeID string, payload *service.ClickCaptchaChallengePayloadRef) error {
	if c == nil || c.rdb == nil {
		return fmt.Errorf("click captcha cache not configured")
	}
	raw, err := service.MarshalClickCaptchaChallengePayload(payload)
	if err != nil {
		return err
	}
	ttl := time.Duration(payload.ExpiresAt-time.Now().Unix()) * time.Second
	if ttl <= 0 {
		ttl = time.Second
	}
	return c.rdb.Set(ctx, clickCaptchaChallengeKeyPrefix+challengeID, raw, ttl).Err()
}

// GetChallenge 读取一道题（原子消费前）。
func (c *ClickCaptchaCache) GetChallenge(ctx context.Context, challengeID string) (*service.ClickCaptchaChallengePayloadRef, error) {
	if c == nil || c.rdb == nil {
		return nil, fmt.Errorf("click captcha cache not configured")
	}
	raw, err := c.rdb.Get(ctx, clickCaptchaChallengeKeyPrefix+challengeID).Result()
	if err != nil {
		return nil, err
	}
	return service.UnmarshalClickCaptchaChallengePayload(raw)
}

// DeleteChallenge 删除一道题。
func (c *ClickCaptchaCache) DeleteChallenge(ctx context.Context, challengeID string) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Del(ctx, clickCaptchaChallengeKeyPrefix+challengeID).Err()
}

// SetToken 写入一次性 token。
func (c *ClickCaptchaCache) SetToken(ctx context.Context, token string, payload *service.ClickCaptchaTokenPayloadRef) error {
	if c == nil || c.rdb == nil {
		return fmt.Errorf("click captcha cache not configured")
	}
	raw, err := service.MarshalClickCaptchaTokenPayload(payload)
	if err != nil {
		return err
	}
	ttl := time.Duration(payload.ExpiresAt-time.Now().Unix()) * time.Second
	if ttl <= 0 {
		ttl = time.Second
	}
	return c.rdb.Set(ctx, clickCaptchaTokenKeyPrefix+token, raw, ttl).Err()
}

// TakeToken 原子消费 token（GETDEL 防重放）。
func (c *ClickCaptchaCache) TakeToken(ctx context.Context, token string) (*service.ClickCaptchaTokenPayloadRef, error) {
	if c == nil || c.rdb == nil {
		return nil, fmt.Errorf("click captcha cache not configured")
	}
	raw, err := c.rdb.GetDel(ctx, clickCaptchaTokenKeyPrefix+token).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return service.UnmarshalClickCaptchaTokenPayload(raw)
}

// PeekToken 读取 token 但不消费（发送验证码前的非破坏性校验）。
func (c *ClickCaptchaCache) PeekToken(ctx context.Context, token string) (*service.ClickCaptchaTokenPayloadRef, error) {
	if c == nil || c.rdb == nil {
		return nil, fmt.Errorf("click captcha cache not configured")
	}
	raw, err := c.rdb.Get(ctx, clickCaptchaTokenKeyPrefix+token).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return service.UnmarshalClickCaptchaTokenPayload(raw)
}
