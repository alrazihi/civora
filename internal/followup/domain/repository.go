package domain

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

type FollowUpRepository interface {
	DB() *sql.DB
	Save(ctx context.Context, f *FollowUp) error
	SaveTx(ctx context.Context, tx *sql.Tx, f *FollowUp) error
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*FollowUp, error)
	FindByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID, limit, offset int) ([]*FollowUp, error)
	CountByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) (int, error)
	FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*FollowUp, error)
	CountByOrganization(ctx context.Context, orgID uuid.UUID) (int, error)
}
