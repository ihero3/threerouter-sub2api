package admin

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// GovernanceHandler 提供 AI 治理与合规管理端接口。
//
// 路由前缀 /api/v1/admin/governance/*（见 docs/合规方案.md 0.2 与第五章，
// 避开 AdminComplianceGuard 对 /admin/compliance* 的豁免）。
type GovernanceHandler struct {
	service             *service.ComplianceService
	templateService     *service.PolicyTemplateService
	mappingService      *service.ComplianceMappingService
	credentialService   *service.ComplianceCredentialService
}

// NewGovernanceHandler 创建治理管理端处理器。
func NewGovernanceHandler(svc *service.ComplianceService, templateService *service.PolicyTemplateService, mappingService *service.ComplianceMappingService, credentialService *service.ComplianceCredentialService) *GovernanceHandler {
	return &GovernanceHandler{service: svc, templateService: templateService, mappingService: mappingService, credentialService: credentialService}
}

// GetStatus GET /admin/governance/status
// 返回合规模块的总体状态（角色定位、能力开关等）。
func (h *GovernanceHandler) GetStatus(c *gin.Context) {
	response.Success(c, gin.H{
		"module":          "ai_governance",
		"primary_role":    service.EUAIActRoleInfrastructureProvider,
		"secondary_roles": []string{service.EUAIActRoleModelAccessProvider},
		"risk_tier":       service.EUAIActRiskTierNonHighRisk,
		"capabilities": gin.H{
			"risk_tagging":       true,
			"audit_logging":      true,
			"gdpr_erasure":       true,
			"gdpr_data_export":   true,
			"consent_management": true,
			"eu_ai_act_report":   true,
		},
	})
}

// ListAuditLogs GET /admin/governance/audit-logs
func (h *GovernanceHandler) ListAuditLogs(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filter := service.ComplianceAuditLogFilter{
		ComplianceType: strings.TrimSpace(c.Query("compliance_type")),
		SubjectType:    strings.TrimSpace(c.Query("subject_type")),
		Pagination: pagination.PaginationParams{
			Page:      page,
			PageSize:  pageSize,
			SortOrder: pagination.SortOrderDesc,
		},
	}
	if raw := strings.TrimSpace(c.Query("subject_id")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid subject_id")
			return
		}
		filter.SubjectID = &id
	}
	if raw := strings.TrimSpace(c.Query("from")); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			response.BadRequest(c, "Invalid from (RFC3339 expected)")
			return
		}
		filter.StartTime = &t
	}
	if raw := strings.TrimSpace(c.Query("to")); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			response.BadRequest(c, "Invalid to (RFC3339 expected)")
			return
		}
		filter.EndTime = &t
	}
	items, pageResult, err := h.service.ListComplianceAuditLogs(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, pageResult.Total, pageResult.Page, pageResult.PageSize)
}

// RiskTags GET /admin/governance/risk-tags
// 返回系统支持的风险标签目录（模型属性标签 + 合规风险标签）。
func (h *GovernanceHandler) RiskTags(c *gin.Context) {
	response.Success(c, gin.H{
		"model_tags": []gin.H{
			{"tag": "MODEL_FRONTIER", "description": "使用前沿模型"},
			{"tag": "MODEL_OPEN_SOURCE", "description": "使用开源模型"},
			{"tag": "MODEL_EXTERNAL_PROVIDER", "description": "使用外部提供者模型"},
			{"tag": "MODEL_DATA_RETENTION_UNKNOWN", "description": "模型提供者数据保留策略未知"},
		},
		"risk_tags": []gin.H{
			{"tag": service.RiskTagPIIDetected, "description": "检测到个人身份信息"},
			{"tag": service.RiskTagHighRiskUseCase, "description": "高风险应用场景"},
			{"tag": service.RiskTagCrossBorderTransfer, "description": "跨境数据传输"},
			{"tag": service.RiskTagSanctionedRegion, "description": "制裁区域访问"},
			{"tag": service.RiskTagContentPolicyViolate, "description": "内容政策违规"},
			{"tag": service.RiskTagOutputControlLimited, "description": "输出控制受限"},
			{"tag": service.RiskTagNoTrainingGuarantee, "description": "无训练数据保障"},
			{"tag": service.RiskTagRateLimitExceeded, "description": "超限调用"},
			{"tag": service.RiskTagAnomalousBehavior, "description": "异常行为"},
		},
	})
}

// EUAIActAssessment GET /admin/governance/eu-ai-act/assessment
// 生成 EU AI Act 合规评估报告（ZDR 兼容版，基于聚合指标）。
func (h *GovernanceHandler) EUAIActAssessment(c *gin.Context) {
	report := h.service.GenerateEUAIActAssessment(c.Request.Context())
	response.Success(c, report)
}

