package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// complianceCredentialRepository implements service.ComplianceCredentialRepository.
type complianceCredentialRepository struct {
	db *sql.DB
}

// NewComplianceCredentialRepository creates a compliance credential repository.
func NewComplianceCredentialRepository(db *sql.DB) service.ComplianceCredentialRepository {
	return &complianceCredentialRepository{db: db}
}

func (r *complianceCredentialRepository) Create(ctx context.Context, cred *service.ComplianceCredential) error {
	var metadataJSON []byte
	if cred.Metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(cred.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
	}
	query := `INSERT INTO compliance_credentials (credential_id, credential_type, issuer, issuer_type, scope, status, valid_from, valid_until, evidence_hashes, digital_signature, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at`
	err := r.db.QueryRowContext(ctx, query,
		cred.CredentialID,
		cred.CredentialType,
		cred.Issuer,
		cred.IssuerType,
		nullableString(cred.Scope),
		cred.Status,
		cred.ValidFrom,
		cred.ValidUntil,
		nullableString(cred.EvidenceHashes),
		nullableString(cred.DigitalSignature),
		metadataJSON,
	).Scan(&cred.ID, &cred.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert compliance credential: %w", err)
	}
	return nil
}

func (r *complianceCredentialRepository) GetByID(ctx context.Context, id int64) (*service.ComplianceCredential, error) {
	query := `SELECT id, credential_id, credential_type, issuer, issuer_type, scope, status, valid_from, valid_until, evidence_hashes, digital_signature, metadata, created_at
		FROM compliance_credentials WHERE id = $1`
	return scanComplianceCredential(r.db.QueryRowContext(ctx, query, id))
}

func (r *complianceCredentialRepository) GetByCredentialID(ctx context.Context, credentialID string) (*service.ComplianceCredential, error) {
	query := `SELECT id, credential_id, credential_type, issuer, issuer_type, scope, status, valid_from, valid_until, evidence_hashes, digital_signature, metadata, created_at
		FROM compliance_credentials WHERE credential_id = $1`
	return scanComplianceCredential(r.db.QueryRowContext(ctx, query, credentialID))
}

func (r *complianceCredentialRepository) List(ctx context.Context, credentialType string, status string) ([]service.ComplianceCredential, error) {
	query := `SELECT id, credential_id, credential_type, issuer, issuer_type, scope, status, valid_from, valid_until, evidence_hashes, digital_signature, metadata, created_at
		FROM compliance_credentials WHERE 1=1`
	var args []interface{}
	argIndex := 1
	if credentialType != "" {
		query += fmt.Sprintf(" AND credential_type = $%d", argIndex)
		args = append(args, credentialType)
		argIndex++
	}
	if status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, status)
		argIndex++
	}
	query += " ORDER BY created_at DESC"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list credentials: %w", err)
	}
	defer rows.Close()
	var credentials []service.ComplianceCredential
	for rows.Next() {
		cred, err := scanComplianceCredential(rows)
		if err != nil {
			return nil, fmt.Errorf("scan credential: %w", err)
		}
		if cred != nil {
			credentials = append(credentials, *cred)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows err: %w", err)
	}
	return credentials, nil
}

func (r *complianceCredentialRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	result, err := r.db.ExecContext(ctx,
		"UPDATE compliance_credentials SET status = $1 WHERE id = $2",
		status, id,
	)
	if err != nil {
		return fmt.Errorf("update credential status: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("credential not found")
	}
	return nil
}

func (r *complianceCredentialRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx,
		"DELETE FROM compliance_credentials WHERE id = $1",
		id,
	)
	if err != nil {
		return fmt.Errorf("delete credential: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("credential not found")
	}
	return nil
}

func scanComplianceCredential(s interface{ Scan(...any) error }) (*service.ComplianceCredential, error) {
	var cred service.ComplianceCredential
	var scope sql.NullString
	var evidenceHashes sql.NullString
	var digitalSignature sql.NullString
	var metadataRaw []byte
	err := s.Scan(
		&cred.ID,
		&cred.CredentialID,
		&cred.CredentialType,
		&cred.Issuer,
		&cred.IssuerType,
		&scope,
		&cred.Status,
		&cred.ValidFrom,
		&cred.ValidUntil,
		&evidenceHashes,
		&digitalSignature,
		&metadataRaw,
		&cred.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan compliance credential: %w", err)
	}
	if scope.Valid {
		cred.Scope = scope.String
	}
	if evidenceHashes.Valid {
		cred.EvidenceHashes = evidenceHashes.String
	}
	if digitalSignature.Valid {
		cred.DigitalSignature = digitalSignature.String
	}
	if metadataRaw != nil {
		var metadata map[string]interface{}
		if err := json.Unmarshal(metadataRaw, &metadata); err == nil {
			cred.Metadata = metadata
		}
	}
	return &cred, nil
}


