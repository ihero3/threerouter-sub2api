package service

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

// 支持的公司注册法域（合规方案 4.4.1）。
const (
	JurisdictionEU           = "EU"
	JurisdictionChina        = "China"
	JurisdictionUSCalifornia = "US-California"
)

// 风险等级。
const (
	JurisdictionRiskLow    = "low"
	JurisdictionRiskMedium = "medium"
	JurisdictionRiskHigh   = "high"
)

// JurisdictionMappingRequest 跨法域合规映射请求。
type JurisdictionMappingRequest struct {
	CompanyRegion string `json:"company_region"` // EU / China / US-California
	Industry      string `json:"industry"`       // healthcare / finance / ecommerce / education / ...
	ServiceType   string `json:"service_type"`   // ai_chatbot / ai_analysis / ai_recommendation
}

// JurisdictionMappingResult 跨法域合规映射结果。
type JurisdictionMappingResult struct {
	CompanyRegion         string   `json:"company_region"`
	Industry              string   `json:"industry"`
	ServiceType           string   `json:"service_type"`
	ApplicableRegulations []string `json:"applicable_regulations"`
	RequiredMeasures      []string `json:"required_measures"`
	RiskLevel             string   `json:"risk_level"`
	RecommendedActions    []string `json:"recommended_actions"`
}

