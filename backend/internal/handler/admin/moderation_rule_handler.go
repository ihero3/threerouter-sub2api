package admin

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ModerationRuleHandler 提供内容审核自定义规则的管理端 CRUD 接口。
//
// 路由前缀 /api/v1/admin/governance/moderation-rules/*
// （见 docs/合规方案.md 0.3 与 4.1.3）。
type ModerationRuleHandler struct {
	service *service.ModerationRuleService
}

// NewModerationRuleHandler 创建审核规则管理端处理器。
func NewModerationRuleHandler(svc *service.ModerationRuleService) *ModerationRuleHandler {
	return &ModerationRuleHandler{service: svc}
}

// ListRules GET /admin/governance/moderation-rules
func (h *ModerationRuleHandler) ListRules(c *gin.Context) {
	rules, err := h.service.ListRules(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": rules})
}

type createModerationRuleRequest struct {
	RuleID       string  `json:"rule_id"`
	RuleName     string  `json:"rule_name"`
	RuleType     string  `json:"rule_type"`
	RulePattern  string  `json:"rule_pattern"`
	Threshold    float64 `json:"threshold"`
	Action       string  `json:"action"`
	RiskCategory string  `json:"risk_category"`
	Enabled      bool    `json:"enabled"`
	Priority     int     `json:"priority"`
}

// CreateRule POST /admin/governance/moderation-rules
func (h *ModerationRuleHandler) CreateRule(c *gin.Context) {
	var req createModerationRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	rule, err := h.service.CreateRule(c.Request.Context(), service.CreateRuleInput{
		RuleID:       req.RuleID,
		RuleName:     req.RuleName,
		RuleType:     req.RuleType,
		RulePattern:  req.RulePattern,
		Threshold:    req.Threshold,
		Action:       req.Action,
		RiskCategory: req.RiskCategory,
		Enabled:      req.Enabled,
		Priority:     req.Priority,
		CreatedBy:    adminOperatorFromContext(c),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, rule)
}

type updateModerationRuleRequest struct {
	RuleName     *string  `json:"rule_name"`
	RuleType     *string  `json:"rule_type"`
	RulePattern  *string  `json:"rule_pattern"`
	Threshold    *float64 `json:"threshold"`
	Action       *string  `json:"action"`
	RiskCategory *string  `json:"risk_category"`
	Enabled      *bool    `json:"enabled"`
	Priority     *int     `json:"priority"`
}

// UpdateRule PUT /admin/governance/moderation-rules/:ruleId
func (h *ModerationRuleHandler) UpdateRule(c *gin.Context) {
	ruleID := strings.TrimSpace(c.Param("ruleId"))
	if ruleID == "" {
		response.BadRequest(c, "rule_id is required")
		return
	}
	var req updateModerationRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	rule, err := h.service.UpdateRule(c.Request.Context(), ruleID, service.UpdateRuleInput{
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

// DeleteRule DELETE /admin/governance/moderation-rules/:ruleId
func (h *ModerationRuleHandler) DeleteRule(c *gin.Context) {
	ruleID := strings.TrimSpace(c.Param("ruleId"))
	if ruleID == "" {
		response.BadRequest(c, "rule_id is required")
		return
	}
	if err := h.service.DeleteRule(c.Request.Context(), ruleID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"rule_id": ruleID, "deleted": true})
}

type setStrategyRequest struct {
	Strategy string `json:"strategy"`
}

// SetStrategy POST /admin/governance/moderation-rules/strategy
// 设置策略组合逻辑（OR/AND/WEIGHTED）。
func (h *ModerationRuleHandler) SetStrategy(c *gin.Context) {
	var req setStrategyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	h.service.SetStrategy(req.Strategy)
	response.Success(c, gin.H{"strategy": strings.ToUpper(strings.TrimSpace(req.Strategy))})
}
