package domain

import (
	"context"

	"github.com/google/uuid"
)

type UserRepository interface {
	Save(ctx context.Context, user *User) error
	FindByEmail(ctx context.Context, orgID uuid.UUID, email string) (*User, error)
	FindByID(ctx context.Context, orgID, userID uuid.UUID) (*User, error)
	FindByOrganization(ctx context.Context, orgID uuid.UUID) ([]*User, error)
}

type RoleRepository interface {
	Save(ctx context.Context, role *Role) error
	FindByID(ctx context.Context, orgID, roleID uuid.UUID) (*Role, error)
	FindByName(ctx context.Context, orgID uuid.UUID, name string) (*Role, error)
	FindByOrganization(ctx context.Context, orgID uuid.UUID) ([]*Role, error)
}

type TokenService interface {
	GenerateToken(userID, organizationID, role string) (string, error)
}
