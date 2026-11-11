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

// 验证码模式：random = 三种随机出现；image_click / slider / rotation = 固定为对应模式。
const (
	RegistrationCaptchaModeRandom     = "random"
	RegistrationCaptchaModeImageClick = "image_click"
	RegistrationCaptchaModeSlider     = "slider"
	RegistrationCaptchaModeRotation   = "rotation"
)

// 验证码难度。
const (
	RegistrationCaptchaDifficultyLow    = "low"
	RegistrationCaptchaDifficultyMedium = "medium"
	RegistrationCaptchaDifficultyHigh   = "high"
)

// 归一化模式：非法/空值回退 random（安全默认，三种并存）。
func normalizeRegistrationCaptchaMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case RegistrationCaptchaModeImageClick:
		return RegistrationCaptchaModeImageClick
	case RegistrationCaptchaModeSlider:
		return RegistrationCaptchaModeSlider
	case RegistrationCaptchaModeRotation:
		return RegistrationCaptchaModeRotation
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

// ErrRegistrationClickCaptchaSendLimit 同一个一次性 token 已绑定另一个发码邮箱。
//
// 背景：发码接口此前用非消费式校验（ValidateToken 只看不动），一次人机验证拿到的
// token 可以反复调用 /auth/send-verify-code 给**任意邮箱**发信——刷 SMTP 日配额、
// 骚扰第三方邮箱。发码额度绑定邮箱地址：同一 token 可以给**同一邮箱**重发（邮件
// 延迟时的正常重试不受影响），但换一个邮箱即拒绝。绑定独立于 token 本身：token
// 仍由注册接口做一次性消费，这里只锁"一次解题 = 一个收件地址"。
var ErrRegistrationClickCaptchaSendLimit = infraerrors.TooManyRequests("REGISTRATION_CLICK_CAPTCHA_SEND_LIMIT", "this captcha is already used by another email address, please complete the verification again")

