package handler

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// GovernanceUserHandler 提供 AI 治理与合规的用户端接口（GDPR 数据主体权利 + Account 级合规配置）。
//
// 路由前缀 /api/v1/governance/*（见 docs/合规方案.md 第五章）。
type GovernanceUserHandler struct {
	service                  *service.ComplianceService
	complianceProfileService *service.UserComplianceProfileService
	policyTemplateService    *service.PolicyTemplateService
	moderationRuleService    *service.ModerationRuleService
	mappingService           *service.ComplianceMappingService
	credentialService        *service.ComplianceCredentialService
}

// NewGovernanceUserHandler 创建用户端治理处理器。
func NewGovernanceUserHandler(
	svc *service.ComplianceService,
	complianceProfileService *service.UserComplianceProfileService,
	policyTemplateService *service.PolicyTemplateService,
	moderationRuleService *service.ModerationRuleService,
	mappingService *service.ComplianceMappingService,
	credentialService *service.ComplianceCredentialService,
) *GovernanceUserHandler {
	return &GovernanceUserHandler{
		service:                  svc,
		complianceProfileService: complianceProfileService,
		policyTemplateService:    policyTemplateService,
		moderationRuleService:    moderationRuleService,
		mappingService:           mappingService,
		credentialService:        credentialService,
	}
}

func (h *GovernanceUserHandler) currentUserID(c *gin.Context) (int64, bool) {
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok && subject.UserID > 0 {
		return subject.UserID, true
	}
	return 0, false
}

type dataErasureRequestBody struct {
	RequestType  string `json:"request_type" binding:"required"`
	ScopeDetails string `json:"scope_details"`
}