// UserJurisdictionMapping 存储用户的跨法域映射配置。
type UserJurisdictionMapping struct {
	ID                    int64     `json:"id"`
	UserID                int64     `json:"user_id"`
	CompanyRegion         string    `json:"company_region"`
	Industry              string    `json:"industry"`
	ServiceType           string    `json:"service_type"`
	RiskLevel             string    `json:"risk_level"`
	ApplicableRegulations []string  `json:"applicable_regulations"`
	RequiredMeasures      []string  `json:"required_measures"`
	RecommendedActions    []string  `json:"recommended_actions"`
	AppliedRules          []string  `json:"applied_rules"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// UserJurisdictionMappingRepository 定义用户跨法域映射仓储接口。
type UserJurisdictionMappingRepository interface {
	GetByUserID(ctx context.Context, userID int64) (*UserJurisdictionMapping, error)
	Create(ctx context.Context, mapping *UserJurisdictionMapping) error
	Update(ctx context.Context, mapping *UserJurisdictionMapping) error
	Upsert(ctx context.Context, mapping *UserJurisdictionMapping) error
	Delete(ctx context.Context, userID int64) error
}

// jurisdictionProfile 描述单个法域的基线法规与措施。
type jurisdictionProfile struct {
	regulations []string
	measures    []string
	actions     []string
}

var jurisdictionProfiles = map[string]jurisdictionProfile{
	JurisdictionEU: {
		regulations: []string{"GDPR", "EU AI Act", "NIS2"},
		measures: []string{
			"Data Protection Impact Assessment (DPIA)",
			"Transparency Notice & User Consent Management",
			"Data Subject Rights Response (Access/Correction/Deletion/Portability)",
			"Security Measures & Incident Reporting (NIS2)",
		},
		actions: []string{
			"Maintain GDPR Art 30 Records of Processing Activities (ROPA)",
			"Clarify role positioning & transparency statement in AI service chain",
			"Adopt SCC or equivalent safeguards for cross-border transfers",
		},
	},
	JurisdictionChina: {
		regulations: []string{"Data Security Law", "Personal Information Protection Law", "Interim Measures for Generative AI Service Management"},
		measures: []string{
			"Data Localization Storage",
			"Content Safety Review & Violation Blocking",
			"Algorithm & Service Filing",
			"Personal Information Processing Notice & Separate Consent",
		},
		actions: []string{
			"Complete generative AI service filing & security assessment",
			"Deploy content moderation rules & post-review (output side)",
			"Ensure important data does not cross border or passes security assessment",
		},
	},
	JurisdictionUSCalifornia: {
		regulations: []string{"CCPA", "CPRA"},
		measures: []string{
			"Consumer Right to Know & Delete Response",
			"Data Minimization & Purpose Limitation",
			"Transparent Privacy Notice (Notice at Collection)",
			"Sensitive Personal Information Opt-Out Mechanism",
		},
		actions: []string{
			"Provide Do Not Sell/Share options",
			"Publish privacy policy with data categories & purposes",
			"Establish consumer request handling & verification process",
		},
	},
}

// 高风险行业（触发额外人工监督与审计要求）。
var jurisdictionHighRiskIndustries = map[string]bool{
	"healthcare": true,
	"finance":    true,
}

// ComplianceMappingService 提供跨法域合规映射（合规方案 4.4.1）。
//
// 采用静态规则映射（法域 × 行业 × 服务类型 → 适用法规/措施/风险等级/建议），
// 支持保存用户映射配置并自动应用到合规规则。
type ComplianceMappingService struct {
	repo                  UserJurisdictionMappingRepository
	ruleService           *ModerationRuleService
	complianceProfileService *UserComplianceProfileService
}

// NewComplianceMappingService 创建跨法域合规映射服务。
func NewComplianceMappingService(repo UserJurisdictionMappingRepository, ruleService *ModerationRuleService, complianceProfileService *UserComplianceProfileService) *ComplianceMappingService {
	return &ComplianceMappingService{
		repo:                     repo,
		ruleService:              ruleService,
		complianceProfileService: complianceProfileService,
	}
}

// SupportedJurisdictions 返回支持的法域列表。
func (s *ComplianceMappingService) SupportedJurisdictions() []string {
	return []string{JurisdictionEU, JurisdictionChina, JurisdictionUSCalifornia}
}

// MapJurisdiction 执行跨法域合规映射。CompanyRegion 为必填且须为受支持法域。
func (s *ComplianceMappingService) MapJurisdiction(ctx context.Context, req JurisdictionMappingRequest) (*JurisdictionMappingResult, error) {
	region := normalizeJurisdiction(req.CompanyRegion)
	profile, ok := jurisdictionProfiles[region]
	if !ok {
		return nil, fmt.Errorf("unsupported company_region: %s", req.CompanyRegion)
	}
	industry := strings.TrimSpace(req.Industry)
	serviceType := strings.TrimSpace(req.ServiceType)

	regulations := append([]string(nil), profile.regulations...)
	measures := append([]string(nil), profile.measures...)
	actions := append([]string(nil), profile.actions...)

	riskLevel := JurisdictionRiskMedium
	if jurisdictionHighRiskIndustries[strings.ToLower(industry)] {
		riskLevel = JurisdictionRiskHigh
		measures = appendUnique(measures, "Mandatory Human Oversight (high-risk use case)")
		actions = appendUnique(actions, "Establish complete audit trail & human review mechanism for high-risk scenarios")
	}

	switch strings.ToLower(serviceType) {
	case "ai_recommendation":
		measures = appendUnique(measures, "User Profiling & Automated Decision Notice")
		if region == JurisdictionEU {
			actions = appendUnique(actions, "Assess if falling under EU AI Act transparency obligations")
		}
	case "ai_analysis":
		measures = appendUnique(measures, "Analysis Purpose Limitation & Data Minimization")
	case "ai_chatbot":
		measures = appendUnique(measures, "AI Interaction Identity Disclosure (users must be informed they are interacting with AI)")
	}

	sort.Strings(regulations)
	return &JurisdictionMappingResult{
		CompanyRegion:         region,
		Industry:              industry,
		ServiceType:           serviceType,
		ApplicableRegulations: regulations,
		RequiredMeasures:      measures,
		RiskLevel:             riskLevel,
		RecommendedActions:    actions,
	}, nil
}

func normalizeJurisdiction(region string) string {
	r := strings.TrimSpace(region)
	switch strings.ToLower(r) {
	case "eu", "europe", "european union":
		return JurisdictionEU
	case "china", "cn", "中国":
		return JurisdictionChina
	case "us-california", "california", "us-ca", "ccpa", "cpra":
		return JurisdictionUSCalifornia
	default:
		return r
	}
}

func appendUnique(list []string, value string) []string {
	for _, item := range list {
		if item == value {
			return list
		}
	}
	return append(list, value)
}

// GetUserMapping 返回用户保存的跨法域映射配置。
func (s *ComplianceMappingService) GetUserMapping(ctx context.Context, userID int64) (*UserJurisdictionMapping, error) {
	if s.repo == nil {
		return nil, nil
	}
	return s.repo.GetByUserID(ctx, userID)
}

// SaveMapping 保存映射结果到数据库。
func (s *ComplianceMappingService) SaveMapping(ctx context.Context, userID int64, result *JurisdictionMappingResult) error {
	if s.repo == nil {
		return nil
	}
	mapping := &UserJurisdictionMapping{
		UserID:                userID,
		CompanyRegion:         result.CompanyRegion,
		Industry:              result.Industry,
		ServiceType:           result.ServiceType,
		RiskLevel:             result.RiskLevel,
		ApplicableRegulations: result.ApplicableRegulations,
		RequiredMeasures:      result.RequiredMeasures,
		RecommendedActions:    result.RecommendedActions,
		AppliedRules:          []string{},
	}
	return s.repo.Upsert(ctx, mapping)
}

// ApplyMappingToRules 根据映射结果自动配置用户的合规规则。
func (s *ComplianceMappingService) ApplyMappingToRules(ctx context.Context, userID int64, result *JurisdictionMappingResult) ([]string, error) {
	if s.ruleService == nil || s.complianceProfileService == nil {
		return []string{}, nil
	}

	rules, err := s.ruleService.ListRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("list rules: %w", err)
	}

	enabledRuleIDs := make([]string, 0)
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if shouldEnableRule(rule, result) {
			enabledRuleIDs = append(enabledRuleIDs, rule.RuleID)
		}
	}

	if _, err := s.complianceProfileService.UpdateProfile(ctx, userID, UpdateProfileInput{
		ModerationPolicy: &UserModerationPolicy{EnabledRuleIDs: enabledRuleIDs},
	}); err != nil {
		return nil, fmt.Errorf("update compliance profile: %w", err)
	}

	if s.repo != nil {
		mapping, err := s.repo.GetByUserID(ctx, userID)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("get mapping: %w", err)
		}
		if mapping != nil {
			mapping.AppliedRules = enabledRuleIDs
			if err := s.repo.Update(ctx, mapping); err != nil {
				return nil, fmt.Errorf("update mapping applied rules: %w", err)
			}
		}
	}

	return enabledRuleIDs, nil
}

// shouldEnableRule 根据映射结果判断是否应该启用某条规则。
func shouldEnableRule(rule ModerationRule, result *JurisdictionMappingResult) bool {
	category := strings.ToLower(rule.RiskCategory)
	region := strings.ToLower(result.CompanyRegion)

	switch region {
	case "eu":
		return category == "privacy" || category == "pornography" || category == "violence" || category == "hate"
	case "china":
		return category == "privacy" || category == "pornography" || category == "violence" || category == "political" || category == "minors"
	case "us-california":
		return category == "privacy" || category == "pornography" || category == "violence"
	default:
		return true
	}
}
