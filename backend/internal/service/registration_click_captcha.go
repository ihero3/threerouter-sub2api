package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// 验证码模式：random = 两种随机出现；image_click / slider = 固定为对应模式。
const (
	RegistrationCaptchaModeRandom     = "random"
	RegistrationCaptchaModeImageClick = "image_click"
	RegistrationCaptchaModeSlider     = "slider"
)

// 验证码难度。
const (
	RegistrationCaptchaDifficultyLow    = "low"
	RegistrationCaptchaDifficultyMedium = "medium"
	RegistrationCaptchaDifficultyHigh   = "high"
)

// 归一化模式：非法/空值回退 random（安全默认，两种并存）。
func normalizeRegistrationCaptchaMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case RegistrationCaptchaModeImageClick:
		return RegistrationCaptchaModeImageClick
	case RegistrationCaptchaModeSlider:
		return RegistrationCaptchaModeSlider
	default:
		return RegistrationCaptchaModeRandom
	}
}

// 归一化难度：非法/空值回退 medium。
func normalizeRegistrationCaptchaDifficulty(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case RegistrationCaptchaDifficultyLow:
		return RegistrationCaptchaDifficultyLow
	case RegistrationCaptchaDifficultyHigh:
		return RegistrationCaptchaDifficultyHigh
	default:
		return RegistrationCaptchaDifficultyMedium
	}
}

// ErrRegistrationClickCaptchaDisabled 验证码功能未启用时的错误。
var ErrRegistrationClickCaptchaDisabled = infraerrors.BadRequest("REGISTRATION_CLICK_CAPTCHA_DISABLED", "registration click captcha is disabled")

// ErrRegistrationClickCaptchaInvalid 验证失败或 challenge 无效。
var ErrRegistrationClickCaptchaInvalid = infraerrors.BadRequest("REGISTRATION_CLICK_CAPTCHA_INVALID", "registration click captcha challenge invalid")

// ErrRegistrationClickCaptchaTokenInvalid 一次性 token 无效、过期或已使用。
var ErrRegistrationClickCaptchaTokenInvalid = infraerrors.BadRequest("REGISTRATION_CLICK_CAPTCHA_TOKEN_INVALID", "registration click captcha token invalid or expired, please complete the verification again")

// RegistrationClickCaptchaChallenge 一次验证题（判别联合：按 mode 二选一）。
type RegistrationClickCaptchaChallenge struct {
	ChallengeID string               `json:"challenge_id"`
	Mode        string               `json:"mode"` // image_click | slider
	ExpiresIn   int                  `json:"expires_in"`
	ImageClick  *ImageClickChallenge `json:"image_click,omitempty"`
	Slider      *SliderChallenge     `json:"slider,omitempty"`
}

// ImageClickChallenge 图片点选题面：只含图片与"颜色+形状"提示，不含任何坐标。
type ImageClickChallenge struct {
	Image      string       `json:"image"` // base64 PNG（无 data URL 前缀）
	Width      int          `json:"width"`
	Height     int          `json:"height"`
	Prompt     []TargetHint `json:"prompt"`
	ClickCount int          `json:"click_count"`
}

// TargetHint 目标提示（颜色 + 形状组合）。
type TargetHint struct {
	Shape string `json:"shape"`
	Color string `json:"color"`
}

