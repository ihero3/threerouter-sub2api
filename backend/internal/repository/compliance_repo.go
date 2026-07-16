package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"time"
)

// ============================================================================
// AI 治理与合规模块仓储实现
//   - compliance_audit_logs  → complianceAuditLogRepository
//   - data_erasure_requests  → dataErasureRequestRepository
//   - user_consents          → userConsentRepository
// 见 migrations/159~161 与 docs/合规方案.md 4.1.4。
// ============================================================================

// ---------------------------------------------------------------------------
// ComplianceAuditLogRepository
// ---------------------------------------------------------------------------

type complianceAuditLogRepository struct {
	db *sql.DB
}

// NewComplianceAuditLogRepository 创建合规审计日志仓储。
func NewComplianceAuditLogRepository(db *sql.DB) service.ComplianceAuditLogRepository {
	return &complianceAuditLogRepository{db: db}
}

func (r *complianceAuditLogRepository) Create(ctx context.Context, log *service.ComplianceAuditLog) error {
	if log == nil {
		return nil
	}
	var subjectID any
	if log.SubjectID != nil {
		subjectID = *log.SubjectID
	}
	var evidenceHash any
	if strings.TrimSpace(log.EvidenceHash) != "" {
		evidenceHash = log.EvidenceHash
	}
	err := r.db.QueryRowContext(ctx, `
INSERT INTO compliance_audit_logs (
    compliance_type, subject_type, subject_id, details, operator, evidence_hash
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, created_at`,
		log.ComplianceType, log.SubjectType, subjectID, log.Details, log.Operator, evidenceHash,
	).Scan(&log.ID, &log.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert compliance audit log: %w", err)
	}
	return nil
}

