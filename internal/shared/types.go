package shared

import (
	"context"

	casesdomain "github.com/alrazihi/civora/internal/cases/domain"
	peopledomain "github.com/alrazihi/civora/internal/people/domain"
	"github.com/google/uuid"
)

type CaseFinder interface {
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*casesdomain.Case, error)
}

type UserChecker interface {
	BelongsToOrganization(ctx context.Context, orgID, userID uuid.UUID) (bool, error)
}

type PersonFinder interface {
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*peopledomain.Person, error)
}
