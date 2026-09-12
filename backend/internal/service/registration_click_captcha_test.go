package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type clickCaptchaMemoryStore struct {
	challenges map[string]*ClickCaptchaChallengePayloadRef
	tokens     map[string]*ClickCaptchaTokenPayloadRef
}

func (m *clickCaptchaMemoryStore) SetChallenge(ctx context.Context, id string, p *ClickCaptchaChallengePayloadRef) error {
	if m.challenges == nil {
		m.challenges = map[string]*ClickCaptchaChallengePayloadRef{}
	}
	m.challenges[id] = p
	return nil
}
func (m *clickCaptchaMemoryStore) GetChallenge(ctx context.Context, id string) (*ClickCaptchaChallengePayloadRef, error) {
	if m.challenges == nil {
		return nil, nil
	}
	return m.challenges[id], nil
}
func (m *clickCaptchaMemoryStore) DeleteChallenge(ctx context.Context, id string) error {
	if m.challenges == nil {
		return nil
	}
	delete(m.challenges, id)
	return nil
}
func (m *clickCaptchaMemoryStore) SetToken(ctx context.Context, token string, p *ClickCaptchaTokenPayloadRef) error {
	if m.tokens == nil {
		m.tokens = map[string]*ClickCaptchaTokenPayloadRef{}
	}
	m.tokens[token] = p
	return nil
}
func (m *clickCaptchaMemoryStore) TakeToken(ctx context.Context, token string) (*ClickCaptchaTokenPayloadRef, error) {
	if m.tokens == nil {
		return nil, nil
	}
	p := m.tokens[token]
	delete(m.tokens, token)
	return p, nil
}
func (m *clickCaptchaMemoryStore) PeekToken(ctx context.Context, token string) (*ClickCaptchaTokenPayloadRef, error) {
	if m.tokens == nil {
		return nil, nil
	}
	return m.tokens[token], nil
}

func TestRegistrationClickCaptcha_Lifecycle(t *testing.T) {
	store := &clickCaptchaMemoryStore{}
	svc := NewRegistrationClickCaptchaService(store)
	ctx := context.Background()
	ipHash := "ip-a"
	uaHash := "ua-a"

	ch, err := svc.CreateChallenge(ctx, ipHash, uaHash)
	require.NoError(t, err)
	require.Equal(t, 9, len(ch.Grid))
	require.Equal(t, 3, len(ch.Prompt))
	require.NotEmpty(t, ch.ChallengeID)

	// 错误顺序
	_, _, err = svc.VerifyChallenge(ctx, ch.ChallengeID, []string{"c0", "c1", "c2"}, ipHash, uaHash)
	require.Error(t, err)

	// 正确顺序：题目答案即 Prompt 对应的格子顺序（简化为按格子排列验证）
	payload := store.challenges[ch.ChallengeID]
	require.NotNil(t, payload)
	token, ttl, err := svc.VerifyChallenge(ctx, ch.ChallengeID, payload.AnswerCells, ipHash, uaHash)
	require.NoError(t, err)
	require.Equal(t, int64(300), ttl)
	require.NotEmpty(t, token)

	// 重复消费失败
	ref, err := svc.ConsumeToken(ctx, token, ipHash, uaHash)
	require.NoError(t, err)
	require.NotNil(t, ref)
	_, err = svc.ConsumeToken(ctx, token, ipHash, uaHash)
	require.ErrorIs(t, err, ErrRegistrationClickCaptchaTokenInvalid)

	// 归还后可再次消费（模拟注册失败重试）
	require.NoError(t, svc.RestoreToken(ctx, token, ref))
	ref, err = svc.ConsumeToken(ctx, token, ipHash, uaHash)
	require.NoError(t, err)
	require.NotNil(t, ref)

	// 过期载荷不应归还
	expired := &ClickCaptchaTokenPayloadRef{IPHash: ipHash, UAHash: uaHash, ExpiresAt: time.Now().Add(-time.Second).Unix()}
	require.ErrorIs(t, svc.RestoreToken(ctx, token, expired), ErrRegistrationClickCaptchaTokenInvalid)
}

