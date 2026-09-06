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

// OIDCProvider is an abstraction for external OpenID Connect / OAuth2
// identity providers. When OIDCIssuer is configured, the IdentityService
// delegates authentication to the provider and maps the external user to a
// local User record on first login.
//
// This interface is defined but not yet implemented. A concrete provider
// (e.g., for Keycloak, Auth0, or Google) will be added when OIDC
// authentication becomes an operational requirement.
type OIDCProvider interface {
	// ExchangeCode exchanges an authorization code for tokens.
	ExchangeCode(ctx context.Context, code string) (accessToken, idToken string, err error)

	// VerifyToken verifies an ID token and returns the user info.
	VerifyToken(ctx context.Context, idToken string) (externalID, email, name string, err error)
}
