package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/alrazihi/civora/internal/evidence/domain"
	"github.com/google/uuid"
)

type PostgresEvidenceRepository struct {
	db *sql.DB
}

type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func NewPostgresEvidenceRepository(db *sql.DB) *PostgresEvidenceRepository {
	return &PostgresEvidenceRepository{db: db}
}

func (r *PostgresEvidenceRepository) DB() *sql.DB {
	return r.db
}

func (r *PostgresEvidenceRepository) Save(ctx context.Context, e *domain.Evidence) error {
	return r.saveEvidence(ctx, r.db, e)
}

func (r *PostgresEvidenceRepository) SaveTx(ctx context.Context, tx *sql.Tx, e *domain.Evidence) error {
	return r.saveEvidence(ctx, tx, e)
}

func (r *PostgresEvidenceRepository) saveEvidence(ctx context.Context, ex sqlExecer, e *domain.Evidence) error {
	metadataJSON, err := json.Marshal(e.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if metadataJSON == nil {
		metadataJSON = []byte("{}")
	}

	var customTypeVal any
	if e.CustomType != nil {
		customTypeVal = *e.CustomType
	} else {
		customTypeVal = nil
	}

	query := `
		INSERT INTO evidence (
			id, organization_id, service_request_id, type, custom_type,
			description, storage_reference, uploaded_by, person_id,
			evidence_source, metadata, verification_status,
			verified_by, verified_at, verification_reason, verification_method,
			created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`
	_, err = ex.ExecContext(ctx, query,
		e.ID, e.OrganizationID, e.ServiceRequestID, e.Type, customTypeVal,
		e.Description, e.StorageReference, e.UploadedBy, e.PersonID,
		e.Source, metadataJSON, e.VerificationStatus,
		e.VerifiedBy, e.VerifiedAt, e.VerificationReason, e.VerificationMethod,
		e.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert evidence: %w", err)
	}
	return nil
}

func (r *PostgresEvidenceRepository) UpdateVerification(ctx context.Context, e *domain.Evidence) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	return r.UpdateVerificationTx(ctx, tx, e)
}

func (r *PostgresEvidenceRepository) UpdateVerificationTx(ctx context.Context, tx *sql.Tx, e *domain.Evidence) error {
	// Lock the row and fetch current evidence
	var currentEvidence domain.Evidence
	var customType sql.NullString
	var metadataJSON []byte
	var verificationReason sql.NullString
	var verificationMethod sql.NullString
	err := tx.QueryRowContext(ctx, `
		SELECT id, organization_id, service_request_id, type, custom_type,
		       description, storage_reference, uploaded_by, person_id,
		       evidence_source, metadata, verification_status,
		       verified_by, verified_at, verification_reason, verification_method,
		       created_at
		FROM evidence
		WHERE organization_id = $1 AND id = $2
		FOR UPDATE
	`, e.OrganizationID, e.ID).Scan(
		&currentEvidence.ID, &currentEvidence.OrganizationID, &currentEvidence.ServiceRequestID, &currentEvidence.Type, &customType,
		&currentEvidence.Description, &currentEvidence.StorageReference, &currentEvidence.UploadedBy, &currentEvidence.PersonID,
		&currentEvidence.Source, &metadataJSON, &currentEvidence.VerificationStatus,
		&currentEvidence.VerifiedBy, &currentEvidence.VerifiedAt, &verificationReason, &verificationMethod,
		&currentEvidence.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrEvidenceNotFound
		}
		return fmt.Errorf("failed to lock and fetch evidence: %w", err)
	}

	// Convert null types
	currentEvidence.CustomType = nullableStringPtr(customType)
	currentEvidence.VerificationReason = nullString(verificationReason)
	currentEvidence.VerificationMethod = nullString(verificationMethod)
	if len(metadataJSON) > 0 {
		_ = json.Unmarshal(metadataJSON, &currentEvidence.Metadata)
	}

	// Validate the transition
	if !domain.IsValidVerificationTransition(currentEvidence.VerificationStatus, e.VerificationStatus) {
		return fmt.Errorf("invalid verification transition from %s to %s: %w", currentEvidence.VerificationStatus, e.VerificationStatus, domain.ErrVerificationInvalid)
	}

	// Now, update the evidence with the new verification fields
	updateQuery := `
		UPDATE evidence SET
			verification_status = $1,
			verified_by = $2,
			verified_at = $3,
			verification_reason = $4,
			verification_method = $5
		WHERE organization_id = $6 AND id = $7
	`
	result, err := tx.ExecContext(ctx, updateQuery,
		e.VerificationStatus, e.VerifiedBy, e.VerifiedAt,
		e.VerificationReason, e.VerificationMethod,
		e.OrganizationID, e.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update evidence verification: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return domain.ErrEvidenceNotFound
	}

	// Create verification history record
	var verifiedAt time.Time
	if e.VerifiedAt != nil {
		verifiedAt = *e.VerifiedAt
	}
	rec := &domain.VerificationRecord{
		ID:             uuid.New(),
		EvidenceID:     e.ID,
		OrganizationID: e.OrganizationID,
		Status:         e.VerificationStatus,
		VerifierID:     e.VerifiedBy,
		VerifiedAt:     verifiedAt,
		Reason:         e.VerificationReason,
		Method:         e.VerificationMethod,
		Notes:          "",
		CreatedAt:      time.Now().UTC(),
	}

	// Save the verification history
	err = r.saveVerificationHistory(ctx, tx, rec)
	if err != nil {
		return fmt.Errorf("failed to save verification history: %w", err)
	}

	return nil
}

