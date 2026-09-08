package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/alrazihi/civora/internal/assessment/domain"
	"github.com/google/uuid"
)

type PostgresAssessmentRepository struct {
	db *sql.DB
}

type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func NewPostgresAssessmentRepository(db *sql.DB) *PostgresAssessmentRepository {
	return &PostgresAssessmentRepository{db: db}
}

func (r *PostgresAssessmentRepository) DB() *sql.DB {
	return r.db
}

func (r *PostgresAssessmentRepository) Save(ctx context.Context, a *domain.Assessment) error {
	return r.saveAssessment(ctx, r.db, a)
}

func (r *PostgresAssessmentRepository) SaveTx(ctx context.Context, tx *sql.Tx, a *domain.Assessment) error {
	return r.saveAssessment(ctx, tx, a)
}

func (r *PostgresAssessmentRepository) saveAssessment(ctx context.Context, ex sqlExecer, a *domain.Assessment) error {
	query := `
		INSERT INTO assessments (
			id, organization_id, service_request_id, findings,
			needs_identified, recommendation, assessor, assessed_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := ex.ExecContext(ctx, query,
		a.ID, a.OrganizationID, a.ServiceRequestID, a.Findings,
		a.NeedsIdentified, a.Recommendation, a.Assessor, a.AssessedAt, a.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert assessment: %w", err)
	}
	return nil
}

func (r *PostgresAssessmentRepository) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Assessment, error) {
	query := `
		SELECT id, organization_id, service_request_id, findings,
			   needs_identified, recommendation, assessor, assessed_at, created_at
		FROM assessments
		WHERE organization_id = $1 AND id = $2
	`
	return r.scanAssessment(r.db.QueryRowContext(ctx, query, orgID, id))
}

func (r *PostgresAssessmentRepository) FindByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) (*domain.Assessment, error) {
	query := `
		SELECT id, organization_id, service_request_id, findings,
			   needs_identified, recommendation, assessor, assessed_at, created_at
		FROM assessments
		WHERE organization_id = $1 AND service_request_id = $2
		LIMIT 1
	`
	return r.scanAssessment(r.db.QueryRowContext(ctx, query, orgID, serviceRequestID))
}

func (r *PostgresAssessmentRepository) FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Assessment, error) {
	query := `
		SELECT id, organization_id, service_request_id, findings,
			   needs_identified, recommendation, assessor, assessed_at, created_at
		FROM assessments
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query assessments: %w", err)
	}
	defer rows.Close()

	var items []*domain.Assessment
	for rows.Next() {
		a, err := r.scanAssessmentFromRows(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return items, nil
}

func (r *PostgresAssessmentRepository) CountByOrganization(ctx context.Context, orgID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM assessments WHERE organization_id = $1`
	var total int
	err := r.db.QueryRowContext(ctx, query, orgID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to count assessments: %w", err)
	}
	return total, nil
}

func (r *PostgresAssessmentRepository) scanAssessment(row interface {
	Scan(dest ...any) error
}) (*domain.Assessment, error) {
	var a domain.Assessment
	if err := row.Scan(
		&a.ID, &a.OrganizationID, &a.ServiceRequestID, &a.Findings,
		&a.NeedsIdentified, &a.Recommendation, &a.Assessor, &a.AssessedAt, &a.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrAssessmentNotFound
		}
		return nil, fmt.Errorf("failed to scan assessment: %w", err)
	}
	return &a, nil
}

func (r *PostgresAssessmentRepository) scanAssessmentFromRows(rows *sql.Rows) (*domain.Assessment, error) {
	var a domain.Assessment
	if err := rows.Scan(
		&a.ID, &a.OrganizationID, &a.ServiceRequestID, &a.Findings,
		&a.NeedsIdentified, &a.Recommendation, &a.Assessor, &a.AssessedAt, &a.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan assessment: %w", err)
	}
	return &a, nil
}
