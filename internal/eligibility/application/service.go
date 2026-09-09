package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	casesdomain "github.com/alrazihi/civora/internal/cases/domain"
	"github.com/alrazihi/civora/internal/database"
	eligibilitydomain "github.com/alrazihi/civora/internal/eligibility/domain"
	intmid "github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/google/uuid"
)

var (
	ErrEligibilityNotFound = errors.New("eligibility not found")
	ErrEligibilityInput    = errors.New("invalid eligibility input")
	ErrCaseNotFound        = errors.New("case not found")
	ErrCaseClosed          = errors.New("case is closed")
	ErrUserNotFound        = errors.New("user not found")
)

type EligibilityService struct {
	repo        eligibilitydomain.EligibilityRepository
	caseRepo    shared.CaseFinder
	userChecker shared.UserChecker
	auditor     auditdomain.EventRecorder
}

func NewEligibilityService(repo eligibilitydomain.EligibilityRepository, caseRepo shared.CaseFinder, userChecker shared.UserChecker, auditor auditdomain.EventRecorder) *EligibilityService {
	return &EligibilityService{repo: repo, caseRepo: caseRepo, userChecker: userChecker, auditor: auditor}
}

type CreateEligibilityParams struct {
	OrganizationID   uuid.UUID
	ServiceRequestID uuid.UUID
	Criteria         map[string]interface{}
	Explanation      string
	ActorID          uuid.UUID
}

func (s *EligibilityService) CreateEligibility(ctx context.Context, params CreateEligibilityParams) (*eligibilitydomain.Eligibility, error) {
	c, err := s.caseRepo.FindByID(ctx, params.OrganizationID, params.ServiceRequestID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCaseNotFound, err)
	}
	if c.OrganizationID != params.OrganizationID {
		return nil, ErrCaseNotFound
	}
	if casesdomain.IsClosed(c.Status) {
		return nil, ErrCaseClosed
	}

	existing, err := s.repo.FindByServiceRequest(ctx, params.OrganizationID, params.ServiceRequestID)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("%w: eligibility already exists for this service request", ErrEligibilityInput)
	}

	valid, err := s.userChecker.BelongsToOrganization(ctx, params.OrganizationID, params.ActorID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate actor: %w", err)
	}
	if !valid {
		return nil, ErrUserNotFound
	}

	e, err := eligibilitydomain.NewEligibility(params.OrganizationID, params.ServiceRequestID, params.ActorID, params.Criteria, params.Explanation)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEligibilityInput, err)
	}

	var result *eligibilitydomain.Eligibility
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.SaveTx(ctx, tx, e); err != nil {
			return fmt.Errorf("failed to save eligibility: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: e.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "eligibility.assessed",
				Resource:       "eligibility",
				ResourceID:     shared.StrPtr(e.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"service_request_id": e.ServiceRequestID.String(),
					"result":             string(e.Result),
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		result = e
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *EligibilityService) GetEligibility(ctx context.Context, orgID, id uuid.UUID) (*eligibilitydomain.Eligibility, error) {
	e, err := s.repo.FindByID(ctx, orgID, id)
	if err != nil {
		return nil, ErrEligibilityNotFound
	}
	return e, nil
}

func (s *EligibilityService) GetEligibilityByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) (*eligibilitydomain.Eligibility, error) {
	e, err := s.repo.FindByServiceRequest(ctx, orgID, serviceRequestID)
	if err != nil {
		return nil, ErrEligibilityNotFound
	}
	return e, nil
}

func (s *EligibilityService) ListEligibilities(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*eligibilitydomain.Eligibility, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	total, err := s.repo.CountByOrganization(ctx, orgID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count eligibilities: %w", err)
	}

	items, err := s.repo.FindByOrganization(ctx, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list eligibilities: %w", err)
	}

	return items, total, nil
}

func (s *EligibilityService) UpdateEligibilityResult(ctx context.Context, orgID, id uuid.UUID, result eligibilitydomain.EligibilityResult, actorID uuid.UUID) (*eligibilitydomain.Eligibility, error) {
	e, err := s.repo.FindByID(ctx, orgID, id)
	if err != nil {
		return nil, ErrEligibilityNotFound
	}

	c, err := s.caseRepo.FindByID(ctx, orgID, e.ServiceRequestID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCaseNotFound, err)
	}
	if c.OrganizationID != orgID {
		return nil, ErrCaseNotFound
	}
	if casesdomain.IsClosed(c.Status) {
		return nil, ErrCaseClosed
	}

	e.SetResult(result)

	var updated *eligibilitydomain.Eligibility
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.SaveTx(ctx, tx, e); err != nil {
			return fmt.Errorf("failed to update eligibility: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: e.OrganizationID,
				ActorID:        &actorID,
				Action:         "eligibility.result_updated",
				Resource:       "eligibility",
				ResourceID:     shared.StrPtr(e.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"service_request_id": e.ServiceRequestID.String(),
					"result":             string(result),
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		updated = e
		return nil
	})
	if err != nil {
		return nil, err
	}

	return updated, nil
}
