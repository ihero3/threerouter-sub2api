package service

import (
	"context"
	"fmt"
	"time"
)

// ComplianceCredentialRepository defines persistence for compliance credentials.
type ComplianceCredentialRepository interface {
	Create(ctx context.Context, cred *ComplianceCredential) error
	GetByID(ctx context.Context, id int64) (*ComplianceCredential, error)
	GetByCredentialID(ctx context.Context, credentialID string) (*ComplianceCredential, error)
	List(ctx context.Context, credentialType string, status string) ([]ComplianceCredential, error)
	UpdateStatus(ctx context.Context, id int64, status string) error
	Delete(ctx context.Context, id int64) error
}

// ComplianceCredential represents a compliance credential.
type ComplianceCredential struct {
	ID               int64                  `json:"id"`
	CredentialID     string                 `json:"credential_id"`
	CredentialType   string                 `json:"credential_type"`
	Issuer           string                 `json:"issuer"`
	IssuerType       string                 `json:"issuer_type"`
	Scope            string                 `json:"scope,omitempty"`
	Status           string                 `json:"status"`
	ValidFrom        time.Time              `json:"valid_from"`
	ValidUntil       time.Time              `json:"valid_until"`
	EvidenceHashes   string                 `json:"evidence_hashes,omitempty"`
	DigitalSignature string                 `json:"digital_signature,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
}

// ComplianceCredentialService manages compliance credentials (evidence packages).
type ComplianceCredentialService struct {
	repo         ComplianceCredentialRepository
	complianceSvc *ComplianceService
}

// NewComplianceCredentialService creates a compliance credential service.
func NewComplianceCredentialService(repo ComplianceCredentialRepository, complianceSvc *ComplianceService) *ComplianceCredentialService {
	return &ComplianceCredentialService{repo: repo, complianceSvc: complianceSvc}
}

// ListCredentials lists compliance credentials with optional filters.
func (s *ComplianceCredentialService) ListCredentials(ctx context.Context, credentialType string, status string) ([]ComplianceCredential, error) {
	return s.repo.List(ctx, credentialType, status)
}

// GetCredential gets a credential by ID.
func (s *ComplianceCredentialService) GetCredential(ctx context.Context, id int64) (*ComplianceCredential, error) {
	return s.repo.GetByID(ctx, id)
}

// CreateCredential creates a new compliance credential.
func (s *ComplianceCredentialService) CreateCredential(ctx context.Context, input CreateCredentialInput) (*ComplianceCredential, error) {
	now := time.Now().UTC()
	cred := &ComplianceCredential{
		CredentialID:     input.CredentialID,
		CredentialType:   input.CredentialType,
		Issuer:           input.Issuer,
		IssuerType:       input.IssuerType,
		Scope:            input.Scope,
		Status:           "active",
		ValidFrom:        input.ValidFrom,
		ValidUntil:       input.ValidUntil,
		EvidenceHashes:   input.EvidenceHashes,
		DigitalSignature: input.DigitalSignature,
		Metadata:         input.Metadata,
	}
	if cred.IssuerType == "" {
		cred.IssuerType = "SELF_ASSERTION"
	}
	if cred.ValidFrom.IsZero() {
		cred.ValidFrom = now
	}
	if err := s.repo.Create(ctx, cred); err != nil {
		return nil, err
	}

	_ = s.complianceSvc.LogComplianceEvent(ctx, ComplianceEvent{
		ComplianceType: "CREDENTIAL_CREATED",
		SubjectType:    ComplianceSubjectSystem,
		Details:        fmt.Sprintf("credential_type=%s credential_id=%s", cred.CredentialType, cred.CredentialID),
		Operator:       "system",
	})
	return cred, nil
}

// CreateCredentialInput is used to create a compliance credential.
type CreateCredentialInput struct {
	CredentialID     string                 `json:"credential_id"`
	CredentialType   string                 `json:"credential_type"`
	Issuer           string                 `json:"issuer"`
	IssuerType       string                 `json:"issuer_type"`
	Scope            string                 `json:"scope"`
	ValidFrom        time.Time              `json:"valid_from"`
	ValidUntil       time.Time              `json:"valid_until"`
	EvidenceHashes   string                 `json:"evidence_hashes"`
	DigitalSignature string                 `json:"digital_signature"`
	Metadata         map[string]interface{} `json:"metadata"`
}

// RevokeCredential revokes a credential (sets status to revoked).
func (s *ComplianceCredentialService) RevokeCredential(ctx context.Context, id int64) error {
	if err := s.repo.UpdateStatus(ctx, id, "revoked"); err != nil {
		return err
	}

	_ = s.complianceSvc.LogComplianceEvent(ctx, ComplianceEvent{
		ComplianceType: "CREDENTIAL_REVOKED",
		SubjectType:    ComplianceSubjectSystem,
		Details:        fmt.Sprintf("credential_id=%d", id),
		Operator:       "system",
	})
	return nil
}

// UpdateCredentialStatus updates the status of a credential.
func (s *ComplianceCredentialService) UpdateCredentialStatus(ctx context.Context, id int64, status string) error {
	if err := s.repo.UpdateStatus(ctx, id, status); err != nil {
		return err
	}

	_ = s.complianceSvc.LogComplianceEvent(ctx, ComplianceEvent{
		ComplianceType: "CREDENTIAL_STATUS_UPDATED",
		SubjectType:    ComplianceSubjectSystem,
		Details:        fmt.Sprintf("credential_id=%d status=%s", id, status),
		Operator:       "system",
	})
	return nil
}

// DeleteCredential deletes a credential.
func (s *ComplianceCredentialService) DeleteCredential(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}


