package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// 内容审核自定义规则引擎（ModerationRule）
// 见 docs/合规方案.md 0.3 与 4.1.3，持久化表 moderation_rules（migration 163）。
//
// 本增量在既有 ContentModerationService 的关键词/第三方审核之外，补充
// 可管理的自定义规则（KEYWORD / REGEX / PATTERN），支持 CRUD 与优先级排序，
// 并通过 ModerationRuleEngine 在入口审核流程中评估，产出 ALLOW/REVIEW/BLOCK 决策。
// ============================================================================

// 规则类型常量。
const (
	ModerationRuleTypeKeyword = "KEYWORD" // 关键词包含匹配（大小写不敏感）
	ModerationRuleTypeRegex   = "REGEX"   // 正则表达式匹配
	ModerationRuleTypePattern = "PATTERN" // 通配符模式（* 匹配任意字符），内部编译为正则
)

// 规则动作常量（与内容审核动作语义对齐）。
const (
	ModerationRuleActionAllow  = "ALLOW"
	ModerationRuleActionReview = "REVIEW"
	ModerationRuleActionBlock  = "BLOCK"
)

// 策略组合逻辑常量。
const (
	ModerationStrategyOR       = "OR"       // 任一命中即触发（取最严动作）
	ModerationStrategyAND      = "AND"      // 全部命中才触发
	ModerationStrategyWeighted = "WEIGHTED" // 命中权重（threshold）累加超过 1.0 触发
)

