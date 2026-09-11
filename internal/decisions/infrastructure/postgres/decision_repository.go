package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/alrazihi/civora/internal/decisions/domain"
	"github.com/google/uuid"
)

type PostgresDecisionRepository struct {
	db *sql.DB
}

type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func NewPostgresDecisionRepository(db *sql.DB) *PostgresDecisionRepository {
	return &PostgresDecisionRepository{db: db}
}

func (r *PostgresDecisionRepository) DB() *sql.DB {
	return r.db
}

func (r *PostgresDecisionRepository) Save(ctx context.Context, d *domain.Decision) error {
	return r.saveDecision(ctx, r.db, d)
}

func (r *PostgresDecisionRepository) SaveTx(ctx context.Context, tx *sql.Tx, d *domain.Decision) error {
	return r.saveDecision(ctx, tx, d)
}

func (r *PostgresDecisionRepository) saveDecision(ctx context.Context, ex sqlExecer, d *domain.Decision) error {
	query := `
		INSERT INTO decisions (
			id, organization_id, service_request_id, decision, reason,
			decision_maker, decided_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := ex.ExecContext(ctx, query,
		d.ID, d.OrganizationID, d.ServiceRequestID, d.Decision, d.Reason,
		d.DecisionMaker, d.DecidedAt, d.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert decision: %w", err)
	}
	return nil
}

func (r *PostgresDecisionRepository) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Decision, error) {
	query := `
		SELECT id, organization_id, service_request_id, decision, reason,
			   decision_maker, decided_at, created_at
		FROM decisions
		WHERE organization_id = $1 AND id = $2
	`
	return r.scanDecision(r.db.QueryRowContext(ctx, query, orgID, id))
}

func (r *PostgresDecisionRepository) FindByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) (*domain.Decision, error) {
	query := `
		SELECT id, organization_id, service_request_id, decision, reason,
			   decision_maker, decided_at, created_at
		FROM decisions
		WHERE organization_id = $1 AND service_request_id = $2
		LIMIT 1
	`
	return r.scanDecision(r.db.QueryRowContext(ctx, query, orgID, serviceRequestID))
}

func (r *PostgresDecisionRepository) FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Decision, error) {
	query := `
		SELECT id, organization_id, service_request_id, decision, reason,
			   decision_maker, decided_at, created_at
		FROM decisions
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query decisions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []*domain.Decision
	for rows.Next() {
		d, err := r.scanDecisionFromRows(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return items, nil
}

func (r *PostgresDecisionRepository) CountByOrganization(ctx context.Context, orgID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM decisions WHERE organization_id = $1`
	var total int
	err := r.db.QueryRowContext(ctx, query, orgID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to count decisions: %w", err)
	}
	return total, nil
}

func (r *PostgresDecisionRepository) scanDecision(row interface {
	Scan(dest ...any) error
}) (*domain.Decision, error) {
	var d domain.Decision
	if err := row.Scan(
		&d.ID, &d.OrganizationID, &d.ServiceRequestID, &d.Decision, &d.Reason,
		&d.DecisionMaker, &d.DecidedAt, &d.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrDecisionNotFound
		}
		return nil, fmt.Errorf("failed to scan decision: %w", err)
	}
	return &d, nil
}

func (r *PostgresDecisionRepository) scanDecisionFromRows(rows *sql.Rows) (*domain.Decision, error) {
	var d domain.Decision
	if err := rows.Scan(
		&d.ID, &d.OrganizationID, &d.ServiceRequestID, &d.Decision, &d.Reason,
		&d.DecisionMaker, &d.DecidedAt, &d.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan decision: %w", err)
	}
	return &d, nil
}
