package domain

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

type UserRepository interface {
	DB() *sql.DB
	Save(ctx context.Context, user *User) error
	SaveTx(ctx context.Context, tx *sql.Tx, user *User) error
	FindByEmail(ctx context.Context, orgID uuid.UUID, email string) (*User, error)
	FindByID(ctx context.Context, orgID, userID uuid.UUID) (*User, error)
	FindByOrganization(ctx context.Context, orgID uuid.UUID) ([]*User, error)
	CountByOrganization(ctx context.Context, orgID uuid.UUID) (int, error)
	CountByOrganizationTx(ctx context.Context, tx *sql.Tx, orgID uuid.UUID) (int, error)
}

type RoleRepository interface {
	DB() *sql.DB
	Save(ctx context.Context, role *Role) error
	SaveTx(ctx context.Context, tx *sql.Tx, role *Role) error
	FindByID(ctx context.Context, orgID, roleID uuid.UUID) (*Role, error)
	FindByName(ctx context.Context, orgID uuid.UUID, name string) (*Role, error)
	FindByOrganization(ctx context.Context, orgID uuid.UUID) ([]*Role, error)
}

type TokenService interface {
	GenerateToken(userID, organizationID, role string) (string, error)
}
