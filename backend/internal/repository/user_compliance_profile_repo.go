package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ============================================================================
// 用户级合规档案仓储（user_compliance_profiles）
// 见 docs/合规升级方案.md 4.1 节，每个 User Account 拥有独立的合规配置。
// ============================================================================

type userComplianceProfileRepository struct {
	db *sql.DB
}

// NewUserComplianceProfileRepository 创建用户合规档案仓储。
func NewUserComplianceProfileRepository(db *sql.DB) service.UserComplianceProfileRepository {
	return &userComplianceProfileRepository{db: db}
}

func (r *userComplianceProfileRepository) GetByUserID(ctx context.Context, userID int64) (*service.UserComplianceProfile, error) {
	query := `SELECT id, user_id, active_template_code, zdr_mode, detail_retention_days, compliance_frameworks, moderation_policy, created_at, updated_at
		FROM user_compliance_profiles WHERE user_id = $1`
	row := r.db.QueryRowContext(ctx, query, userID)
	return scanUserComplianceProfile(row)
}

func (r *userComplianceProfileRepository) Create(ctx context.Context, profile *service.UserComplianceProfile) error {
	frameworksJSON, err := json.Marshal(profile.ComplianceFrameworks)
	if err != nil {
		return fmt.Errorf("marshal compliance_frameworks: %w", err)
	}
	policyJSON, err := json.Marshal(profile.ModerationPolicy)
	if err != nil {
		return fmt.Errorf("marshal moderation_policy: %w", err)
	}
	query := `INSERT INTO user_compliance_profiles (user_id, active_template_code, zdr_mode, detail_retention_days, compliance_frameworks, moderation_policy)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`
	var activeTemplateCode interface{}
	if profile.ActiveTemplateCode != nil {
		activeTemplateCode = *profile.ActiveTemplateCode
	}
	err = r.db.QueryRowContext(ctx, query,
		profile.UserID,
		activeTemplateCode,
		profile.ZDRMode,
		profile.DetailRetentionDays,
		frameworksJSON,
		policyJSON,
	).Scan(&profile.ID, &profile.CreatedAt, &profile.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert user compliance profile: %w", err)
	}
	return nil
}

func (r *userComplianceProfileRepository) Update(ctx context.Context, profile *service.UserComplianceProfile) error {
	frameworksJSON, err := json.Marshal(profile.ComplianceFrameworks)
	if err != nil {
		return fmt.Errorf("marshal compliance_frameworks: %w", err)
	}
	policyJSON, err := json.Marshal(profile.ModerationPolicy)
	if err != nil {
		return fmt.Errorf("marshal moderation_policy: %w", err)
	}
	query := `UPDATE user_compliance_profiles
		SET active_template_code = $1, zdr_mode = $2, detail_retention_days = $3, compliance_frameworks = $4, moderation_policy = $5, updated_at = NOW()
		WHERE user_id = $6`
	var activeTemplateCode interface{}
	if profile.ActiveTemplateCode != nil {
		activeTemplateCode = *profile.ActiveTemplateCode
	}
	_, err = r.db.ExecContext(ctx, query,
		activeTemplateCode,
		profile.ZDRMode,
		profile.DetailRetentionDays,
		frameworksJSON,
		policyJSON,
		profile.UserID,
	)
	if err != nil {
		return fmt.Errorf("update user compliance profile: %w", err)
	}
	return nil
}

func (r *userComplianceProfileRepository) Upsert(ctx context.Context, profile *service.UserComplianceProfile) error {
	existing, err := r.GetByUserID(ctx, profile.UserID)
	if err != nil {
		return fmt.Errorf("get existing profile: %w", err)
	}
	if existing == nil {
		return r.Create(ctx, profile)
	}
	profile.ID = existing.ID
	return r.Update(ctx, profile)
}

func scanUserComplianceProfile(s interface{ Scan(...any) error }) (*service.UserComplianceProfile, error) {
	var profile service.UserComplianceProfile
	var activeTemplateCode sql.NullString
	var frameworksRaw []byte
	var policyRaw []byte
	if err := s.Scan(
		&profile.ID,
		&profile.UserID,
		&activeTemplateCode,
		&profile.ZDRMode,
		&profile.DetailRetentionDays,
		&frameworksRaw,
		&policyRaw,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan user compliance profile: %w", err)
	}
	if activeTemplateCode.Valid {
		profile.ActiveTemplateCode = &activeTemplateCode.String
	}
	if len(frameworksRaw) > 0 {
		_ = json.Unmarshal(frameworksRaw, &profile.ComplianceFrameworks)
	}
	if len(policyRaw) > 0 {
		_ = json.Unmarshal(policyRaw, &profile.ModerationPolicy)
	}
	return &profile, nil
}
