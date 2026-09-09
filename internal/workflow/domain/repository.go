package domain

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

type WorkflowDefinitionRepository interface {
	DB() *sql.DB
	Save(ctx context.Context, def *WorkflowDefinition) error
	SaveTx(ctx context.Context, tx *sql.Tx, def *WorkflowDefinition) error
	FindByID(ctx context.Context, tenantID, id uuid.UUID) (*WorkflowDefinition, error)
	FindByKeyAndVersion(ctx context.Context, tenantID uuid.UUID, key string, version int) (*WorkflowDefinition, error)
	FindLatestActiveByKey(ctx context.Context, tenantID uuid.UUID, key string) (*WorkflowDefinition, error)
	ListByOrganization(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*WorkflowDefinition, int, error)
	UpdateStatus(ctx context.Context, tenantID, id uuid.UUID, status WorkflowDefinitionStatus, version int) error
	UpdateStatusTx(ctx context.Context, tx *sql.Tx, tenantID, id uuid.UUID, status WorkflowDefinitionStatus, version int) error
}

type WorkflowStateRepository interface {
	DB() *sql.DB
	SaveBatch(ctx context.Context, states []WorkflowState) error
	SaveBatchTx(ctx context.Context, tx *sql.Tx, states []WorkflowState) error
	FindByDefinitionID(ctx context.Context, tenantID uuid.UUID, defID uuid.UUID) ([]WorkflowState, error)
}

type WorkflowTransitionRepository interface {
	DB() *sql.DB
	SaveBatch(ctx context.Context, transitions []WorkflowTransition) error
	SaveBatchTx(ctx context.Context, tx *sql.Tx, transitions []WorkflowTransition) error
	FindByDefinitionID(ctx context.Context, tenantID, defID uuid.UUID) ([]WorkflowTransition, error)
	FindByFromState(ctx context.Context, tenantID uuid.UUID, defID uuid.UUID, fromState string) ([]WorkflowTransition, error)
}

type WorkflowInstanceRepository interface {
	DB() *sql.DB
	Save(ctx context.Context, instance *WorkflowInstance) error
	SaveTx(ctx context.Context, tx *sql.Tx, instance *WorkflowInstance) error
	FindByID(ctx context.Context, tenantID, id uuid.UUID) (*WorkflowInstance, error)
	FindByCaseID(ctx context.Context, tenantID, caseID uuid.UUID) (*WorkflowInstance, error)
	FindByDefinitionID(ctx context.Context, tenantID, defID uuid.UUID, limit, offset int) ([]*WorkflowInstance, int, error)
	UpdateState(ctx context.Context, tenantID, id uuid.UUID, state string, version int) error
	UpdateStateTx(ctx context.Context, tx *sql.Tx, tenantID, id uuid.UUID, state string, version int) error
	Complete(ctx context.Context, tenantID, id uuid.UUID, version int) error
	CompleteTx(ctx context.Context, tx *sql.Tx, tenantID, id uuid.UUID, version int) error
}

type WorkflowTransitionHistoryRepository interface {
	DB() *sql.DB
	Save(ctx context.Context, history *WorkflowTransitionHistory) error
	SaveTx(ctx context.Context, tx *sql.Tx, history *WorkflowTransitionHistory) error
	FindByInstanceID(ctx context.Context, tenantID, instanceID uuid.UUID, limit, offset int) ([]WorkflowTransitionHistory, error)
	FindByCaseID(ctx context.Context, tenantID, caseID uuid.UUID, limit, offset int) ([]WorkflowTransitionHistory, error)
}
