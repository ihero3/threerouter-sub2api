package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// ============================================================================
// 行业合规策略模板（CompliancePolicyTemplate）
// 见 docs/合规方案.md 4.4.2 节，持久化表 compliance_policy_templates（migration 164）。
//
// 预置电商/金融/医疗等行业模板，支持列表、应用到组织、创建自定义模板。
// ============================================================================

// CompliancePolicyTemplate 是一条行业合规策略模板。
//
// Rules 为策略规则集合（键值对数组，见 4.4.2 节 JSON 示例），
// RiskTags 为该模板关联的风险标签列表。
type CompliancePolicyTemplate struct {
	ID           int64            `json:"id"`
	TemplateCode string           `json:"template_code"`
	Industry     string           `json:"industry"`
	Description  string           `json:"description"`
	Rules        []map[string]any `json:"rules"`
	RiskTags     []string         `json:"risk_tags"`
	CreatedAt    time.Time        `json:"created_at"`
}

// CompliancePolicyTemplateRepository 定义行业合规模板持久化。
type CompliancePolicyTemplateRepository interface {
	List(ctx context.Context, industry string) ([]CompliancePolicyTemplate, error)
	GetByCode(ctx context.Context, templateCode string) (*CompliancePolicyTemplate, error)
	Upsert(ctx context.Context, tpl *CompliancePolicyTemplate) error
	Count(ctx context.Context) (int64, error)
}

// PolicyTemplateService 提供行业合规模板的查询、预置与自定义能力。
type PolicyTemplateService struct {
	repo      CompliancePolicyTemplateRepository
	auditRepo ComplianceAuditLogRepository
}

// NewPolicyTemplateService 构造行业合规模板服务。
func NewPolicyTemplateService(
	repo CompliancePolicyTemplateRepository,
	auditRepo ComplianceAuditLogRepository,
) *PolicyTemplateService {
	svc := &PolicyTemplateService{repo: repo, auditRepo: auditRepo}
	return svc
}

// builtinPolicyTemplates 返回方案 4.4.2 节定义的预置行业模板。
func builtinPolicyTemplates() []CompliancePolicyTemplate {
	return []CompliancePolicyTemplate{
		{
			TemplateCode: "ecommerce",
			Industry:     "电子商务",
			Description:  "电商行业合规模板：推荐引擎用户画像告知、数据保留 90 天。",
			Rules: []map[string]any{
				{"name": "HIGH_RISK_USE_CASE", "value": "recommendation_engine"},
				{"name": "HUMAN_OVERSIGHT_REQUIRED", "value": true},
				{"name": "DATA_RETENTION_LIMIT", "value": "90_days"},
				{"name": "USER_PROFILING_NOTICE", "value": true},
			},
			RiskTags: []string{RiskTagPIIDetected, RiskTagOutputControlLimited},
		},
		{
			TemplateCode: "finance",
			Industry:     "金融服务",
			Description:  "金融行业合规模板：信用评分人工监督、审计留痕、反欺诈、数据保留 365 天。",
			Rules: []map[string]any{
				{"name": "HIGH_RISK_USE_CASE", "value": "credit_scoring"},
				{"name": "HUMAN_OVERSIGHT_REQUIRED", "value": true},
				{"name": "DATA_RETENTION_LIMIT", "value": "365_days"},
				{"name": "AUDIT_TRAIL_REQUIRED", "value": true},
				{"name": "FRAUD_DETECTION", "value": true},
			},
			RiskTags: []string{RiskTagHighRiskUseCase, RiskTagCrossBorderTransfer, RiskTagNoTrainingGuarantee},
		},
		{
			TemplateCode: "healthcare",
			Industry:     "医疗健康",
			Description:  "医疗行业合规模板：医疗建议人工监督、HIPAA 合规、患者数据保护、数据保留 730 天。",
			Rules: []map[string]any{
				{"name": "HIGH_RISK_USE_CASE", "value": "medical_advice"},
				{"name": "HUMAN_OVERSIGHT_REQUIRED", "value": true},
				{"name": "DATA_RETENTION_LIMIT", "value": "730_days"},
				{"name": "HIPAA_COMPLIANCE", "value": true},
				{"name": "PATIENT_DATA_PROTECTION", "value": true},
			},
			RiskTags: []string{RiskTagHighRiskUseCase, RiskTagPIIDetected, RiskTagCrossBorderTransfer},
		},
		{
			TemplateCode: "education",
			Industry:     "教育培训",
			Description:  "教育行业合规模板：学习评估人工监督、未成年人数据保护、数据保留 180 天。",
			Rules: []map[string]any{
				{"name": "HIGH_RISK_USE_CASE", "value": "education_assessment"},
				{"name": "HUMAN_OVERSIGHT_REQUIRED", "value": true},
				{"name": "DATA_RETENTION_LIMIT", "value": "180_days"},
				{"name": "MINOR_DATA_PROTECTION", "value": true},
			},
			RiskTags: []string{RiskTagHighRiskUseCase, RiskTagPIIDetected},
		},
	}
}