// RegistrationClickCaptchaChallenge 一次验证题（判别联合：按 mode 三选一）。
type RegistrationClickCaptchaChallenge struct {
	ChallengeID string               `json:"challenge_id"`
	Mode        string               `json:"mode"` // image_click | slider | rotation
	ExpiresIn   int                  `json:"expires_in"`
	ImageClick  *ImageClickChallenge `json:"image_click,omitempty"`
	Slider      *SliderChallenge     `json:"slider,omitempty"`
	Rotation    *RotationChallenge   `json:"rotation,omitempty"`
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

// CaptchaVerifyProof 客户端回传的原始输入（点选坐标 / 滑块落点 / 旋转角度 + 轨迹）。
type CaptchaVerifyProof struct {
	Clicks     []ClickPoint      `json:"clicks"`
	DropX      float64           `json:"drop_x"`
	Angle      float64           `json:"angle"`
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
	// ClaimTokenSendEmail 把一次性 token 绑定到唯一发码邮箱（原子），返回当前
	// 绑定的邮箱地址：首次绑定返回请求的地址，已绑定则返回既有地址供比较。
	ClaimTokenSendEmail(ctx context.Context, token, email string) (string, error)
	// ResetTokenSendEmail 解除发码邮箱绑定（归还额度），仅由发信失败的调用方使用。
	ResetTokenSendEmail(ctx context.Context, token string) error
}

// ClickCaptchaChallengePayloadRef 存入 Redis 的 challenge 内部数据（repository 跨包引用）。
// 答案（坐标 / 缺口位置 / 目标角度）只存于此，绝不下发。
type ClickCaptchaChallengePayloadRef struct {
	Mode       string            `json:"mode"`
	Difficulty string            `json:"difficulty"`
	ImageClick *ImageClickAnswer `json:"image_click,omitempty"`
	Slider     *SliderAnswer     `json:"slider,omitempty"`
	Rotation   *RotationAnswer   `json:"rotation,omitempty"`
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

// resolveMode 依据后台设置决定本次题目模式；random 时用 crypto/rand 等权三选一。
func (s *RegistrationClickCaptchaService) resolveMode(mode string) (string, error) {
	switch normalizeRegistrationCaptchaMode(mode) {
	case RegistrationCaptchaModeImageClick:
		return RegistrationCaptchaModeImageClick, nil
	case RegistrationCaptchaModeSlider:
		return RegistrationCaptchaModeSlider, nil
	case RegistrationCaptchaModeRotation:
		return RegistrationCaptchaModeRotation, nil
	default:
		idx, err := cryptoRandomInts(3, 1)
		if err != nil {
			return "", err
		}
		switch idx[0] {
		case 0:
			return RegistrationCaptchaModeImageClick, nil
		case 1:
			return RegistrationCaptchaModeSlider, nil
		default:
			return RegistrationCaptchaModeRotation, nil
		}
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
	case RegistrationCaptchaModeRotation:
		rt, answer, err := buildRotationChallenge(difficulty)
		if err != nil {
			return nil, err
		}
		payload.Rotation = answer
		challenge.Rotation = rt
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
	case RegistrationCaptchaModeRotation:
		params := rotationParamsFor(payload.Difficulty)
		if err := verifyRotation(payload.Rotation, proof.Angle, proof.Trajectory, params.trajectory); err != nil {
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

// RestoreToken 归还已消费的一次性 token，用于建号后续步骤失败时
// 避免用户重新做一次人机验证。仅当 ConsumeToken 成功取到载荷后才应调用。
// 归还保留原 ExpiresAt；若 token 已过期则自然失效，返回 nil。
func (s *RegistrationClickCaptchaService) RestoreToken(ctx context.Context, token string, payload *ClickCaptchaTokenPayloadRef) error {
	if s == nil || s.cache == nil || payload == nil {
		return ErrRegistrationClickCaptchaDisabled
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return ErrRegistrationClickCaptchaTokenInvalid
	}
	return s.cache.SetToken(ctx, token, payload)
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

// ClaimTokenSendQuota 校验一次性 token（不消费）并把发码额度绑定到该邮箱。
//
// 与 ValidateToken 的差别：本方法把 token 与邮箱的绑定原子写入 Redis——同一
// token 之后只允许给**同一个邮箱**发码（重发不限次），换邮箱即拒绝。绑定独立于
// token 生命周期：注册接口仍按原逻辑一次性消费 token，这里不销毁、不归还 token。
//
// 调用方在**触发发信的调用失败**时应调用 ReleaseTokenSendQuota 归还额度，
// 避免瞬时故障迫使用户重新通过一次人机验证。
func (s *RegistrationClickCaptchaService) ClaimTokenSendQuota(ctx context.Context, token, email, ipHash, uaHash string) error {
	if s == nil || s.cache == nil {
		return ErrRegistrationClickCaptchaDisabled
	}
	if err := s.ValidateToken(ctx, token, ipHash, uaHash); err != nil {
		return err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	bound, err := s.cache.ClaimTokenSendEmail(ctx, strings.TrimSpace(token), email)
	if err != nil {
		// 绑定不可用（Redis 故障）时按 token 无效处理：宁可让用户重做一次
		// 人机验证，也不能让"额度不可知"退化为无限给任意邮箱发信。
		return ErrRegistrationClickCaptchaTokenInvalid
	}
	if bound != email {
		return ErrRegistrationClickCaptchaSendLimit
	}
	return nil
}

// ReleaseTokenSendQuota 归还发码额度（解除邮箱绑定），仅在发信失败时调用。
func (s *RegistrationClickCaptchaService) ReleaseTokenSendQuota(ctx context.Context, token string) error {
	if s == nil || s.cache == nil {
		return ErrRegistrationClickCaptchaDisabled
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return ErrRegistrationClickCaptchaTokenInvalid
	}
	return s.cache.ResetTokenSendEmail(ctx, token)
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
