package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/alrazihi/civora/internal/rules/domain"
	"github.com/google/uuid"
)

type PostgresWorkflowStateRuleAssignmentRepository struct {
	db *sql.DB
}

func NewPostgresWorkflowStateRuleAssignmentRepository(db *sql.DB) *PostgresWorkflowStateRuleAssignmentRepository {
	return &PostgresWorkflowStateRuleAssignmentRepository{db: db}
}

func (r *PostgresWorkflowStateRuleAssignmentRepository) DB() *sql.DB {
	return r.db
}

func (r *PostgresWorkflowStateRuleAssignmentRepository) Save(ctx context.Context, assignment *domain.WorkflowStateRuleAssignment) error {
	return r.SaveTx(ctx, nil, assignment)
}

func (r *PostgresWorkflowStateRuleAssignmentRepository) SaveTx(ctx context.Context, tx *sql.Tx, assignment *domain.WorkflowStateRuleAssignment) error {
	if err := assignment.Validate(); err != nil {
		return fmt.Errorf("invalid assignment: %w", err)
	}
	now := time.Now().UTC()
	assignment.CreatedAt = now
	assignment.UpdatedAt = now

	query := `INSERT INTO rules.workflow_state_rule_assignments
		(id, organization_id, workflow_definition_id, workflow_state_key, rule_set_id, required, active, display_order, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	var execFn func(context.Context, ...interface{}) (sql.Result, error)
	if tx != nil {
		execFn = func(ctx context.Context, args ...interface{}) (sql.Result, error) {
			return tx.ExecContext(ctx, query, args...)
		}
	} else {
		execFn = func(ctx context.Context, args ...interface{}) (sql.Result, error) {
			return r.db.ExecContext(ctx, query, args...)
		}
	}

	_, err := execFn(ctx,
		assignment.ID, assignment.OrganizationID, assignment.WorkflowDefID,
		assignment.WorkflowStateKey, assignment.RuleSetID, assignment.Required,
		assignment.Active, assignment.DisplayOrder, assignment.CreatedAt, assignment.UpdatedAt,
	)
	return err
}

func (r *PostgresWorkflowStateRuleAssignmentRepository) FindByID(ctx context.Context, orgID, id uuid.UUID) (*domain.WorkflowStateRuleAssignment, error) {
	query := `SELECT id, organization_id, workflow_definition_id, workflow_state_key, rule_set_id, required, active, display_order, created_at, updated_at
		FROM rules.workflow_state_rule_assignments
		WHERE id = $1 AND organization_id = $2`
	row := r.db.QueryRowContext(ctx, query, id, orgID)
	return scanRuleAssignment(row)
}

func (r *PostgresWorkflowStateRuleAssignmentRepository) FindByWorkflowAndState(ctx context.Context, orgID uuid.UUID, workflowDefID uuid.UUID, stateKey string) ([]*domain.WorkflowStateRuleAssignment, error) {
	return r.findByWorkflowAndState(ctx, nil, orgID, workflowDefID, stateKey)
}

func (r *PostgresWorkflowStateRuleAssignmentRepository) FindByWorkflowAndStateTx(ctx context.Context, tx *sql.Tx, orgID uuid.UUID, workflowDefID uuid.UUID, stateKey string) ([]*domain.WorkflowStateRuleAssignment, error) {
	return r.findByWorkflowAndState(ctx, tx, orgID, workflowDefID, stateKey)
}

func (r *PostgresWorkflowStateRuleAssignmentRepository) findByWorkflowAndState(ctx context.Context, tx *sql.Tx, orgID uuid.UUID, workflowDefID uuid.UUID, stateKey string) ([]*domain.WorkflowStateRuleAssignment, error) {
	query := `SELECT id, organization_id, workflow_definition_id, workflow_state_key, rule_set_id, required, active, display_order, created_at, updated_at
		FROM rules.workflow_state_rule_assignments
		WHERE organization_id = $1 AND workflow_definition_id = $2 AND workflow_state_key = $3 AND active = true
		ORDER BY display_order`
	rows, err := r.query(ctx, tx, query, orgID, workflowDefID, stateKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRuleAssignmentRows(rows)
}

func (r *PostgresWorkflowStateRuleAssignmentRepository) ListByRuleSet(ctx context.Context, orgID, ruleSetID uuid.UUID) ([]*domain.WorkflowStateRuleAssignment, error) {
	query := `SELECT id, organization_id, workflow_definition_id, workflow_state_key, rule_set_id, required, active, display_order, created_at, updated_at
		FROM rules.workflow_state_rule_assignments
		WHERE organization_id = $1 AND rule_set_id = $2`
	rows, err := r.db.QueryContext(ctx, query, orgID, ruleSetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRuleAssignmentRows(rows)
}

func (r *PostgresWorkflowStateRuleAssignmentRepository) ListByWorkflow(ctx context.Context, orgID, workflowDefID uuid.UUID) ([]*domain.WorkflowStateRuleAssignment, error) {
	query := `SELECT id, organization_id, workflow_definition_id, workflow_state_key, rule_set_id, required, active, display_order, created_at, updated_at
		FROM rules.workflow_state_rule_assignments
		WHERE organization_id = $1 AND workflow_definition_id = $2
		ORDER BY workflow_state_key, display_order`
	rows, err := r.db.QueryContext(ctx, query, orgID, workflowDefID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRuleAssignmentRows(rows)
}

func (r *PostgresWorkflowStateRuleAssignmentRepository) UpdateTx(ctx context.Context, tx *sql.Tx, assignment *domain.WorkflowStateRuleAssignment) error {
	assignment.UpdatedAt = time.Now().UTC()
	query := `UPDATE rules.workflow_state_rule_assignments
		SET rule_set_id = $1, required = $2, active = $3, display_order = $4, updated_at = $5
		WHERE id = $6 AND organization_id = $7`
	_, err := tx.ExecContext(ctx, query,
		assignment.RuleSetID, assignment.Required, assignment.Active,
		assignment.DisplayOrder, assignment.UpdatedAt, assignment.ID, assignment.OrganizationID,
	)
	return err
}

func (r *PostgresWorkflowStateRuleAssignmentRepository) DeleteTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM rules.workflow_state_rule_assignments WHERE id = $1 AND organization_id = $2`, id, orgID)
	return err
}

