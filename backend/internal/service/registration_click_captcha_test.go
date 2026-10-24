//go:build unit

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
	attempts   map[string]int64
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
func (m *clickCaptchaMemoryStore) IncrAttempts(ctx context.Context, id string) (int64, error) {
	if m.attempts == nil {
		m.attempts = map[string]int64{}
	}
	m.attempts[id]++
	return m.attempts[id], nil
}

// captchaServiceForMode 构造固定模式/难度的服务，便于确定性测试。
func captchaServiceForMode(store clickCaptchaCacheStore, mode, difficulty string) *RegistrationClickCaptchaService {
	values := map[string]string{}
	if mode != "" {
		values[SettingKeyRegistrationCaptchaMode] = mode
	}
	if difficulty != "" {
		values[SettingKeyRegistrationCaptchaDifficulty] = difficulty
	}
	settings := &SettingService{settingRepo: &settingRepoStub{values: values}}
	return NewRegistrationClickCaptchaService(store, settings)
}

// validProofFor 根据服务端载荷生成一份正确答案。
func validProofFor(payload *ClickCaptchaChallengePayloadRef) CaptchaVerifyProof {
	switch payload.Mode {
	case RegistrationCaptchaModeImageClick:
		clicks := make([]ClickPoint, 0, len(payload.ImageClick.Targets))
		for _, target := range payload.ImageClick.Targets {
			clicks = append(clicks, ClickPoint{X: float64(target.X), Y: float64(target.Y)})
		}
		return CaptchaVerifyProof{Clicks: clicks}
	case RegistrationCaptchaModeSlider:
		return CaptchaVerifyProof{DropX: float64(payload.Slider.TargetX), Trajectory: naturalTrajectory()}
	default:
		return CaptchaVerifyProof{}
	}
}

func TestRegistrationClickCaptcha_Lifecycle(t *testing.T) {
	store := &clickCaptchaMemoryStore{}
	svc := captchaServiceForMode(store, RegistrationCaptchaModeImageClick, RegistrationCaptchaDifficultyMedium)
	ctx := context.Background()
	ipHash := "ip-a"
	uaHash := "ua-a"

	ch, err := svc.CreateChallenge(ctx, ipHash, uaHash)
	require.NoError(t, err)
	require.Equal(t, RegistrationCaptchaModeImageClick, ch.Mode)
	require.NotNil(t, ch.ImageClick)
	require.NotEmpty(t, ch.ImageClick.Image)
	require.NotEmpty(t, ch.ChallengeID)

	// 错误顺序（反序）应失败但题目仍可重试。
	payload := store.challenges[ch.ChallengeID]
	require.NotNil(t, payload)
	wrong := make([]ClickPoint, 0, len(payload.ImageClick.Targets))
	for i := len(payload.ImageClick.Targets) - 1; i >= 0; i-- {
		wrong = append(wrong, ClickPoint{
			X: float64(payload.ImageClick.Targets[i].X),
			Y: float64(payload.ImageClick.Targets[i].Y),
		})
	}
	if len(wrong) > 1 {
		_, _, err = svc.VerifyChallenge(ctx, ch.ChallengeID, CaptchaVerifyProof{Clicks: wrong}, ipHash, uaHash)
		require.Error(t, err)
	}

	token, ttl, err := svc.VerifyChallenge(ctx, ch.ChallengeID, validProofFor(payload), ipHash, uaHash)
	require.NoError(t, err)
	require.Equal(t, int64(900), ttl)
	require.NotEmpty(t, token)

	// 成功后 challenge 已被删除
	require.Nil(t, store.challenges[ch.ChallengeID])

	// 重复消费失败
	ref, err := svc.ConsumeToken(ctx, token, ipHash, uaHash)
	require.NoError(t, err)
	require.NotNil(t, ref)
	_, err = svc.ConsumeToken(ctx, token, ipHash, uaHash)
	require.ErrorIs(t, err, ErrRegistrationClickCaptchaTokenInvalid)
}

func TestRegistrationClickCaptcha_SliderLifecycle(t *testing.T) {
	store := &clickCaptchaMemoryStore{}
	svc := captchaServiceForMode(store, RegistrationCaptchaModeSlider, RegistrationCaptchaDifficultyMedium)
	ctx := context.Background()

	ch, err := svc.CreateChallenge(ctx, "ip-a", "ua-a")
	require.NoError(t, err)
	require.Equal(t, RegistrationCaptchaModeSlider, ch.Mode)
	require.NotNil(t, ch.Slider)
	require.NotEmpty(t, ch.Slider.BgImage)
	require.NotEmpty(t, ch.Slider.PieceImage)

	payload := store.challenges[ch.ChallengeID]
	require.NotNil(t, payload)
	token, _, err := svc.VerifyChallenge(ctx, ch.ChallengeID, validProofFor(payload), "ip-a", "ua-a")
	require.NoError(t, err)
	require.NotEmpty(t, token)
}

