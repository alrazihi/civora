package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/alrazihi/civora/internal/eligibility/domain"
	"github.com/google/uuid"
)

type PostgresEligibilityRepository struct {
	db *sql.DB
}

type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func NewPostgresEligibilityRepository(db *sql.DB) *PostgresEligibilityRepository {
	return &PostgresEligibilityRepository{db: db}
}

func (r *PostgresEligibilityRepository) DB() *sql.DB {
	return r.db
}

func (r *PostgresEligibilityRepository) Save(ctx context.Context, e *domain.Eligibility) error {
	return r.saveEligibility(ctx, r.db, e)
}

func (r *PostgresEligibilityRepository) SaveTx(ctx context.Context, tx *sql.Tx, e *domain.Eligibility) error {
	return r.saveEligibility(ctx, tx, e)
}

func (r *PostgresEligibilityRepository) saveEligibility(ctx context.Context, ex sqlExecer, e *domain.Eligibility) error {
	query := `
		INSERT INTO eligibilities (
			id, organization_id, service_request_id, criteria, result,
			explanation, assessed_by, assessed_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			organization_id = EXCLUDED.organization_id,
			service_request_id = EXCLUDED.service_request_id,
			criteria = EXCLUDED.criteria,
			result = EXCLUDED.result,
			explanation = EXCLUDED.explanation,
			assessed_by = EXCLUDED.assessed_by,
			assessed_at = EXCLUDED.assessed_at
	`
	_, err := ex.ExecContext(ctx, query,
		e.ID, e.OrganizationID, e.ServiceRequestID, e.Criteria, e.Result,
		e.Explanation, e.AssessedBy, e.AssessedAt, e.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert eligibility: %w", err)
	}
	return nil
}

func (r *PostgresEligibilityRepository) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Eligibility, error) {
	query := `
		SELECT id, organization_id, service_request_id, criteria, result,
			   explanation, assessed_by, assessed_at, created_at
		FROM eligibilities
		WHERE organization_id = $1 AND id = $2
	`
	return r.scanEligibility(r.db.QueryRowContext(ctx, query, orgID, id))
}

func (r *PostgresEligibilityRepository) FindByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) (*domain.Eligibility, error) {
	query := `
		SELECT id, organization_id, service_request_id, criteria, result,
			   explanation, assessed_by, assessed_at, created_at
		FROM eligibilities
		WHERE organization_id = $1 AND service_request_id = $2
		LIMIT 1
	`
	return r.scanEligibility(r.db.QueryRowContext(ctx, query, orgID, serviceRequestID))
}

func (r *PostgresEligibilityRepository) FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Eligibility, error) {
	query := `
		SELECT id, organization_id, service_request_id, criteria, result,
			   explanation, assessed_by, assessed_at, created_at
		FROM eligibilities
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query eligibilities: %w", err)
	}
	defer rows.Close()

	var items []*domain.Eligibility
	for rows.Next() {
		e, err := r.scanEligibilityFromRows(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	return items, nil
}

func (r *PostgresEligibilityRepository) CountByOrganization(ctx context.Context, orgID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM eligibilities WHERE organization_id = $1`
	var total int
	err := r.db.QueryRowContext(ctx, query, orgID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to count eligibilities: %w", err)
	}
	return total, nil
}

func (r *PostgresEligibilityRepository) scanEligibility(row interface {
	Scan(dest ...any) error
}) (*domain.Eligibility, error) {
	var e domain.Eligibility
	var criteriaJSON []byte

	if err := row.Scan(
		&e.ID, &e.OrganizationID, &e.ServiceRequestID, &criteriaJSON, &e.Result,
		&e.Explanation, &e.AssessedBy, &e.AssessedAt, &e.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrEligibilityNotFound
		}
		return nil, fmt.Errorf("failed to scan eligibility: %w", err)
	}

	if len(criteriaJSON) > 0 {
		if err := json.Unmarshal(criteriaJSON, &e.Criteria); err != nil {
			return nil, fmt.Errorf("failed to unmarshal eligibility criteria: %w", err)
		}
	}

	return &e, nil
}

func (r *PostgresEligibilityRepository) scanEligibilityFromRows(rows *sql.Rows) (*domain.Eligibility, error) {
	var e domain.Eligibility
	var criteriaJSON []byte

	if err := rows.Scan(
		&e.ID, &e.OrganizationID, &e.ServiceRequestID, &criteriaJSON, &e.Result,
		&e.Explanation, &e.AssessedBy, &e.AssessedAt, &e.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan eligibility: %w", err)
	}

	if len(criteriaJSON) > 0 {
		if err := json.Unmarshal(criteriaJSON, &e.Criteria); err != nil {
			return nil, fmt.Errorf("failed to unmarshal eligibility criteria: %w", err)
		}
	}

	return &e, nil
}
