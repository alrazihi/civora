package domain

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

type CaseRepository interface {
	DB() *sql.DB
	Save(ctx context.Context, c *Case) error
	SaveTx(ctx context.Context, tx *sql.Tx, c *Case) error
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*Case, error)
	FindByIDTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID) (*Case, error)
	FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*Case, error)
	FindByOrganizationWithFilter(ctx context.Context, orgID uuid.UUID, limit, offset int, filter CaseFilter) ([]*Case, error)
	CountByOrganization(ctx context.Context, orgID uuid.UUID, filter CaseFilter) (int, error)
	UpdateStatus(ctx context.Context, orgID, id uuid.UUID, status CaseStatus, version int) error
	UpdateStatusTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID, status CaseStatus, version int) error
	UpdateWorkflowStateTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID, workflowState string, version int) error
	Assign(ctx context.Context, orgID, id, userID uuid.UUID) error
	AssignTx(ctx context.Context, tx *sql.Tx, orgID, id, userID uuid.UUID) error
	UpdateWorkflowInstanceID(ctx context.Context, orgID, caseID, instanceID uuid.UUID) error
	UpdateWorkflowInstanceIDTx(ctx context.Context, tx *sql.Tx, orgID, caseID, instanceID uuid.UUID) error
}

type CaseFilter struct {
	Status   CaseStatus
	PersonID *uuid.UUID
}
