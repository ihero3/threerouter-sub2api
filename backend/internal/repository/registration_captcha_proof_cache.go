package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const registrationCaptchaProofKeyPrefix = "captcha:proof:registration:"

// RegistrationCaptchaProofCache 存发码阶段签发的一次性注册凭据。
type RegistrationCaptchaProofCache struct {
	rdb *redis.Client
}

func NewRegistrationCaptchaProofCache(rdb *redis.Client) *RegistrationCaptchaProofCache {
	return &RegistrationCaptchaProofCache{rdb: rdb}
}

func (c *RegistrationCaptchaProofCache) SetRegistrationCaptchaProof(ctx context.Context, token string, payload *service.RegistrationCaptchaProofPayload, ttl time.Duration) error {
	if c == nil || c.rdb == nil {
		return fmt.Errorf("registration captcha proof cache not configured")
	}
	raw, err := service.MarshalRegistrationCaptchaProofPayload(payload)
	if err != nil {
		return err
	}
	if ttl <= 0 {
		ttl = time.Second
	}
	return c.rdb.Set(ctx, registrationCaptchaProofKeyPrefix+token, raw, ttl).Err()
}

func (c *RegistrationCaptchaProofCache) PeekRegistrationCaptchaProof(ctx context.Context, token string) (*service.RegistrationCaptchaProofPayload, error) {
	if c == nil || c.rdb == nil {
		return nil, fmt.Errorf("registration captcha proof cache not configured")
	}
	raw, err := c.rdb.Get(ctx, registrationCaptchaProofKeyPrefix+token).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return service.UnmarshalRegistrationCaptchaProofPayload(raw)
}

var takeRegistrationCaptchaProofScript = redis.NewScript(`
local v = redis.call('GET', KEYS[1])
if v then
	redis.call('DEL', KEYS[1])
end
return v
`)

// TakeRegistrationCaptchaProof 原子消费；Lua 不可用时退化 GET+DEL，与点击验证码同口径。
func (c *RegistrationCaptchaProofCache) TakeRegistrationCaptchaProof(ctx context.Context, token string) (*service.RegistrationCaptchaProofPayload, error) {
	if c == nil || c.rdb == nil {
		return nil, fmt.Errorf("registration captcha proof cache not configured")
	}
	key := registrationCaptchaProofKeyPrefix + token
	raw, err := takeRegistrationCaptchaProofScript.Run(ctx, c.rdb, []string{key}).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err == nil {
		s, _ := raw.(string)
		if s == "" {
			return nil, nil
		}
		return service.UnmarshalRegistrationCaptchaProofPayload(s)
	}
	rawGet, getErr := c.rdb.Get(ctx, key).Result()
	if getErr != nil {
		if getErr == redis.Nil {
			return nil, nil
		}
		return nil, getErr
	}
	_ = c.rdb.Del(ctx, key).Err()
	return service.UnmarshalRegistrationCaptchaProofPayload(rawGet)
}

func (c *RegistrationCaptchaProofCache) RestoreRegistrationCaptchaProof(ctx context.Context, token string, payload *service.RegistrationCaptchaProofPayload, ttl time.Duration) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.SetRegistrationCaptchaProof(ctx, token, payload, ttl)
}
