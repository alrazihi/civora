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

type PostgresWorkflowDefinitionRepository struct {
	db *sql.DB
}

func NewPostgresWorkflowDefinitionRepository(db *sql.DB) *PostgresWorkflowDefinitionRepository {
	return &PostgresWorkflowDefinitionRepository{db: db}
}

func (r *PostgresWorkflowDefinitionRepository) DB() *sql.DB {
	return r.db
}

func (r *PostgresWorkflowDefinitionRepository) Save(ctx context.Context, def *domain.WorkflowDefinition) error {
	return r.saveDefinition(ctx, r.db, def)
}

func (r *PostgresWorkflowDefinitionRepository) SaveTx(ctx context.Context, tx *sql.Tx, def *domain.WorkflowDefinition) error {
	return r.saveDefinition(ctx, tx, def)
}

func (r *PostgresWorkflowDefinitionRepository) saveDefinition(ctx context.Context, e interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}, def *domain.WorkflowDefinition) error {
	query := `
		INSERT INTO workflow_definitions (
			id, organization_id, key, name, description, version, status, initial_state, metadata, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	metadataJSON, err := json.Marshal(def.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow definition metadata: %w", err)
	}

	_, err = e.ExecContext(ctx, query,
		def.ID, def.TenantID, def.Key, def.Name, def.Description,
		def.Version, string(def.Status), def.InitialState, metadataJSON,
		def.CreatedAt, def.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert workflow definition: %w", err)
	}
	return nil
}

func (r *PostgresWorkflowDefinitionRepository) FindByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.WorkflowDefinition, error) {
	query := `
		SELECT id, organization_id, key, name, description, version, status, initial_state, metadata, created_at, updated_at
		FROM workflow_definitions
		WHERE organization_id = $1 AND id = $2
	`
	return r.scanDefinition(r.db.QueryRowContext(ctx, query, tenantID, id))
}

func (r *PostgresWorkflowDefinitionRepository) FindByKeyAndVersion(ctx context.Context, tenantID uuid.UUID, key string, version int) (*domain.WorkflowDefinition, error) {
	query := `
		SELECT id, organization_id, key, name, description, version, status, initial_state, metadata, created_at, updated_at
		FROM workflow_definitions
		WHERE organization_id = $1 AND key = $2 AND version = $3
	`
	return r.scanDefinition(r.db.QueryRowContext(ctx, query, tenantID, key, version))
}

func (r *PostgresWorkflowDefinitionRepository) FindLatestActiveByKey(ctx context.Context, tenantID uuid.UUID, key string) (*domain.WorkflowDefinition, error) {
	query := `
		SELECT id, organization_id, key, name, description, version, status, initial_state, metadata, created_at, updated_at
		FROM workflow_definitions
		WHERE organization_id = $1 AND key = $2 AND status = 'ACTIVE'
		ORDER BY version DESC
		LIMIT 1
	`
	return r.scanDefinition(r.db.QueryRowContext(ctx, query, tenantID, key))
}

func (r *PostgresWorkflowDefinitionRepository) ListByOrganization(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*domain.WorkflowDefinition, int, error) {
	query := `
		SELECT id, organization_id, key, name, description, version, status, initial_state, metadata, created_at, updated_at
		FROM workflow_definitions
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, tenantID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query workflow definitions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var definitions []*domain.WorkflowDefinition
	for rows.Next() {
		def, err := r.scanDefinitionFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		definitions = append(definitions, def)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration error: %w", err)
	}

	var total int
	err = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM workflow_definitions WHERE organization_id = $1", tenantID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count workflow definitions: %w", err)
	}

	return definitions, total, nil
}

func (r *PostgresWorkflowDefinitionRepository) UpdateStatus(ctx context.Context, tenantID, id uuid.UUID, status domain.WorkflowDefinitionStatus, version int) error {
	return r.updateStatus(ctx, r.db, tenantID, id, status, version)
}

func (r *PostgresWorkflowDefinitionRepository) UpdateStatusTx(ctx context.Context, tx *sql.Tx, tenantID, id uuid.UUID, status domain.WorkflowDefinitionStatus, version int) error {
	return r.updateStatus(ctx, tx, tenantID, id, status, version)
}

func (r *PostgresWorkflowDefinitionRepository) updateStatus(ctx context.Context, e interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}, tenantID uuid.UUID, id uuid.UUID, status domain.WorkflowDefinitionStatus, version int) error {
	query := `
		UPDATE workflow_definitions
		SET status = $1, updated_at = now()
		WHERE organization_id = $2 AND id = $3 AND version = $4
	`
	result, err := e.ExecContext(ctx, query, string(status), tenantID, id, version)
	if err != nil {
		return fmt.Errorf("failed to update workflow definition status: %w", err)
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

func (r *PostgresWorkflowDefinitionRepository) scanDefinition(row interface {
	Scan(dest ...interface{}) error
}) (*domain.WorkflowDefinition, error) {
	var def domain.WorkflowDefinition
	var metadataJSON []byte
	var createdAt, updatedAt time.Time

	if err := row.Scan(
		&def.ID, &def.TenantID, &def.Key, &def.Name, &def.Description,
		&def.Version, &def.Status, &def.InitialState, &metadataJSON,
		&createdAt, &updatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan workflow definition: %w", err)
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &def.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal workflow definition metadata: %w", err)
		}
	} else {
		def.Metadata = map[string]interface{}{}
	}

	def.CreatedAt = createdAt
	def.UpdatedAt = updatedAt
	return &def, nil
}

func (r *PostgresWorkflowDefinitionRepository) scanDefinitionFromRows(rows *sql.Rows) (*domain.WorkflowDefinition, error) {
	var def domain.WorkflowDefinition
	var metadataJSON []byte
	var createdAt, updatedAt time.Time

	if err := rows.Scan(
		&def.ID, &def.TenantID, &def.Key, &def.Name, &def.Description,
		&def.Version, &def.Status, &def.InitialState, &metadataJSON,
		&createdAt, &updatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan workflow definition: %w", err)
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &def.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal workflow definition metadata: %w", err)
		}
	} else {
		def.Metadata = map[string]interface{}{}
	}

	def.CreatedAt = createdAt
	def.UpdatedAt = updatedAt
	return &def, nil
}
