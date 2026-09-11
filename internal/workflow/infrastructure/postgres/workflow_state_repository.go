package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/alrazihi/civora/internal/workflow/domain"
	"github.com/google/uuid"
)

type PostgresWorkflowStateRepository struct {
	db *sql.DB
}

func NewPostgresWorkflowStateRepository(db *sql.DB) *PostgresWorkflowStateRepository {
	return &PostgresWorkflowStateRepository{db: db}
}

func (r *PostgresWorkflowStateRepository) DB() *sql.DB {
	return r.db
}

func (r *PostgresWorkflowStateRepository) SaveBatch(ctx context.Context, states []domain.WorkflowState) error {
	return r.saveBatch(ctx, r.db, states)
}

func (r *PostgresWorkflowStateRepository) SaveBatchTx(ctx context.Context, tx *sql.Tx, states []domain.WorkflowState) error {
	return r.saveBatch(ctx, tx, states)
}

func (r *PostgresWorkflowStateRepository) saveBatch(ctx context.Context, e interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}, states []domain.WorkflowState) error {
	if len(states) == 0 {
		return nil
	}

	query := `
		INSERT INTO workflow_states (
			id, workflow_definition_id, organization_id, key, name, description,
			category, terminal, display_order, responsible_role, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	for _, state := range states {
		_, err := e.ExecContext(ctx, query,
			state.ID, state.WorkflowDefID, state.TenantID, state.Key, state.Name,
			state.Description, state.Category, state.Terminal, state.DisplayOrder,
			state.ResponsibleRole, state.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to insert workflow state %s: %w", state.Key, err)
		}
	}
	return nil
}

func (r *PostgresWorkflowStateRepository) DeleteBatchByDefinitionID(ctx context.Context, tenantID, defID uuid.UUID) error {
	return r.deleteBatchByDefinitionID(ctx, r.db, tenantID, defID)
}

func (r *PostgresWorkflowStateRepository) DeleteBatchByDefinitionIDTx(ctx context.Context, tx *sql.Tx, tenantID, defID uuid.UUID) error {
	return r.deleteBatchByDefinitionID(ctx, tx, tenantID, defID)
}

func (r *PostgresWorkflowStateRepository) deleteBatchByDefinitionID(ctx context.Context, e interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}, tenantID, defID uuid.UUID) error {
	query := `
		DELETE FROM workflow_states
		WHERE workflow_definition_id = $1 AND organization_id = $2
	`
	_, err := e.ExecContext(ctx, query, defID, tenantID)
	if err != nil {
		return fmt.Errorf("failed to delete workflow states: %w", err)
	}
	return nil
}

func (r *PostgresWorkflowStateRepository) FindByDefinitionID(ctx context.Context, tenantID, defID uuid.UUID) ([]domain.WorkflowState, error) {
	query := `
		SELECT id, workflow_definition_id, organization_id, key, name, description,
			   category, terminal, display_order, responsible_role, created_at
		FROM workflow_states
		WHERE workflow_definition_id = $1 AND organization_id = $2
		ORDER BY display_order, key
	`
	rows, err := r.db.QueryContext(ctx, query, defID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to query workflow states: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var states []domain.WorkflowState
	for rows.Next() {
		var state domain.WorkflowState
		if err := rows.Scan(
			&state.ID, &state.WorkflowDefID, &state.TenantID, &state.Key, &state.Name,
			&state.Description, &state.Category, &state.Terminal, &state.DisplayOrder,
			&state.ResponsibleRole, &state.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan workflow state: %w", err)
		}
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return states, nil
}