func (r *PostgresEvidenceRepository) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Evidence, error) {
	query := `
		SELECT id, organization_id, service_request_id, type, custom_type,
			   description, storage_reference, uploaded_by, person_id,
			   evidence_source, metadata, verification_status,
			   verified_by, verified_at, verification_reason, verification_method,
			   created_at
		FROM evidence
		WHERE organization_id = $1 AND id = $2
	`
	return r.scanEvidence(r.db.QueryRowContext(ctx, query, orgID, id))
}

func (r *PostgresEvidenceRepository) FindByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID, limit, offset int) ([]*domain.Evidence, error) {
	query := `
		SELECT id, organization_id, service_request_id, type, custom_type,
			   description, storage_reference, uploaded_by, person_id,
			   evidence_source, metadata, verification_status,
			   verified_by, verified_at, verification_reason, verification_method,
			   created_at
		FROM evidence
		WHERE organization_id = $1 AND service_request_id = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, serviceRequestID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query evidence: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var items []*domain.Evidence
	for rows.Next() {
		e, err := r.scanEvidenceFromRows(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return items, nil
}

func (r *PostgresEvidenceRepository) CountByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM evidence WHERE organization_id = $1 AND service_request_id = $2`
	var total int
	err := r.db.QueryRowContext(ctx, query, orgID, serviceRequestID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to count evidence: %w", err)
	}
	return total, nil
}

