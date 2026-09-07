package domain

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

type EligibilityRepository interface {
	DB() *sql.DB
	Save(ctx context.Context, e *Eligibility) error
	SaveTx(ctx context.Context, tx *sql.Tx, e *Eligibility) error
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*Eligibility, error)
	FindByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) (*Eligibility, error)
	FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*Eligibility, error)
	CountByOrganization(ctx context.Context, orgID uuid.UUID) (int, error)
}
