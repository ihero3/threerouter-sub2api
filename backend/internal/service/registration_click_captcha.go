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
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

// ErrRegistrationClickCaptchaDisabled 验证码功能未启用时的错误。
var ErrRegistrationClickCaptchaDisabled = infraerrors.BadRequest("REGISTRATION_CLICK_CAPTCHA_DISABLED", "registration click captcha is disabled")

// ErrRegistrationClickCaptchaInvalid 点击顺序错误或 challenge 无效。
var ErrRegistrationClickCaptchaInvalid = infraerrors.BadRequest("REGISTRATION_CLICK_CAPTCHA_INVALID", "registration click captcha challenge invalid")

// ErrRegistrationClickCaptchaTokenInvalid 一次性 token 无效、过期或已使用。
var ErrRegistrationClickCaptchaTokenInvalid = infraerrors.BadRequest("REGISTRATION_CLICK_CAPTCHA_TOKEN_INVALID", "registration click captcha token invalid or expired, please complete the verification again")

// RegistrationClickCaptchaChallenge 一次点击验证题。
type RegistrationClickCaptchaChallenge struct {
	ChallengeID string             `json:"challenge_id"`
	ExpiresIn   int                `json:"expires_in"`
	Prompt      []string           `json:"prompt"`
	Grid        []ClickCaptchaCell `json:"grid"`
}

// ClickCaptchaCell 单个格子。
type ClickCaptchaCell struct {
	CellID  string `json:"cell_id"`
	Content string `json:"content"`
	Type    string `json:"type"`
}

// clickCaptchaCacheStore 定义验证码持久化端口，避免 service 反向依赖 repository。
type clickCaptchaCacheStore interface {
	SetChallenge(ctx context.Context, challengeID string, payload *ClickCaptchaChallengePayloadRef) error
	GetChallenge(ctx context.Context, challengeID string) (*ClickCaptchaChallengePayloadRef, error)
	DeleteChallenge(ctx context.Context, challengeID string) error
	SetToken(ctx context.Context, token string, payload *ClickCaptchaTokenPayloadRef) error
	TakeToken(ctx context.Context, token string) (*ClickCaptchaTokenPayloadRef, error)
	PeekToken(ctx context.Context, token string) (*ClickCaptchaTokenPayloadRef, error)
}

// clickCaptchaPrompt 一个提示项（文本/emoji 均可）。
type clickCaptchaPrompt struct {
	ID      string
	Content string
	CellID  string
}

// ClickCaptchaChallengePayloadRef 存入 Redis 的 challenge 内部数据（repository 跨包引用）。
type ClickCaptchaChallengePayloadRef struct {
	PromptIDs   []string           `json:"prompt_ids"`
	AnswerCells []string           `json:"answer_cells"`
	Grid        []ClickCaptchaCell `json:"grid"`
	IPHash      string             `json:"ip_hash"`
	UAHash      string             `json:"ua_hash"`
	CreatedAt   int64              `json:"created_at"`
	ExpiresAt   int64              `json:"expires_at"`
}

// RegistrationClickCaptchaService 自建顺序点击验证码服务。
type RegistrationClickCaptchaService struct {
	cache clickCaptchaCacheStore
}

// NewRegistrationClickCaptchaService 创建验证码服务。
func NewRegistrationClickCaptchaService(cache clickCaptchaCacheStore) *RegistrationClickCaptchaService {
	return &RegistrationClickCaptchaService{cache: cache}
}

// clickCaptchaWordBank 词库；同一词随机映射到文字或 emoji，避免脚本固定按文本匹配。
var clickCaptchaWordBank = []struct {
	Text  string
	Emoji string
}{
	{Text: "苹果", Emoji: "🍎"},
	{Text: "香蕉", Emoji: "🍌"},
	{Text: "火车", Emoji: "🚂"},
	{Text: "飞机", Emoji: "✈️"},
	{Text: "汽车", Emoji: "🚗"},
	{Text: "房子", Emoji: "🏠"},
	{Text: "月亮", Emoji: "🌙"},
	{Text: "太阳", Emoji: "☀️"},
	{Text: "狮子", Emoji: "🦁"},
	{Text: "老虎", Emoji: "🐯"},
	{Text: "大树", Emoji: "🌳"},
	{Text: "花朵", Emoji: "🌸"},
}

const (
	clickCaptchaGridSize     = 9 // 3x3
	clickCaptchaClickCount   = 3 // 需要点击的目标数
	clickCaptchaChallengeTTL = 120 * time.Second
)

