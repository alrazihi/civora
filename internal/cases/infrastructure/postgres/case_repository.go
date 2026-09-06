package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/alrazihi/civora/internal/cases/domain"
	"github.com/google/uuid"
)

type PostgresCaseRepository struct {
	db *sql.DB
}

func NewPostgresCaseRepository(db *sql.DB) *PostgresCaseRepository {
	return &PostgresCaseRepository{db: db}
}

func (r *PostgresCaseRepository) Save(ctx context.Context, c *domain.Case) error {
	query := `
		INSERT INTO cases (
			id, organization_id, case_number, title, description,
			status, created_by, assigned_to, created_at, updated_at, closed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := r.db.ExecContext(ctx, query,
		c.ID, c.OrganizationID, c.CaseNumber, c.Title, c.Description,
		c.Status, c.CreatedByID, c.AssignedToID, c.CreatedAt, c.UpdatedAt, c.ClosedAt)
	if err != nil {
		return fmt.Errorf("failed to insert case: %w", err)
	}
	return nil
}

func (r *PostgresCaseRepository) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Case, error) {
	query := `
		SELECT id, organization_id, case_number, title, description,
			   status, created_by, assigned_to, created_at, updated_at, closed_at
		FROM cases
		WHERE organization_id = $1 AND id = $2
	`
	return r.scanCase(r.db.QueryRowContext(ctx, query, orgID, id))
}

func (r *PostgresCaseRepository) FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Case, error) {
	query := `
		SELECT id, organization_id, case_number, title, description,
			   status, created_by, assigned_to, created_at, updated_at, closed_at
		FROM cases
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query cases: %w", err)
	}
	defer rows.Close()

	var cases []*domain.Case
	for rows.Next() {
		c, err := r.scanCaseFromRows(rows)
		if err != nil {
			return nil, err
		}
		cases = append(cases, c)
	}
	return cases, nil
}

func (r *PostgresCaseRepository) UpdateStatus(ctx context.Context, orgID, id uuid.UUID, status domain.CaseStatus) error {
	query := `
		UPDATE cases
		SET status = $1, updated_at = now()
		WHERE organization_id = $2 AND id = $3
	`
	result, err := r.db.ExecContext(ctx, query, status, orgID, id)
	if err != nil {
		return fmt.Errorf("failed to update case status: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no rows affected")
	}
	return nil
}

func (r *PostgresCaseRepository) Assign(ctx context.Context, orgID, id, userID uuid.UUID) error {
	query := `
		UPDATE cases
		SET assigned_to = $1, updated_at = now()
		WHERE organization_id = $2 AND id = $3
	`
	result, err := r.db.ExecContext(ctx, query, userID, orgID, id)
	if err != nil {
		return fmt.Errorf("failed to assign case: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no rows affected")
	}
	return nil
}

func (r *PostgresCaseRepository) scanCase(row interface {
	Scan(dest ...any) error
}) (*domain.Case, error) {
	var c domain.Case
	if err := row.Scan(
		&c.ID, &c.OrganizationID, &c.CaseNumber, &c.Title, &c.Description,
		&c.Status, &c.CreatedByID, &c.AssignedToID, &c.CreatedAt, &c.UpdatedAt, &c.ClosedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan case: %w", err)
	}
	return &c, nil
}

func (r *PostgresCaseRepository) scanCaseFromRows(rows *sql.Rows) (*domain.Case, error) {
	var c domain.Case
	if err := rows.Scan(
		&c.ID, &c.OrganizationID, &c.CaseNumber, &c.Title, &c.Description,
		&c.Status, &c.CreatedByID, &c.AssignedToID, &c.CreatedAt, &c.UpdatedAt, &c.ClosedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan case: %w", err)
	}
	return &c, nil
}
