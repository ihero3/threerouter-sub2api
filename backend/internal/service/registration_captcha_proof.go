package service

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// 发码阶段第三方人机验证成功后签发的服务端一次性凭据。
//
// 目的：堵住「注册接口只要带上任意非空 verify_code 就能跳过第三方 captcha」的口子。
// 发码接口完成第三方验证后由服务端签发 proof token，并把邮箱、可信 IP、UA、设备指纹
// 的哈希写进载荷；注册接口必须提交同一个 token 且四项哈希全部一致，才允许继续。
// token 由 Redis 原子消费（GET+DEL），失败归还只在建号失败时发生。
//
// 边界：这里只证明「本次邮箱码是在同设备同网络通过人机验证后领到的」，不是账号
// 身份凭证；仍然依赖邮箱码、设备/IP 配额等既有风控。
const registrationCaptchaProofTTL = 15 * time.Minute

var (
	// ErrRegistrationCaptchaProofInvalid 客户端错误文案必须纯英文。
	ErrRegistrationCaptchaProofInvalid = infraerrors.BadRequest(
		"REGISTRATION_CAPTCHA_PROOF_INVALID",
		"verification session expired or does not match this request; request a new code",
	)
	ErrRegistrationCaptchaProofUnavailable = infraerrors.ServiceUnavailable(
		"REGISTRATION_CAPTCHA_PROOF_UNAVAILABLE",
		"verification session service temporarily unavailable",
	)
)

// RegistrationCaptchaProofPayload 只存哈希，不落原始邮箱/IP/UA。
type RegistrationCaptchaProofPayload struct {
	EmailHash             string `json:"email_hash"`
	IPHash                string `json:"ip_hash"`
	UserAgentHash         string `json:"user_agent_hash"`
	DeviceFingerprintHash string `json:"device_fingerprint_hash"`
	ExpiresAt             int64  `json:"expires_at"`
}

type registrationCaptchaProofCacheStore interface {
	SetRegistrationCaptchaProof(ctx context.Context, token string, payload *RegistrationCaptchaProofPayload, ttl time.Duration) error
	PeekRegistrationCaptchaProof(ctx context.Context, token string) (*RegistrationCaptchaProofPayload, error)
	TakeRegistrationCaptchaProof(ctx context.Context, token string) (*RegistrationCaptchaProofPayload, error)
	RestoreRegistrationCaptchaProof(ctx context.Context, token string, payload *RegistrationCaptchaProofPayload, ttl time.Duration) error
}

// RegistrationCaptchaProofService 签发/校验发码阶段的人机验证凭据。
type RegistrationCaptchaProofService struct {
	cache registrationCaptchaProofCacheStore
}

func NewRegistrationCaptchaProofService(cache registrationCaptchaProofCacheStore) *RegistrationCaptchaProofService {
	return &RegistrationCaptchaProofService{cache: cache}
}

func hashRegistrationProofPart(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}

func normalizeRegistrationProofEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func newRegistrationCaptchaProofPayload(email, clientIP, userAgent, deviceFingerprint string) *RegistrationCaptchaProofPayload {
	return &RegistrationCaptchaProofPayload{
		EmailHash:             hashRegistrationProofPart(normalizeRegistrationProofEmail(email)),
		IPHash:                hashRegistrationProofPart(clientIP),
		UserAgentHash:         hashRegistrationProofPart(userAgent),
		DeviceFingerprintHash: hashRegistrationProofPart(deviceFingerprint),
		ExpiresAt:             time.Now().Add(registrationCaptchaProofTTL).Unix(),
	}
}

func (p *RegistrationCaptchaProofPayload) matches(email, clientIP, userAgent, deviceFingerprint string) bool {
	if p == nil {
		return false
	}
	expected := []string{
		hashRegistrationProofPart(normalizeRegistrationProofEmail(email)),
		hashRegistrationProofPart(clientIP),
		hashRegistrationProofPart(userAgent),
		hashRegistrationProofPart(deviceFingerprint),
	}
	actual := []string{p.EmailHash, p.IPHash, p.UserAgentHash, p.DeviceFingerprintHash}
	for i := range expected {
		if len(actual[i]) != len(expected[i]) || subtle.ConstantTimeCompare([]byte(actual[i]), []byte(expected[i])) != 1 {
			return false
		}
	}
	return p.ExpiresAt > time.Now().Unix()
}

// Issue 生成一次性 proof token。token 本体随机，载荷只存哈希且不回传客户端。
func (s *RegistrationCaptchaProofService) Issue(ctx context.Context, email, clientIP, userAgent, deviceFingerprint string) (string, error) {
	if s == nil || s.cache == nil {
		return "", ErrRegistrationCaptchaProofUnavailable
	}
	token, err := randomHexString(24)
	if err != nil {
		return "", ErrRegistrationCaptchaProofUnavailable
	}
	payload := newRegistrationCaptchaProofPayload(email, clientIP, userAgent, deviceFingerprint)
	if err := s.cache.SetRegistrationCaptchaProof(ctx, token, payload, registrationCaptchaProofTTL); err != nil {
		return "", ErrRegistrationCaptchaProofUnavailable
	}
	return token, nil
}

