package domain

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

type DecisionRepository interface {
	DB() *sql.DB
	Save(ctx context.Context, d *Decision) error
	SaveTx(ctx context.Context, tx *sql.Tx, d *Decision) error
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*Decision, error)
	FindByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) (*Decision, error)
	FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*Decision, error)
	CountByOrganization(ctx context.Context, orgID uuid.UUID) (int, error)
}