// ModerationRule 是一条内容审核自定义规则。
// UserID 为 nil 表示全局规则（Admin 创建），非 nil 表示用户自定义规则。
type ModerationRule struct {
	ID           int64     `json:"id"`
	RuleID       string    `json:"rule_id"`
	RuleName     string    `json:"rule_name"`
	RuleType     string    `json:"rule_type"`
	RulePattern  string    `json:"rule_pattern"`
	Threshold    float64   `json:"threshold"`
	Action       string    `json:"action"`
	RiskCategory string    `json:"risk_category"`
	Enabled      bool      `json:"enabled"`
	Priority     int       `json:"priority"`
	CreatedBy    string    `json:"created_by"`
	UserID       *int64    `json:"user_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ModerationRuleRepository 定义自定义审核规则持久化。
type ModerationRuleRepository interface {
	List(ctx context.Context, enabledOnly bool) ([]ModerationRule, error)
	ListByUser(ctx context.Context, userID int64, enabledOnly bool) ([]ModerationRule, error)
	ListByUserAll(ctx context.Context, enabledOnly bool) ([]ModerationRule, error)
	GetByRuleID(ctx context.Context, ruleID string) (*ModerationRule, error)
	Create(ctx context.Context, rule *ModerationRule) error
	Update(ctx context.Context, rule *ModerationRule) error
	Delete(ctx context.Context, ruleID string) error
}

// ModerationRuleMatch 记录一条规则命中详情。
type ModerationRuleMatch struct {
	RuleID       string  `json:"rule_id"`
	RuleName     string  `json:"rule_name"`
	RuleType     string  `json:"rule_type"`
	RiskCategory string  `json:"risk_category"`
	Action       string  `json:"action"`
	Threshold    float64 `json:"threshold"`
	MatchedText  string  `json:"matched_text"`
}

// ModerationRuleDecision 是规则引擎对一段文本的评估结果。
type ModerationRuleDecision struct {
	Matched bool                  `json:"matched"`
	Action  string                `json:"action"` // 组合后的最终动作
	Matches []ModerationRuleMatch `json:"matches"`
}

// compiledRule 是内部编译后的规则（正则预编译），供引擎高效匹配。
type compiledRule struct {
	rule    ModerationRule
	matcher func(text string) string
}

// ModerationRuleEngine 持有编译后的规则快照，供入口审核热路径评估。
//
// 引擎不直接访问数据库；由 ModerationRuleService 在规则变更时调用 Reload
// 重建快照，保证热路径无锁竞争（读时使用 RWMutex 读锁）。
type ModerationRuleEngine struct {
	mu       sync.RWMutex
	rules    []compiledRule
	strategy string
}

// NewModerationRuleEngine 创建空引擎，默认 OR 组合策略。
func NewModerationRuleEngine() *ModerationRuleEngine {
	return &ModerationRuleEngine{strategy: ModerationStrategyOR}
}

// SetStrategy 设置策略组合逻辑（OR/AND/WEIGHTED）。
func (e *ModerationRuleEngine) SetStrategy(strategy string) {
	strategy = strings.ToUpper(strings.TrimSpace(strategy))
	switch strategy {
	case ModerationStrategyAND, ModerationStrategyWeighted:
	default:
		strategy = ModerationStrategyOR
	}
	e.mu.Lock()
	e.strategy = strategy
	e.mu.Unlock()
}

// Reload 用给定规则集重建编译快照（仅纳入 enabled 规则，按 priority 升序）。
func (e *ModerationRuleEngine) Reload(rules []ModerationRule) error {
	compiled := make([]compiledRule, 0, len(rules))
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		matcher, err := buildRuleMatcher(r)
		if err != nil {
			return fmt.Errorf("compile rule %s: %w", r.RuleID, err)
		}
		compiled = append(compiled, compiledRule{rule: r, matcher: matcher})
	}
	e.mu.Lock()
	e.rules = compiled
	e.mu.Unlock()
	return nil
}

// Evaluate 对文本评估所有启用规则，按策略组合得出最终动作。
// EvaluateForUser 与 Evaluate 等价，但仅评估 ruleID 属于白名单的全局规则 + 用户自定义规则。
// 传入 nil/空切片表示"全部启用"（保持向后兼容）。
// userID 用于匹配用户自定义规则（user_id = userID 的规则始终评估）。
// 详见 docs/合规升级方案.md 5.5。
func (e *ModerationRuleEngine) EvaluateForUser(text string, enabledRuleIDs []string, userID int64) ModerationRuleDecision {
	if len(enabledRuleIDs) == 0 && userID <= 0 {
		return e.Evaluate(text)
	}
	allow := make(map[string]struct{}, len(enabledRuleIDs))
	for _, id := range enabledRuleIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			allow[id] = struct{}{}
		}
	}
	// 如果白名单为空且没有用户ID，回退到全量评估
	if len(allow) == 0 && userID <= 0 {
		return e.Evaluate(text)
	}

	e.mu.RLock()
	rules := e.rules
	strategy := e.strategy
	e.mu.RUnlock()

	if len(rules) == 0 || strings.TrimSpace(text) == "" {
		return ModerationRuleDecision{Action: ModerationRuleActionAllow}
	}

	matches := make([]ModerationRuleMatch, 0)
	var weightSum float64
	enabledCount := 0
	for _, cr := range rules {
		// 用户自定义规则（user_id 匹配）→ 始终评估
		if cr.rule.UserID != nil && *cr.rule.UserID == userID {
			enabledCount++
			if matchedText := cr.matcher(text); matchedText != "" {
				matches = append(matches, ModerationRuleMatch{
					RuleID:       cr.rule.RuleID,
					RuleName:     cr.rule.RuleName,
					RuleType:     cr.rule.RuleType,
					RiskCategory: cr.rule.RiskCategory,
					Action:       cr.rule.Action,
					Threshold:    cr.rule.Threshold,
					MatchedText:  matchedText,
				})
				weightSum += cr.rule.Threshold
			}
			continue
		}
		// 其他用户的规则 → 跳过
		if cr.rule.UserID != nil {
			continue
		}
		// 全局规则 → 检查是否在白名单中
		if len(allow) > 0 {
			if _, ok := allow[cr.rule.RuleID]; !ok {
				continue
			}
		}
		enabledCount++
		if matchedText := cr.matcher(text); matchedText != "" {
			matches = append(matches, ModerationRuleMatch{
				RuleID:       cr.rule.RuleID,
				RuleName:     cr.rule.RuleName,
				RuleType:     cr.rule.RuleType,
				RiskCategory: cr.rule.RiskCategory,
				Action:       cr.rule.Action,
				Threshold:    cr.rule.Threshold,
				MatchedText:  matchedText,
			})
			weightSum += cr.rule.Threshold
		}
	}

	if len(matches) == 0 {
		return ModerationRuleDecision{Action: ModerationRuleActionAllow}
	}

	switch strategy {
	case ModerationStrategyAND:
		// 全部启用的规则均命中才触发；否则放行。
		if len(matches) < enabledCount {
			return ModerationRuleDecision{Action: ModerationRuleActionAllow, Matches: matches}
		}
	case ModerationStrategyWeighted:
		// 命中权重累加不足阈值 1.0 时放行。
		if weightSum < 1.0 {
			return ModerationRuleDecision{Action: ModerationRuleActionAllow, Matches: matches}
		}
	}

	return ModerationRuleDecision{
		Matched: true,
		Action:  strictestRuleAction(matches),
		Matches: matches,
	}
}

func (e *ModerationRuleEngine) Evaluate(text string) ModerationRuleDecision {
	e.mu.RLock()
	rules := e.rules
	strategy := e.strategy
	e.mu.RUnlock()

	if len(rules) == 0 || strings.TrimSpace(text) == "" {
		return ModerationRuleDecision{Action: ModerationRuleActionAllow}
	}

	matches := make([]ModerationRuleMatch, 0)
	var weightSum float64
	for _, cr := range rules {
		if matchedText := cr.matcher(text); matchedText != "" {
			matches = append(matches, ModerationRuleMatch{
				RuleID:       cr.rule.RuleID,
				RuleName:     cr.rule.RuleName,
				RuleType:     cr.rule.RuleType,
				RiskCategory: cr.rule.RiskCategory,
				Action:       cr.rule.Action,
				Threshold:    cr.rule.Threshold,
				MatchedText:  matchedText,
			})
			weightSum += cr.rule.Threshold
		}
	}

	if len(matches) == 0 {
		return ModerationRuleDecision{Action: ModerationRuleActionAllow}
	}

	switch strategy {
	case ModerationStrategyAND:
		// 全部启用规则均命中才触发；否则放行。
		if len(matches) < len(rules) {
			return ModerationRuleDecision{Action: ModerationRuleActionAllow, Matches: matches}
		}
	case ModerationStrategyWeighted:
		// 命中权重累加不足阈值 1.0 时放行。
		if weightSum < 1.0 {
			return ModerationRuleDecision{Action: ModerationRuleActionAllow, Matches: matches}
		}
	}

	return ModerationRuleDecision{
		Matched: true,
		Action:  strictestRuleAction(matches),
		Matches: matches,
	}
}

// strictestRuleAction 返回命中规则中最严格的动作（BLOCK > REVIEW > ALLOW）。
func strictestRuleAction(matches []ModerationRuleMatch) string {
	action := ModerationRuleActionAllow
	for _, m := range matches {
		switch m.Action {
		case ModerationRuleActionBlock:
			return ModerationRuleActionBlock
		case ModerationRuleActionReview:
			action = ModerationRuleActionReview
		}
	}
	return action
}

// buildRuleMatcher 依据规则类型构建匹配函数（正则/模式预编译）。
// 返回匹配到的文本，未匹配返回空字符串。
func buildRuleMatcher(r ModerationRule) (func(text string) string, error) {
	pattern := strings.TrimSpace(r.RulePattern)
	if pattern == "" {
		return func(string) string { return "" }, nil
	}
	switch strings.ToUpper(r.RuleType) {
	case ModerationRuleTypeKeyword:
		lower := strings.ToLower(pattern)
		return func(text string) string {
			lowerText := strings.ToLower(text)
			if idx := strings.Index(lowerText, lower); idx >= 0 {
				return text[idx : idx+len(pattern)]
			}
			return ""
		}, nil
	case ModerationRuleTypeRegex:
		re, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex: %w", err)
		}
		return func(text string) string {
			if match := re.FindString(text); match != "" {
				return match
			}
			return ""
		}, nil
	case ModerationRuleTypePattern:
		re, err := compileWildcardPattern(pattern)
		if err != nil {
			return nil, err
		}
		return func(text string) string {
			if match := re.FindString(text); match != "" {
				return match
			}
			return ""
		}, nil
	default:
		return nil, fmt.Errorf("unsupported rule_type %q", r.RuleType)
	}
}

// compileWildcardPattern 将通配符模式（* 匹配任意字符）编译为正则，其余字符转义。
func compileWildcardPattern(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	for _, ch := range pattern {
		if ch == '*' {
			b.WriteString(".*")
			continue
		}
		b.WriteString(regexp.QuoteMeta(string(ch)))
	}
	re, err := regexp.Compile("(?i)" + b.String())
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}
	return re, nil
}

// ModerationRuleService 提供自定义审核规则的 CRUD 与引擎同步。
type ModerationRuleService struct {
	repo   ModerationRuleRepository
	engine *ModerationRuleEngine
}

// NewModerationRuleService 构造规则服务并加载初始规则到引擎。
func NewModerationRuleService(repo ModerationRuleRepository) *ModerationRuleService {
	svc := &ModerationRuleService{repo: repo, engine: NewModerationRuleEngine()}
	return svc
}

// Engine 返回底层规则引擎（供 ContentModerationService 注入评估能力）。
func (s *ModerationRuleService) Engine() *ModerationRuleEngine {
	return s.engine
}

// ReloadEngine 从仓储加载启用规则并重建引擎快照。
func (s *ModerationRuleService) ReloadEngine(ctx context.Context) error {
	globalRules, err := s.repo.List(ctx, true)
	if err != nil {
		return err
	}
	// 加载所有用户的自定义规则，使 EvaluateForUser 能匹配到
	allUserRules, err := s.repo.ListByUserAll(ctx, true)
	if err != nil {
		return err
	}
	allRules := append(globalRules, allUserRules...)
	return s.engine.Reload(allRules)
}

// SetStrategy 设置策略组合逻辑。
func (s *ModerationRuleService) SetStrategy(strategy string) {
	s.engine.SetStrategy(strategy)
}

// ListRules 列出全部规则（含禁用）。
func (s *ModerationRuleService) ListRules(ctx context.Context) ([]ModerationRule, error) {
	return s.repo.List(ctx, false)
}

// CreateRuleInput 是创建规则的入参。
type CreateRuleInput struct {
	RuleID       string
	RuleName     string
	RuleType     string
	RulePattern  string
	Threshold    float64
	Action       string
	RiskCategory string
	Enabled      bool
	Priority     int
	CreatedBy    string
}

// CreateRule 创建一条规则并刷新引擎。
func (s *ModerationRuleService) CreateRule(ctx context.Context, input CreateRuleInput) (*ModerationRule, error) {
	rule, err := normalizeRuleInput(input)
	if err != nil {
		return nil, err
	}
	// 预编译校验，避免非法正则/模式落库。
	if _, err := buildRuleMatcher(*rule); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, rule); err != nil {
		return nil, err
	}
	_ = s.ReloadEngine(ctx)
	return rule, nil
}

// UpdateRuleInput 是更新规则的入参（指针字段为可选更新）。
type UpdateRuleInput struct {
	RuleName     *string
	RuleType     *string
	RulePattern  *string
	Threshold    *float64
	Action       *string
	RiskCategory *string
	Enabled      *bool
	Priority     *int
}

// UpdateRule 部分更新一条规则并刷新引擎。
func (s *ModerationRuleService) UpdateRule(ctx context.Context, ruleID string, input UpdateRuleInput) (*ModerationRule, error) {
	ruleID = strings.TrimSpace(ruleID)
	if ruleID == "" {
		return nil, fmt.Errorf("rule_id is required")
	}
	rule, err := s.repo.GetByRuleID(ctx, ruleID)
	if err != nil {
		return nil, err
	}
	if rule == nil {
		return nil, fmt.Errorf("moderation rule %q not found", ruleID)
	}
	if input.RuleName != nil {
		rule.RuleName = strings.TrimSpace(*input.RuleName)
	}
	if input.RuleType != nil {
		rule.RuleType = strings.ToUpper(strings.TrimSpace(*input.RuleType))
	}
	if input.RulePattern != nil {
		rule.RulePattern = *input.RulePattern
	}
	if input.Threshold != nil {
		rule.Threshold = *input.Threshold
	}
	if input.Action != nil {
		rule.Action = normalizeRuleAction(*input.Action)
	}
	if input.RiskCategory != nil {
		rule.RiskCategory = strings.TrimSpace(*input.RiskCategory)
	}
	if input.Enabled != nil {
		rule.Enabled = *input.Enabled
	}
	if input.Priority != nil {
		rule.Priority = *input.Priority
	}
	if _, err := buildRuleMatcher(*rule); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, rule); err != nil {
		return nil, err
	}
	_ = s.ReloadEngine(ctx)
	return rule, nil
}

// DeleteRule 删除一条规则并刷新引擎。
func (s *ModerationRuleService) DeleteRule(ctx context.Context, ruleID string) error {
	ruleID = strings.TrimSpace(ruleID)
	if ruleID == "" {
		return fmt.Errorf("rule_id is required")
	}
	if err := s.repo.Delete(ctx, ruleID); err != nil {
		return err
	}
	_ = s.ReloadEngine(ctx)
	return nil
}

// ListUserRules 列出指定用户的自定义规则。
func (s *ModerationRuleService) ListUserRules(ctx context.Context, userID int64) ([]ModerationRule, error) {
	return s.repo.ListByUser(ctx, userID, false)
}

// CreateUserRuleInput 是用户创建自定义规则的入参。
type CreateUserRuleInput struct {
	RuleName     string
	RuleType     string
	RulePattern  string
	Threshold    float64
	Action       string
	RiskCategory string
	Enabled      bool
	Priority     int
}

// CreateUserRule 为用户创建一条自定义规则并刷新引擎。
// rule_id 自动生成（格式：user-{userID}-{shortUUID}），user_id 自动设置为 userID。
func (s *ModerationRuleService) CreateUserRule(ctx context.Context, userID int64, input CreateUserRuleInput) (*ModerationRule, error) {
	ruleID := fmt.Sprintf("user-%d-%s", userID, shortUUID())
	rule, err := normalizeRuleInput(CreateRuleInput{
		RuleID:       ruleID,
		RuleName:     input.RuleName,
		RuleType:     input.RuleType,
		RulePattern:  input.RulePattern,
		Threshold:    input.Threshold,
		Action:       input.Action,
		RiskCategory: input.RiskCategory,
		Enabled:      input.Enabled,
		Priority:     input.Priority,
	})
	if err != nil {
		return nil, err
	}
	rule.UserID = &userID
	if _, err := buildRuleMatcher(*rule); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, rule); err != nil {
		return nil, err
	}
	_ = s.ReloadEngine(ctx)
	return rule, nil
}

// UpdateUserRule 更新用户的自定义规则（仅允许更新自己的规则）。
func (s *ModerationRuleService) UpdateUserRule(ctx context.Context, userID int64, ruleID string, input UpdateRuleInput) (*ModerationRule, error) {
	ruleID = strings.TrimSpace(ruleID)
	if ruleID == "" {
		return nil, fmt.Errorf("rule_id is required")
	}
	rule, err := s.repo.GetByRuleID(ctx, ruleID)
	if err != nil {
		return nil, err
	}
	if rule == nil {
		return nil, fmt.Errorf("moderation rule %q not found", ruleID)
	}
	if rule.UserID == nil || *rule.UserID != userID {
		return nil, fmt.Errorf("moderation rule %q not found or not owned by user", ruleID)
	}
	return s.UpdateRule(ctx, ruleID, input)
}

// DeleteUserRule 删除用户的自定义规则（仅允许删除自己的规则）。
func (s *ModerationRuleService) DeleteUserRule(ctx context.Context, userID int64, ruleID string) error {
	ruleID = strings.TrimSpace(ruleID)
	if ruleID == "" {
		return fmt.Errorf("rule_id is required")
	}
	rule, err := s.repo.GetByRuleID(ctx, ruleID)
	if err != nil {
		return err
	}
	if rule == nil {
		return fmt.Errorf("moderation rule %q not found", ruleID)
	}
	if rule.UserID == nil || *rule.UserID != userID {
		return fmt.Errorf("moderation rule %q not found or not owned by user", ruleID)
	}
	return s.DeleteRule(ctx, ruleID)
}

func shortUUID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func normalizeRuleInput(input CreateRuleInput) (*ModerationRule, error) {
	ruleID := strings.TrimSpace(input.RuleID)
	if ruleID == "" {
		return nil, fmt.Errorf("rule_id is required")
	}
	if strings.TrimSpace(input.RuleName) == "" {
		return nil, fmt.Errorf("rule_name is required")
	}
	ruleType := strings.ToUpper(strings.TrimSpace(input.RuleType))
	switch ruleType {
	case ModerationRuleTypeKeyword, ModerationRuleTypeRegex, ModerationRuleTypePattern:
	default:
		return nil, fmt.Errorf("unsupported rule_type %q", input.RuleType)
	}
	if strings.TrimSpace(input.RulePattern) == "" {
		return nil, fmt.Errorf("rule_pattern is required")
	}
	threshold := input.Threshold
	if threshold <= 0 {
		threshold = 0.8
	}
	priority := input.Priority
	if priority <= 0 {
		priority = 100
	}
	return &ModerationRule{
		RuleID:       ruleID,
		RuleName:     strings.TrimSpace(input.RuleName),
		RuleType:     ruleType,
		RulePattern:  input.RulePattern,
		Threshold:    threshold,
		Action:       normalizeRuleAction(input.Action),
		RiskCategory: strings.TrimSpace(input.RiskCategory),
		Enabled:      input.Enabled,
		Priority:     priority,
		CreatedBy:    strings.TrimSpace(input.CreatedBy),
	}, nil
}

func normalizeRuleAction(action string) string {
	switch strings.ToUpper(strings.TrimSpace(action)) {
	case ModerationRuleActionBlock:
		return ModerationRuleActionBlock
	case ModerationRuleActionReview:
		return ModerationRuleActionReview
	default:
		return ModerationRuleActionAllow
	}
}