// SliderChallenge 滑块题面：带缺口背景 + 拼图块，绝不返回 target_x。
type SliderChallenge struct {
	BgImage    string `json:"bg_image"`
	PieceImage string `json:"piece_image"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	PieceY     int    `json:"piece_y"`
	PieceSize  int    `json:"piece_size"`
}

// CaptchaVerifyProof 客户端回传的原始输入（点选坐标 / 滑块落点与轨迹）。
type CaptchaVerifyProof struct {
	Clicks     []ClickPoint      `json:"clicks"`
	DropX      float64           `json:"drop_x"`
	Trajectory []TrajectoryPoint `json:"trajectory"`
}

// clickCaptchaCacheStore 定义验证码持久化端口，避免 service 反向依赖 repository。
type clickCaptchaCacheStore interface {
	SetChallenge(ctx context.Context, challengeID string, payload *ClickCaptchaChallengePayloadRef) error
	GetChallenge(ctx context.Context, challengeID string) (*ClickCaptchaChallengePayloadRef, error)
	DeleteChallenge(ctx context.Context, challengeID string) error
	SetToken(ctx context.Context, token string, payload *ClickCaptchaTokenPayloadRef) error
	TakeToken(ctx context.Context, token string) (*ClickCaptchaTokenPayloadRef, error)
	PeekToken(ctx context.Context, token string) (*ClickCaptchaTokenPayloadRef, error)
	IncrAttempts(ctx context.Context, challengeID string) (int64, error)
}

// ClickCaptchaChallengePayloadRef 存入 Redis 的 challenge 内部数据（repository 跨包引用）。
// 答案（坐标 / 缺口位置）只存于此，绝不下发。
type ClickCaptchaChallengePayloadRef struct {
	Mode       string            `json:"mode"`
	Difficulty string            `json:"difficulty"`
	ImageClick *ImageClickAnswer `json:"image_click,omitempty"`
	Slider     *SliderAnswer     `json:"slider,omitempty"`
	IPHash     string            `json:"ip_hash"`
	UAHash     string            `json:"ua_hash"`
	CreatedAt  int64             `json:"created_at"`
	ExpiresAt  int64             `json:"expires_at"`
}

// RegistrationClickCaptchaService 自建验证码服务（图片点选 / 拼图滑块）。
type RegistrationClickCaptchaService struct {
	cache      clickCaptchaCacheStore
	settingSvc *SettingService
}

// NewRegistrationClickCaptchaService 创建验证码服务。
func NewRegistrationClickCaptchaService(cache clickCaptchaCacheStore, settingSvc *SettingService) *RegistrationClickCaptchaService {
	return &RegistrationClickCaptchaService{cache: cache, settingSvc: settingSvc}
}

const (
	clickCaptchaChallengeTTL = 120 * time.Second
)

// resolveMode 依据后台设置决定本次题目模式；random 时用 crypto/rand 等权二选一。
func (s *RegistrationClickCaptchaService) resolveMode(mode string) (string, error) {
	switch normalizeRegistrationCaptchaMode(mode) {
	case RegistrationCaptchaModeImageClick:
		return RegistrationCaptchaModeImageClick, nil
	case RegistrationCaptchaModeSlider:
		return RegistrationCaptchaModeSlider, nil
	default:
		idx, err := cryptoRandomInts(2, 1)
		if err != nil {
			return "", err
		}
		if idx[0] == 0 {
			return RegistrationCaptchaModeImageClick, nil
		}
		return RegistrationCaptchaModeSlider, nil
	}
}

// CreateChallenge 生成一道新题并写入 Redis（答案只进 Redis）。
func (s *RegistrationClickCaptchaService) CreateChallenge(ctx context.Context, ipHash, uaHash string) (*RegistrationClickCaptchaChallenge, error) {
	if s == nil || s.cache == nil {
		return nil, ErrRegistrationClickCaptchaDisabled
	}

	mode := RegistrationCaptchaModeRandom
	difficulty := RegistrationCaptchaDifficultyMedium
	if s.settingSvc != nil {
		mode = s.settingSvc.GetRegistrationCaptchaMode(ctx)
		difficulty = s.settingSvc.GetRegistrationCaptchaDifficulty(ctx)
	}
	difficulty = normalizeRegistrationCaptchaDifficulty(difficulty)

	resolved, err := s.resolveMode(mode)
	if err != nil {
		return nil, err
	}

	challengeID, err := randomHexString(16)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	payload := &ClickCaptchaChallengePayloadRef{
		Mode:       resolved,
		Difficulty: difficulty,
		IPHash:     ipHash,
		UAHash:     uaHash,
		CreatedAt:  now.Unix(),
		ExpiresAt:  now.Add(clickCaptchaChallengeTTL).Unix(),
	}
	challenge := &RegistrationClickCaptchaChallenge{
		ChallengeID: challengeID,
		Mode:        resolved,
		ExpiresIn:   int(clickCaptchaChallengeTTL.Seconds()),
	}

	switch resolved {
	case RegistrationCaptchaModeImageClick:
		ic, answer, err := buildImageClickChallenge(difficulty)
		if err != nil {
			return nil, err
		}
		payload.ImageClick = answer
		challenge.ImageClick = ic
	case RegistrationCaptchaModeSlider:
		sl, answer, err := buildSliderChallenge(difficulty)
		if err != nil {
			return nil, err
		}
		payload.Slider = answer
		challenge.Slider = sl
	default:
		return nil, ErrRegistrationClickCaptchaInvalid
	}

	if err := s.cache.SetChallenge(ctx, challengeID, payload); err != nil {
		return nil, err
	}
	return challenge, nil
}

// VerifyChallenge 依据 Redis 中的权威 Mode 校验客户端输入，成功则签发一次性 token。
func (s *RegistrationClickCaptchaService) VerifyChallenge(ctx context.Context, challengeID string, proof CaptchaVerifyProof, ipHash, uaHash string) (string, int64, error) {
	if s == nil || s.cache == nil {
		return "", 0, ErrRegistrationClickCaptchaDisabled
	}
	payload, err := s.cache.GetChallenge(ctx, challengeID)
	if err != nil {
		return "", 0, ErrRegistrationClickCaptchaInvalid
	}
	if payload == nil || time.Now().Unix() > payload.ExpiresAt {
		return "", 0, ErrRegistrationClickCaptchaInvalid
	}
	if payload.IPHash != ipHash || payload.UAHash != uaHash {
		return "", 0, ErrRegistrationClickCaptchaInvalid
	}

	// 单题失败次数上限：超过即删题，防止坐标/落点爆破。
	attempts, err := s.cache.IncrAttempts(ctx, challengeID)
	if err != nil {
		return "", 0, ErrRegistrationClickCaptchaInvalid
	}
	if attempts > clickCaptchaMaxAttempts(payload.Difficulty) {
		_ = s.cache.DeleteChallenge(ctx, challengeID)
		return "", 0, ErrRegistrationClickCaptchaInvalid
	}

	switch payload.Mode {
	case RegistrationCaptchaModeImageClick:
		if err := verifyImageClick(payload.ImageClick, proof.Clicks); err != nil {
			return "", 0, err
		}
	case RegistrationCaptchaModeSlider:
		params := sliderParamsFor(payload.Difficulty)
		if err := verifySlider(payload.Slider, proof.DropX, proof.Trajectory, params.trajectory); err != nil {
			return "", 0, err
		}
	default:
		return "", 0, ErrRegistrationClickCaptchaInvalid
	}

	// 校验成功即删除 challenge，防止同一道题被多次尝试。
	_ = s.cache.DeleteChallenge(ctx, challengeID)

	token, err := randomHexString(24)
	if err != nil {
		return "", 0, err
	}
	now := time.Now()
	ttl := int64(900) // 15 分钟：给用户足够时间收邮件并输入验证码
	tokenPayload := &ClickCaptchaTokenPayloadRef{
		IPHash:    ipHash,
		UAHash:    uaHash,
		CreatedAt: now.Unix(),
		ExpiresAt: now.Unix() + ttl,
	}
	if err := s.cache.SetToken(ctx, token, tokenPayload); err != nil {
		return "", 0, err
	}
	return token, ttl, nil
}

// clickCaptchaMaxAttempts 单题允许的失败次数上限：high=2，其余=3。
func clickCaptchaMaxAttempts(difficulty string) int64 {
	if normalizeRegistrationCaptchaDifficulty(difficulty) == RegistrationCaptchaDifficultyHigh {
		return 2
	}
	return 3
}

// ConsumeToken 一次性校验并消费 token，成功时返回载荷供业务失败后归还。
func (s *RegistrationClickCaptchaService) ConsumeToken(ctx context.Context, token, ipHash, uaHash string) (*ClickCaptchaTokenPayloadRef, error) {
	if s == nil || s.cache == nil {
		return nil, ErrRegistrationClickCaptchaDisabled
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrRegistrationClickCaptchaTokenInvalid
	}
	payload, err := s.cache.TakeToken(ctx, token)
	if err != nil {
		return nil, ErrRegistrationClickCaptchaTokenInvalid
	}
	if payload == nil || time.Now().Unix() > payload.ExpiresAt {
		return nil, ErrRegistrationClickCaptchaTokenInvalid
	}
	if payload.IPHash != ipHash || payload.UAHash != uaHash {
		return nil, ErrRegistrationClickCaptchaTokenInvalid
	}
	return payload, nil
}

// ValidateToken 非破坏性校验一次性 token（不消费），
// 用于服务端强制"发送邮箱验证码前必须通过真人验证"：仅校验存在性、有效期与指纹，
// 不改变 token 状态，后续注册时仍由 ConsumeToken 做一次性消费。
func (s *RegistrationClickCaptchaService) ValidateToken(ctx context.Context, token, ipHash, uaHash string) error {
	if s == nil || s.cache == nil {
		return ErrRegistrationClickCaptchaDisabled
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return ErrRegistrationClickCaptchaTokenInvalid
	}
	payload, err := s.cache.PeekToken(ctx, token)
	if err != nil {
		return ErrRegistrationClickCaptchaTokenInvalid
	}
	if payload == nil || time.Now().Unix() > payload.ExpiresAt {
		return ErrRegistrationClickCaptchaTokenInvalid
	}
	if payload.IPHash != ipHash || payload.UAHash != uaHash {
		return ErrRegistrationClickCaptchaTokenInvalid
	}
	return nil
}

// ClickCaptchaTokenPayloadRef 存入 Redis 的一次性 token 数据（repository 跨包引用）。
type ClickCaptchaTokenPayloadRef struct {
	IPHash    string `json:"ip_hash"`
	UAHash    string `json:"ua_hash"`
	CreatedAt int64  `json:"created_at"`
	ExpiresAt int64  `json:"expires_at"`
}

// cryptoRandomInts 从 [0,poolSize) 中取 count 个不重复随机索引（crypto/rand）。
func cryptoRandomInts(poolSize, count int) ([]int, error) {
	if count > poolSize {
		return nil, fmt.Errorf("count %d exceeds pool %d", count, poolSize)
	}
	seen := make(map[int]struct{}, count)
	out := make([]int, 0, count)
	for len(out) < count {
		buf := make([]byte, 2)
		if _, err := rand.Read(buf); err != nil {
			return nil, err
		}
		v := int(buf[0])<<8 | int(buf[1])
		idx := v % poolSize
		if _, ok := seen[idx]; ok {
			continue
		}
		seen[idx] = struct{}{}
		out = append(out, idx)
	}
	return out, nil
}

// HashFingerprint 生成 IP+UA 的简单指纹哈希。
func HashFingerprint(ip, ua string) string {
	raw := strings.TrimSpace(ip) + "|" + strings.TrimSpace(ua)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// MarshalClickCaptchaChallengePayload 序列化 challenge 载荷。
func MarshalClickCaptchaChallengePayload(p *ClickCaptchaChallengePayloadRef) (string, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// UnmarshalClickCaptchaChallengePayload 反序列化 challenge 载荷。
func UnmarshalClickCaptchaChallengePayload(raw string) (*ClickCaptchaChallengePayloadRef, error) {
	var p ClickCaptchaChallengePayloadRef
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// MarshalClickCaptchaTokenPayload 序列化 token 载荷。
func MarshalClickCaptchaTokenPayload(p *ClickCaptchaTokenPayloadRef) (string, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// UnmarshalClickCaptchaTokenPayload 反序列化 token 载荷。
func UnmarshalClickCaptchaTokenPayload(raw string) (*ClickCaptchaTokenPayloadRef, error) {
	var p ClickCaptchaTokenPayloadRef
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil, err
	}
	return &p, nil
}
