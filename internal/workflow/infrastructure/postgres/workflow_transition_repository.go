package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/alrazihi/civora/internal/workflow/domain"
	"github.com/google/uuid"
)

type PostgresWorkflowTransitionRepository struct {
	db *sql.DB
}

func NewPostgresWorkflowTransitionRepository(db *sql.DB) *PostgresWorkflowTransitionRepository {
	return &PostgresWorkflowTransitionRepository{db: db}
}

func (r *PostgresWorkflowTransitionRepository) DB() *sql.DB {
	return r.db
}

func (r *PostgresWorkflowTransitionRepository) SaveBatch(ctx context.Context, transitions []domain.WorkflowTransition) error {
	return r.saveBatch(ctx, r.db, transitions)
}

func (r *PostgresWorkflowTransitionRepository) SaveBatchTx(ctx context.Context, tx *sql.Tx, transitions []domain.WorkflowTransition) error {
	return r.saveBatch(ctx, tx, transitions)
}

func (r *PostgresWorkflowTransitionRepository) saveBatch(ctx context.Context, e interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}, transitions []domain.WorkflowTransition) error {
	if len(transitions) == 0 {
		return nil
	}

	query := `
		INSERT INTO workflow_transitions (
			id, workflow_definition_id, organization_id, key, name, from_state, to_state,
			description, conditions, allowed_roles, active, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	for _, t := range transitions {
		conditionsJSON, err := json.Marshal(t.Conditions)
		if err != nil {
			return fmt.Errorf("failed to marshal transition conditions: %w", err)
		}
		allowedRolesJSON, err := json.Marshal(t.AllowedRoles)
		if err != nil {
			return fmt.Errorf("failed to marshal transition allowed_roles: %w", err)
		}

		_, err = e.ExecContext(ctx, query,
			t.ID, t.WorkflowDefID, t.TenantID, t.Key, t.Name,
			t.FromState, t.ToState, t.Description, conditionsJSON,
			allowedRolesJSON, t.Active, t.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to insert workflow transition %s: %w", t.Key, err)
		}
	}
	return nil
}

func (r *PostgresWorkflowTransitionRepository) FindByDefinitionID(ctx context.Context, tenantID, defID uuid.UUID) ([]domain.WorkflowTransition, error) {
	query := `
		SELECT id, workflow_definition_id, organization_id, key, name, from_state, to_state,
			   description, conditions, allowed_roles, active, created_at
		FROM workflow_transitions
		WHERE workflow_definition_id = $1 AND organization_id = $2
		ORDER BY key
	`
	rows, err := r.db.QueryContext(ctx, query, defID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to query workflow transitions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var transitions []domain.WorkflowTransition
	for rows.Next() {
		var t domain.WorkflowTransition
		var conditionsJSON, allowedRolesJSON []byte
		if err := rows.Scan(
			&t.ID, &t.WorkflowDefID, &t.TenantID, &t.Key, &t.Name,
			&t.FromState, &t.ToState, &t.Description, &conditionsJSON,
			&allowedRolesJSON, &t.Active, &t.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan workflow transition: %w", err)
		}
		if len(conditionsJSON) > 0 {
			if err := json.Unmarshal(conditionsJSON, &t.Conditions); err != nil {
				return nil, fmt.Errorf("failed to unmarshal transition conditions: %w", err)
			}
		}
		if len(allowedRolesJSON) > 0 {
			if err := json.Unmarshal(allowedRolesJSON, &t.AllowedRoles); err != nil {
				return nil, fmt.Errorf("failed to unmarshal transition allowed_roles: %w", err)
			}
		}
		transitions = append(transitions, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return transitions, nil
}

func (r *PostgresWorkflowTransitionRepository) FindByFromState(ctx context.Context, tenantID uuid.UUID, defID uuid.UUID, fromState string) ([]domain.WorkflowTransition, error) {
	query := `
		SELECT id, workflow_definition_id, organization_id, key, name, from_state, to_state,
			   description, conditions, allowed_roles, active, created_at
		FROM workflow_transitions
		WHERE workflow_definition_id = $1 AND organization_id = $2 AND from_state = $3 AND active = true
	`
	rows, err := r.db.QueryContext(ctx, query, defID, tenantID, fromState)
	if err != nil {
		return nil, fmt.Errorf("failed to query workflow transitions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var transitions []domain.WorkflowTransition
	for rows.Next() {
		var t domain.WorkflowTransition
		var conditionsJSON, allowedRolesJSON []byte
		if err := rows.Scan(
			&t.ID, &t.WorkflowDefID, &t.TenantID, &t.Key, &t.Name,
			&t.FromState, &t.ToState, &t.Description, &conditionsJSON,
			&allowedRolesJSON, &t.Active, &t.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan workflow transition: %w", err)
		}
		if len(conditionsJSON) > 0 {
			if err := json.Unmarshal(conditionsJSON, &t.Conditions); err != nil {
				return nil, fmt.Errorf("failed to unmarshal transition conditions: %w", err)
			}
		}
		if len(allowedRolesJSON) > 0 {
			if err := json.Unmarshal(allowedRolesJSON, &t.AllowedRoles); err != nil {
				return nil, fmt.Errorf("failed to unmarshal transition allowed_roles: %w", err)
			}
		}
		transitions = append(transitions, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return transitions, nil
}
