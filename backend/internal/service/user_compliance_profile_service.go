package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// 用户级合规档案服务（UserComplianceProfileService）
// 见 docs/合规升级方案.md 4.1 节与 5.4/5.5 节。
// ============================================================================

// UserModerationPolicy 定义用户启用的内容审核规则策略。
type UserModerationPolicy struct {
	EnabledRuleIDs []string `json:"enabled_rule_ids"`
}

// UserComplianceProfile 是用户级合规档案模型。
type UserComplianceProfile struct {
	ID                   int64                `json:"id"`
	UserID               int64                `json:"user_id"`
	ActiveTemplateCode   *string              `json:"active_template_code"`
	ZDRMode              string               `json:"zdr_mode"`
	DetailRetentionDays  int                  `json:"detail_retention_days"`
	ComplianceFrameworks []string             `json:"compliance_frameworks"`
	ModerationPolicy     UserModerationPolicy `json:"moderation_policy"`
	CreatedAt            time.Time            `json:"created_at"`
	UpdatedAt            time.Time            `json:"updated_at"`
}

// UserComplianceProfileRepository 定义用户合规档案持久化。
type UserComplianceProfileRepository interface {
	GetByUserID(ctx context.Context, userID int64) (*UserComplianceProfile, error)
	Create(ctx context.Context, profile *UserComplianceProfile) error
	Update(ctx context.Context, profile *UserComplianceProfile) error
	Upsert(ctx context.Context, profile *UserComplianceProfile) error
}

// UserComplianceProfileService 提供用户级合规档案的查询、创建与更新能力。
type UserComplianceProfileService struct {
	repo UserComplianceProfileRepository

	// 本地内存缓存（TTL 60 秒）
	cache   map[int64]*cachedProfile
	cacheMu sync.RWMutex
	cacheTTL time.Duration
}

type cachedProfile struct {
	profile   *UserComplianceProfile
	expiresAt time.Time
}

// NewUserComplianceProfileService 构造用户合规档案服务。
func NewUserComplianceProfileService(repo UserComplianceProfileRepository) *UserComplianceProfileService {
	return &UserComplianceProfileService{
		repo:     repo,
		cache:    make(map[int64]*cachedProfile),
		cacheTTL: 60 * time.Second,
	}
}

// GetOrCreateDefault 获取用户合规档案；不存在时创建默认档案。
func (s *UserComplianceProfileService) GetOrCreateDefault(ctx context.Context, userID int64) (*UserComplianceProfile, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user_id")
	}
	// 先查缓存
	if p := s.getFromCache(userID); p != nil {
		return p, nil
	}

	profile, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user compliance profile: %w", err)
	}
	if profile != nil {
		s.setCache(userID, profile)
		return profile, nil
	}

	// 创建默认档案
	profile = &UserComplianceProfile{
		UserID:               userID,
		ZDRMode:              "aggregate_only",
		DetailRetentionDays:  7,
		ComplianceFrameworks: []string{"gdpr"},
		ModerationPolicy:     UserModerationPolicy{EnabledRuleIDs: []string{}},
	}
	if err := s.repo.Create(ctx, profile); err != nil {
		return nil, fmt.Errorf("create default user compliance profile: %w", err)
	}
	s.setCache(userID, profile)
	return profile, nil
}

// UpdateProfileInput 是更新合规档案的入参。
type UpdateProfileInput struct {
	ZDRMode              *string              `json:"zdr_mode,omitempty"`
	DetailRetentionDays  *int                 `json:"detail_retention_days,omitempty"`
	ComplianceFrameworks []string             `json:"compliance_frameworks,omitempty"`
	ModerationPolicy     *UserModerationPolicy `json:"moderation_policy,omitempty"`
}

// GetActiveTemplateCode 返回当前用户应用的合规模板编码。
func (s *UserComplianceProfileService) GetActiveTemplateCode(ctx context.Context, userID int64) (string, error) {
	if userID <= 0 {
		return "", nil
	}
	profile, err := s.GetOrCreateDefault(ctx, userID)
	if err != nil {
		return "", err
	}
	if profile.ActiveTemplateCode == nil {
		return "", nil
	}
	return *profile.ActiveTemplateCode, nil
}

// UpdateProfile 更新用户合规档案。
func (s *UserComplianceProfileService) UpdateProfile(ctx context.Context, userID int64, input UpdateProfileInput) (*UserComplianceProfile, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user_id")
	}
	profile, err := s.GetOrCreateDefault(ctx, userID)
	if err != nil {
		return nil, err
	}
	if input.ZDRMode != nil {
		profile.ZDRMode = *input.ZDRMode
	}
	if input.DetailRetentionDays != nil {
		profile.DetailRetentionDays = *input.DetailRetentionDays
	}
	if input.ComplianceFrameworks != nil {
		profile.ComplianceFrameworks = input.ComplianceFrameworks
	}
	if input.ModerationPolicy != nil {
		profile.ModerationPolicy = *input.ModerationPolicy
	}
	if err := s.repo.Update(ctx, profile); err != nil {
		return nil, fmt.Errorf("update user compliance profile: %w", err)
	}
	s.setCache(userID, profile)
	return profile, nil
}

