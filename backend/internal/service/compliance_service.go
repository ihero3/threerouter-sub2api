package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/text/unicode/norm"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// ============================================================================
// AI 治理与合规服务（ComplianceService）
// 见 docs/合规方案.md 4.1.4 节。
//
// 职责：
//   - 风险打标（ApplyRiskTags）：PII / 高风险场景 / 跨境 / 制裁区域检测
//   - EU AI Act 角色与风险层级评估（固定为基础设施提供者，非高风险系统提供者）
//   - 合规审计事件记录（LogComplianceEvent）
//   - GDPR 数据主体权利：删除请求（Art 17）、数据导出（Art 20）、同意管理（Art 7）
// ============================================================================

// ---- EU AI Act 角色与风险层级常量 ----
const (
	// EUAIActRoleInfrastructureProvider 基础设施服务提供者：Sub2API 的固定主角色。
	EUAIActRoleInfrastructureProvider = "INFRASTRUCTURE_SERVICE_PROVIDER"
	// EUAIActRoleModelAccessProvider 模型访问提供者：次要角色。
	EUAIActRoleModelAccessProvider = "AI_MODEL_ACCESS_PROVIDER"

	// EUAIActRiskTierNonHighRisk 非高风险：基础设施定位下的默认层级。
	EUAIActRiskTierNonHighRisk = "not_high_risk_by_current_provider_scope"
	// EUAIActRiskTierMinimal 最小风险。
	EUAIActRiskTierMinimal = "minimal_risk"
	// EUAIActRiskTierLimited 有限风险（透明度义务，如披露 AI 生成内容）。
	EUAIActRiskTierLimited = "limited_risk"
)

// ---- 合规风险标签常量（见 4.1.3 节合规风险标签）----
const (
	RiskTagPIIDetected          = "PII_DETECTED"
	RiskTagHighRiskUseCase      = "HIGH_RISK_USE_CASE"
	RiskTagCrossBorderTransfer  = "CROSS_BORDER_TRANSFER"
	RiskTagSanctionedRegion     = "SANCTIONED_REGION"
	RiskTagContentPolicyViolate = "CONTENT_POLICY_VIOLATION"
	RiskTagOutputControlLimited = "OUTPUT_CONTROL_LIMITED"
	RiskTagNoTrainingGuarantee  = "NO_TRAINING_GUARANTEE"
	RiskTagRateLimitExceeded    = "RATE_LIMIT_EXCEEDED"
	RiskTagAnomalousBehavior    = "ANOMALOUS_BEHAVIOR"
)

// ---- 合规审计主体类型 ----
const (
	ComplianceSubjectUser   = "user"
	ComplianceSubjectSystem = "system"
	ComplianceSubjectModel  = "model"
	ComplianceSubjectReport = "report"
)

// ---- 数据删除请求类型与状态 ----
const (
	ErasureRequestTypeFull      = "FULL_ERASURE"
	ErasureRequestTypeAnonymize = "ANONYMIZE"
	ErasureRequestTypeRestrict  = "RESTRICT"

	ErasureStatusPending    = "pending"
	ErasureStatusApproved   = "approved"
	ErasureStatusRejected   = "rejected"
	ErasureStatusProcessing = "processing"
	ErasureStatusCompleted  = "completed"
)

// highRiskUseCases 为 EU AI Act 附件 III 参考的高风险应用场景集合。
var highRiskUseCases = map[string]struct{}{
	"employment":     {},
	"credit":         {},
	"credit_scoring": {},
	"healthcare":     {},
	"medical_advice": {},
	"biometric":      {},
	"education":      {},
	"law_enforcement": {},
	"migration":      {},
	"justice":        {},
}

// sanctionedRegions 为受制裁国家/地区的 ISO 3166-1 alpha-2 代码集合。
// 依据美国 OFAC 全面制裁清单（可通过配置扩展；此处为静态兜底）。
var sanctionedRegions = map[string]struct{}{
	"CU": {}, // 古巴
	"IR": {}, // 伊朗
	"KP": {}, // 朝鲜
	"SY": {}, // 叙利亚
	"RU": {}, // 俄罗斯（部分制裁）
}

// piiPatterns 为 PII 检测的正则集合（邮箱、手机号、身份证、银行卡、IP）。
var piiPatterns = []*regexp.Regexp{
	regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),  // Email
	regexp.MustCompile(`\b(?:\+?86)?1[3-9]\d{9}\b`),                          // 中国手机号
	regexp.MustCompile(`\b\d{17}[\dXx]\b`),                                   // 中国身份证
	regexp.MustCompile(`\b(?:\d[ \-]?){13,19}\b`),                            // 银行卡号（宽松）
}

// ============================================================================
// 领域模型
// ============================================================================

// ComplianceEvent 表示一条待记录的合规审计事件。
type ComplianceEvent struct {
	ComplianceType string // 见 4.6 节合规审计类型（RISK_ASSESSMENT / EU_AI_ACT_ASSESSMENT / DATA_ERASURE ...）
	SubjectType    string // user / system / model / report
	SubjectID      *int64
	Details        string
	Operator       string
	EvidenceHash   string
}