func TestRegistrationClickCaptcha_IPUAMismatch(t *testing.T) {
	store := &clickCaptchaMemoryStore{}
	svc := NewRegistrationClickCaptchaService(store)
	ctx := context.Background()
	ch, err := svc.CreateChallenge(ctx, "ip-a", "ua-a")
	require.NoError(t, err)
	payload := store.challenges[ch.ChallengeID]
	_, _, err = svc.VerifyChallenge(ctx, ch.ChallengeID, payload.AnswerCells, "ip-b", "ua-a")
	require.ErrorIs(t, err, ErrRegistrationClickCaptchaInvalid)
}

func TestRegistrationClickCaptcha_ValidateToken(t *testing.T) {
	store := &clickCaptchaMemoryStore{}
	svc := NewRegistrationClickCaptchaService(store)
	ctx := context.Background()
	ipHash := "ip-a"
	uaHash := "ua-a"

	ch, err := svc.CreateChallenge(ctx, ipHash, uaHash)
	require.NoError(t, err)
	payload := store.challenges[ch.ChallengeID]
	token, _, err := svc.VerifyChallenge(ctx, ch.ChallengeID, payload.AnswerCells, ipHash, uaHash)
	require.NoError(t, err)

	// 非破坏性校验：可重复校验且不影响 token 状态
	require.NoError(t, svc.ValidateToken(ctx, token, ipHash, uaHash))
	require.NoError(t, svc.ValidateToken(ctx, token, ipHash, uaHash))

	// 校验后仍可正常一次性消费
	ref, err := svc.ConsumeToken(ctx, token, ipHash, uaHash)
	require.NoError(t, err)
	require.NotNil(t, ref)

	// 消费后校验失败
	require.ErrorIs(t, svc.ValidateToken(ctx, token, ipHash, uaHash), ErrRegistrationClickCaptchaTokenInvalid)

	// 指纹不匹配的 token 校验失败
	require.NoError(t, store.SetToken(ctx, "tok-other", &ClickCaptchaTokenPayloadRef{
		IPHash:    "ip-b",
		UAHash:    "ua-b",
		CreatedAt: time.Now().Unix(),
		ExpiresAt: time.Now().Add(time.Minute).Unix(),
	}))
	require.ErrorIs(t, svc.ValidateToken(ctx, "tok-other", ipHash, uaHash), ErrRegistrationClickCaptchaTokenInvalid)

	// 空 token 校验失败
	require.ErrorIs(t, svc.ValidateToken(ctx, "  ", ipHash, uaHash), ErrRegistrationClickCaptchaTokenInvalid)
}
func TestRegistrationClickCaptcha_ChallengeExpired(t *testing.T) {
	store := &clickCaptchaMemoryStore{}
	svc := NewRegistrationClickCaptchaService(store)
	ctx := context.Background()
	ch, err := svc.CreateChallenge(ctx, "ip-a", "ua-a")
	require.NoError(t, err)
	payload := store.challenges[ch.ChallengeID]
	payload.ExpiresAt = time.Now().Add(-time.Minute).Unix()
	_, _, err = svc.VerifyChallenge(ctx, ch.ChallengeID, payload.AnswerCells, "ip-a", "ua-a")
	require.ErrorIs(t, err, ErrRegistrationClickCaptchaInvalid)
}

func TestRegistrationClickCaptcha_HashFingerprint(t *testing.T) {
	a := HashFingerprint("1.2.3.4", "ua-1")
	b := HashFingerprint("1.2.3.4", "ua-1")
	c := HashFingerprint("1.2.3.4", "ua-2")
	require.Equal(t, a, b)
	require.NotEqual(t, a, c)
	require.False(t, strings.Contains(a, "1.2.3.4"))
}