// ExportEUAIActAssessment POST /admin/governance/eu-ai-act/assessment
// 导出评估报告为下载文件（JSON）。
func (h *GovernanceHandler) ExportEUAIActAssessment(c *gin.Context) {
	data, filename, err := h.service.ExportEUAIActAssessment(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(200, "application/json", data)
}

// DataProcessingRecord GET /admin/governance/gdpr/data-processing-record
// 返回 GDPR Art 30 数据处理活动记录（ROPA）。
func (h *GovernanceHandler) DataProcessingRecord(c *gin.Context) {
	record := h.service.GenerateDataProcessingRecord(c.Request.Context())
	response.Success(c, record)
}

// ListErasureRequests GET /admin/governance/gdpr/erasure-requests
func (h *GovernanceHandler) ListErasureRequests(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filter := service.DataErasureRequestFilter{
		Status: strings.TrimSpace(c.Query("status")),
		Pagination: pagination.PaginationParams{
			Page:      page,
			PageSize:  pageSize,
			SortOrder: pagination.SortOrderDesc,
		},
	}
	if raw := strings.TrimSpace(c.Query("user_id")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid user_id")
			return
		}
		filter.UserID = &id
	}
	items, pageResult, err := h.service.ListDataErasureRequests(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, pageResult.Total, pageResult.Page, pageResult.PageSize)
}

type processErasureRequest struct {
	Approved bool   `json:"approved"`
	Reason   string `json:"reason"`
}

// ProcessErasureRequest POST /admin/governance/gdpr/erasure-requests/:id/process
func (h *GovernanceHandler) ProcessErasureRequest(c *gin.Context) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid id")
		return
	}
	var req processErasureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if !req.Approved && strings.TrimSpace(req.Reason) == "" {
		response.BadRequest(c, "Rejection requires a reason")
		return
	}
	operator := adminOperatorFromContext(c)
	if err := h.service.ProcessDataErasure(c.Request.Context(), id, req.Approved, req.Reason, operator); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"id": id, "approved": req.Approved})
}

// adminOperatorFromContext 从鉴权上下文提取操作者标识（用户 ID 字符串），失败时回退 "admin"。
func adminOperatorFromContext(c *gin.Context) string {
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok && subject.UserID > 0 {
		return "admin:" + strconv.FormatInt(subject.UserID, 10)
	}
	return "admin"
}

// GetComplianceTemplates GET /admin/governance/templates
// 列出行业合规模板，支持 ?industry= 过滤。同时返回当前激活的模板编码。
func (h *GovernanceHandler) GetComplianceTemplates(c *gin.Context) {
	industry := strings.TrimSpace(c.Query("industry"))
	items, err := h.templateService.ListTemplates(c.Request.Context(), industry)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	activeCode, _ := h.templateService.GetActiveTemplateCode(c.Request.Context())
	response.Success(c, gin.H{"items": items, "active_template_code": activeCode})
}

type applyTemplateRequest struct {
	TemplateCode string `json:"template_code"`
}

