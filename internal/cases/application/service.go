package application

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/cases/domain"
	"github.com/google/uuid"
)

var (
	ErrCaseNotFound        = errors.New("case not found")
	ErrCaseInvalidInput    = errors.New("invalid input")
	ErrCaseTransition      = errors.New("invalid state transition")
	ErrCaseUserNotFound    = errors.New("assignee not found")
	ErrCaseTenantViolation = errors.New("user does not belong to organization")
)

type UserChecker interface {
	BelongsToOrganization(ctx context.Context, orgID, userID uuid.UUID) (bool, error)
}

type CaseService struct {
	repo        domain.CaseRepository
	userChecker UserChecker
	auditor     auditdomain.EventRecorder
}

func NewCaseService(repo domain.CaseRepository, userChecker UserChecker, auditor auditdomain.EventRecorder) *CaseService {
	return &CaseService{
		repo:        repo,
		userChecker: userChecker,
		auditor:     auditor,
	}
}

type CreateCaseParams struct {
	OrganizationID uuid.UUID
	Title          string
	Description    string
	CreatedByID    uuid.UUID
}

func (s *CaseService) CreateCase(ctx context.Context, params CreateCaseParams) (*domain.Case, error) {
	if strings.TrimSpace(params.Title) == "" {
		return nil, fmt.Errorf("%w: title is required", ErrCaseInvalidInput)
	}

	c := domain.NewCase(params.OrganizationID, params.CreatedByID, params.Title, params.Description)

	if err := s.repo.Save(ctx, c); err != nil {
		return nil, fmt.Errorf("failed to save case: %w", err)
	}

	if s.auditor != nil {
		if err := s.auditor.RecordEvent(ctx, auditdomain.RecordEventParams{
			OrganizationID: c.OrganizationID,
			ActorID:        &c.CreatedByID,
			Action:         "case.created",
			Resource:       "case",
			ResourceID:     strPtr(c.ID.String()),
			Outcome:        "success",
		}); err != nil {
			log.Printf("audit event recording failed: %v", err)
		}
	}

	return c, nil
}

type ChangeCaseStatusParams struct {
	OrganizationID uuid.UUID
	CaseID         uuid.UUID
	Status         domain.CaseStatus
	ActorID        uuid.UUID
}

func (s *CaseService) ChangeStatus(ctx context.Context, params ChangeCaseStatusParams) (*domain.Case, error) {
	c, err := s.repo.FindByID(ctx, params.OrganizationID, params.CaseID)
	if err != nil {
		return nil, ErrCaseNotFound
	}

	if err := c.TransitionTo(params.Status); err != nil {
		if errors.Is(err, domain.ErrInvalidStateTransition) {
			return nil, fmt.Errorf("%w: from %s to %s", ErrCaseTransition, c.Status, params.Status)
		}
		return nil, err
	}

	if err := s.repo.UpdateStatus(ctx, params.OrganizationID, params.CaseID, params.Status); err != nil {
		return nil, fmt.Errorf("failed to update case status: %w", err)
	}

	if s.auditor != nil {
		if err := s.auditor.RecordEvent(ctx, auditdomain.RecordEventParams{
			OrganizationID: c.OrganizationID,
			ActorID:        &params.ActorID,
			Action:         fmt.Sprintf("case.transition"),
			Resource:       "case",
			ResourceID:     strPtr(c.ID.String()),
			Outcome:        "success",
			Metadata: map[string]interface{}{
				"from": string(c.Status),
				"to":   string(params.Status),
			},
		}); err != nil {
			log.Printf("audit event recording failed: %v", err)
		}
	}

	return c, nil
}

type AssignCaseParams struct {
	OrganizationID uuid.UUID
	CaseID         uuid.UUID
	UserID         uuid.UUID
	ActorID        uuid.UUID
}

func (s *CaseService) AssignCase(ctx context.Context, params AssignCaseParams) (*domain.Case, error) {
	c, err := s.repo.FindByID(ctx, params.OrganizationID, params.CaseID)
	if err != nil {
		return nil, ErrCaseNotFound
	}

	if s.userChecker != nil {
		valid, err := s.userChecker.BelongsToOrganization(ctx, params.OrganizationID, params.UserID)
		if err != nil {
			return nil, fmt.Errorf("failed to validate assignee: %w", err)
		}
		if !valid {
			return nil, ErrCaseTenantViolation
		}
	}

	c.AssignTo(params.UserID)

	if err := s.repo.Assign(ctx, params.OrganizationID, params.CaseID, params.UserID); err != nil {
		return nil, fmt.Errorf("failed to assign case: %w", err)
	}

	if s.auditor != nil {
		if err := s.auditor.RecordEvent(ctx, auditdomain.RecordEventParams{
			OrganizationID: c.OrganizationID,
			ActorID:        &params.ActorID,
			Action:         "case.assigned",
			Resource:       "case",
			ResourceID:     strPtr(c.ID.String()),
			Outcome:        "success",
			Metadata: map[string]interface{}{
				"assigned_to": params.UserID.String(),
			},
		}); err != nil {
			log.Printf("audit event recording failed: %v", err)
		}
	}

	return c, nil
}

func (s *CaseService) GetCase(ctx context.Context, orgID, caseID uuid.UUID) (*domain.Case, error) {
	c, err := s.repo.FindByID(ctx, orgID, caseID)
	if err != nil {
		return nil, ErrCaseNotFound
	}
	return c, nil
}

func (s *CaseService) ListCases(ctx context.Context, orgID uuid.UUID, limit, offset int, filter domain.CaseFilter) ([]*domain.Case, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	total, err := s.repo.CountByOrganization(ctx, orgID, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count cases: %w", err)
	}

	cases, err := s.repo.FindByOrganizationWithFilter(ctx, orgID, limit, offset, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list cases: %w", err)
	}

	return cases, total, nil
}

func strPtr(s string) *string {
	return &s
}
