package domain

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	Email          string     `json:"email"`
	Name           string     `json:"name"`
	RoleID         *uuid.UUID `json:"role_id"`
	PasswordHash   *string    `json:"-"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func NewUser(orgID uuid.UUID, email, name string, roleID *uuid.UUID) *User {
	now := time.Now().UTC()
	return &User{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Email:          email,
		Name:           name,
		RoleID:         roleID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, hash string) (bool, error)
}

type BCryptHasher struct {
	cost int
}

func NewBCryptHasher(cost int) *BCryptHasher {
	if cost < 4 {
		cost = 4
	}
	if cost > 31 {
		cost = 31
	}
	return &BCryptHasher{cost: cost}
}

func (h *BCryptHasher) Hash(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (h *BCryptHasher) Verify(password, hash string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

type DefaultRoleCreator struct {
	roleRepo RoleRepository
}

func NewDefaultRoleCreator(roleRepo RoleRepository) *DefaultRoleCreator {
	return &DefaultRoleCreator{roleRepo: roleRepo}
}

func (c *DefaultRoleCreator) CreateDefaultRoles(ctx context.Context, orgID uuid.UUID) error {
	return c.createDefaultRoles(ctx, nil, orgID)
}

func (c *DefaultRoleCreator) CreateDefaultRolesTx(ctx context.Context, tx *sql.Tx, orgID uuid.UUID) error {
	return c.createDefaultRoles(ctx, tx, orgID)
}

func (c *DefaultRoleCreator) createDefaultRoles(ctx context.Context, tx *sql.Tx, orgID uuid.UUID) error {
	defaultRoles := []struct {
		name        string
		description string
		permissions []string
	}{
		{name: "admin", description: "Full access to all organization resources", permissions: []string{"*"}},
		{name: "staff", description: "Standard user with case and customer access", permissions: []string{"cases:*"}},
	}

	for _, dr := range defaultRoles {
		role := NewRole(orgID, dr.name, dr.description, dr.permissions)
		if tx != nil {
			if err := c.roleRepo.SaveTx(ctx, tx, role); err != nil {
				return fmt.Errorf("failed to save default role %q: %w", dr.name, err)
			}
		} else {
			if err := c.roleRepo.Save(ctx, role); err != nil {
				return fmt.Errorf("failed to save default role %q: %w", dr.name, err)
			}
		}
	}
	return nil
}

type OrganizationUserChecker struct {
	userRepo UserRepository
}

func NewOrganizationUserChecker(userRepo UserRepository) *OrganizationUserChecker {
	return &OrganizationUserChecker{userRepo: userRepo}
}

func (c *OrganizationUserChecker) BelongsToOrganization(ctx context.Context, orgID, userID uuid.UUID) (bool, error) {
	user, err := c.userRepo.FindByID(ctx, orgID, userID)
	if err != nil {
		return false, nil
	}
	return user.OrganizationID == orgID, nil
}
