package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	RoleAdmin = "admin"
	RoleStaff = "staff"
)

type Role struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Permissions    []string  `json:"permissions"`
	CreatedAt      time.Time `json:"created_at"`
}

func NewRole(orgID uuid.UUID, name, description string, permissions []string) *Role {
	return &Role{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           name,
		Description:    description,
		Permissions:    permissions,
		CreatedAt:      time.Now().UTC(),
	}
}

func (r *Role) HasPermission(permission string) bool {
	for _, p := range r.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}