// CreateChallenge 生成一道新题并写入 Redis。
func (s *RegistrationClickCaptchaService) CreateChallenge(ctx context.Context, ipHash, uaHash string) (*RegistrationClickCaptchaChallenge, error) {
	if s == nil || s.cache == nil {
		return nil, ErrRegistrationClickCaptchaDisabled
	}

	indices, err := cryptoRandomInts(len(clickCaptchaWordBank), clickCaptchaGridSize)
	if err != nil {
		return nil, err
	}
	grid := make([]ClickCaptchaCell, 0, clickCaptchaGridSize)
	cellIDs := make([]string, 0, clickCaptchaGridSize)
	for i, idx := range indices {
		word := clickCaptchaWordBank[idx]
		cellID := fmt.Sprintf("c%d", i)
		// 全部使用 emoji，避免暴露文字给 OCR / 文本识别型机器人。
		grid = append(grid, ClickCaptchaCell{CellID: cellID, Content: word.Emoji, Type: "emoji"})
		cellIDs = append(cellIDs, cellID)
	}

	pickIndices, err := cryptoRandomInts(clickCaptchaGridSize, clickCaptchaClickCount)
	if err != nil {
		return nil, err
	}
	promptIDs := make([]string, 0, clickCaptchaClickCount)
	answerCells := make([]string, 0, clickCaptchaClickCount)
	prompt := make([]string, 0, clickCaptchaClickCount)
	for _, pi := range pickIndices {
		cellID := cellIDs[pi]
		cell := grid[pi]
		promptIDs = append(promptIDs, cellID)
		answerCells = append(answerCells, cellID)
		// 提示直接展示 emoji 目标，用户按图找图，机器人需图像识别。
		prompt = append(prompt, cell.Content)
	}

	challengeID, err := randomHexString(16)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	payload := &ClickCaptchaChallengePayloadRef{
		PromptIDs:   promptIDs,
		AnswerCells: answerCells,
		Grid:        grid,
		IPHash:      ipHash,
		UAHash:      uaHash,
		CreatedAt:   now.Unix(),
		ExpiresAt:   now.Add(clickCaptchaChallengeTTL).Unix(),
	}
	if err := s.cache.SetChallenge(ctx, challengeID, payload); err != nil {
		return nil, err
	}

	return &RegistrationClickCaptchaChallenge{
		ChallengeID: challengeID,
		ExpiresIn:   int(clickCaptchaChallengeTTL.Seconds()),
		Prompt:      prompt,
		Grid:        grid,
	}, nil
}

// VerifyChallenge 校验点击顺序并签发一次性 token。
func (s *RegistrationClickCaptchaService) VerifyChallenge(ctx context.Context, challengeID string, clicks []string, ipHash, uaHash string) (string, int64, error) {
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
	if len(clicks) != len(payload.AnswerCells) {
		return "", 0, ErrRegistrationClickCaptchaInvalid
	}
	for i := range clicks {
		if clicks[i] != payload.AnswerCells[i] {
			return "", 0, ErrRegistrationClickCaptchaInvalid
		}
	}

	// 校验成功即删除 challenge，防止同一道题被多次尝试。
	_ = s.cache.DeleteChallenge(ctx, challengeID)

	token, err := randomHexString(24)
	if err != nil {
		return "", 0, err
	}
	now := time.Now()
	ttl := int64(300) // 5 分钟
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

// RestoreToken 将已消费但注册业务失败的 token 归还缓存，
// 保证"验证码错误/邮箱冲突"等注册失败不烧掉人机验证结果，用户可携同一 token 重试。
// 归还沿用原 ExpiresAt（缓存实现按剩余有效期写入）；已过期或载荷为空则不归还。
func (s *RegistrationClickCaptchaService) RestoreToken(ctx context.Context, token string, payload *ClickCaptchaTokenPayloadRef) error {
	if s == nil || s.cache == nil || payload == nil || strings.TrimSpace(token) == "" {
		return ErrRegistrationClickCaptchaTokenInvalid
	}
	if time.Now().Unix() >= payload.ExpiresAt {
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

// ClickCaptchaTokenPayloadRef 存入 Redis 的一次性 token 数据（repository 跨包引用）。
type ClickCaptchaTokenPayloadRef struct {
	IPHash    string `json:"ip_hash"`
	UAHash    string `json:"ua_hash"`
	CreatedAt int64  `json:"created_at"`
	ExpiresAt int64  `json:"expires_at"`
}

// cryptoRandomInts 从 [0,poolSize) 中取 count 个不重复随机索引。
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

var _ = logger.L
var _ = zap.String
