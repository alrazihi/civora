package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/alrazihi/civora/internal/workflow/domain"
	"github.com/google/uuid"
)

type PostgresWorkflowInstanceRepository struct {
	db *sql.DB
}

func NewPostgresWorkflowInstanceRepository(db *sql.DB) *PostgresWorkflowInstanceRepository {
	return &PostgresWorkflowInstanceRepository{db: db}
}

func (r *PostgresWorkflowInstanceRepository) DB() *sql.DB {
	return r.db
}

func (r *PostgresWorkflowInstanceRepository) Save(ctx context.Context, instance *domain.WorkflowInstance) error {
	return r.saveInstance(ctx, r.db, instance)
}

func (r *PostgresWorkflowInstanceRepository) SaveTx(ctx context.Context, tx *sql.Tx, instance *domain.WorkflowInstance) error {
	return r.saveInstance(ctx, tx, instance)
}

func (r *PostgresWorkflowInstanceRepository) saveInstance(ctx context.Context, e interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}, instance *domain.WorkflowInstance) error {
	query := `
		INSERT INTO workflow_instances (
			id, organization_id, workflow_definition_id, workflow_definition_version,
			case_id, current_state, started_at, completed_at, metadata, version
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	metadataJSON, err := json.Marshal(instance.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow instance metadata: %w", err)
	}

	_, err = e.ExecContext(ctx, query,
		instance.ID, instance.TenantID, instance.WorkflowDefID, instance.WorkflowDefVersion,
		instance.CaseID, instance.CurrentState, instance.StartedAt,
		instance.CompletedAt, metadataJSON, 1,
	)
	if err != nil {
		return fmt.Errorf("failed to insert workflow instance: %w", err)
	}
	return nil
}

func (r *PostgresWorkflowInstanceRepository) FindByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.WorkflowInstance, error) {
	query := `
		SELECT id, organization_id, workflow_definition_id, workflow_definition_version,
			   case_id, current_state, started_at, completed_at, metadata, version
		FROM workflow_instances
		WHERE organization_id = $1 AND id = $2
	`
	return r.scanInstance(r.db.QueryRowContext(ctx, query, tenantID, id))
}

func (r *PostgresWorkflowInstanceRepository) FindByCaseID(ctx context.Context, tenantID, caseID uuid.UUID) (*domain.WorkflowInstance, error) {
	query := `
		SELECT id, organization_id, workflow_definition_id, workflow_definition_version,
			   case_id, current_state, started_at, completed_at, metadata, version
		FROM workflow_instances
		WHERE organization_id = $1 AND case_id = $2
	`
	return r.scanInstance(r.db.QueryRowContext(ctx, query, tenantID, caseID))
}

func (r *PostgresWorkflowInstanceRepository) FindByDefinitionID(ctx context.Context, tenantID, defID uuid.UUID, limit, offset int) ([]*domain.WorkflowInstance, int, error) {
	query := `
		SELECT id, organization_id, workflow_definition_id, workflow_definition_version,
			   case_id, current_state, started_at, completed_at, metadata, version
		FROM workflow_instances
		WHERE organization_id = $1 AND workflow_definition_id = $2
		ORDER BY started_at DESC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.db.QueryContext(ctx, query, tenantID, defID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query workflow instances: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var instances []*domain.WorkflowInstance
	for rows.Next() {
		instance, err := r.scanInstanceFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		instances = append(instances, instance)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration error: %w", err)
	}

	var total int
	err = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM workflow_instances WHERE organization_id = $1 AND workflow_definition_id = $2", tenantID, defID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count workflow instances: %w", err)
	}

	return instances, total, nil
}

func (r *PostgresWorkflowInstanceRepository) UpdateState(ctx context.Context, tenantID, id uuid.UUID, state string, version int) error {
	return r.updateState(ctx, r.db, tenantID, id, state, version)
}

func (r *PostgresWorkflowInstanceRepository) UpdateStateTx(ctx context.Context, tx *sql.Tx, tenantID, id uuid.UUID, state string, version int) error {
	return r.updateState(ctx, tx, tenantID, id, state, version)
}

func (r *PostgresWorkflowInstanceRepository) updateState(ctx context.Context, e interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}, tenantID uuid.UUID, id uuid.UUID, state string, version int) error {
	query := `
		UPDATE workflow_instances
		SET current_state = $1, version = version + 1
		WHERE organization_id = $2 AND id = $3 AND version = $4
	`
	result, err := e.ExecContext(ctx, query, state, tenantID, id, version)
	if err != nil {
		return fmt.Errorf("failed to update workflow instance state: %w", err)
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

func (r *PostgresWorkflowInstanceRepository) Complete(ctx context.Context, tenantID, id uuid.UUID, version int) error {
	return r.complete(ctx, r.db, tenantID, id, version)
}

func (r *PostgresWorkflowInstanceRepository) CompleteTx(ctx context.Context, tx *sql.Tx, tenantID, id uuid.UUID, version int) error {
	return r.complete(ctx, tx, tenantID, id, version)
}

func (r *PostgresWorkflowInstanceRepository) complete(ctx context.Context, e interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}, tenantID uuid.UUID, id uuid.UUID, version int) error {
	query := `
		UPDATE workflow_instances
		SET completed_at = now(), version = version + 1
		WHERE organization_id = $1 AND id = $2 AND version = $3
	`
	result, err := e.ExecContext(ctx, query, tenantID, id, version)
	if err != nil {
		return fmt.Errorf("failed to complete workflow instance: %w", err)
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

func (r *PostgresWorkflowInstanceRepository) scanInstance(row interface {
	Scan(dest ...interface{}) error
}) (*domain.WorkflowInstance, error) {
	var instance domain.WorkflowInstance
	var startedAt time.Time
	var completedAt *time.Time
	var metadataJSON []byte

	if err := row.Scan(
		&instance.ID, &instance.TenantID, &instance.WorkflowDefID, &instance.WorkflowDefVersion,
		&instance.CaseID, &instance.CurrentState, &startedAt, &completedAt,
		&metadataJSON, &instance.Version,
	); err != nil {
		return nil, fmt.Errorf("failed to scan workflow instance: %w", err)
	}

	instance.StartedAt = startedAt
	if completedAt != nil {
		instance.CompletedAt = completedAt
	}
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &instance.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal workflow instance metadata: %w", err)
		}
	} else {
		instance.Metadata = map[string]interface{}{}
	}

	return &instance, nil
}

func (r *PostgresWorkflowInstanceRepository) scanInstanceFromRows(rows *sql.Rows) (*domain.WorkflowInstance, error) {
	var instance domain.WorkflowInstance
	var startedAt time.Time
	var completedAt *time.Time
	var metadataJSON []byte

	if err := rows.Scan(
		&instance.ID, &instance.TenantID, &instance.WorkflowDefID, &instance.WorkflowDefVersion,
		&instance.CaseID, &instance.CurrentState, &startedAt, &completedAt,
		&metadataJSON, &instance.Version,
	); err != nil {
		return nil, fmt.Errorf("failed to scan workflow instance: %w", err)
	}

	instance.StartedAt = startedAt
	if completedAt != nil {
		instance.CompletedAt = completedAt
	}
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &instance.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal workflow instance metadata: %w", err)
		}
	} else {
		instance.Metadata = map[string]interface{}{}
	}

	return &instance, nil
}
