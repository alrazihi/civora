package domain

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

type PersonRepository interface {
	DB() *sql.DB
	Save(ctx context.Context, p *Person) error
	SaveTx(ctx context.Context, tx *sql.Tx, p *Person) error
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*Person, error)
	FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*Person, error)
	FindByExternalReference(ctx context.Context, orgID uuid.UUID, ref string) (*Person, error)
	CountByOrganization(ctx context.Context, orgID uuid.UUID) (int, error)
	Update(ctx context.Context, p *Person) error
	UpdateTx(ctx context.Context, tx *sql.Tx, p *Person) error
}
