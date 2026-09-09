package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/alrazihi/civora/internal/cases/domain"
	"github.com/google/uuid"
)

// isPostgresUniqueViolation reports whether err is a Postgres unique-violation
// (SQLSTATE 23505). The pgx/v5 stdlib driver returns *pgconn.PgError from the
// github.com/jackc/pgx/v5/pgconn package, so we cannot rely on a single
// imported *pgconn.PgError type across the module. Matching on the SQLSTATE
// string is robust to both the plain pgconn and pgx/v5/pgconn packages.
func isPostgresUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	if strings.Contains(err.Error(), "SQLSTATE 23505") {
		return true
	}
	return false
}

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
			created_at, updated_at, closed_at, version, workflow_instance_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`
	const maxRetries = 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		_, err := e.ExecContext(ctx, query,
			c.ID, c.OrganizationID, c.CaseNumber, c.Title, c.Description,
			c.Status, c.ServiceType, c.Priority, c.PersonID, c.CreatedByID, c.AssignedToID,
			c.CreatedAt, c.UpdatedAt, c.ClosedAt, c.Version, c.WorkflowInstanceID,
		)
		if err == nil {
			return nil
		}
		if isPostgresUniqueViolation(err) {
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
			   created_at, updated_at, closed_at, version, workflow_instance_id
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
			   created_at, updated_at, closed_at, version, workflow_instance_id
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

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argPos, argPos+1) //nolint:gosec
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query cases: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var cases []*domain.Case
	for rows.Next() {
		c, err := r.scanCaseFromRows(rows)
		if err != nil {
			return nil, err
		}
		cases = append(cases, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
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
	}

	var total int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to count cases: %w", err)
	}
	return total, nil
}

func (r *PostgresCaseRepository) UpdateStatus(ctx context.Context, orgID, id uuid.UUID, status domain.CaseStatus, version int) error {
	return r.updateStatus(ctx, r.db, orgID, id, status, version)
}

func (r *PostgresCaseRepository) UpdateStatusTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID, status domain.CaseStatus, version int) error {
	return r.updateStatus(ctx, tx, orgID, id, status, version)
}

func (r *PostgresCaseRepository) updateStatus(ctx context.Context, e sqlExecer, orgID, id uuid.UUID, status domain.CaseStatus, version int) error {
	query := `
		UPDATE cases
		SET status = $1, version = version + 1, updated_at = now()
		WHERE organization_id = $2 AND id = $3 AND version = $4
	`
	result, err := e.ExecContext(ctx, query, status, orgID, id, version)
	if err != nil {
		return fmt.Errorf("failed to update case status: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("concurrent modification detected")
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
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("no rows affected")
	}
	return nil
}

func (r *PostgresCaseRepository) UpdateWorkflowInstanceID(ctx context.Context, orgID, caseID, instanceID uuid.UUID) error {
	return r.updateWorkflowInstanceID(ctx, r.db, orgID, caseID, instanceID)
}

func (r *PostgresCaseRepository) UpdateWorkflowInstanceIDTx(ctx context.Context, tx *sql.Tx, orgID, caseID, instanceID uuid.UUID) error {
	return r.updateWorkflowInstanceID(ctx, tx, orgID, caseID, instanceID)
}

func (r *PostgresCaseRepository) updateWorkflowInstanceID(ctx context.Context, e sqlExecer, orgID, caseID, instanceID uuid.UUID) error {
	query := `
		UPDATE cases
		SET workflow_instance_id = $1, updated_at = now()
		WHERE organization_id = $2 AND id = $3
	`
	result, err := e.ExecContext(ctx, query, instanceID, orgID, caseID)
	if err != nil {
		return fmt.Errorf("failed to update workflow instance id: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if affected == 0 {
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
		&c.CreatedAt, &c.UpdatedAt, &c.ClosedAt, &c.Version, &c.WorkflowInstanceID,
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
		&c.CreatedAt, &c.UpdatedAt, &c.ClosedAt, &c.Version, &c.WorkflowInstanceID,
	); err != nil {
		return nil, fmt.Errorf("failed to scan case: %w", err)
	}
	return &c, nil
}