// Validate 非破坏性校验，用于注册入口提前拒绝不匹配的 proof。
func (s *RegistrationCaptchaProofService) Validate(ctx context.Context, token, email, clientIP, userAgent, deviceFingerprint string) (*RegistrationCaptchaProofPayload, error) {
	if s == nil || s.cache == nil {
		return nil, ErrRegistrationCaptchaProofUnavailable
	}
	if strings.TrimSpace(token) == "" {
		return nil, ErrRegistrationCaptchaProofInvalid
	}
	payload, err := s.cache.PeekRegistrationCaptchaProof(ctx, token)
	if err != nil {
		return nil, ErrRegistrationCaptchaProofUnavailable
	}
	if !payload.matches(email, clientIP, userAgent, deviceFingerprint) {
		return nil, ErrRegistrationCaptchaProofInvalid
	}
	return payload, nil
}

// Consume 原子消费 proof token，防止同一个 proof 并发注册多个账号。
func (s *RegistrationCaptchaProofService) Consume(ctx context.Context, token, email, clientIP, userAgent, deviceFingerprint string) (*RegistrationCaptchaProofPayload, error) {
	if s == nil || s.cache == nil {
		return nil, ErrRegistrationCaptchaProofUnavailable
	}
	if strings.TrimSpace(token) == "" {
		return nil, ErrRegistrationCaptchaProofInvalid
	}
	payload, err := s.cache.TakeRegistrationCaptchaProof(ctx, token)
	if err != nil {
		return nil, ErrRegistrationCaptchaProofUnavailable
	}
	if !payload.matches(email, clientIP, userAgent, deviceFingerprint) {
		// 载荷不匹配时 token 已被消费，绝不放回：避免攻击者用错误上下文反复探测。
		return nil, ErrRegistrationCaptchaProofInvalid
	}
	return payload, nil
}

// Restore 只在建号失败时归还 proof，保留原始过期时间。
func (s *RegistrationCaptchaProofService) Restore(ctx context.Context, token string, payload *RegistrationCaptchaProofPayload) error {
	if s == nil || s.cache == nil || payload == nil || strings.TrimSpace(token) == "" {
		return nil
	}
	ttl := time.Until(time.Unix(payload.ExpiresAt, 0))
	if ttl <= 0 {
		return nil
	}
	return s.cache.RestoreRegistrationCaptchaProof(ctx, token, payload, ttl)
}

// MarshalRegistrationCaptchaProofPayload 序列化载荷供 Redis 存储。
func MarshalRegistrationCaptchaProofPayload(p *RegistrationCaptchaProofPayload) (string, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// UnmarshalRegistrationCaptchaProofPayload 反序列化 Redis 载荷。
func UnmarshalRegistrationCaptchaProofPayload(raw string) (*RegistrationCaptchaProofPayload, error) {
	var payload RegistrationCaptchaProofPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

// IssueRegistrationCaptchaProof 发码成功后签发 proof。邮箱验证关闭时无需 proof。
func (s *AuthService) IssueRegistrationCaptchaProof(ctx context.Context, email, clientIP, userAgent, deviceFingerprint string) (string, error) {
	if s == nil || s.settingService == nil || !s.IsEmailVerifyEnabled(ctx) {
		return "", nil
	}
	if s.registrationCaptchaProof == nil {
		return "", ErrRegistrationCaptchaProofUnavailable
	}
	return s.registrationCaptchaProof.Issue(ctx, email, clientIP, userAgent, deviceFingerprint)
}

// ConsumeRegistrationCaptchaProof 注册阶段原子消费 proof。
func (s *AuthService) ConsumeRegistrationCaptchaProof(ctx context.Context, token, email, clientIP, userAgent, deviceFingerprint string) (*RegistrationCaptchaProofPayload, error) {
	if s == nil || s.settingService == nil || !s.IsEmailVerifyEnabled(ctx) {
		return nil, nil
	}
	if s.registrationCaptchaProof == nil {
		return nil, ErrRegistrationCaptchaProofUnavailable
	}
	return s.registrationCaptchaProof.Consume(ctx, token, email, clientIP, userAgent, deviceFingerprint)
}

// RestoreRegistrationCaptchaProof 注册失败后归还 proof。
func (s *AuthService) RestoreRegistrationCaptchaProof(ctx context.Context, token string, payload *RegistrationCaptchaProofPayload) error {
	if s == nil || s.registrationCaptchaProof == nil || payload == nil {
		return nil
	}
	return s.registrationCaptchaProof.Restore(ctx, token, payload)
}
