package domain

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

type AssistanceRepository interface {
	DB() *sql.DB
	Save(ctx context.Context, a *Assistance) error
	SaveTx(ctx context.Context, tx *sql.Tx, a *Assistance) error
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*Assistance, error)
	FindByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID, limit, offset int) ([]*Assistance, error)
	CountByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) (int, error)
	FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*Assistance, error)
	CountByOrganization(ctx context.Context, orgID uuid.UUID) (int, error)
}
