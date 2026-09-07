package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/alrazihi/civora/internal/cases/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgconn"
)

type PostgresCaseRepository struct {
	db *sql.DB
}

type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func NewPostgresCaseRepository(db *sql.DB) *PostgresCaseRepository {
	return &PostgresCaseRepository{db: db}
}

func (r *PostgresCaseRepository) DB() *sql.DB {
	return r.db
}

func (r *PostgresCaseRepository) Save(ctx context.Context, c *domain.Case) error {
	return r.saveCase(ctx, r.db, c)
}

func (r *PostgresCaseRepository) SaveTx(ctx context.Context, tx *sql.Tx, c *domain.Case) error {
	return r.saveCase(ctx, tx, c)
}

func (r *PostgresCaseRepository) saveCase(ctx context.Context, e sqlExecer, c *domain.Case) error {
	query := `
		INSERT INTO cases (
			id, organization_id, case_number, title, description,
			status, service_type, priority, person_id, created_by, assigned_to,
			created_at, updated_at, closed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`
	const maxRetries = 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		_, err := e.ExecContext(ctx, query,
			c.ID, c.OrganizationID, c.CaseNumber, c.Title, c.Description,
			c.Status, c.ServiceType, c.Priority, c.PersonID, c.CreatedByID, c.AssignedToID,
			c.CreatedAt, c.UpdatedAt, c.ClosedAt,
		)
		if err == nil {
			return nil
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			c.CaseNumber = domain.GenerateCaseNumber(time.Now().UTC())
			continue
		}
		return fmt.Errorf("failed to insert case: %w", err)
	}
	return domain.ErrCaseNumberConflict
}

func (r *PostgresCaseRepository) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Case, error) {
	query := `
		SELECT id, organization_id, case_number, title, description,
			   status, service_type, priority, person_id, created_by, assigned_to,
			   created_at, updated_at, closed_at
		FROM cases
		WHERE organization_id = $1 AND id = $2
	`
	return r.scanCase(r.db.QueryRowContext(ctx, query, orgID, id))
}

func (r *PostgresCaseRepository) FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Case, error) {
	return r.FindByOrganizationWithFilter(ctx, orgID, limit, offset, domain.CaseFilter{})
}

func (r *PostgresCaseRepository) FindByOrganizationWithFilter(ctx context.Context, orgID uuid.UUID, limit, offset int, filter domain.CaseFilter) ([]*domain.Case, error) {
	query := `
		SELECT id, organization_id, case_number, title, description,
			   status, service_type, priority, person_id, created_by, assigned_to,
			   created_at, updated_at, closed_at
		FROM cases
		WHERE organization_id = $1
	`
	args := []interface{}{orgID}
	argPos := 2

	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argPos)
		args = append(args, string(filter.Status))
		argPos++
	}

	if filter.PersonID != nil {
		query += fmt.Sprintf(" AND person_id = $%d", argPos)
		args = append(args, *filter.PersonID)
		argPos++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argPos, argPos+1)
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
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

func (r *PostgresCaseRepository) CountByOrganization(ctx context.Context, orgID uuid.UUID, filter domain.CaseFilter) (int, error) {
	query := `SELECT COUNT(*) FROM cases WHERE organization_id = $1`
	args := []interface{}{orgID}
	argPos := 2

	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argPos)
		args = append(args, string(filter.Status))
		argPos++
	}

	if filter.PersonID != nil {
		query += fmt.Sprintf(" AND person_id = $%d", argPos)
		args = append(args, *filter.PersonID)
		argPos++
	}

	var total int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to count cases: %w", err)
	}
	return total, nil
}

func (r *PostgresCaseRepository) UpdateStatus(ctx context.Context, orgID, id uuid.UUID, status domain.CaseStatus) error {
	return r.updateStatus(ctx, r.db, orgID, id, status)
}

func (r *PostgresCaseRepository) UpdateStatusTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID, status domain.CaseStatus) error {
	return r.updateStatus(ctx, tx, orgID, id, status)
}

func (r *PostgresCaseRepository) updateStatus(ctx context.Context, e sqlExecer, orgID, id uuid.UUID, status domain.CaseStatus) error {
	query := `
		UPDATE cases
		SET status = $1, updated_at = now()
		WHERE organization_id = $2 AND id = $3
	`
	result, err := e.ExecContext(ctx, query, status, orgID, id)
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
	return r.assign(ctx, r.db, orgID, id, userID)
}

func (r *PostgresCaseRepository) AssignTx(ctx context.Context, tx *sql.Tx, orgID, id, userID uuid.UUID) error {
	return r.assign(ctx, tx, orgID, id, userID)
}

func (r *PostgresCaseRepository) assign(ctx context.Context, e sqlExecer, orgID, id, userID uuid.UUID) error {
	query := `
		UPDATE cases
		SET assigned_to = $1, updated_at = now()
		WHERE organization_id = $2 AND id = $3
	`
	result, err := e.ExecContext(ctx, query, userID, orgID, id)
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
		&c.Status, &c.ServiceType, &c.Priority, &c.PersonID, &c.CreatedByID, &c.AssignedToID,
		&c.CreatedAt, &c.UpdatedAt, &c.ClosedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan case: %w", err)
	}
	return &c, nil
}

func (r *PostgresCaseRepository) scanCaseFromRows(rows *sql.Rows) (*domain.Case, error) {
	var c domain.Case
	if err := rows.Scan(
		&c.ID, &c.OrganizationID, &c.CaseNumber, &c.Title, &c.Description,
		&c.Status, &c.ServiceType, &c.Priority, &c.PersonID, &c.CreatedByID, &c.AssignedToID,
		&c.CreatedAt, &c.UpdatedAt, &c.ClosedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan case: %w", err)
	}
	return &c, nil
}