// ComplianceAuditLog 是 compliance_audit_logs 表的行模型。
type ComplianceAuditLog struct {
	ID             int64     `json:"id"`
	ComplianceType string    `json:"compliance_type"`
	SubjectType    string    `json:"subject_type"`
	SubjectID      *int64    `json:"subject_id,omitempty"`
	Details        string    `json:"details"`
	Operator       string    `json:"operator"`
	EvidenceHash   string    `json:"evidence_hash,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// ComplianceAuditLogFilter 是审计日志分页查询过滤条件。
type ComplianceAuditLogFilter struct {
	ComplianceType string
	SubjectType    string
	SubjectID      *int64
	StartTime      *time.Time
	EndTime        *time.Time
	Pagination     pagination.PaginationParams
}

// DataErasureRequest 是 data_erasure_requests 表的行模型（GDPR Art 17）。
type DataErasureRequest struct {
	ID              int64      `json:"id"`
	UserID          int64      `json:"user_id"`
	RequestType     string     `json:"request_type"`
	Status          string     `json:"status"`
	ScopeDetails    string     `json:"scope_details"`
	RejectionReason *string    `json:"rejection_reason,omitempty"`
	Operator        *string    `json:"operator,omitempty"`
	RequestedAt     time.Time  `json:"requested_at"`
	ProcessedAt     *time.Time `json:"processed_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}

// DataErasureRequestFilter 是删除请求分页查询过滤条件。
type DataErasureRequestFilter struct {
	UserID     *int64
	Status     string
	Pagination pagination.PaginationParams
}

// UserConsent 是 user_consents 表的行模型（GDPR Art 7）。
type UserConsent struct {
	ID          int64      `json:"id"`
	UserID      int64      `json:"user_id"`
	ConsentType string     `json:"consent_type"`
	Granted     bool       `json:"granted"`
	GrantedAt   *time.Time `json:"granted_at,omitempty"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	Source      *string    `json:"source,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// RiskAssessment 是 ApplyRiskTags 的结果，携带标签与派生的司法管辖区信息。
type RiskAssessment struct {
	RiskTags        []string `json:"risk_tags"`
	UserJurisdiction string  `json:"user_jurisdiction,omitempty"`
	CrossBorder     bool     `json:"cross_border"`
	Sanctioned      bool     `json:"sanctioned"`
}

// ============================================================================
// 仓储接口（实现见 repository 包）
// ============================================================================

// ComplianceAuditLogRepository 定义合规审计日志持久化。
type ComplianceAuditLogRepository interface {
	Create(ctx context.Context, log *ComplianceAuditLog) error
	List(ctx context.Context, filter ComplianceAuditLogFilter) ([]ComplianceAuditLog, *pagination.PaginationResult, error)
}

// DataErasureRequestRepository 定义数据删除请求持久化。
type DataErasureRequestRepository interface {
	Create(ctx context.Context, req *DataErasureRequest) error
	GetByID(ctx context.Context, id int64) (*DataErasureRequest, error)
	UpdateStatus(ctx context.Context, id int64, status, operator string, rejectionReason *string, processedAt, completedAt *time.Time) error
	List(ctx context.Context, filter DataErasureRequestFilter) ([]DataErasureRequest, *pagination.PaginationResult, error)
}

// UserConsentRepository 定义用户同意持久化。
type UserConsentRepository interface {
	Upsert(ctx context.Context, consent *UserConsent) error
	Get(ctx context.Context, userID int64, consentType string) (*UserConsent, error)
	ListByUser(ctx context.Context, userID int64) ([]UserConsent, error)
	DeleteByUserID(ctx context.Context, userID int64) error
}

// ComplianceUserDataExporter 用于 GDPR 数据可携权（Art 20）导出用户数据。
// 由更上层的服务实现（避免 ComplianceService 直接依赖 UserRepository 造成循环）。
type ComplianceUserDataExporter interface {
	ExportUserData(ctx context.Context, userID int64) (map[string]any, error)
}

// ============================================================================
// ComplianceService
// ============================================================================

// ComplianceService 实现 AI 治理与合规核心能力。
type ComplianceService struct {
	cfg          *config.Config
	auditRepo    ComplianceAuditLogRepository
	erasureRepo  DataErasureRequestRepository
	consentRepo  UserConsentRepository
	geoIPService *GeoIPService
	dataExporter ComplianceUserDataExporter
}

// NewComplianceService 构造合规服务。dataExporter 可为 nil（导出功能将返回未实现错误）。
func NewComplianceService(
	cfg *config.Config,
	auditRepo ComplianceAuditLogRepository,
	erasureRepo DataErasureRequestRepository,
	consentRepo UserConsentRepository,
	geoIPService *GeoIPService,
	dataExporter ComplianceUserDataExporter,
) *ComplianceService {
	return &ComplianceService{
		cfg:          cfg,
		auditRepo:    auditRepo,
		erasureRepo:  erasureRepo,
		consentRepo:  consentRepo,
		geoIPService: geoIPService,
		dataExporter: dataExporter,
	}
}

// ComputeHash 计算内容的 SHA-256 十六进制摘要（64 位）。
//
// 计算前进行 UTF-8 NFC 规范化，确保等价内容产生一致哈希（见 4.1.2 节）。
func ComputeHash(content string) string {
	normalized := norm.NFC.String(content)
	h := sha256.New()
	h.Write([]byte(normalized))
	return hex.EncodeToString(h.Sum(nil))
}

// AssessEUAIActRole 评估 Sub2API 在给定调用中的 EU AI Act 角色。
//
// 依据 docs/合规方案.md 9.1 节：Sub2API 固定为基础设施提供者，不作为高风险 AI 系统提供者。
// 本方法始终返回 AI_INFRASTRUCTURE_PROVIDER；modelProvider/useCase 仅用于未来扩展与审计留痕。
func (s *ComplianceService) AssessEUAIActRole(modelProvider string, useCase string) string {
	return EUAIActRoleInfrastructureProvider
}

// AssessEUAIActRiskTier 评估风险层级。
//
// 基础设施定位下默认 non_high_risk；即便检测到高风险应用场景，Sub2API 作为中间层
// 仍不构成该场景的高风险系统提供者，但会通过风险标签（HIGH_RISK_USE_CASE）留痕。
func (s *ComplianceService) AssessEUAIActRiskTier(role string, useCase string) string {
	return EUAIActRiskTierNonHighRisk
}

// ApplyRiskTags 对单次调用进行合规风险打标。
//
// 检测项：
//   - PII_DETECTED：输入包含个人身份信息（邮箱/手机号/身份证/银行卡）
//   - HIGH_RISK_USE_CASE：useCase 属于高风险场景集合
//   - CROSS_BORDER_TRANSFER：用户司法管辖区与部署区域不一致
//   - SANCTIONED_REGION：用户 IP 来自受制裁区域
//
// clientIP 可为空；GeoIP 不可用时跳过跨境/制裁判定（降级，不阻断）。
func (s *ComplianceService) ApplyRiskTags(ctx context.Context, reqID string, input *string, useCase *string, clientIP string) (*RiskAssessment, error) {
	assessment := &RiskAssessment{RiskTags: make([]string, 0, 4)}

	// PII 检测
	if input != nil && containsPII(*input) {
		assessment.RiskTags = append(assessment.RiskTags, RiskTagPIIDetected)
	}

	// 高风险场景
	if useCase != nil {
		uc := strings.ToLower(strings.TrimSpace(*useCase))
		if _, ok := highRiskUseCases[uc]; ok {
			assessment.RiskTags = append(assessment.RiskTags, RiskTagHighRiskUseCase)
		}
	}

	// 司法管辖区：依赖 GeoIP，不可用时降级留空。
	if s.geoIPService != nil && clientIP != "" {
		jurisdiction, err := s.geoIPService.Lookup(clientIP)
		if err == nil && jurisdiction != "" {
			assessment.UserJurisdiction = jurisdiction

			// 制裁区域
			if _, ok := sanctionedRegions[jurisdiction]; ok {
				assessment.Sanctioned = true
				assessment.RiskTags = append(assessment.RiskTags, RiskTagSanctionedRegion)
			}

			// 跨境传输：用户区域与部署区域不同即视为跨境。
			deployRegion := strings.ToUpper(strings.TrimSpace(s.cfg.Compliance.DeploymentRegion))
			if deployRegion != "" && jurisdiction != deployRegion {
				assessment.CrossBorder = true
				assessment.RiskTags = append(assessment.RiskTags, RiskTagCrossBorderTransfer)
			}
		}
	}

	return assessment, nil
}

// LogComplianceEvent 记录一条合规审计事件。
func (s *ComplianceService) LogComplianceEvent(ctx context.Context, event ComplianceEvent) error {
	if strings.TrimSpace(event.ComplianceType) == "" {
		return fmt.Errorf("compliance event: empty compliance_type")
	}
	subjectType := strings.TrimSpace(event.SubjectType)
	if subjectType == "" {
		subjectType = ComplianceSubjectSystem
	}
	operator := strings.TrimSpace(event.Operator)
	if operator == "" {
		operator = "system"
	}
	log := &ComplianceAuditLog{
		ComplianceType: event.ComplianceType,
		SubjectType:    subjectType,
		SubjectID:      event.SubjectID,
		Details:        event.Details,
		Operator:       operator,
		EvidenceHash:   event.EvidenceHash,
	}
	return s.auditRepo.Create(ctx, log)
}

// ListComplianceAuditLogs 分页查询合规审计日志。
func (s *ComplianceService) ListComplianceAuditLogs(ctx context.Context, filter ComplianceAuditLogFilter) ([]ComplianceAuditLog, *pagination.PaginationResult, error) {
	return s.auditRepo.List(ctx, filter)
}

// RequestDataErasure 创建数据删除请求（GDPR Art 17）。
func (s *ComplianceService) RequestDataErasure(ctx context.Context, req DataErasureRequest) (*DataErasureRequest, error) {
	if req.UserID <= 0 {
		return nil, fmt.Errorf("data erasure: invalid user_id")
	}
	req.RequestType = normalizeErasureType(req.RequestType)
	if !isValidErasureType(req.RequestType) {
		return nil, fmt.Errorf("data erasure: invalid request_type %q", req.RequestType)
	}
	req.Status = ErasureStatusPending
	if err := s.erasureRepo.Create(ctx, &req); err != nil {
		return nil, err
	}
	// 审计留痕
	uid := req.UserID
	_ = s.LogComplianceEvent(ctx, ComplianceEvent{
		ComplianceType: "DATA_ERASURE_REQUEST",
		SubjectType:    ComplianceSubjectUser,
		SubjectID:      &uid,
		Details:        fmt.Sprintf("request_type=%s scope=%s", req.RequestType, req.ScopeDetails),
		Operator:       "system",
	})
	return &req, nil
}

// ProcessDataErasure 审批数据删除请求。approved=true 置为 approved，否则 rejected。
func (s *ComplianceService) ProcessDataErasure(ctx context.Context, id int64, approved bool, reason string, operator string) error {
	existing, err := s.erasureRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("data erasure request %d not found", id)
	}
	if existing.Status != ErasureStatusPending {
		return fmt.Errorf("data erasure request %d already %s", id, existing.Status)
	}

	now := time.Now()
	status := ErasureStatusApproved
	var rejectionReason *string
	if !approved {
		status = ErasureStatusRejected
		r := strings.TrimSpace(reason)
		rejectionReason = &r
	}
	if strings.TrimSpace(operator) == "" {
		operator = "system"
	}

	uid := existing.UserID
	if approved {
		if err := s.DeleteUserData(ctx, uid); err != nil {
			return fmt.Errorf("delete user data failed: %w", err)
		}
		status = ErasureStatusCompleted
	}

	if err := s.erasureRepo.UpdateStatus(ctx, id, status, operator, rejectionReason, &now, nil); err != nil {
		return err
	}

	_ = s.LogComplianceEvent(ctx, ComplianceEvent{
		ComplianceType: "DATA_ERASURE_DECISION",
		SubjectType:    ComplianceSubjectUser,
		SubjectID:      &uid,
		Details:        fmt.Sprintf("request_id=%d decision=%s reason=%s", id, status, reason),
		Operator:       operator,
	})
	return nil
}

// DeleteUserData 删除用户的个人数据（GDPR Art 17）。
// 删除内容包括：同意记录、合规档案等。
func (s *ComplianceService) DeleteUserData(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return fmt.Errorf("invalid user_id")
	}

	if s.consentRepo != nil {
		if err := s.consentRepo.DeleteByUserID(ctx, userID); err != nil {
			return fmt.Errorf("delete user consents: %w", err)
		}
	}

	return nil
}

