package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ============================================================================
// 行业合规策略模板仓储（compliance_policy_templates → policyTemplateRepository）
// 见 migrations/164 与 docs/合规方案.md 4.4.2。
// ============================================================================

type policyTemplateRepository struct {
	db *sql.DB
}

// NewCompliancePolicyTemplateRepository 创建行业合规模板仓储。
func NewCompliancePolicyTemplateRepository(db *sql.DB) service.CompliancePolicyTemplateRepository {
	return &policyTemplateRepository{db: db}
}

func (r *policyTemplateRepository) Count(ctx context.Context) (int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM compliance_policy_templates").Scan(&total); err != nil {
		return 0, fmt.Errorf("count policy templates: %w", err)
	}
	return total, nil
}

func (r *policyTemplateRepository) List(ctx context.Context, industry string) ([]service.CompliancePolicyTemplate, error) {
	query := `
SELECT id, template_code, industry, description, rules, risk_tags, created_at
FROM compliance_policy_templates`
	args := []any{}
	if strings.TrimSpace(industry) != "" {
		query += " WHERE industry = $1"
		args = append(args, industry)
	}
	query += " ORDER BY template_code"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list policy templates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.CompliancePolicyTemplate, 0)
	for rows.Next() {
		tpl, err := scanPolicyTemplate(rows)
		if err != nil {
			return nil, fmt.Errorf("scan policy template: %w", err)
		}
		items = append(items, *tpl)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate policy templates: %w", err)
	}
	return items, nil
}

func (r *policyTemplateRepository) GetByCode(ctx context.Context, templateCode string) (*service.CompliancePolicyTemplate, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, template_code, industry, description, rules, risk_tags, created_at
FROM compliance_policy_templates WHERE template_code = $1`, templateCode)
	tpl, err := scanPolicyTemplate(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get policy template: %w", err)
	}
	return tpl, nil
}

func (r *policyTemplateRepository) Upsert(ctx context.Context, tpl *service.CompliancePolicyTemplate) error {
	if tpl == nil {
		return nil
	}
	rulesJSON, err := json.Marshal(tpl.Rules)
	if err != nil {
		return fmt.Errorf("marshal policy template rules: %w", err)
	}
	riskTags := strings.Join(tpl.RiskTags, ",")
	err = r.db.QueryRowContext(ctx, `
INSERT INTO compliance_policy_templates (template_code, industry, description, rules, risk_tags)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (template_code) DO UPDATE SET
    industry = EXCLUDED.industry,
    description = EXCLUDED.description,
    rules = EXCLUDED.rules,
    risk_tags = EXCLUDED.risk_tags
RETURNING id, created_at`,
		tpl.TemplateCode, tpl.Industry, tpl.Description, rulesJSON, riskTags,
	).Scan(&tpl.ID, &tpl.CreatedAt)
	if err != nil {
		return fmt.Errorf("upsert policy template: %w", err)
	}
	return nil
}

// scanPolicyTemplate 从行扫描器读取一条模板，解析 JSONB rules 与逗号分隔的 risk_tags。
func scanPolicyTemplate(s interface{ Scan(...any) error }) (*service.CompliancePolicyTemplate, error) {
	var tpl service.CompliancePolicyTemplate
	var description sql.NullString
	var rulesRaw []byte
	var riskTags sql.NullString
	if err := s.Scan(
		&tpl.ID, &tpl.TemplateCode, &tpl.Industry, &description, &rulesRaw, &riskTags, &tpl.CreatedAt,
	); err != nil {
		return nil, err
	}
	if description.Valid {
		tpl.Description = description.String
	}
	tpl.Rules = []map[string]any{}
	if len(rulesRaw) > 0 {
		if err := json.Unmarshal(rulesRaw, &tpl.Rules); err != nil {
			return nil, fmt.Errorf("unmarshal policy template rules: %w", err)
		}
	}
	tpl.RiskTags = []string{}
	if riskTags.Valid && strings.TrimSpace(riskTags.String) != "" {
		for _, tag := range strings.Split(riskTags.String, ",") {
			if t := strings.TrimSpace(tag); t != "" {
				tpl.RiskTags = append(tpl.RiskTags, t)
			}
		}
	}
	return &tpl, nil
}
