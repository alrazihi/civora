package domain

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

type ReviewQueueRepository interface {
	DB() *sql.DB
	Save(ctx context.Context, r *ReviewQueueEntry) error
	SaveTx(ctx context.Context, tx *sql.Tx, r *ReviewQueueEntry) error
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*ReviewQueueEntry, error)
	FindByIDTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID) (*ReviewQueueEntry, error)
	FindByCaseID(ctx context.Context, orgID, caseID uuid.UUID) (*ReviewQueueEntry, error)
	ListByOrganization(ctx context.Context, orgID uuid.UUID, status *ReviewStatus, assignedToID *uuid.UUID, limit, offset int) ([]*ReviewQueueEntry, int, error)
	UpdateStatusTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID, status ReviewStatus, assignedToID *uuid.UUID) error
	Delete(ctx context.Context, orgID, id uuid.UUID) error
	DeleteTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID) error
}