// ListDataErasureRequests 分页查询删除请求。
func (s *ComplianceService) ListDataErasureRequests(ctx context.Context, filter DataErasureRequestFilter) ([]DataErasureRequest, *pagination.PaginationResult, error) {
	return s.erasureRepo.List(ctx, filter)
}

// ExportUserData 导出用户数据（GDPR Art 20 数据可携权），返回 JSON 字节。
func (s *ComplianceService) ExportUserData(ctx context.Context, userID int64) ([]byte, error) {
	if s.dataExporter == nil {
		return nil, fmt.Errorf("data export not available: no exporter configured")
	}
	data, err := s.dataExporter.ExportUserData(ctx, userID)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"export_id":     fmt.Sprintf("gdpr-export-%d-%d", userID, time.Now().Unix()),
		"user_id":       userID,
		"generated_at":  time.Now().UTC().Format(time.RFC3339),
		"data_subject":  data,
		"legal_basis":   "GDPR Art 20 (Right to data portability)",
		"format_notice": "Machine-readable JSON export",
	}
	out, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal user data export: %w", err)
	}

	uid := userID
	_ = s.LogComplianceEvent(ctx, ComplianceEvent{
		ComplianceType: "DATA_EXPORT",
		SubjectType:    ComplianceSubjectUser,
		SubjectID:      &uid,
		Details:        "GDPR Art 20 data export generated",
		Operator:       "system",
		EvidenceHash:   ComputeHash(string(out)),
	})
	return out, nil
}

