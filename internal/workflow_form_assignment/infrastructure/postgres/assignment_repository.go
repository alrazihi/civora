package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	assignmentdomain "github.com/alrazihi/civora/internal/workflow_form_assignment/domain"
	"github.com/google/uuid"
)

type PostgresWorkflowStateFormAssignmentRepository struct {
	db *sql.DB
}

func NewPostgresWorkflowStateFormAssignmentRepository(db *sql.DB) *PostgresWorkflowStateFormAssignmentRepository {
	return &PostgresWorkflowStateFormAssignmentRepository{db: db}
}

func (r *PostgresWorkflowStateFormAssignmentRepository) DB() *sql.DB {
	return r.db
}

func (r *PostgresWorkflowStateFormAssignmentRepository) SaveTx(ctx context.Context, tx *sql.Tx, assignment *assignmentdomain.WorkflowStateFormAssignment) error {
	query := `
		INSERT INTO workflow_state_form_assignments
			(id, tenant_id, workflow_definition_id, workflow_state_key, form_id, form_version_id, required, display_order, active, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err := tx.ExecContext(ctx, query,
		assignment.ID,
		assignment.TenantID,
		assignment.WorkflowDefinitionID,
		assignment.WorkflowStateKey,
		assignment.FormID,
		assignment.FormVersionID,
		assignment.Required,
		assignment.DisplayOrder,
		assignment.Active,
		assignment.CreatedBy,
		assignment.CreatedAt,
		assignment.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save workflow state form assignment: %w", err)
	}
	return nil
}

func (r *PostgresWorkflowStateFormAssignmentRepository) Save(ctx context.Context, assignment *assignmentdomain.WorkflowStateFormAssignment) error {
	return r.SaveTx(ctx, nil, assignment)
}

func (r *PostgresWorkflowStateFormAssignmentRepository) FindByIDTx(ctx context.Context, tx *sql.Tx, tenantID, id uuid.UUID) (*assignmentdomain.WorkflowStateFormAssignment, error) {
	query := `
		SELECT id, tenant_id, workflow_definition_id, workflow_state_key, form_id, form_version_id, required, display_order, active, created_by, created_at, updated_at
		FROM workflow_state_form_assignments
		WHERE tenant_id = $1 AND id = $2
	`
	row := r.queryRow(ctx, tx, query, tenantID, id)
	return r.scanAssignment(row)
}

func (r *PostgresWorkflowStateFormAssignmentRepository) FindByID(ctx context.Context, tenantID, id uuid.UUID) (*assignmentdomain.WorkflowStateFormAssignment, error) {
	return r.FindByIDTx(ctx, nil, tenantID, id)
}

func (r *PostgresWorkflowStateFormAssignmentRepository) FindByWorkflowAndStateTx(ctx context.Context, tx *sql.Tx, tenantID, workflowDefID uuid.UUID, stateKey string) ([]*assignmentdomain.WorkflowStateFormAssignment, error) {
	query := `
		SELECT id, tenant_id, workflow_definition_id, workflow_state_key, form_id, form_version_id, required, display_order, active, created_by, created_at, updated_at
		FROM workflow_state_form_assignments
		WHERE tenant_id = $1 AND workflow_definition_id = $2 AND workflow_state_key = $3
		ORDER BY display_order ASC
	`
	rows, err := r.query(ctx, tx, query, tenantID, workflowDefID, stateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to query assignments by workflow and state: %w", err)
	}
	defer rows.Close()

	var assignments []*assignmentdomain.WorkflowStateFormAssignment
	for rows.Next() {
		assignment, err := r.scanAssignmentFromRows(rows)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, assignment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating assignment rows: %w", err)
	}
	return assignments, nil
}

func (r *PostgresWorkflowStateFormAssignmentRepository) FindByWorkflowAndState(ctx context.Context, tenantID, workflowDefID uuid.UUID, stateKey string) ([]*assignmentdomain.WorkflowStateFormAssignment, error) {
	return r.FindByWorkflowAndStateTx(ctx, nil, tenantID, workflowDefID, stateKey)
}

func (r *PostgresWorkflowStateFormAssignmentRepository) ListByWorkflow(ctx context.Context, tenantID, workflowDefID uuid.UUID) ([]*assignmentdomain.WorkflowStateFormAssignment, error) {
	query := `
		SELECT id, tenant_id, workflow_definition_id, workflow_state_key, form_id, form_version_id, required, display_order, active, created_by, created_at, updated_at
		FROM workflow_state_form_assignments
		WHERE tenant_id = $1 AND workflow_definition_id = $2
		ORDER BY workflow_state_key ASC, display_order ASC
	`
	rows, err := r.query(ctx, nil, query, tenantID, workflowDefID)
	if err != nil {
		return nil, fmt.Errorf("failed to list assignments by workflow: %w", err)
	}
	defer rows.Close()

	var assignments []*assignmentdomain.WorkflowStateFormAssignment
	for rows.Next() {
		assignment, err := r.scanAssignmentFromRows(rows)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, assignment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating assignment rows: %w", err)
	}
	return assignments, nil
}

func (r *PostgresWorkflowStateFormAssignmentRepository) UpdateTx(ctx context.Context, tx *sql.Tx, assignment *assignmentdomain.WorkflowStateFormAssignment) error {
	query := `
		UPDATE workflow_state_form_assignments
		SET required = $3, display_order = $4, active = $5, updated_at = $6
		WHERE tenant_id = $1 AND id = $2
	`
	_, err := tx.ExecContext(ctx, query,
		assignment.TenantID,
		assignment.ID,
		assignment.Required,
		assignment.DisplayOrder,
		assignment.Active,
		assignment.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to update workflow state form assignment: %w", err)
	}
	return nil
}

func (r *PostgresWorkflowStateFormAssignmentRepository) Update(ctx context.Context, assignment *assignmentdomain.WorkflowStateFormAssignment) error {
	return r.UpdateTx(ctx, nil, assignment)
}

func (r *PostgresWorkflowStateFormAssignmentRepository) DeleteTx(ctx context.Context, tx *sql.Tx, tenantID, id uuid.UUID) error {
	query := `DELETE FROM workflow_state_form_assignments WHERE tenant_id = $1 AND id = $2`
	_, err := tx.ExecContext(ctx, query, tenantID, id)
	if err != nil {
		return fmt.Errorf("failed to delete workflow state form assignment: %w", err)
	}
	return nil
}

func (r *PostgresWorkflowStateFormAssignmentRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return r.DeleteTx(ctx, nil, tenantID, id)
}

func (r *PostgresWorkflowStateFormAssignmentRepository) DeleteByWorkflowAndStateTx(ctx context.Context, tx *sql.Tx, tenantID, workflowDefID uuid.UUID, stateKey string) error {
	query := `DELETE FROM workflow_state_form_assignments WHERE tenant_id = $1 AND workflow_definition_id = $2 AND workflow_state_key = $3`
	_, err := tx.ExecContext(ctx, query, tenantID, workflowDefID, stateKey)
	if err != nil {
		return fmt.Errorf("failed to delete assignments by workflow and state: %w", err)
	}
	return nil
}

func (r *PostgresWorkflowStateFormAssignmentRepository) DeleteByWorkflowAndState(ctx context.Context, tenantID, workflowDefID uuid.UUID, stateKey string) error {
	return r.DeleteByWorkflowAndStateTx(ctx, nil, tenantID, workflowDefID, stateKey)
}

func (r *PostgresWorkflowStateFormAssignmentRepository) queryRow(ctx context.Context, tx *sql.Tx, query string, args ...interface{}) *sql.Row {
	if tx != nil {
		return tx.QueryRowContext(ctx, query, args...)
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *PostgresWorkflowStateFormAssignmentRepository) query(ctx context.Context, tx *sql.Tx, query string, args ...interface{}) (*sql.Rows, error) {
	if tx != nil {
		return tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r *PostgresWorkflowStateFormAssignmentRepository) scanAssignment(row interface{ Scan(...interface{}) error }) (*assignmentdomain.WorkflowStateFormAssignment, error) {
	var a assignmentdomain.WorkflowStateFormAssignment
	var createdAt, updatedAt time.Time

	err := row.Scan(
		&a.ID,
		&a.TenantID,
		&a.WorkflowDefinitionID,
		&a.WorkflowStateKey,
		&a.FormID,
		&a.FormVersionID,
		&a.Required,
		&a.DisplayOrder,
		&a.Active,
		&a.CreatedBy,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, assignmentdomain.ErrAssignmentNotFound
		}
		return nil, fmt.Errorf("failed to scan assignment: %w", err)
	}

	a.CreatedAt = createdAt.UTC()
	a.UpdatedAt = updatedAt.UTC()
	return &a, nil
}

func (r *PostgresWorkflowStateFormAssignmentRepository) scanAssignmentFromRows(rows *sql.Rows) (*assignmentdomain.WorkflowStateFormAssignment, error) {
	var a assignmentdomain.WorkflowStateFormAssignment
	var createdAt, updatedAt time.Time

	err := rows.Scan(
		&a.ID,
		&a.TenantID,
		&a.WorkflowDefinitionID,
		&a.WorkflowStateKey,
		&a.FormID,
		&a.FormVersionID,
		&a.Required,
		&a.DisplayOrder,
		&a.Active,
		&a.CreatedBy,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan assignment: %w", err)
	}

	a.CreatedAt = createdAt.UTC()
	a.UpdatedAt = updatedAt.UTC()
	return &a, nil
}
