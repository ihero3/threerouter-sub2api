package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// captchaConsumeStore 内存态 clickCaptchaCacheStore，用于验证一次性 token 的
// 消费语义（GETDEL 等价）与归还路径。
type captchaConsumeStore struct {
	tokens map[string]*ClickCaptchaTokenPayloadRef
	// sendEmails 模拟发码邮箱绑定键（token → email）。
	sendEmails map[string]string
	// sendEmailErr 非空时 ClaimTokenSendEmail 返回错误，模拟 Redis 故障。
	sendEmailErr error
}

func (m *captchaConsumeStore) SetChallenge(ctx context.Context, id string, p *ClickCaptchaChallengePayloadRef) error {
	return nil
}

func (m *captchaConsumeStore) GetChallenge(ctx context.Context, id string) (*ClickCaptchaChallengePayloadRef, error) {
	return nil, nil
}

func (m *captchaConsumeStore) DeleteChallenge(ctx context.Context, id string) error { return nil }

func (m *captchaConsumeStore) SetToken(ctx context.Context, token string, p *ClickCaptchaTokenPayloadRef) error {
	if m.tokens == nil {
		m.tokens = map[string]*ClickCaptchaTokenPayloadRef{}
	}
	m.tokens[token] = p
	return nil
}

func (m *captchaConsumeStore) TakeToken(ctx context.Context, token string) (*ClickCaptchaTokenPayloadRef, error) {
	p, ok := m.tokens[token]
	if !ok {
		return nil, nil
	}
	delete(m.tokens, token)
	return p, nil
}

func (m *captchaConsumeStore) PeekToken(ctx context.Context, token string) (*ClickCaptchaTokenPayloadRef, error) {
	return m.tokens[token], nil
}

func (m *captchaConsumeStore) IncrAttempts(ctx context.Context, challengeID string) (int64, error) {
	return 1, nil
}

func (m *captchaConsumeStore) ClaimTokenSendEmail(ctx context.Context, token, email string) (string, error) {
	if m.sendEmailErr != nil {
		return "", m.sendEmailErr
	}
	if m.sendEmails == nil {
		m.sendEmails = map[string]string{}
	}
	if bound, ok := m.sendEmails[token]; ok {
		return bound, nil
	}
	m.sendEmails[token] = email
	return email, nil
}

func (m *captchaConsumeStore) ResetTokenSendEmail(ctx context.Context, token string) error {
	delete(m.sendEmails, token)
	return nil
}

func newCaptchaConsumeFixture(t *testing.T) (*RegistrationClickCaptchaService, *captchaConsumeStore, string, string) {
	t.Helper()
	store := &captchaConsumeStore{}
	svc := NewRegistrationClickCaptchaService(store, nil)
	ipHash := HashFingerprint("203.0.113.9", "unit-test-ua")
	token := "captcha-token-1"
	require.NoError(t, store.SetToken(context.Background(), token, &ClickCaptchaTokenPayloadRef{
		IPHash:    ipHash,
		UAHash:    ipHash,
		CreatedAt: time.Now().Unix(),
		ExpiresAt: time.Now().Add(time.Minute).Unix(),
	}))
	return svc, store, token, ipHash
}

// TestRegistrationClickCaptcha_TokenIsSingleUse 锁死一次性语义：
// 注册改为「提交前原子消费」正是为了让并发请求中只有一个能拿到 token，
// 若此处退化为可重复消费，就意味着一次人机验证又能换多个账号。
func TestRegistrationClickCaptcha_TokenIsSingleUse(t *testing.T) {
	svc, _, token, ipHash := newCaptchaConsumeFixture(t)
	ctx := context.Background()

	first, err := svc.ConsumeToken(ctx, token, ipHash, ipHash)
	require.NoError(t, err)
	require.NotNil(t, first)

	second, err := svc.ConsumeToken(ctx, token, ipHash, ipHash)
	require.ErrorIs(t, err, ErrRegistrationClickCaptchaTokenInvalid)
	require.Nil(t, second)
}

// TestRegistrationClickCaptcha_RestoreTokenAfterFailedSignup 验证建号失败后的归还：
// 用户不需要重新做一次人机验证，但归还后的 token 仍然只能被消费一次。
func TestRegistrationClickCaptcha_RestoreTokenAfterFailedSignup(t *testing.T) {
	svc, _, token, ipHash := newCaptchaConsumeFixture(t)
	ctx := context.Background()

	payload, err := svc.ConsumeToken(ctx, token, ipHash, ipHash)
	require.NoError(t, err)
	require.NotNil(t, payload)

	require.NoError(t, svc.RestoreToken(ctx, token, payload))

	again, err := svc.ConsumeToken(ctx, token, ipHash, ipHash)
	require.NoError(t, err)
	require.NotNil(t, again)

	// 归还不会让 token 变成可重复使用
	replay, err := svc.ConsumeToken(ctx, token, ipHash, ipHash)
	require.ErrorIs(t, err, ErrRegistrationClickCaptchaTokenInvalid)
	require.Nil(t, replay)
}

// TestRegistrationClickCaptcha_RestoreTokenRejectsMismatch 归还必须携带取到的载荷，
// 空载荷会让 token 变成可无限重放的凭据。
func TestRegistrationClickCaptcha_RestoreTokenRejectsMismatch(t *testing.T) {
	svc, _, token, _ := newCaptchaConsumeFixture(t)
	ctx := context.Background()

	require.ErrorIs(t, svc.RestoreToken(ctx, token, nil), ErrRegistrationClickCaptchaDisabled)
	require.ErrorIs(t, svc.RestoreToken(ctx, "", &ClickCaptchaTokenPayloadRef{}), ErrRegistrationClickCaptchaTokenInvalid)
}
