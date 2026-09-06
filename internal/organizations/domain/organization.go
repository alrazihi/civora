package domain

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidOrgName = errors.New("organization name is required")
	ErrInvalidOrgSlug = errors.New("organization slug is required")
)

type Organization struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Slug        string    `json:"slug"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func NewOrganization(name, description, slug string) *Organization {
	now := time.Now().UTC()
	return &Organization{
		ID:          uuid.New(),
		Name:        name,
		Description: description,
		Slug:        slug,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func (o *Organization) Validate() error {
	if strings.TrimSpace(o.Name) == "" {
		return ErrInvalidOrgName
	}
	if strings.TrimSpace(o.Slug) == "" {
		return ErrInvalidOrgSlug
	}
	return nil
}

type OrganizationRepository interface {
	Save(ctx context.Context, org *Organization) error
	FindByID(ctx context.Context, id uuid.UUID) (*Organization, error)
	FindBySlug(ctx context.Context, slug string) (*Organization, error)
}