// EnsureBuiltinTemplates 幂等地预置内置行业模板（仅在表为空时写入）。
// 启动时调用；写入失败仅记录，不阻断启动。
func (s *PolicyTemplateService) EnsureBuiltinTemplates(ctx context.Context) error {
	count, err := s.repo.Count(ctx)
	if err != nil {
		return fmt.Errorf("count policy templates: %w", err)
	}
	if count > 0 {
		return nil
	}
	for _, tpl := range builtinPolicyTemplates() {
		t := tpl
		if err := s.repo.Upsert(ctx, &t); err != nil {
			return fmt.Errorf("seed policy template %s: %w", tpl.TemplateCode, err)
		}
	}
	return nil
}

// ListTemplates 列出行业合规模板；industry 为空时返回全部。
func (s *PolicyTemplateService) ListTemplates(ctx context.Context, industry string) ([]CompliancePolicyTemplate, error) {
	return s.repo.List(ctx, strings.TrimSpace(industry))
}

// GetTemplate 按模板编码获取模板。
func (s *PolicyTemplateService) GetTemplate(ctx context.Context, templateCode string) (*CompliancePolicyTemplate, error) {
	return s.repo.GetByCode(ctx, strings.TrimSpace(templateCode))
}

// ApplyTemplateInput 是应用模板的入参。
type ApplyTemplateInput struct {
	TemplateCode string
	Operator     string
}

// ApplyTemplate 将指定模板应用到组织，记录合规审计事件并返回被应用的模板。
//
// 说明：本方案未定义组织级策略持久化表，"应用"体现为一条不可篡改的审计留痕
// （compliance_type=policy_template_applied，evidence_hash=模板规则哈希），
// 作为该组织采纳此合规策略的证据。
func (s *PolicyTemplateService) ApplyTemplate(ctx context.Context, input ApplyTemplateInput) (*CompliancePolicyTemplate, error) {
	code := strings.TrimSpace(input.TemplateCode)
	if code == "" {
		return nil, fmt.Errorf("template_code is required")
	}
	tpl, err := s.repo.GetByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if tpl == nil {
		return nil, fmt.Errorf("policy template %q not found", code)
	}
	details, _ := json.Marshal(map[string]any{
		"template_code": tpl.TemplateCode,
		"industry":      tpl.Industry,
		"rules":         tpl.Rules,
		"risk_tags":     tpl.RiskTags,
	})
	operator := strings.TrimSpace(input.Operator)
	if operator == "" {
		operator = "admin"
	}
	if s.auditRepo != nil {
		_ = s.auditRepo.Create(ctx, &ComplianceAuditLog{
			ComplianceType: "policy_template_applied",
			SubjectType:    "organization",
			Details:        string(details),
			Operator:       operator,
			EvidenceHash:   ComputeHash(string(details)),
		})
	}
	return tpl, nil
}

// CreateCustomTemplateInput 是创建自定义模板的入参。
type CreateCustomTemplateInput struct {
	TemplateCode string
	Industry     string
	Description  string
	Rules        []map[string]any
	RiskTags     []string
	Operator     string
}

// CreateCustomTemplate 创建（或覆盖同 code 的）自定义行业合规模板。
func (s *PolicyTemplateService) CreateCustomTemplate(ctx context.Context, input CreateCustomTemplateInput) (*CompliancePolicyTemplate, error) {
	code := strings.TrimSpace(input.TemplateCode)
	if code == "" {
		return nil, fmt.Errorf("template_code is required")
	}
	if strings.TrimSpace(input.Industry) == "" {
		return nil, fmt.Errorf("industry is required")
	}
	tpl := &CompliancePolicyTemplate{
		TemplateCode: code,
		Industry:     strings.TrimSpace(input.Industry),
		Description:  strings.TrimSpace(input.Description),
		Rules:        input.Rules,
		RiskTags:     input.RiskTags,
	}
	if tpl.Rules == nil {
		tpl.Rules = []map[string]any{}
	}
	if tpl.RiskTags == nil {
		tpl.RiskTags = []string{}
	}
	if err := s.repo.Upsert(ctx, tpl); err != nil {
		return nil, err
	}
	operator := strings.TrimSpace(input.Operator)
	if operator == "" {
		operator = "admin"
	}
	if s.auditRepo != nil {
		details, _ := json.Marshal(map[string]any{"template_code": tpl.TemplateCode, "industry": tpl.Industry})
		_ = s.auditRepo.Create(ctx, &ComplianceAuditLog{
			ComplianceType: "policy_template_created",
			SubjectType:    "organization",
			Details:        string(details),
			Operator:       operator,
			EvidenceHash:   ComputeHash(string(details)),
		})
	}
	return tpl, nil
}

// GetActiveTemplateCode 从审计日志中查询最新应用的模板编码。
func (s *PolicyTemplateService) GetActiveTemplateCode(ctx context.Context) (string, error) {
	if s.auditRepo == nil {
		return "", nil
	}
	filter := ComplianceAuditLogFilter{
		ComplianceType: "policy_template_applied",
		Pagination: pagination.PaginationParams{Page: 1, PageSize: 1},
	}
	logs, _, err := s.auditRepo.List(ctx, filter)
	if err != nil {
		return "", fmt.Errorf("list audit logs: %w", err)
	}
	if len(logs) == 0 {
		return "", nil
	}
	var details map[string]any
	if err := json.Unmarshal([]byte(logs[0].Details), &details); err != nil {
		return "", nil
	}
	code, _ := details["template_code"].(string)
	return code, nil
}
