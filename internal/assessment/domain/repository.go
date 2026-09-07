package domain

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

type AssessmentRepository interface {
	DB() *sql.DB
	Save(ctx context.Context, a *Assessment) error
	SaveTx(ctx context.Context, tx *sql.Tx, a *Assessment) error
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*Assessment, error)
	FindByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) (*Assessment, error)
	FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*Assessment, error)
	CountByOrganization(ctx context.Context, orgID uuid.UUID) (int, error)
}
