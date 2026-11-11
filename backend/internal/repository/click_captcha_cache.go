package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	clickCaptchaChallengeKeyPrefix      = "captcha:click:challenge:"
	clickCaptchaTokenKeyPrefix          = "captcha:click:token:"
	clickCaptchaAttemptsKeyPrefix       = "captcha:click:attempts:"
	clickCaptchaTokenSendEmailKeyPrefix = "captcha:click:sendemail:"
	// clickCaptchaTokenSendEmailTTL 发码邮箱绑定键的存活时间，与 token 的 15 分钟
	// 生命周期同量级；token 过期后绑定键自然失效，不产生堆积。
	clickCaptchaTokenSendEmailTTL = 15 * time.Minute
	// clickCaptchaAttemptsTTL 尝试计数键的存活时间；与 challenge 同量级，
	// 题目作废后计数键最多再多存活该时长即可自然过期。
	clickCaptchaAttemptsTTL = 2 * time.Minute
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

// takeTokenScript 原子取一次性 token（GET+DEL）。早于 Redis 6.2 的实例以及
// 部分集群代理不支持 GETDEL 命令，而 Lua EVAL 自 Redis 2.6 起可用、兼容面广得多。
var takeTokenScript = redis.NewScript(`
local v = redis.call('GET', KEYS[1])
if v then
	redis.call('DEL', KEYS[1])
end
return v
`)

// TakeToken 原子消费 token（防重放）。
// 优先走 Lua 脚本；脚本不可用时退化为 GET+DEL：宁可短暂放宽防重放窗口，
// 也不能让注册因 Redis 能力差异被整体阻断（见 2026-11 线上注册故障）。
func (c *ClickCaptchaCache) TakeToken(ctx context.Context, token string) (*service.ClickCaptchaTokenPayloadRef, error) {
	if c == nil || c.rdb == nil {
		return nil, fmt.Errorf("click captcha cache not configured")
	}
	key := clickCaptchaTokenKeyPrefix + token
	raw, err := takeTokenScript.Run(ctx, c.rdb, []string{key}).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err == nil {
		s, _ := raw.(string)
		if s == "" {
			return nil, nil
		}
		return service.UnmarshalClickCaptchaTokenPayload(s)
	}
	// 脚本执行失败（受限代理不支持 EVAL 等）：GET+DEL 兜底。
	rawGet, getErr := c.rdb.Get(ctx, key).Result()
	if getErr != nil {
		if getErr == redis.Nil {
			return nil, nil
		}
		return nil, getErr
	}
	_ = c.rdb.Del(ctx, key).Err()
	return service.UnmarshalClickCaptchaTokenPayload(rawGet)
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

// IncrAttempts 原子自增同一道题的尝试次数，并在每次写入时刷新计数键 TTL。
// 计数独立于 challenge 载荷，保证 INCR 原子性且不破坏反序列化结构。
func (c *ClickCaptchaCache) IncrAttempts(ctx context.Context, challengeID string) (int64, error) {
	if c == nil || c.rdb == nil {
		return 0, fmt.Errorf("click captcha cache not configured")
	}
	key := clickCaptchaAttemptsKeyPrefix + challengeID
	pipe := c.rdb.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, clickCaptchaAttemptsTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

// ClaimTokenSendEmail 把一次性 token 原子绑定到唯一发码邮箱（一次人机验证只允许
// 给一个收件地址发信；同一地址重发不限次）。返回当前绑定的邮箱：首次绑定返回
// 请求的地址，已绑定则返回既有地址，由 service 层比较是否换址。
// 用 SETNX 原子抢占：并发请求中只有一个地址能赢得绑定，其余地址拿到既有值。
func (c *ClickCaptchaCache) ClaimTokenSendEmail(ctx context.Context, token, email string) (string, error) {
	if c == nil || c.rdb == nil {
		return "", fmt.Errorf("click captcha cache not configured")
	}
	key := clickCaptchaTokenSendEmailKeyPrefix + token
	set, err := c.rdb.SetNX(ctx, key, email, clickCaptchaTokenSendEmailTTL).Result()
	if err != nil {
		return "", err
	}
	if set {
		return email, nil
	}
	bound, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// 绑定键恰好在 SetNX 与 GET 之间过期：视为本次请求赢得绑定。
			return email, nil
		}
		return "", err
	}
	return bound, nil
}

// ResetTokenSendEmail 删除发码邮箱绑定（归还发码额度），仅由发信失败的调用方使用。
func (c *ClickCaptchaCache) ResetTokenSendEmail(ctx context.Context, token string) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Del(ctx, clickCaptchaTokenSendEmailKeyPrefix+token).Err()
}
