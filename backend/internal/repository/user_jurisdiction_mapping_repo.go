package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type userJurisdictionMappingRepository struct {
	db *sql.DB
}

func NewUserJurisdictionMappingRepository(db *sql.DB) service.UserJurisdictionMappingRepository {
	return &userJurisdictionMappingRepository{db: db}
}

func (r *userJurisdictionMappingRepository) GetByUserID(ctx context.Context, userID int64) (*service.UserJurisdictionMapping, error) {
	query := `SELECT id, user_id, company_region, industry, service_type, risk_level,
		applicable_regulations, required_measures, recommended_actions, applied_rules, created_at, updated_at
		FROM user_jurisdiction_mappings WHERE user_id = $1`
	row := r.db.QueryRowContext(ctx, query, userID)
	return scanUserJurisdictionMapping(row)
}

func (r *userJurisdictionMappingRepository) Create(ctx context.Context, mapping *service.UserJurisdictionMapping) error {
	applicableRegulationsJSON, err := json.Marshal(mapping.ApplicableRegulations)
	if err != nil {
		return fmt.Errorf("marshal applicable_regulations: %w", err)
	}
	requiredMeasuresJSON, err := json.Marshal(mapping.RequiredMeasures)
	if err != nil {
		return fmt.Errorf("marshal required_measures: %w", err)
	}
	recommendedActionsJSON, err := json.Marshal(mapping.RecommendedActions)
	if err != nil {
		return fmt.Errorf("marshal recommended_actions: %w", err)
	}
	appliedRulesJSON, err := json.Marshal(mapping.AppliedRules)
	if err != nil {
		return fmt.Errorf("marshal applied_rules: %w", err)
	}
	query := `INSERT INTO user_jurisdiction_mappings (user_id, company_region, industry, service_type,
		risk_level, applicable_regulations, required_measures, recommended_actions, applied_rules)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at`
	var industry, serviceType interface{}
	if mapping.Industry != "" {
		industry = mapping.Industry
	}
	if mapping.ServiceType != "" {
		serviceType = mapping.ServiceType
	}
	err = r.db.QueryRowContext(ctx, query,
		mapping.UserID,
		mapping.CompanyRegion,
		industry,
		serviceType,
		mapping.RiskLevel,
		applicableRegulationsJSON,
		requiredMeasuresJSON,
		recommendedActionsJSON,
		appliedRulesJSON,
	).Scan(&mapping.ID, &mapping.CreatedAt, &mapping.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert user jurisdiction mapping: %w", err)
	}
	return nil
}

func (r *userJurisdictionMappingRepository) Update(ctx context.Context, mapping *service.UserJurisdictionMapping) error {
	applicableRegulationsJSON, err := json.Marshal(mapping.ApplicableRegulations)
	if err != nil {
		return fmt.Errorf("marshal applicable_regulations: %w", err)
	}
	requiredMeasuresJSON, err := json.Marshal(mapping.RequiredMeasures)
	if err != nil {
		return fmt.Errorf("marshal required_measures: %w", err)
	}
	recommendedActionsJSON, err := json.Marshal(mapping.RecommendedActions)
	if err != nil {
		return fmt.Errorf("marshal recommended_actions: %w", err)
	}
	appliedRulesJSON, err := json.Marshal(mapping.AppliedRules)
	if err != nil {
		return fmt.Errorf("marshal applied_rules: %w", err)
	}
	query := `UPDATE user_jurisdiction_mappings
		SET company_region = $1, industry = $2, service_type = $3, risk_level = $4,
			applicable_regulations = $5, required_measures = $6, recommended_actions = $7,
			applied_rules = $8, updated_at = NOW()
		WHERE user_id = $9`
	var industry, serviceType interface{}
	if mapping.Industry != "" {
		industry = mapping.Industry
	}
	if mapping.ServiceType != "" {
		serviceType = mapping.ServiceType
	}
	_, err = r.db.ExecContext(ctx, query,
		mapping.CompanyRegion,
		industry,
		serviceType,
		mapping.RiskLevel,
		applicableRegulationsJSON,
		requiredMeasuresJSON,
		recommendedActionsJSON,
		appliedRulesJSON,
		mapping.UserID,
	)
	if err != nil {
		return fmt.Errorf("update user jurisdiction mapping: %w", err)
	}
	return nil
}

func (r *userJurisdictionMappingRepository) Upsert(ctx context.Context, mapping *service.UserJurisdictionMapping) error {
	existing, err := r.GetByUserID(ctx, mapping.UserID)
	if err != nil {
		return fmt.Errorf("get existing mapping: %w", err)
	}
	if existing == nil {
		return r.Create(ctx, mapping)
	}
	mapping.ID = existing.ID
	return r.Update(ctx, mapping)
}

func (r *userJurisdictionMappingRepository) Delete(ctx context.Context, userID int64) error {
	query := `DELETE FROM user_jurisdiction_mappings WHERE user_id = $1`
	_, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("delete user jurisdiction mapping: %w", err)
	}
	return nil
}

func scanUserJurisdictionMapping(s interface{ Scan(...any) error }) (*service.UserJurisdictionMapping, error) {
	var mapping service.UserJurisdictionMapping
	var industry, serviceType sql.NullString
	var applicableRegulationsRaw, requiredMeasuresRaw, recommendedActionsRaw, appliedRulesRaw []byte
	if err := s.Scan(
		&mapping.ID,
		&mapping.UserID,
		&mapping.CompanyRegion,
		&industry,
		&serviceType,
		&mapping.RiskLevel,
		&applicableRegulationsRaw,
		&requiredMeasuresRaw,
		&recommendedActionsRaw,
		&appliedRulesRaw,
		&mapping.CreatedAt,
		&mapping.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan user jurisdiction mapping: %w", err)
	}
	if industry.Valid {
		mapping.Industry = industry.String
	}
	if serviceType.Valid {
		mapping.ServiceType = serviceType.String
	}
	if len(applicableRegulationsRaw) > 0 {
		_ = json.Unmarshal(applicableRegulationsRaw, &mapping.ApplicableRegulations)
	}
	if len(requiredMeasuresRaw) > 0 {
		_ = json.Unmarshal(requiredMeasuresRaw, &mapping.RequiredMeasures)
	}
	if len(recommendedActionsRaw) > 0 {
		_ = json.Unmarshal(recommendedActionsRaw, &mapping.RecommendedActions)
	}
	if len(appliedRulesRaw) > 0 {
		_ = json.Unmarshal(appliedRulesRaw, &mapping.AppliedRules)
	}
	return &mapping, nil
}