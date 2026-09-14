package domain

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

type RuleAssignmentRepository interface {
	DB() *sql.DB
	Save(ctx context.Context, assignment *WorkflowStateRuleAssignment) error
	SaveTx(ctx context.Context, tx *sql.Tx, assignment *WorkflowStateRuleAssignment) error

	FindByID(ctx context.Context, orgID, id uuid.UUID) (*WorkflowStateRuleAssignment, error)
	FindByWorkflowAndState(ctx context.Context, orgID uuid.UUID, workflowDefID uuid.UUID, stateKey string) ([]*WorkflowStateRuleAssignment, error)
	ListByRuleSet(ctx context.Context, orgID, ruleSetID uuid.UUID) ([]*WorkflowStateRuleAssignment, error)
	ListByWorkflow(ctx context.Context, orgID, workflowDefID uuid.UUID) ([]*WorkflowStateRuleAssignment, error)

	UpdateTx(ctx context.Context, tx *sql.Tx, assignment *WorkflowStateRuleAssignment) error
	DeleteTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID) error
}