func (r *complianceAuditLogRepository) List(ctx context.Context, filter service.ComplianceAuditLogFilter) ([]service.ComplianceAuditLog, *pagination.PaginationResult, error) {
	where := []string{"1=1"}
	args := []any{}
	idx := 1
	if strings.TrimSpace(filter.ComplianceType) != "" {
		where = append(where, fmt.Sprintf("compliance_type = $%d", idx))
		args = append(args, filter.ComplianceType)
		idx++
	}
	if strings.TrimSpace(filter.SubjectType) != "" {
		where = append(where, fmt.Sprintf("subject_type = $%d", idx))
		args = append(args, filter.SubjectType)
		idx++
	}
	if filter.SubjectID != nil {
		where = append(where, fmt.Sprintf("subject_id = $%d", idx))
		args = append(args, *filter.SubjectID)
		idx++
	}
	if filter.StartTime != nil {
		where = append(where, fmt.Sprintf("created_at >= $%d", idx))
		args = append(args, *filter.StartTime)
		idx++
	}
	if filter.EndTime != nil {
		where = append(where, fmt.Sprintf("created_at <= $%d", idx))
		args = append(args, *filter.EndTime)
		idx++
	}
	whereSQL := "WHERE " + strings.Join(where, " AND ")

	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM compliance_audit_logs "+whereSQL, args...).Scan(&total); err != nil {
		return nil, nil, fmt.Errorf("count compliance audit logs: %w", err)
	}

	params := normalizePagination(filter.Pagination)
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, params.Limit(), params.Offset())
	rows, err := r.db.QueryContext(ctx, `
SELECT id, compliance_type, subject_type, subject_id, details, operator, evidence_hash, created_at
FROM compliance_audit_logs `+whereSQL+`
ORDER BY created_at DESC, id DESC
LIMIT $`+fmt.Sprint(len(queryArgs)-1)+` OFFSET $`+fmt.Sprint(len(queryArgs)),
		queryArgs...,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("list compliance audit logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.ComplianceAuditLog, 0)
	for rows.Next() {
		var item service.ComplianceAuditLog
		var subjectID sql.NullInt64
		var evidenceHash sql.NullString
		if err := rows.Scan(
			&item.ID, &item.ComplianceType, &item.SubjectType, &subjectID,
			&item.Details, &item.Operator, &evidenceHash, &item.CreatedAt,
		); err != nil {
			return nil, nil, fmt.Errorf("scan compliance audit log: %w", err)
		}
		if subjectID.Valid {
			v := subjectID.Int64
			item.SubjectID = &v
		}
		if evidenceHash.Valid {
			item.EvidenceHash = evidenceHash.String
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate compliance audit logs: %w", err)
	}
	return items, paginationResultFromTotal(total, params), nil
}

// ---------------------------------------------------------------------------
// DataErasureRequestRepository
// ---------------------------------------------------------------------------

type dataErasureRequestRepository struct {
	db *sql.DB
}

// NewDataErasureRequestRepository 创建数据删除请求仓储。
func NewDataErasureRequestRepository(db *sql.DB) service.DataErasureRequestRepository {
	return &dataErasureRequestRepository{db: db}
}

func (r *dataErasureRequestRepository) Create(ctx context.Context, req *service.DataErasureRequest) error {
	if req == nil {
		return nil
	}
	err := r.db.QueryRowContext(ctx, `
INSERT INTO data_erasure_requests (
    user_id, request_type, status, scope_details
) VALUES ($1, $2, $3, $4)
RETURNING id, requested_at`,
		req.UserID, req.RequestType, req.Status, req.ScopeDetails,
	).Scan(&req.ID, &req.RequestedAt)
	if err != nil {
		return fmt.Errorf("insert data erasure request: %w", err)
	}
	return nil
}

func (r *dataErasureRequestRepository) GetByID(ctx context.Context, id int64) (*service.DataErasureRequest, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, user_id, request_type, status, scope_details, rejection_reason, operator,
       requested_at, processed_at, completed_at
FROM data_erasure_requests WHERE id = $1`, id)
	req, err := scanDataErasureRequest(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get data erasure request: %w", err)
	}
	return req, nil
}

func (r *dataErasureRequestRepository) UpdateStatus(ctx context.Context, id int64, status, operator string, rejectionReason *string, processedAt, completedAt *time.Time) error {
	var reason any
	if rejectionReason != nil {
		reason = *rejectionReason
	}
	res, err := r.db.ExecContext(ctx, `
UPDATE data_erasure_requests
SET status = $2, operator = $3, rejection_reason = $4, processed_at = $5, completed_at = $6
WHERE id = $1`,
		id, status, operator, reason, processedAt, completedAt,
	)
	if err != nil {
		return fmt.Errorf("update data erasure request status: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("data erasure request %d not found", id)
	}
	return nil
}

func (r *dataErasureRequestRepository) List(ctx context.Context, filter service.DataErasureRequestFilter) ([]service.DataErasureRequest, *pagination.PaginationResult, error) {
	where := []string{"1=1"}
	args := []any{}
	idx := 1
	if filter.UserID != nil {
		where = append(where, fmt.Sprintf("user_id = $%d", idx))
		args = append(args, *filter.UserID)
		idx++
	}
	if strings.TrimSpace(filter.Status) != "" {
		where = append(where, fmt.Sprintf("status = $%d", idx))
		args = append(args, filter.Status)
		idx++
	}
	whereSQL := "WHERE " + strings.Join(where, " AND ")

	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM data_erasure_requests "+whereSQL, args...).Scan(&total); err != nil {
		return nil, nil, fmt.Errorf("count data erasure requests: %w", err)
	}

	params := normalizePagination(filter.Pagination)
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, params.Limit(), params.Offset())
	rows, err := r.db.QueryContext(ctx, `
SELECT id, user_id, request_type, status, scope_details, rejection_reason, operator,
       requested_at, processed_at, completed_at
FROM data_erasure_requests `+whereSQL+`
ORDER BY requested_at DESC, id DESC
LIMIT $`+fmt.Sprint(len(queryArgs)-1)+` OFFSET $`+fmt.Sprint(len(queryArgs)),
		queryArgs...,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("list data erasure requests: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.DataErasureRequest, 0)
	for rows.Next() {
		req, err := scanDataErasureRequest(rows)
		if err != nil {
			return nil, nil, fmt.Errorf("scan data erasure request: %w", err)
		}
		items = append(items, *req)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate data erasure requests: %w", err)
	}
	return items, paginationResultFromTotal(total, params), nil
}

// scanDataErasureRequest 从行扫描器读取一条删除请求。
func scanDataErasureRequest(s interface{ Scan(...any) error }) (*service.DataErasureRequest, error) {
	var req service.DataErasureRequest
	var rejectionReason, operator sql.NullString
	var processedAt, completedAt sql.NullTime
	if err := s.Scan(
		&req.ID, &req.UserID, &req.RequestType, &req.Status, &req.ScopeDetails,
		&rejectionReason, &operator, &req.RequestedAt, &processedAt, &completedAt,
	); err != nil {
		return nil, err
	}
	if rejectionReason.Valid {
		req.RejectionReason = &rejectionReason.String
	}
	if operator.Valid {
		req.Operator = &operator.String
	}
	if processedAt.Valid {
		req.ProcessedAt = &processedAt.Time
	}
	if completedAt.Valid {
		req.CompletedAt = &completedAt.Time
	}
	return &req, nil
}

// ---------------------------------------------------------------------------
// UserConsentRepository
// ---------------------------------------------------------------------------

type userConsentRepository struct {
	db *sql.DB
}

// NewUserConsentRepository 创建用户同意仓储。
func NewUserConsentRepository(db *sql.DB) service.UserConsentRepository {
	return &userConsentRepository{db: db}
}

func (r *userConsentRepository) Upsert(ctx context.Context, consent *service.UserConsent) error {
	if consent == nil {
		return nil
	}
	var source any
	if consent.Source != nil {
		source = *consent.Source
	}
	err := r.db.QueryRowContext(ctx, `
INSERT INTO user_consents (user_id, consent_type, granted, granted_at, revoked_at, source, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, NOW())
ON CONFLICT (user_id, consent_type) DO UPDATE SET
    granted = EXCLUDED.granted,
    granted_at = EXCLUDED.granted_at,
    revoked_at = EXCLUDED.revoked_at,
    source = EXCLUDED.source,
    updated_at = NOW()
RETURNING id, created_at, updated_at`,
		consent.UserID, consent.ConsentType, consent.Granted, consent.GrantedAt, consent.RevokedAt, source,
	).Scan(&consent.ID, &consent.CreatedAt, &consent.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert user consent: %w", err)
	}
	return nil
}

func (r *userConsentRepository) Get(ctx context.Context, userID int64, consentType string) (*service.UserConsent, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, user_id, consent_type, granted, granted_at, revoked_at, source, created_at, updated_at
FROM user_consents WHERE user_id = $1 AND consent_type = $2`, userID, consentType)
	consent, err := scanUserConsent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user consent: %w", err)
	}
	return consent, nil
}