func scanRuleAssignment(row sqlRowScanner) (*domain.WorkflowStateRuleAssignment, error) {
	var a domain.WorkflowStateRuleAssignment
	var createdAt, updatedAt time.Time
	err := row.Scan(
		&a.ID, &a.OrganizationID, &a.WorkflowDefID, &a.WorkflowStateKey,
		&a.RuleSetID, &a.Required, &a.Active, &a.DisplayOrder,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	a.CreatedAt = createdAt
	a.UpdatedAt = updatedAt
	return &a, nil
}

func scanRuleAssignmentRows(rows *sql.Rows) ([]*domain.WorkflowStateRuleAssignment, error) {
	var assignments []*domain.WorkflowStateRuleAssignment
	for rows.Next() {
		var a domain.WorkflowStateRuleAssignment
		var createdAt, updatedAt time.Time
		if err := rows.Scan(
			&a.ID, &a.OrganizationID, &a.WorkflowDefID, &a.WorkflowStateKey,
			&a.RuleSetID, &a.Required, &a.Active, &a.DisplayOrder,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}
		a.CreatedAt = createdAt
		a.UpdatedAt = updatedAt
		assignments = append(assignments, &a)
	}
	return assignments, rows.Close()
}

func (r *PostgresWorkflowStateRuleAssignmentRepository) query(ctx context.Context, tx *sql.Tx, query string, args ...interface{}) (*sql.Rows, error) {
	if tx != nil {
		return tx.QueryContext(ctx, query, args...)
	}
	return r.db.QueryContext(ctx, query, args...)
}

type sqlRowScanner interface {
	Scan(dest ...interface{}) error
}
