package domain

import (
	"context"

	"github.com/google/uuid"
)

type CaseRepository interface {
	Save(ctx context.Context, c *Case) error
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*Case, error)
	FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*Case, error)
	UpdateStatus(ctx context.Context, orgID, id uuid.UUID, status CaseStatus) error
	Assign(ctx context.Context, orgID, id uuid.UUID, userID uuid.UUID) error
}