func (r *userConsentRepository) ListByUser(ctx context.Context, userID int64) ([]service.UserConsent, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, user_id, consent_type, granted, granted_at, revoked_at, source, created_at, updated_at
FROM user_consents WHERE user_id = $1
ORDER BY consent_type`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user consents: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.UserConsent, 0)
	for rows.Next() {
		consent, err := scanUserConsent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan user consent: %w", err)
		}
		items = append(items, *consent)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user consents: %w", err)
	}
	return items, nil
}

func (r *userConsentRepository) DeleteByUserID(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx, `
DELETE FROM user_consents WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("delete user consents: %w", err)
	}
	return nil
}

// scanUserConsent 从行扫描器读取一条同意记录。
func scanUserConsent(s interface{ Scan(...any) error }) (*service.UserConsent, error) {
	var consent service.UserConsent
	var grantedAt, revokedAt sql.NullTime
	var source sql.NullString
	if err := s.Scan(
		&consent.ID, &consent.UserID, &consent.ConsentType, &consent.Granted,
		&grantedAt, &revokedAt, &source, &consent.CreatedAt, &consent.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if grantedAt.Valid {
		consent.GrantedAt = &grantedAt.Time
	}
	if revokedAt.Valid {
		consent.RevokedAt = &revokedAt.Time
	}
	if source.Valid {
		consent.Source = &source.String
	}
	return &consent, nil
}

// normalizePagination 归一化分页参数（页码/页大小的默认值与上限）。
func normalizePagination(params pagination.PaginationParams) pagination.PaginationParams {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}
	return params
}