func TestRegistrationClickCaptcha_IPUAMismatch(t *testing.T) {
	store := &clickCaptchaMemoryStore{}
	svc := captchaServiceForMode(store, RegistrationCaptchaModeImageClick, RegistrationCaptchaDifficultyMedium)
	ctx := context.Background()
	ch, err := svc.CreateChallenge(ctx, "ip-a", "ua-a")
	require.NoError(t, err)
	payload := store.challenges[ch.ChallengeID]
	_, _, err = svc.VerifyChallenge(ctx, ch.ChallengeID, validProofFor(payload), "ip-b", "ua-a")
	require.ErrorIs(t, err, ErrRegistrationClickCaptchaInvalid)
}

func TestRegistrationClickCaptcha_ValidateToken(t *testing.T) {
	store := &clickCaptchaMemoryStore{}
	svc := captchaServiceForMode(store, RegistrationCaptchaModeImageClick, RegistrationCaptchaDifficultyMedium)
	ctx := context.Background()
	ipHash := "ip-a"
	uaHash := "ua-a"

	ch, err := svc.CreateChallenge(ctx, ipHash, uaHash)
	require.NoError(t, err)
	payload := store.challenges[ch.ChallengeID]
	token, _, err := svc.VerifyChallenge(ctx, ch.ChallengeID, validProofFor(payload), ipHash, uaHash)
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
	svc := captchaServiceForMode(store, RegistrationCaptchaModeImageClick, RegistrationCaptchaDifficultyMedium)
	ctx := context.Background()
	ch, err := svc.CreateChallenge(ctx, "ip-a", "ua-a")
	require.NoError(t, err)
	payload := store.challenges[ch.ChallengeID]
	payload.ExpiresAt = time.Now().Add(-time.Minute).Unix()
	_, _, err = svc.VerifyChallenge(ctx, ch.ChallengeID, validProofFor(payload), "ip-a", "ua-a")
	require.ErrorIs(t, err, ErrRegistrationClickCaptchaInvalid)
}

// TestRegistrationClickCaptcha_AttemptLimit 超过单题失败上限即删题，防止坐标爆破。
func TestRegistrationClickCaptcha_AttemptLimit(t *testing.T) {
	store := &clickCaptchaMemoryStore{}
	svc := captchaServiceForMode(store, RegistrationCaptchaModeImageClick, RegistrationCaptchaDifficultyMedium)
	ctx := context.Background()
	ch, err := svc.CreateChallenge(ctx, "ip-a", "ua-a")
	require.NoError(t, err)

	// 点位远离所有目标的错误答案
	bad := CaptchaVerifyProof{Clicks: []ClickPoint{{X: -1000, Y: -1000}, {X: -1000, Y: -1000}, {X: -1000, Y: -1000}}}
	for i := 0; i < 3; i++ {
		_, _, err = svc.VerifyChallenge(ctx, ch.ChallengeID, bad, "ip-a", "ua-a")
		require.ErrorIs(t, err, ErrRegistrationClickCaptchaInvalid)
	}
	// 第 4 次超过上限：即使答案正确也被拒绝，且题目被删除
	payload := store.challenges[ch.ChallengeID]
	require.NotNil(t, payload)
	_, _, err = svc.VerifyChallenge(ctx, ch.ChallengeID, validProofFor(payload), "ip-a", "ua-a")
	require.ErrorIs(t, err, ErrRegistrationClickCaptchaInvalid)
	require.Nil(t, store.challenges[ch.ChallengeID])
}

// TestRegistrationClickCaptcha_ModeDistribution random 模式下两种验证码都会出现。
func TestRegistrationClickCaptcha_ModeDistribution(t *testing.T) {
	store := &clickCaptchaMemoryStore{}
	svc := captchaServiceForMode(store, RegistrationCaptchaModeRandom, RegistrationCaptchaDifficultyLow)
	ctx := context.Background()

	seen := map[string]int{}
	for i := 0; i < 200; i++ {
		ch, err := svc.CreateChallenge(ctx, "ip-a", "ua-a")
		require.NoError(t, err)
		seen[ch.Mode]++
	}
	require.Positive(t, seen[RegistrationCaptchaModeImageClick], "image_click should appear")
	require.Positive(t, seen[RegistrationCaptchaModeSlider], "slider should appear")
}

func TestRegistrationClickCaptcha_HashFingerprint(t *testing.T) {
	a := HashFingerprint("1.2.3.4", "ua-1")
	b := HashFingerprint("1.2.3.4", "ua-1")
	c := HashFingerprint("1.2.3.4", "ua-2")
	require.Equal(t, a, b)
	require.NotEqual(t, a, c)
	require.False(t, strings.Contains(a, "1.2.3.4"))
}
