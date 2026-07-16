package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// userDataExporter implements ComplianceUserDataExporter for GDPR Art 20 data portability.
type userDataExporter struct {
	userRepo    UserRepository
	apiKeyRepo  APIKeyRepository
	consentRepo UserConsentRepository
}

// NewComplianceUserDataExporter creates a user data exporter for GDPR Art 20.
func NewComplianceUserDataExporter(
	userRepo UserRepository,
	apiKeyRepo APIKeyRepository,
	consentRepo UserConsentRepository,
) ComplianceUserDataExporter {
	return &userDataExporter{
		userRepo:    userRepo,
		apiKeyRepo:  apiKeyRepo,
		consentRepo: consentRepo,
	}
}

func (e *userDataExporter) ExportUserData(ctx context.Context, userID int64) (map[string]any, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user_id")
	}

	result := map[string]any{
		"user_id": userID,
	}

	if e.userRepo != nil {
		user, err := e.userRepo.GetByID(ctx, userID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("get user: %w", err)
		}
		if user != nil {
			result["user"] = map[string]any{
				"id":         user.ID,
				"email":      user.Email,
				"username":   user.Username,
				"created_at": user.CreatedAt,
				"updated_at": user.UpdatedAt,
				"status":     user.Status,
				"role":       user.Role,
			}
		}
	}

	if e.apiKeyRepo != nil {
		apiKeys, _, err := e.apiKeyRepo.ListByUserID(ctx, userID, pagination.PaginationParams{PageSize: 100}, APIKeyListFilters{})
		if err != nil {
			return nil, fmt.Errorf("list api keys: %w", err)
		}
		if len(apiKeys) > 0 {
			var keyData []map[string]any
			for _, key := range apiKeys {
				keyData = append(keyData, map[string]any{
					"id":         key.ID,
					"name":       key.Name,
					"status":     key.Status,
					"created_at": key.CreatedAt,
					"expires_at": key.ExpiresAt,
					"quota":      key.Quota,
					"quota_used": key.QuotaUsed,
				})
			}
			result["api_keys"] = keyData
		}
	}

	if e.consentRepo != nil {
		consents, err := e.consentRepo.ListByUser(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("list consents: %w", err)
		}
		if len(consents) > 0 {
			var consentData []map[string]any
			for _, c := range consents {
				consentData = append(consentData, map[string]any{
					"consent_type": c.ConsentType,
					"granted":      c.Granted,
					"granted_at":   c.GrantedAt,
					"revoked_at":   c.RevokedAt,
					"source":       c.Source,
					"updated_at":   c.UpdatedAt,
				})
			}
			result["consents"] = consentData
		}
	}

	return result, nil
}
