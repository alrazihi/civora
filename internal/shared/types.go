package shared

import (
	"context"
	"database/sql"

	casesdomain "github.com/alrazihi/civora/internal/cases/domain"
	peopledomain "github.com/alrazihi/civora/internal/people/domain"
	"github.com/google/uuid"
)

type CaseFinder interface {
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*casesdomain.Case, error)
}

type CaseUpdater interface {
	UpdateStatusTx(ctx context.Context, tx *sql.Tx, orgID, id uuid.UUID, status casesdomain.CaseStatus, version int) error
}

type UserChecker interface {
	BelongsToOrganization(ctx context.Context, orgID, userID uuid.UUID) (bool, error)
}

type PersonFinder interface {
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*peopledomain.Person, error)
}
