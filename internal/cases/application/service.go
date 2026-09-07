package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/cases/domain"
	"github.com/alrazihi/civora/internal/database"
	intmid "github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/google/uuid"
)

var (
	ErrCaseNotFound          = errors.New("case not found")
	ErrCaseInvalidInput      = errors.New("invalid input")
	ErrCaseTransition        = errors.New("invalid state transition")
	ErrCaseUserNotFound      = errors.New("assignee not found")
	ErrCaseTenantViolation   = errors.New("user does not belong to organization")
	ErrPersonNotFound        = errors.New("person not found")
	ErrPersonTenantViolation = errors.New("person does not belong to organization")
)

type UserChecker interface {
	BelongsToOrganization(ctx context.Context, orgID, userID uuid.UUID) (bool, error)
}

type txEventRecorder interface {
	RecordEventInTx(ctx context.Context, tx *sql.Tx, params auditdomain.RecordEventParams) error
}

type CaseService struct {
	repo         domain.CaseRepository
	personFinder shared.PersonFinder
	userChecker  UserChecker
	auditor      auditdomain.EventRecorder
}

func NewCaseService(repo domain.CaseRepository, personFinder shared.PersonFinder, userChecker UserChecker, auditor auditdomain.EventRecorder) *CaseService {
	return &CaseService{
		repo:         repo,
		personFinder: personFinder,
		userChecker:  userChecker,
		auditor:      auditor,
	}
}

type CreateCaseParams struct {
	OrganizationID uuid.UUID
	Title          string
	Description    string
	ServiceType    domain.ServiceType
	Priority       domain.Priority
	PersonID       *uuid.UUID
	CreatedByID    uuid.UUID
}

func (s *CaseService) CreateCase(ctx context.Context, params CreateCaseParams) (*domain.Case, error) {
	if params.Title == "" {
		return nil, fmt.Errorf("%w: title is required", ErrCaseInvalidInput)
	}

	if params.PersonID != nil {
		p, err := s.personFinder.FindByID(ctx, params.OrganizationID, *params.PersonID)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrPersonNotFound, err)
		}
		if p.OrganizationID != params.OrganizationID {
			return nil, ErrPersonTenantViolation
		}
	}

	c, err := domain.NewCase(params.OrganizationID, params.CreatedByID, params.Title, params.Description, params.ServiceType, params.Priority, params.PersonID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCaseInvalidInput, err)
	}

	const maxRetries = 3
	var result *domain.Case
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		for attempts := 0; ; attempts++ {
			if err := s.repo.SaveTx(ctx, tx, c); err != nil {
				if errors.Is(err, domain.ErrCaseNumberConflict) && attempts < maxRetries-1 {
					c.RegenerateCaseNumber()
					continue
				}
				return fmt.Errorf("failed to save case: %w", err)
			}

			if s.auditor != nil {
				if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
					OrganizationID: c.OrganizationID,
					ActorID:        &c.CreatedByID,
					Action:         "case.created",
					Resource:       "case",
					ResourceID:     shared.StrPtr(c.ID.String()),
					Outcome:        "success",
					RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				}); err != nil {
					return fmt.Errorf("failed to record audit event: %w", err)
				}
			}

			result = c
			return nil
		}
	})
	if err != nil {
		return nil, err
	}

	return result, nil
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

	var result *domain.Case
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.UpdateStatusTx(ctx, tx, params.OrganizationID, params.CaseID, params.Status, c.Version); err != nil {
			return fmt.Errorf("failed to update case status: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: c.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "case.transition",
				Resource:       "case",
				ResourceID:     shared.StrPtr(c.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"from": string(c.Status),
					"to":   string(params.Status),
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		result = c
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
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

	var result *domain.Case
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.AssignTx(ctx, tx, params.OrganizationID, params.CaseID, params.UserID); err != nil {
			return fmt.Errorf("failed to assign case: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: c.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "case.assigned",
				Resource:       "case",
				ResourceID:     shared.StrPtr(c.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"assigned_to": params.UserID.String(),
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		result = c
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
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
