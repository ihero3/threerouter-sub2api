package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ============================================================================
// 内容审核自定义规则仓储（moderation_rules → moderationRuleRepository）
// 见 migrations/163 与 docs/合规方案.md 0.3 / 4.1.3。
// ============================================================================

type moderationRuleRepository struct {
	db *sql.DB
}

// NewModerationRuleRepository 创建审核规则仓储。
func NewModerationRuleRepository(db *sql.DB) service.ModerationRuleRepository {
	return &moderationRuleRepository{db: db}
}

func (r *moderationRuleRepository) List(ctx context.Context, enabledOnly bool) ([]service.ModerationRule, error) {
	query := `
SELECT id, rule_id, rule_name, rule_type, rule_pattern, threshold, action,
       risk_category, enabled, priority, created_by, user_id, created_at, updated_at
FROM moderation_rules WHERE user_id IS NULL`
	if enabledOnly {
		query += " AND enabled = TRUE"
	}
	query += " ORDER BY priority ASC, id ASC"

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list moderation rules: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.ModerationRule, 0)
	for rows.Next() {
		rule, err := scanModerationRule(rows)
		if err != nil {
			return nil, fmt.Errorf("scan moderation rule: %w", err)
		}
		items = append(items, *rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate moderation rules: %w", err)
	}
	return items, nil
}

func (r *moderationRuleRepository) GetByRuleID(ctx context.Context, ruleID string) (*service.ModerationRule, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, rule_id, rule_name, rule_type, rule_pattern, threshold, action,
       risk_category, enabled, priority, created_by, user_id, created_at, updated_at
FROM moderation_rules WHERE rule_id = $1`, ruleID)
	rule, err := scanModerationRule(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get moderation rule: %w", err)
	}
	return rule, nil
}

func (r *moderationRuleRepository) ListByUser(ctx context.Context, userID int64, enabledOnly bool) ([]service.ModerationRule, error) {
	query := `
SELECT id, rule_id, rule_name, rule_type, rule_pattern, threshold, action,
       risk_category, enabled, priority, created_by, user_id, created_at, updated_at
FROM moderation_rules WHERE user_id = $1`
	if enabledOnly {
		query += " AND enabled = TRUE"
	}
	query += " ORDER BY priority ASC, id ASC"

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list user moderation rules: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.ModerationRule, 0)
	for rows.Next() {
		rule, err := scanModerationRule(rows)
		if err != nil {
			return nil, fmt.Errorf("scan moderation rule: %w", err)
		}
		items = append(items, *rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate moderation rules: %w", err)
	}
	return items, nil
}

// ListByUserAll 列出所有用户的自定义规则（用于引擎加载）。
func (r *moderationRuleRepository) ListByUserAll(ctx context.Context, enabledOnly bool) ([]service.ModerationRule, error) {
	query := `
SELECT id, rule_id, rule_name, rule_type, rule_pattern, threshold, action,
       risk_category, enabled, priority, created_by, user_id, created_at, updated_at
FROM moderation_rules WHERE user_id IS NOT NULL`
	if enabledOnly {
		query += " AND enabled = TRUE"
	}
	query += " ORDER BY priority ASC, id ASC"

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list all user moderation rules: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.ModerationRule, 0)
	for rows.Next() {
		rule, err := scanModerationRule(rows)
		if err != nil {
			return nil, fmt.Errorf("scan moderation rule: %w", err)
		}
		items = append(items, *rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate moderation rules: %w", err)
	}
	return items, nil
}

func (r *moderationRuleRepository) Create(ctx context.Context, rule *service.ModerationRule) error {
	if rule == nil {
		return nil
	}
	err := r.db.QueryRowContext(ctx, `
INSERT INTO moderation_rules
    (rule_id, rule_name, rule_type, rule_pattern, threshold, action, risk_category, enabled, priority, created_by, user_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING id, created_at, updated_at`,
		rule.RuleID, rule.RuleName, rule.RuleType, rule.RulePattern, rule.Threshold,
		rule.Action, nullableString(rule.RiskCategory), rule.Enabled, rule.Priority, nullableString(rule.CreatedBy), nullableInt64(rule.UserID),
	).Scan(&rule.ID, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create moderation rule: %w", err)
	}
	return nil
}

func (r *moderationRuleRepository) Update(ctx context.Context, rule *service.ModerationRule) error {
	if rule == nil {
		return nil
	}
	err := r.db.QueryRowContext(ctx, `
UPDATE moderation_rules SET
    rule_name = $2,
    rule_type = $3,
    rule_pattern = $4,
    threshold = $5,
    action = $6,
    risk_category = $7,
    enabled = $8,
    priority = $9,
    updated_at = NOW()
WHERE rule_id = $1
RETURNING id, created_at, updated_at`,
		rule.RuleID, rule.RuleName, rule.RuleType, rule.RulePattern, rule.Threshold,
		rule.Action, nullableString(rule.RiskCategory), rule.Enabled, rule.Priority,
	).Scan(&rule.ID, &rule.CreatedAt, &rule.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("moderation rule %q not found", rule.RuleID)
	}
	if err != nil {
		return fmt.Errorf("update moderation rule: %w", err)
	}
	return nil
}

func (r *moderationRuleRepository) Delete(ctx context.Context, ruleID string) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM moderation_rules WHERE rule_id = $1", ruleID)
	if err != nil {
		return fmt.Errorf("delete moderation rule: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("moderation rule %q not found", ruleID)
	}
	return nil
}

func scanModerationRule(s interface{ Scan(...any) error }) (*service.ModerationRule, error) {
	var rule service.ModerationRule
	var rulePattern sql.NullString
	var riskCategory sql.NullString
	var createdBy sql.NullString
	var userID sql.NullInt64
	if err := s.Scan(
		&rule.ID, &rule.RuleID, &rule.RuleName, &rule.RuleType, &rulePattern, &rule.Threshold,
		&rule.Action, &riskCategory, &rule.Enabled, &rule.Priority, &createdBy, &userID, &rule.CreatedAt, &rule.UpdatedAt,
	); err != nil {
		return nil, err
	}
	rule.RulePattern = rulePattern.String
	rule.RiskCategory = riskCategory.String
	rule.CreatedBy = createdBy.String
	if userID.Valid {
		rule.UserID = &userID.Int64
	}
	return &rule, nil
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableInt64(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}
