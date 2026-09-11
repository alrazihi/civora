package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/alrazihi/civora/internal/assistance/domain"
	"github.com/google/uuid"
)

type PostgresAssistanceRepository struct {
	db *sql.DB
}

type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func NewPostgresAssistanceRepository(db *sql.DB) *PostgresAssistanceRepository {
	return &PostgresAssistanceRepository{db: db}
}

func (r *PostgresAssistanceRepository) DB() *sql.DB {
	return r.db
}

func (r *PostgresAssistanceRepository) Save(ctx context.Context, a *domain.Assistance) error {
	return r.saveAssistance(ctx, r.db, a)
}

func (r *PostgresAssistanceRepository) SaveTx(ctx context.Context, tx *sql.Tx, a *domain.Assistance) error {
	return r.saveAssistance(ctx, tx, a)
}

func (r *PostgresAssistanceRepository) saveAssistance(ctx context.Context, ex sqlExecer, a *domain.Assistance) error {
	query := `
		INSERT INTO assistance (
			id, organization_id, service_request_id, type, description,
			status, responsible_staff, started_at, completed_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO UPDATE SET
			organization_id = EXCLUDED.organization_id,
			service_request_id = EXCLUDED.service_request_id,
			type = EXCLUDED.type,
			description = EXCLUDED.description,
			status = EXCLUDED.status,
			responsible_staff = EXCLUDED.responsible_staff,
			started_at = EXCLUDED.started_at,
			completed_at = EXCLUDED.completed_at,
			updated_at = EXCLUDED.updated_at
	`
	_, err := ex.ExecContext(ctx, query,
		a.ID, a.OrganizationID, a.ServiceRequestID, a.Type, a.Description,
		a.Status, a.ResponsibleStaff, a.StartedAt, a.CompletedAt, a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert assistance: %w", err)
	}
	return nil
}

func (r *PostgresAssistanceRepository) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Assistance, error) {
	query := `
		SELECT id, organization_id, service_request_id, type, description,
			   status, responsible_staff, started_at, completed_at, created_at, updated_at
		FROM assistance
		WHERE organization_id = $1 AND id = $2
	`
	return r.scanAssistance(r.db.QueryRowContext(ctx, query, orgID, id))
}

func (r *PostgresAssistanceRepository) FindByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID, limit, offset int) ([]*domain.Assistance, error) {
	query := `
		SELECT id, organization_id, service_request_id, type, description,
			   status, responsible_staff, started_at, completed_at, created_at, updated_at
		FROM assistance
		WHERE organization_id = $1 AND service_request_id = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, serviceRequestID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query assistance: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []*domain.Assistance
	for rows.Next() {
		a, err := r.scanAssistanceFromRows(rows)
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

func (r *PostgresAssistanceRepository) CountByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM assistance WHERE organization_id = $1 AND service_request_id = $2`
	var total int
	err := r.db.QueryRowContext(ctx, query, orgID, serviceRequestID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to count assistance: %w", err)
	}
	return total, nil
}

func (r *PostgresAssistanceRepository) FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Assistance, error) {
	query := `
		SELECT id, organization_id, service_request_id, type, description,
			   status, responsible_staff, started_at, completed_at, created_at, updated_at
		FROM assistance
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query assistance: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []*domain.Assistance
	for rows.Next() {
		a, err := r.scanAssistanceFromRows(rows)
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

func (r *PostgresAssistanceRepository) CountByOrganization(ctx context.Context, orgID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM assistance WHERE organization_id = $1`
	var total int
	err := r.db.QueryRowContext(ctx, query, orgID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to count assistance: %w", err)
	}
	return total, nil
}

func (r *PostgresAssistanceRepository) scanAssistance(row interface {
	Scan(dest ...any) error
}) (*domain.Assistance, error) {
	var a domain.Assistance
	if err := row.Scan(
		&a.ID, &a.OrganizationID, &a.ServiceRequestID, &a.Type, &a.Description,
		&a.Status, &a.ResponsibleStaff, &a.StartedAt, &a.CompletedAt, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrAssistanceNotFound
		}
		return nil, fmt.Errorf("failed to scan assistance: %w", err)
	}
	return &a, nil
}

func (r *PostgresAssistanceRepository) scanAssistanceFromRows(rows *sql.Rows) (*domain.Assistance, error) {
	var a domain.Assistance
	if err := rows.Scan(
		&a.ID, &a.OrganizationID, &a.ServiceRequestID, &a.Type, &a.Description,
		&a.Status, &a.ResponsibleStaff, &a.StartedAt, &a.CompletedAt, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan assistance: %w", err)
	}
	return &a, nil
}