// ApplyComplianceTemplate POST /admin/governance/templates/apply
func (h *GovernanceHandler) ApplyComplianceTemplate(c *gin.Context) {
	var req applyTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if strings.TrimSpace(req.TemplateCode) == "" {
		response.BadRequest(c, "template_code is required")
		return
	}
	tpl, err := h.templateService.ApplyTemplate(c.Request.Context(), service.ApplyTemplateInput{
		TemplateCode: req.TemplateCode,
		Operator:     adminOperatorFromContext(c),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, tpl)
}

type createCustomTemplateRequest struct {
	TemplateCode string           `json:"template_code"`
	Industry     string           `json:"industry"`
	Description  string           `json:"description"`
	Rules        []map[string]any `json:"rules"`
	RiskTags     []string         `json:"risk_tags"`
}

// CreateCustomTemplate POST /admin/governance/templates/custom
func (h *GovernanceHandler) CreateCustomTemplate(c *gin.Context) {
	var req createCustomTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	tpl, err := h.templateService.CreateCustomTemplate(c.Request.Context(), service.CreateCustomTemplateInput{
		TemplateCode: req.TemplateCode,
		Industry:     req.Industry,
		Description:  req.Description,
		Rules:        req.Rules,
		RiskTags:     req.RiskTags,
		Operator:     adminOperatorFromContext(c),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, tpl)
}

type generateDPARequest struct {
	ControllerName    string `json:"controller_name"`
	ControllerContact string `json:"controller_contact"`
}

// GenerateDPA POST /admin/governance/gdpr/dpa/generate
// 生成数据处理协议（DPA，GDPR Art 28）。返回可下载的 JSON 文件。
func (h *GovernanceHandler) GenerateDPA(c *gin.Context) {
	var req generateDPARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if strings.TrimSpace(req.ControllerName) == "" {
		response.BadRequest(c, "controller_name is required")
		return
	}
	if strings.TrimSpace(req.ControllerContact) == "" {
		response.BadRequest(c, "controller_contact is required")
		return
	}
	data, filename, err := h.service.GenerateDPA(c.Request.Context(), req.ControllerName, req.ControllerContact)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(200, "application/json", data)
}

// ListCredentials GET /admin/governance/credentials
// 列出合规证据包（合规凭证）。支持 ?type= 和 ?status= 过滤。
func (h *GovernanceHandler) ListCredentials(c *gin.Context) {
	credentialType := strings.TrimSpace(c.Query("type"))
	status := strings.TrimSpace(c.Query("status"))
	items, err := h.credentialService.ListCredentials(c.Request.Context(), credentialType, status)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

// GetCredential GET /admin/governance/credentials/:id
// 获取单个合规凭证详情。
func (h *GovernanceHandler) GetCredential(c *gin.Context) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid id")
		return
	}
	cred, err := h.credentialService.GetCredential(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if cred == nil {
		response.BadRequest(c, "Credential not found")
		return
	}
	response.Success(c, cred)
}

type createCredentialRequest struct {
	CredentialID     string                 `json:"credential_id"`
	CredentialType   string                 `json:"credential_type"`
	Issuer           string                 `json:"issuer"`
	IssuerType       string                 `json:"issuer_type"`
	Scope            string                 `json:"scope"`
	ValidFrom        string                 `json:"valid_from"`
	ValidUntil       string                 `json:"valid_until"`
	EvidenceHashes   string                 `json:"evidence_hashes"`
	DigitalSignature string                 `json:"digital_signature"`
	Metadata         map[string]interface{} `json:"metadata"`
}

// CreateCredential POST /admin/governance/credentials
// 创建合规凭证（证据包）。
func (h *GovernanceHandler) CreateCredential(c *gin.Context) {
	var req createCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if strings.TrimSpace(req.CredentialID) == "" {
		response.BadRequest(c, "credential_id is required")
		return
	}
	if strings.TrimSpace(req.CredentialType) == "" {
		response.BadRequest(c, "credential_type is required")
		return
	}
	if strings.TrimSpace(req.Issuer) == "" {
		response.BadRequest(c, "issuer is required")
		return
	}
	var validFrom, validUntil time.Time
	var err error
	if req.ValidFrom != "" {
		validFrom, err = time.Parse(time.RFC3339, req.ValidFrom)
		if err != nil {
			response.BadRequest(c, "Invalid valid_from (RFC3339 expected)")
			return
		}
	}
	if req.ValidUntil != "" {
		validUntil, err = time.Parse(time.RFC3339, req.ValidUntil)
		if err != nil {
			response.BadRequest(c, "Invalid valid_until (RFC3339 expected)")
			return
		}
	}
	cred, err := h.credentialService.CreateCredential(c.Request.Context(), service.CreateCredentialInput{
		CredentialID:     req.CredentialID,
		CredentialType:   req.CredentialType,
		Issuer:           req.Issuer,
		IssuerType:       req.IssuerType,
		Scope:            req.Scope,
		ValidFrom:        validFrom,
		ValidUntil:       validUntil,
		EvidenceHashes:   req.EvidenceHashes,
		DigitalSignature: req.DigitalSignature,
		Metadata:         req.Metadata,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cred)
}

// RevokeCredential POST /admin/governance/credentials/:id/revoke
// 吊销合规凭证（设置为 revoked 状态）。
func (h *GovernanceHandler) RevokeCredential(c *gin.Context) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid id")
		return
	}
	if err := h.credentialService.RevokeCredential(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"id": id, "status": "revoked"})
}

// ActivateCredential POST /admin/governance/credentials/:id/activate
// 激活合规凭证（设置为 active 状态）。
func (h *GovernanceHandler) ActivateCredential(c *gin.Context) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid id")
		return
	}
	if err := h.credentialService.UpdateCredentialStatus(c.Request.Context(), id, "active"); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"id": id, "status": "active"})
}

// DeleteCredential DELETE /admin/governance/credentials/:id
// 删除合规凭证。
func (h *GovernanceHandler) DeleteCredential(c *gin.Context) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid id")
		return
	}
	if err := h.credentialService.DeleteCredential(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, nil)
}

// GetJurisdictionMapping GET /admin/governance/jurisdiction/mapping
// 跨法域合规映射（合规方案 4.4.1）。查询参数：company_region（必填）、industry、service_type。
// 无参数时返回支持的法域列表。
func (h *GovernanceHandler) GetJurisdictionMapping(c *gin.Context) {
	region := strings.TrimSpace(c.Query("company_region"))
	if region == "" {
		response.Success(c, gin.H{"supported_jurisdictions": h.mappingService.SupportedJurisdictions()})
		return
	}
	result, err := h.mappingService.MapJurisdiction(c.Request.Context(), service.JurisdictionMappingRequest{
		CompanyRegion: region,
		Industry:      strings.TrimSpace(c.Query("industry")),
		ServiceType:   strings.TrimSpace(c.Query("service_type")),
	})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}
