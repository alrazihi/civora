package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/alrazihi/civora/internal/followup/domain"
	"github.com/google/uuid"
)

type PostgresFollowUpRepository struct {
	db *sql.DB
}

type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func NewPostgresFollowUpRepository(db *sql.DB) *PostgresFollowUpRepository {
	return &PostgresFollowUpRepository{db: db}
}

func (r *PostgresFollowUpRepository) DB() *sql.DB {
	return r.db
}

func (r *PostgresFollowUpRepository) Save(ctx context.Context, f *domain.FollowUp) error {
	return r.saveFollowUp(ctx, r.db, f)
}

func (r *PostgresFollowUpRepository) SaveTx(ctx context.Context, tx *sql.Tx, f *domain.FollowUp) error {
	return r.saveFollowUp(ctx, tx, f)
}

func (r *PostgresFollowUpRepository) saveFollowUp(ctx context.Context, ex sqlExecer, f *domain.FollowUp) error {
	query := `
		INSERT INTO follow_ups (
			id, organization_id, service_request_id, scheduled_date, completed_date,
			outcome, notes, performed_by, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			organization_id = EXCLUDED.organization_id,
			service_request_id = EXCLUDED.service_request_id,
			scheduled_date = EXCLUDED.scheduled_date,
			completed_date = EXCLUDED.completed_date,
			outcome = EXCLUDED.outcome,
			notes = EXCLUDED.notes,
			performed_by = EXCLUDED.performed_by,
			updated_at = EXCLUDED.updated_at
	`
	_, err := ex.ExecContext(ctx, query,
		f.ID, f.OrganizationID, f.ServiceRequestID, f.ScheduledDate, f.CompletedDate,
		f.Outcome, f.Notes, f.PerformedBy, f.CreatedAt, f.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert follow-up: %w", err)
	}
	return nil
}

func (r *PostgresFollowUpRepository) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.FollowUp, error) {
	query := `
		SELECT id, organization_id, service_request_id, scheduled_date, completed_date,
			   outcome, notes, performed_by, created_at, updated_at
		FROM follow_ups
		WHERE organization_id = $1 AND id = $2
	`
	return r.scanFollowUp(r.db.QueryRowContext(ctx, query, orgID, id))
}

func (r *PostgresFollowUpRepository) FindByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID, limit, offset int) ([]*domain.FollowUp, error) {
	query := `
		SELECT id, organization_id, service_request_id, scheduled_date, completed_date,
			   outcome, notes, performed_by, created_at, updated_at
		FROM follow_ups
		WHERE organization_id = $1 AND service_request_id = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, serviceRequestID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query follow-ups: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var items []*domain.FollowUp
	for rows.Next() {
		f, err := r.scanFollowUpFromRows(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return items, nil
}

func (r *PostgresFollowUpRepository) CountByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM follow_ups WHERE organization_id = $1 AND service_request_id = $2`
	var total int
	err := r.db.QueryRowContext(ctx, query, orgID, serviceRequestID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to count follow-ups: %w", err)
	}
	return total, nil
}

func (r *PostgresFollowUpRepository) FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.FollowUp, error) {
	query := `
		SELECT id, organization_id, service_request_id, scheduled_date, completed_date,
			   outcome, notes, performed_by, created_at, updated_at
		FROM follow_ups
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query follow-ups: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var items []*domain.FollowUp
	for rows.Next() {
		f, err := r.scanFollowUpFromRows(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return items, nil
}

func (r *PostgresFollowUpRepository) CountByOrganization(ctx context.Context, orgID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM follow_ups WHERE organization_id = $1`
	var total int
	err := r.db.QueryRowContext(ctx, query, orgID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to count follow-ups: %w", err)
	}
	return total, nil
}

func (r *PostgresFollowUpRepository) scanFollowUp(row interface {
	Scan(dest ...any) error
}) (*domain.FollowUp, error) {
	var f domain.FollowUp
	if err := row.Scan(
		&f.ID, &f.OrganizationID, &f.ServiceRequestID, &f.ScheduledDate, &f.CompletedDate,
		&f.Outcome, &f.Notes, &f.PerformedBy, &f.CreatedAt, &f.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrFollowUpNotFound
		}
		return nil, fmt.Errorf("failed to scan follow-up: %w", err)
	}
	return &f, nil
}

func (r *PostgresFollowUpRepository) scanFollowUpFromRows(rows *sql.Rows) (*domain.FollowUp, error) {
	var f domain.FollowUp
	if err := rows.Scan(
		&f.ID, &f.OrganizationID, &f.ServiceRequestID, &f.ScheduledDate, &f.CompletedDate,
		&f.Outcome, &f.Notes, &f.PerformedBy, &f.CreatedAt, &f.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan follow-up: %w", err)
	}
	return &f, nil
}
