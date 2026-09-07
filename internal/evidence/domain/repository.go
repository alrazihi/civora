package domain

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

type EvidenceRepository interface {
	DB() *sql.DB
	Save(ctx context.Context, e *Evidence) error
	SaveTx(ctx context.Context, tx *sql.Tx, e *Evidence) error
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*Evidence, error)
	FindByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID, limit, offset int) ([]*Evidence, error)
	CountByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) (int, error)
	FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*Evidence, error)
}
