package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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
	query := `
		INSERT INTO evidence (
			id, organization_id, service_request_id, type, description,
			storage_reference, uploaded_by, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := ex.ExecContext(ctx, query,
		e.ID, e.OrganizationID, e.ServiceRequestID, e.Type, e.Description,
		e.StorageReference, e.UploadedBy, e.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert evidence: %w", err)
	}
	return nil
}

func (r *PostgresEvidenceRepository) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Evidence, error) {
	query := `
		SELECT id, organization_id, service_request_id, type, description,
			   storage_reference, uploaded_by, created_at
		FROM evidence
		WHERE organization_id = $1 AND id = $2
	`
	return r.scanEvidence(r.db.QueryRowContext(ctx, query, orgID, id))
}

func (r *PostgresEvidenceRepository) FindByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID, limit, offset int) ([]*domain.Evidence, error) {
	query := `
		SELECT id, organization_id, service_request_id, type, description,
			   storage_reference, uploaded_by, created_at
		FROM evidence
		WHERE organization_id = $1 AND service_request_id = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, serviceRequestID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query evidence: %w", err)
	}
	defer rows.Close()

	var items []*domain.Evidence
	for rows.Next() {
		e, err := r.scanEvidenceFromRows(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, e)
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
		SELECT id, organization_id, service_request_id, type, description,
			   storage_reference, uploaded_by, created_at
		FROM evidence
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query evidence: %w", err)
	}
	defer rows.Close()

	var items []*domain.Evidence
	for rows.Next() {
		e, err := r.scanEvidenceFromRows(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	return items, nil
}

func (r *PostgresEvidenceRepository) scanEvidence(row interface {
	Scan(dest ...any) error
}) (*domain.Evidence, error) {
	var e domain.Evidence
	if err := row.Scan(
		&e.ID, &e.OrganizationID, &e.ServiceRequestID, &e.Type, &e.Description,
		&e.StorageReference, &e.UploadedBy, &e.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrEvidenceNotFound
		}
		return nil, fmt.Errorf("failed to scan evidence: %w", err)
	}
	return &e, nil
}

func (r *PostgresEvidenceRepository) scanEvidenceFromRows(rows *sql.Rows) (*domain.Evidence, error) {
	var e domain.Evidence
	if err := rows.Scan(
		&e.ID, &e.OrganizationID, &e.ServiceRequestID, &e.Type, &e.Description,
		&e.StorageReference, &e.UploadedBy, &e.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan evidence: %w", err)
	}
	return &e, nil
}