// RequestDataErasure POST /governance/data-erasure/request
// 用户提交数据删除请求（GDPR Art 17）。
func (h *GovernanceUserHandler) RequestDataErasure(c *gin.Context) {
	userID, ok := h.currentUserID(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var body dataErasureRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	req, err := h.service.RequestDataErasure(c.Request.Context(), service.DataErasureRequest{
		UserID:       userID,
		RequestType:  strings.TrimSpace(body.RequestType),
		ScopeDetails: body.ScopeDetails,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, req)
}

// ExportData POST /governance/data-export
// 用户导出个人数据（GDPR Art 20 数据可携权）。
func (h *GovernanceUserHandler) ExportData(c *gin.Context) {
	userID, ok := h.currentUserID(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	data, err := h.service.ExportUserData(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Content-Disposition", "attachment; filename=personal-data-export.json")
	c.Data(200, "application/json", data)
}

// ListDataErasureRequests GET /governance/data-erasure/requests
// 获取用户的数据删除请求历史。
func (h *GovernanceUserHandler) ListDataErasureRequests(c *gin.Context) {
	userID, ok := h.currentUserID(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	reqs, _, err := h.service.ListDataErasureRequests(c.Request.Context(), service.DataErasureRequestFilter{
		UserID: &userID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, reqs)
}

// GetConsent GET /governance/consent
// 获取用户的同意状态列表。
func (h *GovernanceUserHandler) GetConsent(c *gin.Context) {
	userID, ok := h.currentUserID(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if consentType := strings.TrimSpace(c.Query("consent_type")); consentType != "" {
		consent, err := h.service.GetUserConsent(c.Request.Context(), userID, consentType)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		response.Success(c, convertConsentToResponse(consent))
		return
	}
	consents, err := h.service.ListUserConsents(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result := make([]gin.H, 0, len(consents))
	for i := range consents {
		result = append(result, convertConsentToResponse(&consents[i]))
	}
	response.Success(c, result)
}

func convertConsentToResponse(consent *service.UserConsent) gin.H {
	if consent == nil {
		return nil
	}
	status := "revoked"
	if consent.Granted {
		status = "granted"
	}
	return gin.H{
		"id":          consent.ID,
		"user_id":     consent.UserID,
		"consent_type": consent.ConsentType,
		"status":      status,
		"granted_at":  consent.GrantedAt,
		"revoked_at":  consent.RevokedAt,
		"source":      consent.Source,
	}
}

type setConsentBody struct {
	ConsentType string `json:"consent_type" binding:"required"`
	Granted     bool   `json:"granted"`
	Source      string `json:"source"`
}

// SetConsent POST /governance/consent
// 设置用户同意（GDPR Art 7）。
func (h *GovernanceUserHandler) SetConsent(c *gin.Context) {
	userID, ok := h.currentUserID(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var body setConsentBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	source := body.Source
	if source == "" {
		source = "user_portal"
	}
	if err := h.service.SetUserConsent(c.Request.Context(), userID, body.ConsentType, body.Granted, source); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"consent_type": body.ConsentType, "granted": body.Granted})
}

// ============================================================================
// Account 级 AI Governance & Compliance 配置接口
// ============================================================================

// GetComplianceProfile GET /governance/profile
// 获取当前 Account 的合规档案。
func (h *GovernanceUserHandler) GetComplianceProfile(c *gin.Context) {
	userID, ok := h.currentUserID(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	profile, err := h.complianceProfileService.GetOrCreateDefault(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, profile)
}

// updateComplianceProfileBody 是更新合规档案的请求体。
type updateComplianceProfileBody struct {
	ZDRMode              string               `json:"zdr_mode"`
	DetailRetentionDays  int                  `json:"detail_retention_days"`
	ComplianceFrameworks []string             `json:"compliance_frameworks"`
	ModerationPolicy     *service.UserModerationPolicy `json:"moderation_policy"`
}

// UpdateComplianceProfile PUT /governance/profile
// 更新当前 Account 的合规档案。
func (h *GovernanceUserHandler) UpdateComplianceProfile(c *gin.Context) {
	userID, ok := h.currentUserID(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var body updateComplianceProfileBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	input := service.UpdateProfileInput{
		ZDRMode:              &body.ZDRMode,
		DetailRetentionDays:  &body.DetailRetentionDays,
		ComplianceFrameworks: body.ComplianceFrameworks,
	}
	if body.ModerationPolicy != nil {
		input.ModerationPolicy = body.ModerationPolicy
	}
	profile, err := h.complianceProfileService.UpdateProfile(c.Request.Context(), userID, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, profile)
}

// applyComplianceTemplateBody 是应用模板的请求体。
type applyComplianceTemplateBody struct {
	TemplateCode string `json:"template_code" binding:"required"`
}

// ApplyComplianceTemplate POST /governance/templates/apply
// 应用某个行业模板到当前 Account。
func (h *GovernanceUserHandler) ApplyComplianceTemplate(c *gin.Context) {
	userID, ok := h.currentUserID(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var body applyComplianceTemplateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	profile, err := h.complianceProfileService.ApplyTemplate(c.Request.Context(), userID, body.TemplateCode)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, profile)
}

// ListComplianceTemplates GET /governance/templates
// 列出可用行业模板。
func (h *GovernanceUserHandler) ListComplianceTemplates(c *gin.Context) {
	userID, ok := h.currentUserID(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	templates, err := h.policyTemplateService.ListTemplates(c.Request.Context(), c.Query("industry"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var items []gin.H
	for _, tpl := range templates {
		items = append(items, gin.H{
			"code":        tpl.TemplateCode,
			"industry":    tpl.Industry,
			"description": tpl.Description,
		})
	}
	activeCode, err := h.complianceProfileService.GetActiveTemplateCode(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{
		"items":                items,
		"active_template_code": activeCode,
	})
}

// ListModerationRules GET /governance/moderation-rules
// 列出平台规则库（供用户选择启用）。
func (h *GovernanceUserHandler) ListModerationRules(c *gin.Context) {
	rules, err := h.moderationRuleService.ListRules(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, rules)
}

// ListUserModerationRules GET /governance/moderation-rules/user
// 列出当前用户的自定义规则。
func (h *GovernanceUserHandler) ListUserModerationRules(c *gin.Context) {
	userID, ok := h.currentUserID(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	rules, err := h.moderationRuleService.ListUserRules(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, rules)
}

type createUserModerationRuleRequest struct {
	RuleName     string  `json:"rule_name" binding:"required"`
	RuleType     string  `json:"rule_type" binding:"required"`
	RulePattern  string  `json:"rule_pattern" binding:"required"`
	Threshold    float64 `json:"threshold"`
	Action       string  `json:"action"`
	RiskCategory string  `json:"risk_category"`
	Enabled      bool    `json:"enabled"`
	Priority     int     `json:"priority"`
}

// CreateUserModerationRule POST /governance/moderation-rules/user
// 创建当前用户的自定义规则。
func (h *GovernanceUserHandler) CreateUserModerationRule(c *gin.Context) {
	userID, ok := h.currentUserID(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req createUserModerationRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	rule, err := h.moderationRuleService.CreateUserRule(c.Request.Context(), userID, service.CreateUserRuleInput{
		RuleName:     req.RuleName,
		RuleType:     req.RuleType,
		RulePattern:  req.RulePattern,
		Threshold:    req.Threshold,
		Action:       req.Action,
		RiskCategory: req.RiskCategory,
		Enabled:      req.Enabled,
		Priority:     req.Priority,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, rule)
}

type updateUserModerationRuleRequest struct {
	RuleName     *string  `json:"rule_name"`
	RuleType     *string  `json:"rule_type"`
	RulePattern  *string  `json:"rule_pattern"`
	Threshold    *float64 `json:"threshold"`
	Action       *string  `json:"action"`
	RiskCategory *string  `json:"risk_category"`
	Enabled      *bool    `json:"enabled"`
	Priority     *int     `json:"priority"`
}

// UpdateUserModerationRule PUT /governance/moderation-rules/user/:ruleId
// 更新当前用户的自定义规则。
func (h *GovernanceUserHandler) UpdateUserModerationRule(c *gin.Context) {
	userID, ok := h.currentUserID(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	ruleID := strings.TrimSpace(c.Param("ruleId"))
	if ruleID == "" {
		response.BadRequest(c, "rule_id is required")
		return
	}
	var req updateUserModerationRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	rule, err := h.moderationRuleService.UpdateUserRule(c.Request.Context(), userID, ruleID, service.UpdateRuleInput{
		RuleName:     req.RuleName,
		RuleType:     req.RuleType,
		RulePattern:  req.RulePattern,
		Threshold:    req.Threshold,
		Action:       req.Action,
		RiskCategory: req.RiskCategory,
		Enabled:      req.Enabled,
		Priority:     req.Priority,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, rule)
}

// DeleteUserModerationRule DELETE /governance/moderation-rules/user/:ruleId
// 删除当前用户的自定义规则。
func (h *GovernanceUserHandler) DeleteUserModerationRule(c *gin.Context) {
	userID, ok := h.currentUserID(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	ruleID := strings.TrimSpace(c.Param("ruleId"))
	if ruleID == "" {
		response.BadRequest(c, "rule_id is required")
		return
	}
	if err := h.moderationRuleService.DeleteUserRule(c.Request.Context(), userID, ruleID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"rule_id": ruleID, "deleted": true})
}

// GetComplianceStatus GET /governance/status
// 获取当前 Account 的合规状态。
func (h *GovernanceUserHandler) GetComplianceStatus(c *gin.Context) {
	userID, ok := h.currentUserID(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	profile, err := h.complianceProfileService.GetOrCreateDefault(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	_, retentionDays, retentionExpiresAt, err := h.complianceProfileService.GetEffectiveZDRSettings(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{
		"profile":              profile,
		"retention_days":         retentionDays,
		"retention_expires_at":   retentionExpiresAt,
	})
}

// GetJurisdictionMapping GET /governance/jurisdiction/mapping
// 跨法域合规映射。查询参数：company_region（必填）、industry、service_type。
// 无参数时返回支持的法域列表。
func (h *GovernanceUserHandler) GetJurisdictionMapping(c *gin.Context) {
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

// GetUserJurisdictionMapping GET /governance/jurisdiction/mapping/user
// 获取用户保存的跨法域映射配置。
func (h *GovernanceUserHandler) GetUserJurisdictionMapping(c *gin.Context) {
	userID, ok := h.currentUserID(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	mapping, err := h.mappingService.GetUserMapping(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, mapping)
}

// SaveJurisdictionMapping POST /governance/jurisdiction/mapping/save
// 保存跨法域映射结果，并可选择自动应用到合规规则。
// Body: { company_region, industry, service_type, apply_rules: boolean }
func (h *GovernanceUserHandler) SaveJurisdictionMapping(c *gin.Context) {
	userID, ok := h.currentUserID(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req struct {
		CompanyRegion string `json:"company_region" binding:"required"`
		Industry      string `json:"industry"`
		ServiceType   string `json:"service_type"`
		ApplyRules    bool   `json:"apply_rules"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.mappingService.MapJurisdiction(c.Request.Context(), service.JurisdictionMappingRequest{
		CompanyRegion: req.CompanyRegion,
		Industry:      req.Industry,
		ServiceType:   req.ServiceType,
	})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.mappingService.SaveMapping(c.Request.Context(), userID, result); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	var appliedRules []string
	if req.ApplyRules {
		appliedRules, err = h.mappingService.ApplyMappingToRules(c.Request.Context(), userID, result)
		if err != nil {
			response.InternalError(c, err.Error())
			return
		}
	}
	response.Success(c, gin.H{
		"result":        result,
		"applied_rules": appliedRules,
	})
}

// GenerateDPA POST /governance/gdpr/dpa/generate
// 生成数据处理协议（DPA）。返回可下载的 JSON 文件。
func (h *GovernanceUserHandler) GenerateDPA(c *gin.Context) {
	if _, ok := h.currentUserID(c); !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req struct {
		ControllerName    string `json:"controller_name"`
		ControllerContact string `json:"controller_contact"`
	}
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

// ListCredentials GET /governance/credentials
// 列出当前用户的合规凭证。支持 ?type= 和 ?status= 过滤。
func (h *GovernanceUserHandler) ListCredentials(c *gin.Context) {
	if _, ok := h.currentUserID(c); !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	credentialType := strings.TrimSpace(c.Query("type"))
	status := strings.TrimSpace(c.Query("status"))
	items, err := h.credentialService.ListCredentials(c.Request.Context(), credentialType, status)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

// ListAuditLogs GET /governance/audit-logs
// 用户查看自己的合规审计日志。
func (h *GovernanceUserHandler) ListAuditLogs(c *gin.Context) {
	userID, ok := h.currentUserID(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
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
	userIDInt64 := int64(userID)
	filter.SubjectID = &userIDInt64
	items, pageResult, err := h.service.ListComplianceAuditLogs(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, pageResult.Total, pageResult.Page, pageResult.PageSize)
}

// RiskTags GET /governance/risk-tags
// 返回系统支持的风险标签目录。
func (h *GovernanceUserHandler) RiskTags(c *gin.Context) {
	if _, ok := h.currentUserID(c); !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
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

// EUAIActAssessment GET /governance/eu-ai-act/assessment
// 用户查看 EU AI Act 合规评估报告。
func (h *GovernanceUserHandler) EUAIActAssessment(c *gin.Context) {
	_, ok := h.currentUserID(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	report := h.service.GenerateEUAIActAssessment(c.Request.Context())
	response.Success(c, report)
}

// ExportEUAIActAssessment POST /governance/eu-ai-act/assessment
// 用户导出 EU AI Act 评估报告。
func (h *GovernanceUserHandler) ExportEUAIActAssessment(c *gin.Context) {
	_, ok := h.currentUserID(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	data, filename, err := h.service.ExportEUAIActAssessment(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(200, "application/json", data)
}

// DataProcessingRecord GET /governance/gdpr/data-processing-record
// 用户查看 GDPR Art 30 数据处理活动记录（ROPA）。
func (h *GovernanceUserHandler) DataProcessingRecord(c *gin.Context) {
	_, ok := h.currentUserID(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	record := h.service.GenerateDataProcessingRecord(c.Request.Context())
	response.Success(c, record)
}
