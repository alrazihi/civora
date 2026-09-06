package domain

import (
	"errors"
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
	IsOIDCUser     bool       `json:"is_oidc_user"`
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
