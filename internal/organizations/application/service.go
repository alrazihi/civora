package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/database"
	intmid "github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/organizations/domain"
	"github.com/google/uuid"
)

var (
	ErrOrgNotFound     = errors.New("organization not found")
	ErrOrgInvalidInput = errors.New("invalid input")
	ErrOrgSlugTaken    = errors.New("organization slug already taken")
)

type RoleCreator interface {
	CreateDefaultRoles(ctx context.Context, orgID uuid.UUID) error
	CreateDefaultRolesTx(ctx context.Context, tx *sql.Tx, orgID uuid.UUID) error
}

type OrganizationService struct {
	repo        domain.OrganizationRepository
	roleCreator RoleCreator
	auditor     auditdomain.EventRecorder
}

func NewOrganizationService(repo domain.OrganizationRepository, roleCreator RoleCreator, auditor auditdomain.EventRecorder) *OrganizationService {
	return &OrganizationService{
		repo:        repo,
		roleCreator: roleCreator,
		auditor:     auditor,
	}
}

type CreateOrganizationParams struct {
	Name        string
	Description string
	Slug        string
}

type txEventRecorder interface {
	RecordEventInTx(ctx context.Context, tx *sql.Tx, params auditdomain.RecordEventParams) error
}

func (s *OrganizationService) CreateOrganization(ctx context.Context, params CreateOrganizationParams) (*domain.Organization, error) {
	if strings.TrimSpace(params.Name) == "" {
		return nil, ErrOrgInvalidInput
	}
	if strings.TrimSpace(params.Slug) == "" {
		return nil, ErrOrgInvalidInput
	}

	existing, err := s.repo.FindBySlug(ctx, params.Slug)
	if err != nil && !errors.Is(err, domain.ErrOrgNotFound) {
		return nil, fmt.Errorf("failed to check existing organization slug: %w", err)
	}
	if existing != nil {
		return nil, ErrOrgSlugTaken
	}

	org := domain.NewOrganization(params.Name, params.Description, params.Slug)
	if err := org.Validate(); err != nil {
		return nil, err
	}

	var result *domain.Organization
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.SaveTx(ctx, tx, org); err != nil {
			return fmt.Errorf("failed to save organization: %w", err)
		}

		if s.roleCreator != nil {
			if err := s.roleCreator.CreateDefaultRolesTx(ctx, tx, org.ID); err != nil {
				return fmt.Errorf("failed to create default roles: %w", err)
			}
		}

		if s.auditor != nil {
			if err := recordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: org.ID,
				Action:         "organization.created",
				Resource:       "organization",
				ResourceID:     strPtr(org.ID.String()),
				Outcome:        "success",
				RequestID:      strPtr(intmid.RequestIDFromContext(ctx)),
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		result = org
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
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

func recordAuditEventInTx(ctx context.Context, tx *sql.Tx, auditor auditdomain.EventRecorder, params auditdomain.RecordEventParams) error {
	if txRecorder, ok := auditor.(txEventRecorder); ok {
		return txRecorder.RecordEventInTx(ctx, tx, params)
	}
	return auditor.RecordEvent(ctx, params)
}
