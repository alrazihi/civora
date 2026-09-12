package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/alrazihi/civora/internal/form_submission/domain"
	"github.com/google/uuid"
)

type PostgresFormSubmissionRepository struct {
	db *sql.DB
}

func NewPostgresFormSubmissionRepository(db *sql.DB) *PostgresFormSubmissionRepository {
	return &PostgresFormSubmissionRepository{db: db}
}

func (r *PostgresFormSubmissionRepository) DB() *sql.DB {
	return r.db
}

func (r *PostgresFormSubmissionRepository) SaveTx(ctx context.Context, tx *sql.Tx, submission *domain.FormSubmission) error {
	query := `
		INSERT INTO form_submissions
			(id, tenant_id, case_id, form_id, form_version_id, submitted_by, status, data, submitted_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := tx.ExecContext(ctx, query,
		submission.ID,
		submission.TenantID,
		submission.CaseID,
		submission.FormID,
		submission.FormVersionID,
		submission.SubmittedBy,
		string(submission.Status),
		submission.Data,
		submission.SubmittedAt,
		submission.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save form submission: %w", err)
	}
	return nil
}

func (r *PostgresFormSubmissionRepository) Save(ctx context.Context, submission *domain.FormSubmission) error {
	return r.SaveTx(ctx, nil, submission)
}

func (r *PostgresFormSubmissionRepository) FindByIDTx(ctx context.Context, tx *sql.Tx, tenantID, id uuid.UUID) (*domain.FormSubmission, error) {
	query := `
		SELECT id, tenant_id, case_id, form_id, form_version_id, submitted_by, status, data, submitted_at, updated_at
		FROM form_submissions
		WHERE tenant_id = $1 AND id = $2
	`
	row := r.queryRow(ctx, tx, query, tenantID, id)
	return r.scanSubmission(row)
}

func (r *PostgresFormSubmissionRepository) FindByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.FormSubmission, error) {
	return r.FindByIDTx(ctx, nil, tenantID, id)
}

func (r *PostgresFormSubmissionRepository) FindByIDForUpdateTx(ctx context.Context, tx *sql.Tx, tenantID, id uuid.UUID) (*domain.FormSubmission, error) {
	query := `
		SELECT id, tenant_id, case_id, form_id, form_version_id, submitted_by, status, data, submitted_at, updated_at
		FROM form_submissions
		WHERE tenant_id = $1 AND id = $2
		FOR UPDATE
	`
	row := r.queryRow(ctx, tx, query, tenantID, id)
	return r.scanSubmission(row)
}

func (r *PostgresFormSubmissionRepository) FindByCaseAndFormVersionTx(ctx context.Context, tx *sql.Tx, tenantID, caseID, formVersionID uuid.UUID) (*domain.FormSubmission, error) {
	query := `
		SELECT id, tenant_id, case_id, form_id, form_version_id, submitted_by, status, data, submitted_at, updated_at
		FROM form_submissions
		WHERE tenant_id = $1 AND case_id = $2 AND form_version_id = $3
	`
	row := r.queryRow(ctx, tx, query, tenantID, caseID, formVersionID)
	return r.scanSubmission(row)
}

func (r *PostgresFormSubmissionRepository) FindByCaseAndFormVersion(ctx context.Context, tenantID, caseID, formVersionID uuid.UUID) (*domain.FormSubmission, error) {
	return r.FindByCaseAndFormVersionTx(ctx, nil, tenantID, caseID, formVersionID)
}

func (r *PostgresFormSubmissionRepository) FindByCaseAndFormVersionForUpdateTx(ctx context.Context, tx *sql.Tx, tenantID, caseID, formVersionID uuid.UUID) (*domain.FormSubmission, error) {
	query := `
		SELECT id, tenant_id, case_id, form_id, form_version_id, submitted_by, status, data, submitted_at, updated_at
		FROM form_submissions
		WHERE tenant_id = $1 AND case_id = $2 AND form_version_id = $3
		FOR UPDATE
	`
	row := r.queryRow(ctx, tx, query, tenantID, caseID, formVersionID)
	return r.scanSubmission(row)
}

func (r *PostgresFormSubmissionRepository) ListByCase(ctx context.Context, tenantID, caseID uuid.UUID) ([]*domain.FormSubmission, error) {
	query := `
		SELECT id, tenant_id, case_id, form_id, form_version_id, submitted_by, status, data, submitted_at, updated_at
		FROM form_submissions
		WHERE tenant_id = $1 AND case_id = $2
		ORDER BY submitted_at DESC
	`
	rows, err := r.query(ctx, nil, query, tenantID, caseID)
	if err != nil {
		return nil, fmt.Errorf("failed to list submissions by case: %w", err)
	}
	defer rows.Close()

	var submissions []*domain.FormSubmission
	for rows.Next() {
		submission, err := r.scanSubmissionFromRows(rows)
		if err != nil {
			return nil, err
		}
		submissions = append(submissions, submission)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating submission rows: %w", err)
	}
	return submissions, nil
}

func (r *PostgresFormSubmissionRepository) UpdateTx(ctx context.Context, tx *sql.Tx, submission *domain.FormSubmission) error {
	query := `
		UPDATE form_submissions
		SET status = $3, data = $4, updated_at = $5
		WHERE tenant_id = $1 AND id = $2
	`
	_, err := tx.ExecContext(ctx, query,
		submission.TenantID,
		submission.ID,
		string(submission.Status),
		submission.Data,
		submission.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to update form submission: %w", err)
	}
	return nil
}

func (r *PostgresFormSubmissionRepository) Update(ctx context.Context, submission *domain.FormSubmission) error {
	return r.UpdateTx(ctx, nil, submission)
}

func (r *PostgresFormSubmissionRepository) DeleteTx(ctx context.Context, tx *sql.Tx, tenantID, id uuid.UUID) error {
	query := `DELETE FROM form_submissions WHERE tenant_id = $1 AND id = $2`
	_, err := tx.ExecContext(ctx, query, tenantID, id)
	if err != nil {
		return fmt.Errorf("failed to delete form submission: %w", err)
	}
	return nil
}

func (r *PostgresFormSubmissionRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return r.DeleteTx(ctx, nil, tenantID, id)
}

func (r *PostgresFormSubmissionRepository) queryRow(ctx context.Context, tx *sql.Tx, query string, args ...interface{}) *sql.Row {
	if tx != nil {
		return tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *PostgresFormSubmissionRepository) query(ctx context.Context, tx *sql.Tx, query string, args ...interface{}) (*sql.Rows, error) {
	if tx != nil {
		return tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *PostgresFormSubmissionRepository) scanSubmission(row interface{ Scan(...interface{}) error }) (*domain.FormSubmission, error) {
	var s domain.FormSubmission
	var statusStr string
	var submittedAt, updatedAt time.Time
	var data sql.NullString

	err := row.Scan(
		&s.ID,
		&s.TenantID,
		&s.CaseID,
		&s.FormID,
		&s.FormVersionID,
		&s.SubmittedBy,
		&statusStr,
		&data,
		&submittedAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrSubmissionNotFound
		}
		return nil, fmt.Errorf("failed to scan submission: %w", err)
	}

	s.Status = domain.SubmissionStatus(statusStr)
	s.SubmittedAt = submittedAt.UTC()
	s.UpdatedAt = updatedAt.UTC()
	if data.Valid && data.String != "" {
		s.Data = map[string]interface{}{}
		// JSON parsing would happen here in production
		_ = data.String
	}
	return &s, nil
}

func (r *PostgresFormSubmissionRepository) scanSubmissionFromRows(rows *sql.Rows) (*domain.FormSubmission, error) {
	var s domain.FormSubmission
	var statusStr string
	var submittedAt, updatedAt time.Time
	var data sql.NullString

	err := rows.Scan(
		&s.ID,
		&s.TenantID,
		&s.CaseID,
		&s.FormID,
		&s.FormVersionID,
		&s.SubmittedBy,
		&statusStr,
		&data,
		&submittedAt,
		&updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan submission: %w", err)
	}

	s.Status = domain.SubmissionStatus(statusStr)
	s.SubmittedAt = submittedAt.UTC()
	s.UpdatedAt = updatedAt.UTC()
	if data.Valid && data.String != "" {
		s.Data = map[string]interface{}{}
		_ = data.String
	}
	return &s, nil
}