func (r *PostgresEvidenceRepository) FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Evidence, error) {
	query := `
		SELECT id, organization_id, service_request_id, type, custom_type,
			   description, storage_reference, uploaded_by, person_id,
			   evidence_source, metadata, verification_status,
			   verified_by, verified_at, verification_reason, verification_method,
			   created_at
		FROM evidence
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query evidence: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var items []*domain.Evidence
	for rows.Next() {
		e, err := r.scanEvidenceFromRows(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return items, nil
}

func (r *PostgresEvidenceRepository) scanEvidence(row interface {
	Scan(dest ...any) error
}) (*domain.Evidence, error) {
	var e domain.Evidence
	var customType sql.NullString
	var metadataJSON []byte
	var verificationReason sql.NullString
	var verificationMethod sql.NullString

	if err := row.Scan(
		&e.ID, &e.OrganizationID, &e.ServiceRequestID, &e.Type, &customType,
		&e.Description, &e.StorageReference, &e.UploadedBy, &e.PersonID,
		&e.Source, &metadataJSON, &e.VerificationStatus,
		&e.VerifiedBy, &e.VerifiedAt, &verificationReason, &verificationMethod,
		&e.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrEvidenceNotFound
		}
		return nil, fmt.Errorf("failed to scan evidence: %w", err)
	}
	e.CustomType = nullableStringPtr(customType)
	e.VerificationReason = nullString(verificationReason)
	e.VerificationMethod = nullString(verificationMethod)
	if len(metadataJSON) > 0 {
		_ = json.Unmarshal(metadataJSON, &e.Metadata)
	}
	return &e, nil
}

func (r *PostgresEvidenceRepository) scanEvidenceFromRows(rows *sql.Rows) (*domain.Evidence, error) {
	var e domain.Evidence
	var customType sql.NullString
	var metadataJSON []byte
	var verificationReason sql.NullString
	var verificationMethod sql.NullString

	if err := rows.Scan(
		&e.ID, &e.OrganizationID, &e.ServiceRequestID, &e.Type, &customType,
		&e.Description, &e.StorageReference, &e.UploadedBy, &e.PersonID,
		&e.Source, &metadataJSON, &e.VerificationStatus,
		&e.VerifiedBy, &e.VerifiedAt, &verificationReason, &verificationMethod,
		&e.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan evidence: %w", err)
	}
	e.CustomType = nullableStringPtr(customType)
	e.VerificationReason = nullString(verificationReason)
	e.VerificationMethod = nullString(verificationMethod)
	if len(metadataJSON) > 0 {
		_ = json.Unmarshal(metadataJSON, &e.Metadata)
	}
	return &e, nil
}

func nullString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func nullableStringPtr(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
}

func (r *PostgresEvidenceRepository) SaveVerificationHistory(ctx context.Context, rec *domain.VerificationRecord) error {
	return r.saveVerificationHistory(ctx, r.db, rec)
}

func (r *PostgresEvidenceRepository) SaveVerificationHistoryTx(ctx context.Context, tx *sql.Tx, rec *domain.VerificationRecord) error {
	return r.saveVerificationHistory(ctx, tx, rec)
}

func (r *PostgresEvidenceRepository) saveVerificationHistory(ctx context.Context, ex sqlExecer, rec *domain.VerificationRecord) error {
	query := `
		INSERT INTO evidence_verification_history (
			id, organization_id, evidence_id, verification_status,
			verifier_id, verified_at, reason, method, notes, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := ex.ExecContext(ctx, query,
		rec.ID, rec.OrganizationID, rec.EvidenceID, rec.Status,
		rec.VerifierID, rec.VerifiedAt, rec.Reason, rec.Method, rec.Notes, rec.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert verification history: %w", err)
	}
	return nil
}

func (r *PostgresEvidenceRepository) FindVerificationHistory(ctx context.Context, orgID, evidenceID uuid.UUID, limit, offset int) ([]*domain.VerificationRecord, error) {
	query := `
		SELECT id, organization_id, evidence_id, verification_status,
			   verifier_id, verified_at, reason, method, notes, created_at
		FROM evidence_verification_history
		WHERE organization_id = $1 AND evidence_id = $2
		ORDER BY verified_at ASC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, evidenceID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query verification history: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var items []*domain.VerificationRecord
	for rows.Next() {
		rec, err := r.scanVerificationRecord(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return items, nil
}

func (r *PostgresEvidenceRepository) CountVerificationHistory(ctx context.Context, orgID, evidenceID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM evidence_verification_history WHERE organization_id = $1 AND evidence_id = $2`
	var total int
	err := r.db.QueryRowContext(ctx, query, orgID, evidenceID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to count verification history: %w", err)
	}
	return total, nil
}

func (r *PostgresEvidenceRepository) scanVerificationRecord(rows *sql.Rows) (*domain.VerificationRecord, error) {
	var rec domain.VerificationRecord

	if err := rows.Scan(
		&rec.ID, &rec.OrganizationID, &rec.EvidenceID, &rec.Status,
		&rec.VerifierID, &rec.VerifiedAt, &rec.Reason, &rec.Method, &rec.Notes, &rec.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan verification history: %w", err)
	}
	return &rec, nil
}

func (r *PostgresEvidenceRepository) SaveDocument(ctx context.Context, d *domain.Document) error {
	return r.SaveDocumentTx(ctx, nil, d)
}

func (r *PostgresEvidenceRepository) SaveDocumentTx(ctx context.Context, tx *sql.Tx, d *domain.Document) error {
	var ex sqlExecer = r.db
	if tx != nil {
		ex = tx
	}

	query := `
		INSERT INTO evidence_documents (
			id, organization_id, evidence_id, file_name, content_type,
			size_bytes, checksum, storage_key, storage_provider,
			uploaded_by, uploaded_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err := ex.ExecContext(ctx, query,
		d.ID, d.OrganizationID, d.EvidenceID, d.FileName, d.ContentType,
		d.SizeBytes, d.Checksum, d.StorageKey, d.StorageProvider,
		d.UploadedBy, d.UploadedAt, d.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert evidence document: %w", err)
	}
	return nil
}

func (r *PostgresEvidenceRepository) FindDocumentByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Document, error) {
	query := `
		SELECT id, organization_id, evidence_id, file_name, content_type,
			   size_bytes, checksum, storage_key, storage_provider,
			   uploaded_by, uploaded_at, created_at
		FROM evidence_documents
		WHERE organization_id = $1 AND id = $2
	`
	return r.scanDocument(r.db.QueryRowContext(ctx, query, orgID, id))
}

func (r *PostgresEvidenceRepository) FindDocumentByEvidence(ctx context.Context, orgID, evidenceID uuid.UUID) (*domain.Document, error) {
	query := `
		SELECT id, organization_id, evidence_id, file_name, content_type,
			   size_bytes, checksum, storage_key, storage_provider,
			   uploaded_by, uploaded_at, created_at
		FROM evidence_documents
		WHERE organization_id = $1 AND evidence_id = $2
		ORDER BY created_at DESC
		LIMIT 1
	`
	return r.scanDocument(r.db.QueryRowContext(ctx, query, orgID, evidenceID))
}

func (r *PostgresEvidenceRepository) CountDocuments(ctx context.Context, orgID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM evidence_documents WHERE organization_id = $1`
	var total int
	err := r.db.QueryRowContext(ctx, query, orgID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to count documents: %w", err)
	}
	return total, nil
}

func (r *PostgresEvidenceRepository) ListDocuments(ctx context.Context, orgID, evidenceID uuid.UUID, limit, offset int) ([]*domain.Document, int, error) {
	query := `
		SELECT id, organization_id, evidence_id, file_name, content_type,
			   size_bytes, checksum, storage_key, storage_provider,
			   uploaded_by, uploaded_at, created_at
		FROM evidence_documents
		WHERE organization_id = $1 AND evidence_id = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`

	countQuery := `SELECT COUNT(*) FROM evidence_documents WHERE organization_id = $1 AND evidence_id = $2`
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, orgID, evidenceID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count documents: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, orgID, evidenceID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list documents: %w", err)
	}
	defer rows.Close()

	var docs []*domain.Document
	for rows.Next() {
		var d domain.Document
		if err := rows.Scan(
			&d.ID, &d.OrganizationID, &d.EvidenceID, &d.FileName, &d.ContentType,
			&d.SizeBytes, &d.Checksum, &d.StorageKey, &d.StorageProvider,
			&d.UploadedBy, &d.UploadedAt, &d.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan document: %w", err)
		}
		docs = append(docs, &d)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to iterate documents: %w", err)
	}
	return docs, total, nil
}

func (r *PostgresEvidenceRepository) scanDocument(row interface {
	Scan(dest ...any) error
}) (*domain.Document, error) {
	var d domain.Document
	if err := row.Scan(
		&d.ID, &d.OrganizationID, &d.EvidenceID, &d.FileName, &d.ContentType,
		&d.SizeBytes, &d.Checksum, &d.StorageKey, &d.StorageProvider,
		&d.UploadedBy, &d.UploadedAt, &d.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrDocumentNotFound
		}
		return nil, fmt.Errorf("failed to scan document: %w", err)
	}
	return &d, nil
}

func (r *PostgresEvidenceRepository) DeleteDocument(ctx context.Context, orgID, id uuid.UUID) error {
	query := `
		DELETE FROM evidence_documents
		WHERE organization_id = $1 AND id = $2
	`
	result, err := r.db.ExecContext(ctx, query, orgID, id)
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return domain.ErrDocumentNotFound
	}
	return nil
}