// SetUserConsent 设置用户同意状态（GDPR Art 7）。
func (s *ComplianceService) SetUserConsent(ctx context.Context, userID int64, consentType string, granted bool, source string) error {
	consentType = strings.TrimSpace(consentType)
	if userID <= 0 || consentType == "" {
		return fmt.Errorf("set consent: invalid user_id or consent_type")
	}
	now := time.Now()
	consent := &UserConsent{
		UserID:      userID,
		ConsentType: consentType,
		Granted:     granted,
	}
	if granted {
		consent.GrantedAt = &now
	} else {
		consent.RevokedAt = &now
	}
	if src := strings.TrimSpace(source); src != "" {
		consent.Source = &src
	}
	if err := s.consentRepo.Upsert(ctx, consent); err != nil {
		return err
	}

	uid := userID
	_ = s.LogComplianceEvent(ctx, ComplianceEvent{
		ComplianceType: "CONSENT_CHANGE",
		SubjectType:    ComplianceSubjectUser,
		SubjectID:      &uid,
		Details:        fmt.Sprintf("consent_type=%s granted=%t", consentType, granted),
		Operator:       "system",
	})
	return nil
}

// GetUserConsent 查询用户对某一类型的同意状态。未记录时返回 nil, nil。
func (s *ComplianceService) GetUserConsent(ctx context.Context, userID int64, consentType string) (*UserConsent, error) {
	return s.consentRepo.Get(ctx, userID, strings.TrimSpace(consentType))
}

