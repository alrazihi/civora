package domain

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

type WorkflowStateFormAssignmentRepository interface {
	DB() *sql.DB

	Save(ctx context.Context, assignment *WorkflowStateFormAssignment) error
	SaveTx(ctx context.Context, tx *sql.Tx, assignment *WorkflowStateFormAssignment) error

	FindByID(ctx context.Context, tenantID, id uuid.UUID) (*WorkflowStateFormAssignment, error)
	FindByIDTx(ctx context.Context, tx *sql.Tx, tenantID, id uuid.UUID) (*WorkflowStateFormAssignment, error)

	FindByWorkflowAndState(ctx context.Context, tenantID, workflowDefID uuid.UUID, stateKey string) ([]*WorkflowStateFormAssignment, error)
	FindByWorkflowAndStateTx(ctx context.Context, tx *sql.Tx, tenantID, workflowDefID uuid.UUID, stateKey string) ([]*WorkflowStateFormAssignment, error)

	ListByWorkflow(ctx context.Context, tenantID, workflowDefID uuid.UUID) ([]*WorkflowStateFormAssignment, error)

	Update(ctx context.Context, assignment *WorkflowStateFormAssignment) error
	UpdateTx(ctx context.Context, tx *sql.Tx, assignment *WorkflowStateFormAssignment) error

	Delete(ctx context.Context, tenantID, id uuid.UUID) error
	DeleteTx(ctx context.Context, tx *sql.Tx, tenantID, id uuid.UUID) error

	DeleteByWorkflowAndState(ctx context.Context, tenantID, workflowDefID uuid.UUID, stateKey string) error
	DeleteByWorkflowAndStateTx(ctx context.Context, tx *sql.Tx, tenantID, workflowDefID uuid.UUID, stateKey string) error
}