// ApplyTemplate 将指定模板应用到用户合规档案，并联动设置合规配置。
func (s *UserComplianceProfileService) ApplyTemplate(ctx context.Context, userID int64, templateCode string) (*UserComplianceProfile, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user_id")
	}
	profile, err := s.GetOrCreateDefault(ctx, userID)
	if err != nil {
		return nil, err
	}
	profile.ActiveTemplateCode = &templateCode

	templateConfig := s.resolveTemplateConfig(templateCode)
	if templateConfig.ZDRMode != "" {
		profile.ZDRMode = templateConfig.ZDRMode
	}
	if len(templateConfig.ComplianceFrameworks) > 0 {
		profile.ComplianceFrameworks = templateConfig.ComplianceFrameworks
	}
	if templateConfig.DetailRetentionDays > 0 {
		profile.DetailRetentionDays = templateConfig.DetailRetentionDays
	}

	if err := s.repo.Update(ctx, profile); err != nil {
		return nil, fmt.Errorf("apply template to user compliance profile: %w", err)
	}
	s.setCache(userID, profile)
	return profile, nil
}

type templateConfig struct {
	ZDRMode              string
	ComplianceFrameworks []string
	DetailRetentionDays  int
}

func (s *UserComplianceProfileService) resolveTemplateConfig(templateCode string) templateConfig {
	switch strings.ToLower(templateCode) {
	case "medical_default", "medical_custom":
		return templateConfig{
			ZDRMode:              "audit",
			ComplianceFrameworks: []string{"HIPAA", "GDPR", "EU_AI_Act"},
			DetailRetentionDays:  30,
		}
	case "finance_default", "finance_custom":
		return templateConfig{
			ZDRMode:              "audit",
			ComplianceFrameworks: []string{"GDPR", "EU_AI_Act", "Data Security Law"},
			DetailRetentionDays:  90,
		}
	case "ecommerce_default", "ecommerce_custom":
		return templateConfig{
			ZDRMode:              "audit",
			ComplianceFrameworks: []string{"GDPR", "EU_AI_Act", "Personal Information Protection Law"},
			DetailRetentionDays:  45,
		}
	case "education_default", "education_custom":
		return templateConfig{
			ZDRMode:              "audit",
			ComplianceFrameworks: []string{"GDPR", "EU_AI_Act"},
			DetailRetentionDays:  60,
		}
	case "enterprise_default", "enterprise_custom":
		return templateConfig{
			ZDRMode:              "audit",
			ComplianceFrameworks: []string{"GDPR", "EU_AI_Act", "Data Security Law"},
			DetailRetentionDays:  7,
		}
	default:
		return templateConfig{
			ZDRMode:              "aggregate_only",
			ComplianceFrameworks: []string{"EU_AI_Act"},
			DetailRetentionDays:  7,
		}
	}
}

// GetEffectiveZDRSettings 返回用户生效的 ZDR 设置。
// aggregateOnly=true 表示仅保留聚合数据；false 表示 audit 模式，返回明细保留天数和到期时间。
func (s *UserComplianceProfileService) GetEffectiveZDRSettings(ctx context.Context, userID int64) (aggregateOnly bool, retentionDays int, retentionExpiresAt *time.Time, err error) {
	if userID <= 0 {
		return true, 7, nil, nil
	}
	profile, err := s.GetOrCreateDefault(ctx, userID)
	if err != nil {
		return true, 7, nil, err
	}
	if profile.ZDRMode == "audit" {
		expiresAt := time.Now().AddDate(0, 0, profile.DetailRetentionDays)
		return false, profile.DetailRetentionDays, &expiresAt, nil
	}
	return true, profile.DetailRetentionDays, nil, nil
}

// GetEnabledRuleIDs 返回用户启用的内容审核规则 ID 列表。
// 如果为空（默认），表示启用所有全局规则。
func (s *UserComplianceProfileService) GetEnabledRuleIDs(ctx context.Context, userID int64) ([]string, error) {
	if userID <= 0 {
		return nil, nil
	}
	profile, err := s.GetOrCreateDefault(ctx, userID)
	if err != nil {
		return nil, err
	}
	return profile.ModerationPolicy.EnabledRuleIDs, nil
}

// InvalidateCache 使指定用户的缓存失效（用于更新后）。
func (s *UserComplianceProfileService) InvalidateCache(userID int64) {
	s.cacheMu.Lock()
	delete(s.cache, userID)
	s.cacheMu.Unlock()
}

func (s *UserComplianceProfileService) getFromCache(userID int64) *UserComplianceProfile {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	if cp, ok := s.cache[userID]; ok && cp.expiresAt.After(time.Now()) {
		return cp.profile
	}
	return nil
}

func (s *UserComplianceProfileService) setCache(userID int64, profile *UserComplianceProfile) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.cache[userID] = &cachedProfile{
		profile:   profile,
		expiresAt: time.Now().Add(s.cacheTTL),
	}
}