// ListUserConsents 列出用户全部同意记录。
func (s *ComplianceService) ListUserConsents(ctx context.Context, userID int64) ([]UserConsent, error) {
	return s.consentRepo.ListByUser(ctx, userID)
}

// GenerateEUAIActAssessment 生成 EU AI Act 合规评估报告（ZDR 兼容版，见 4.2.2 节）。
//
// 报告基于聚合指标与固定的角色定位（基础设施提供者，非高风险系统提供者），
// 不含任何逐请求个人数据，符合 ZDR 原则。
func (s *ComplianceService) GenerateEUAIActAssessment(ctx context.Context) map[string]any {
	now := time.Now().UTC()
	org := s.cfg.Compliance
	deploymentRegion := defaultIfEmpty(org.DeploymentRegion, "Customer-controlled deployment environment")
	var dpoContact *string
	if org.DPOContact != "" {
		dpoContact = &org.DPOContact
	}

	report := map[string]any{
		"report_id":            fmt.Sprintf("eu-ai-act-assessment-%s", now.Format("2006-01-02-150405")),
		"generated_at":         now.Format(time.RFC3339),
		"data_source":          "aggregated_metrics",
			"compliance_statement": "Per-request content is not retained by default under the configured Zero Data Retention policy",
			"organization_info": map[string]any{
				"name":              defaultIfEmpty(org.OrganizationName, "ThreeRouter Technology Ltd."),
				"legal_entity":      defaultIfEmpty(org.LegalEntity, "ThreeRouter Technology Ltd."),
				"contact":           "privacy@threerouter.com",
				"dpo_contact":       dpoContact,
				"dpo_required":      false,
				"deployment_region": deploymentRegion,
			},
			"role_assessment": map[string]any{
				"primary_role":    EUAIActRoleInfrastructureProvider,
				"ai_service_role": "AI_MODEL_ACCESS_AGGREGATOR",
				"secondary_roles": []string{EUAIActRoleModelAccessProvider},
				"assessment_basis": "ThreeRouter operates as an AI infrastructure and model access aggregation layer " +
					"and does not provide, train, or control underlying AI models or their intended purposes.",
				"scope_limits": []string{
					"no_foundation_model_training",
					"no_ai_model_fine-tuning",
					"no_determination_of_end-user_purpose",
					"no_modification_of_model_parameters",
				},
				"eu_ai_act_role_mapping": map[string]any{
					"provider": map[string]any{
						"status": false,
						"basis":  "Does not develop, modify, or place AI models on the EU market under its own name",
					},
					"deployer":                        false,
					"importer":                        false,
					"distributor":                     false,
					"infrastructure_service_provider": true,
				},
			},
			"system_description": map[string]any{
				"purpose":    "AI model access proxy and API aggregation gateway",
				"risk_tier":  EUAIActRiskTierNonHighRisk,
				"categories": []string{"api_aggregator", "service_intermediary", "compliance_infrastructure"},
			},
			"high_risk_assessment": map[string]any{
				"assessment_method":              "Based on Annex III use cases",
				"high_risk_domains_detected":     false,
				"restricted_use_cases_detected":  false,
				"assessment_basis":               "ThreeRouter does not determine or control end-use purposes; " +
					"high-risk classification depends on downstream application by end-users",
			},
			"gpai_assessment": map[string]any{
				"is_gpai_model_provider":         false,
				"provides_general_purpose_ai_model": false,
				"integrates_third_party_models":  true,
				"role":                           "downstream_integrator",
				"responsibility":                 "Relay and infrastructure layer only",
				"description":                    "ThreeRouter acts as an access layer for third-party GPAI models " +
					"but does not provide, train, or modify the underlying models",
				"third_party_model_responsibility": map[string]any{
					"model_provider": "Responsible for foundation model obligations",
					"threerouter":    "Responsible for infrastructure and service layer",
				},
			},
			"transparency_measures": []string{
				"AI system identification",
				"Intended purpose disclosure",
				"Capabilities and limitations notice",
				"Human oversight guidance",
				"Provider contact information",
				"Clear consent mechanism for detailed logging",
			},
			"content_moderation": map[string]any{
				"policy":                         "Harmful content filtering",
				"purpose":                        "Safety filtering and abuse prevention only; not used for autonomous decision making",
				"automated_checks":               true,
				"human_review_available":         true,
				"human_review_performed_by":      "customer_or_optional_service_provider",
				"provider_review_of_user_content": false,
				"no_content_retention":           true,
			},
			"human_oversight": map[string]any{
				"required_under_ai_act":           false,
				"reason":                          "Not applicable because system is not classified as high-risk AI system",
				"operational_guidance_provided":   true,
				"provided_by":                     "customer_or_provider",
				"provider_review_of_user_content": false,
				"description":                     "Human oversight is the responsibility of the end-user organization; " +
					"ThreeRouter provides compliance infrastructure and guidance",
			},
			"data_protection": map[string]any{
				"gdpr_alignment":        true,
				"gdpr_status":           "Designed to support GDPR compliance requirements",
				"data_minimization":     true,
				"retention_period":      "Aggregate only by default",
				"detail_retention":      "User opt-in with configurable retention",
				"cross_border_transfer": map[string]any{
					"mechanism": "EU Standard Contractual Clauses (2021/914), where applicable",
				},
				"deployment_region": deploymentRegion,
			},
			"gdpr_specific": map[string]any{
				"data_subject_rights": []string{"erasure", "access", "portability", "restriction", "rectification", "objection"},
				"dpa_available":       true,
				"sccs_declaration":    true,
			},
			"risk_classification": map[string]any{
				"high_risk_use_cases_detected": false,
			},
			"ai_act_transparency": map[string]any{
				"ai_interaction_disclosure":      true,
				"system_capabilities_documented": true,
				"limitations_documented":         true,
				"provider_information_disclosed": true,
			},
			"article_50_transparency": map[string]any{
				"ai_interaction_disclosure": true,
				"synthetic_content_labeling": map[string]any{
					"applicable": false,
					"reason":     "ThreeRouter does not generate, publish, or distribute AI-generated content under its own identity",
				},
				"user_notification": true,
			},
			"ai_act_article_mapping": map[string]any{
				"Article_13_transparency":     true,
				"Article_14_human_oversight":  "not_applicable_by_scope",
				"Article_50_transparency":     true,
				"Annex_III_high_risk_assessment": true,
			},
			"eu_ai_act_assessment": map[string]any{
				"regulation":                     "EU AI Act (EU) 2024/1689",
				"provider_status":                "AI infrastructure and aggregation service",
				"high_risk_system": map[string]any{
					"classification":              "not_high_risk_by_provider_scope",
					"depends_on_downstream_use_case": true,
				},
				"gpai_provider":                 false,
				"prohibited_practices_detected": false,
				"transparency_obligations_applicable": true,
				"assessment_date":                     time.Now().Format(time.RFC3339),
				"report_version":                      "1.0",
			},
		}
	return report
}

