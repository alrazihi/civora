package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/alrazihi/civora/internal/workflow/domain"
	"github.com/google/uuid"
)

type PostgresWorkflowTransitionHistoryRepository struct {
	db *sql.DB
}

func NewPostgresWorkflowTransitionHistoryRepository(db *sql.DB) *PostgresWorkflowTransitionHistoryRepository {
	return &PostgresWorkflowTransitionHistoryRepository{db: db}
}

func (r *PostgresWorkflowTransitionHistoryRepository) DB() *sql.DB {
	return r.db
}

func (r *PostgresWorkflowTransitionHistoryRepository) Save(ctx context.Context, history *domain.WorkflowTransitionHistory) error {
	return r.saveHistory(ctx, r.db, history)
}

func (r *PostgresWorkflowTransitionHistoryRepository) SaveTx(ctx context.Context, tx *sql.Tx, history *domain.WorkflowTransitionHistory) error {
	return r.saveHistory(ctx, tx, history)
}

func (r *PostgresWorkflowTransitionHistoryRepository) saveHistory(ctx context.Context, e interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}, history *domain.WorkflowTransitionHistory) error {
	query := `
		INSERT INTO workflow_transition_history (
			id, organization_id, workflow_instance_id, case_id, from_state, to_state,
			transition_key, actor_id, occurred_at, reason, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	metadataJSON, err := json.Marshal(history.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow transition history metadata: %w", err)
	}

	_, err = e.ExecContext(ctx, query,
		history.ID, history.TenantID, history.WorkflowInstanceID, history.CaseID,
		history.FromState, history.ToState, history.TransitionKey, history.ActorID,
		history.OccurredAt, history.Reason, metadataJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to insert workflow transition history: %w", err)
	}
	return nil
}

func (r *PostgresWorkflowTransitionHistoryRepository) FindByInstanceID(ctx context.Context, tenantID, instanceID uuid.UUID, limit, offset int) ([]domain.WorkflowTransitionHistory, error) {
	query := `
		SELECT id, organization_id, workflow_instance_id, case_id, from_state, to_state,
			   transition_key, actor_id, occurred_at, reason, metadata
		FROM workflow_transition_history
		WHERE workflow_instance_id = $1 AND organization_id = $2
		ORDER BY occurred_at ASC, id ASC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.db.QueryContext(ctx, query, instanceID, tenantID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query workflow transition history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var histories []domain.WorkflowTransitionHistory
	for rows.Next() {
		history, err := r.scanHistoryFromRows(rows)
		if err != nil {
			return nil, err
		}
		histories = append(histories, history)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return histories, nil
}

func (r *PostgresWorkflowTransitionHistoryRepository) FindByCaseID(ctx context.Context, tenantID, caseID uuid.UUID, limit, offset int) ([]domain.WorkflowTransitionHistory, error) {
	query := `
		SELECT id, organization_id, workflow_instance_id, case_id, from_state, to_state,
			   transition_key, actor_id, occurred_at, reason, metadata
		FROM workflow_transition_history
		WHERE case_id = $1 AND organization_id = $2
		ORDER BY occurred_at ASC, id ASC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.db.QueryContext(ctx, query, caseID, tenantID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query workflow transition history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var histories []domain.WorkflowTransitionHistory
	for rows.Next() {
		history, err := r.scanHistoryFromRows(rows)
		if err != nil {
			return nil, err
		}
		histories = append(histories, history)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return histories, nil
}

func (r *PostgresWorkflowTransitionHistoryRepository) scanHistoryFromRows(rows *sql.Rows) (domain.WorkflowTransitionHistory, error) {
	var history domain.WorkflowTransitionHistory
	var metadataJSON []byte

	if err := rows.Scan(
		&history.ID, &history.TenantID, &history.WorkflowInstanceID, &history.CaseID,
		&history.FromState, &history.ToState, &history.TransitionKey, &history.ActorID,
		&history.OccurredAt, &history.Reason, &metadataJSON,
	); err != nil {
		return domain.WorkflowTransitionHistory{}, fmt.Errorf("failed to scan workflow transition history: %w", err)
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &history.Metadata); err != nil {
			return domain.WorkflowTransitionHistory{}, fmt.Errorf("failed to unmarshal workflow transition history metadata: %w", err)
		}
	} else {
		history.Metadata = map[string]interface{}{}
	}

	return history, nil
}
