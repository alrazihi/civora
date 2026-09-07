package application

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/organizations/domain"
	"github.com/google/uuid"
)

var (
	ErrOrgNotFound     = errors.New("organization not found")
	ErrOrgInvalidInput = errors.New("invalid input")
	ErrOrgSlugTaken    = errors.New("organization slug already taken")
)

type OrganizationService struct {
	repo    domain.OrganizationRepository
	auditor auditdomain.EventRecorder
}

func NewOrganizationService(repo domain.OrganizationRepository, auditor auditdomain.EventRecorder) *OrganizationService {
	return &OrganizationService{
		repo:    repo,
		auditor: auditor,
	}
}

type CreateOrganizationParams struct {
	Name        string
	Description string
	Slug        string
}

func (s *OrganizationService) CreateOrganization(ctx context.Context, params CreateOrganizationParams) (*domain.Organization, error) {
	if strings.TrimSpace(params.Name) == "" {
		return nil, ErrOrgInvalidInput
	}
	if strings.TrimSpace(params.Slug) == "" {
		return nil, ErrOrgInvalidInput
	}

	existing, _ := s.repo.FindBySlug(ctx, params.Slug)
	if existing != nil {
		return nil, ErrOrgSlugTaken
	}

	org := domain.NewOrganization(params.Name, params.Description, params.Slug)
	if err := org.Validate(); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, org); err != nil {
		return nil, fmt.Errorf("failed to save organization: %w", err)
	}

	if s.auditor != nil {
		if err := s.auditor.RecordEvent(ctx, auditdomain.RecordEventParams{
			OrganizationID: org.ID,
			Action:         "organization.created",
			Resource:       "organization",
			ResourceID:     strPtr(org.ID.String()),
			Outcome:        "success",
		}); err != nil {
			log.Printf("audit event recording failed: %v", err)
		}
	}

	return org, nil
}

func (s *OrganizationService) GetOrganization(ctx context.Context, id uuid.UUID) (*domain.Organization, error) {
	org, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrOrgNotFound
	}
	return org, nil
}

func (s *OrganizationService) GetBySlug(ctx context.Context, slug string) (*domain.Organization, error) {
	org, err := s.repo.FindBySlug(ctx, slug)
	if err != nil {
		return nil, ErrOrgNotFound
	}
	return org, nil
}

func strPtr(s string) *string {
	return &s
}