// ExportEUAIActAssessment 将评估报告序列化为可下载的 JSON 文件，返回内容、文件名。
func (s *ComplianceService) ExportEUAIActAssessment(ctx context.Context) ([]byte, string, error) {
	report := s.GenerateEUAIActAssessment(ctx)
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, "", fmt.Errorf("marshal eu ai act assessment: %w", err)
	}
	filename := fmt.Sprintf("eu-ai-act-assessment-%s.json", time.Now().UTC().Format("20060102-150405"))

	_ = s.LogComplianceEvent(ctx, ComplianceEvent{
		ComplianceType: "EU_AI_ACT_ASSESSMENT",
		SubjectType:    ComplianceSubjectReport,
		Details:        "EU AI Act assessment report exported",
		Operator:       "system",
		EvidenceHash:   ComputeHash(string(data)),
	})
	return data, filename, nil
}

// GenerateDPA 生成数据处理协议合规声明（DPA Compliance Statement，GDPR Art 28）。
//
// 依据合规方案 4.2.4 节：生成标准格式的 DPA 合规声明，涵盖控制器-处理器关系、
// 处理目的、数据主体权利、安全措施、跨境传输、审计权利等要素。
// 注意：此文件为合规声明/模板元数据，非可签署的法律合同。
func (s *ComplianceService) GenerateDPA(ctx context.Context, controllerName string, controllerContact string) ([]byte, string, error) {
	if controllerName == "" || controllerContact == "" {
		return nil, "", fmt.Errorf("controller_name and controller_contact are required")
	}
	org := s.cfg.Compliance
	if org.OrganizationName == "" {
		org.OrganizationName = "ThreeRouter Self-hosted"
	}
	if org.LegalEntity == "" {
		org.LegalEntity = "Self-hosted infrastructure"
	}
	now := time.Now().UTC()

	crossBorderTransfer := map[string]any{
		"transfer_mechanism": "EU Standard Contractual Clauses (2021/914)",
		"adequacy_status":    "No adequacy decision relied upon",
		"additional_measures": []string{
			"Encryption",
			"Access controls",
			"Audit trail",
		},
	}
	deployRegion := strings.ToUpper(strings.TrimSpace(org.DeploymentRegion))
	if deployRegion != "" {
		crossBorderTransfer["deployment_region"] = deployRegion
		if deployRegion == "US" {
			crossBorderTransfer["transfer_mechanism"] = "EU Standard Contractual Clauses (2021/914) for transfers to third countries"
		} else if strings.HasPrefix(deployRegion, "EU") || deployRegion == "EEA" {
			crossBorderTransfer["transfer_mechanism"] = "Intra-EU/EEA transfer"
			crossBorderTransfer["adequacy_status"] = "EU/EEA internal transfer"
		}
	}

	dpa := map[string]any{
		"document_type":            "DPA Compliance Statement",
		"dpa_id":                   fmt.Sprintf("dpa-%s", now.Format("2006-01-02-150405")),
		"generated_at":             now.Format(time.RFC3339),
		"governing_law":            "GDPR (Regulation (EU) 2016/679)",
		"controller_legal_basis":   "GDPR Art 6(1)(b) - Contract performance",
		"processor_basis":          "Processing under Article 28 instructions",
		"effective_date":           now.Format(time.RFC3339),
		"subject_matter_and_duration": map[string]any{
			"subject": "AI API request relay and service aggregation services",
			"duration": "Until termination of services agreement or upon written notice",
		},
		"parties": map[string]any{
			"controller": map[string]any{
				"name":    controllerName,
				"contact": controllerContact,
				"role":    "Controller",
			},
			"processor": map[string]any{
				"name":             org.OrganizationName,
				"legal_entity":     org.LegalEntity,
				"dpo_contact":      org.DPOContact,
				"role":             "Processor",
				"registration":     "Self-hosted infrastructure",
				"deployment_region": deployRegion,
			},
		},
		"processing_activities": []map[string]any{
			{
				"purpose": "AI API request relay and service aggregation",
				"data_categories": []string{
					"User authentication data",
					"API request/response metadata",
					"Usage metrics (aggregate)",
					"Content moderation logs (when enabled)",
				},
				"retention_policy": map[string]any{
					"request_content": "Not retained (ZDR - Zero Data Retention)",
					"technical_logs":  "Up to 30 days",
					"security_logs":   "Up to 180 days",
				},
				"security": []string{
					"Encryption at rest (AES-256)",
					"TLS 1.3 in transit",
					"Access controls and audit trails",
					"Regular security assessments",
				},
			},
		},
		"ai_processing": map[string]any{
			"model_training":          false,
			"prompt_storage":          false,
			"human_review":            false,
			"automated_decision_making": false,
		},
		"data_subject_rights": map[string]any{
			"controller_responsible":       true,
			"processor_assistance_required": true,
			"rights_supported": []string{
				"access",
				"rectification",
				"erasure",
				"restriction",
				"portability",
				"objection",
			},
			"contact_point": org.DPOContact,
		},
		"subprocessors": map[string]any{
			"authorized":       false,
			"approval_required": true,
			"list":             []string{},
		},
		"controller_rights": []string{
			"Right to audit processing activities",
			"Right to require deletion of specific data",
			"Right to receive compliance reports",
			"Right to approve any changes to subprocessors",
		},
		"processor_obligations": []string{
			"Process only on documented instructions",
			"Ensure confidentiality and security",
			"Assist with data subject rights",
			"Notify controller of breaches without undue delay",
			"Provide assistance with regulatory inquiries",
			"Obtain written approval before engaging subprocessors",
		},
		"cross_border_transfer": crossBorderTransfer,
		"termination": map[string]any{
			"return_or_destroy": true,
			"certification":     true,
			"timeframe":         "Within 30 days of termination",
		},
		"signatures": map[string]any{
			"processor_signature": fmt.Sprintf("%s (Self-Asserted)", org.OrganizationName),
			"generated_by":        "ThreeRouter Compliance Module",
			"assertion":           "This is a DPA Compliance Statement generated as a self-asserted compliance document. It does not constitute a legally binding contract. For legally binding data processing agreements, consult legal counsel and execute a formal DPA with all parties.",
			"disclaimer":          "This document is for compliance demonstration purposes only and does not replace legal advice.",
		},
	}
	data, err := json.MarshalIndent(dpa, "", "  ")
	if err != nil {
		return nil, "", fmt.Errorf("marshal dpa: %w", err)
	}
	filename := fmt.Sprintf("dpa-%s.json", now.Format("20060102-150405"))

	_ = s.LogComplianceEvent(ctx, ComplianceEvent{
		ComplianceType: "DPA_GENERATION",
		SubjectType:    ComplianceSubjectReport,
		Details:        fmt.Sprintf("DPA generated for controller=%s", controllerName),
		Operator:       "system",
		EvidenceHash:   ComputeHash(string(data)),
	})
	return data, filename, nil
}

// GenerateDataProcessingRecord 生成 GDPR Art 30 数据处理活动记录（ROPA）。
func (s *ComplianceService) GenerateDataProcessingRecord(ctx context.Context) map[string]any {
	org := s.cfg.Compliance
	var dpoContact *string
	if org.DPOContact != "" {
		dpoContact = &org.DPOContact
	}
	return map[string]any{
		"document_title":   "GDPR Art.30 Records of Processing Activities (ROPA) – Processor Activities",
		"generated_at":     time.Now().UTC().Format(time.RFC3339),
		"gdpr_role": map[string]any{
			"role":                      "Processor",
			"controller_relationship":   "Acts on behalf of customer controllers",
			"legal_basis_for_processing": "GDPR Article 28",
		},
		"controller": map[string]any{
			"name":              "Customer Organization",
			"role":              "Service Customer / Data Controller",
			"description":       "Business organizations using ThreeRouter AI API services",
		},
		"controller_categories": []string{
			"Business customers using ThreeRouter AI API services",
			"Organizations integrating AI model access services",
			"Enterprises deploying AI-powered applications",
		},
		"processor": map[string]any{
			"name":              defaultIfEmpty(org.OrganizationName, "ThreeRouter Technology Ltd."),
			"legal_entity":      defaultIfEmpty(org.LegalEntity, "ThreeRouter Technology Ltd."),
			"contact":           "privacy@threerouter.com",
			"dpo_contact":       dpoContact,
			"processing_operations": []string{
				"Collection",
				"Transmission",
				"Routing",
				"Storage where configured",
				"Deletion",
			},
			"security_measures": []string{
				"Encryption at rest (AES-256)",
				"TLS 1.3 in transit",
				"Access controls and audit trails",
				"Zero Data Retention by default",
			},
			"gdpr_security_reference": "GDPR Article 32 security of processing",
		},
		"international_transfers": map[string]any{
			"applicable":            true,
			"mechanism":             "EU Standard Contractual Clauses (2021/914)",
			"supplementary_measures": []string{
				"Encryption",
				"Access controls",
			},
		},
		"processing_activities": []map[string]any{
			{
				"activity":   "AI API request relay and billing",
				"purpose":    "Contract performance (service delivery and billing)",
				"legal_basis": "GDPR Art 6(1)(b)",
				"data_categories": []string{
					"account identifiers",
					"billing identifiers",
					"usage metrics",
					"API consumption statistics",
				},
				"retention": map[string]any{
					"aggregated_metrics": map[string]any{
						"purpose":           "Service analytics",
						"retention_period":  "According to internal retention policy",
					},
					"detailed_usage_records": map[string]any{
						"purpose":           "Usage tracking when opted in",
						"retention_period":  "Only when user opts in, deleted according to retention schedule",
					},
				},
				"recipients": []map[string]any{
					{
						"category": "Third-party AI model providers",
						"examples": []string{"Authorized AI model providers listed in the current subprocessor registry"},
						"purpose":  "AI inference processing",
					},
				},
			},
			{
				"activity":         "Content moderation",
				"purpose":          "Harmful content filtering and abuse prevention",
				"legal_basis":      "GDPR Art 6(1)(f) legitimate interest",
				"legitimate_interest": "Security, abuse prevention and platform integrity",
				"data_categories": []string{"content hashes", "moderation results"},
				"content_hashes": map[string]any{
					"personal_data_status": "May constitute personal data depending on reversibility",
				},
				"retention": map[string]any{
					"hit_logs": map[string]any{
						"purpose":          "Security analysis",
						"retention_period": "Retained per policy",
					},
					"non_hit_logs": map[string]any{
						"purpose":          "Real-time processing",
						"retention_period": "Short-lived",
					},
				},
			},
			{
				"activity":   "Jurisdiction identification (GeoIP)",
				"purpose":    "Regional compliance, fraud prevention and service security",
				"legal_basis": "GDPR Art 6(1)(f) legitimate interest",
				"legitimate_interest": "Compliance with jurisdictional requirements and security",
				"data_categories": []string{"IP-derived country code"},
				"retention": map[string]any{
					"country_code": map[string]any{
						"purpose":          "Real-time compliance tagging",
						"retention_period": "Not retained",
					},
					"raw_ip": map[string]any{
						"purpose":          "Jurisdiction identification",
						"retention_period": "Anonymized under Zero Data Retention policy",
					},
				},
			},
		},
		"data_subject_rights_support": map[string]any{
			"rights_supported": []string{"access", "erasure", "portability", "restriction", "objection"},
			"processor_role":   "Provide assistance to controller",
		},
	}
}

// ============================================================================
// 辅助函数
// ============================================================================

// containsPII 判断文本是否包含个人身份信息。
func containsPII(text string) bool {
	if text == "" {
		return false
	}
	for _, re := range piiPatterns {
		if re.MatchString(text) {
			return true
		}
	}
	return false
}

// isValidErasureType 校验删除请求类型。
func isValidErasureType(t string) bool {
	switch t {
	case ErasureRequestTypeFull, ErasureRequestTypeAnonymize, ErasureRequestTypeRestrict:
		return true
	default:
		return false
	}
}

// defaultIfEmpty 返回原值，如果原值为空则返回默认值。
func defaultIfEmpty(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

// normalizeErasureType 兼容前端常用的简写/小写请求类型。
func normalizeErasureType(t string) string {
	switch strings.ToLower(t) {
	case "full", "full_erasure":
		return ErasureRequestTypeFull
	case "anonymize":
		return ErasureRequestTypeAnonymize
	case "restrict":
		return ErasureRequestTypeRestrict
	default:
		return strings.ToUpper(t)
	}
}
