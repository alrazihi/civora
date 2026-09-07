package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/database"
	eligibilitydomain "github.com/alrazihi/civora/internal/eligibility/domain"
	intmid "github.com/alrazihi/civora/internal/middleware"
	"github.com/google/uuid"
)

var (
	ErrEligibilityNotFound = errors.New("eligibility not found")
	ErrEligibilityInput    = errors.New("invalid input")
)

type EligibilityService struct {
	repo    eligibilitydomain.EligibilityRepository
	auditor auditdomain.EventRecorder
}

type txEventRecorder interface {
	RecordEventInTx(ctx context.Context, tx *sql.Tx, params auditdomain.RecordEventParams) error
}

func NewEligibilityService(repo eligibilitydomain.EligibilityRepository, auditor auditdomain.EventRecorder) *EligibilityService {
	return &EligibilityService{repo: repo, auditor: auditor}
}

type CreateEligibilityParams struct {
	OrganizationID   uuid.UUID
	ServiceRequestID uuid.UUID
	Criteria         map[string]interface{}
	Explanation      string
	ActorID          uuid.UUID
}

func (s *EligibilityService) CreateEligibility(ctx context.Context, params CreateEligibilityParams) (*eligibilitydomain.Eligibility, error) {
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
			if err := recordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: e.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "eligibility.assessed",
				Resource:       "eligibility",
				ResourceID:     strPtr(e.ID.String()),
				Outcome:        "success",
				RequestID:      strPtr(intmid.RequestIDFromContext(ctx)),
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

	e.SetResult(result)

	var updated *eligibilitydomain.Eligibility
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.SaveTx(ctx, tx, e); err != nil {
			return fmt.Errorf("failed to update eligibility: %w", err)
		}

		if s.auditor != nil {
			if err := recordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: e.OrganizationID,
				ActorID:        &actorID,
				Action:         "eligibility.result_updated",
				Resource:       "eligibility",
				ResourceID:     strPtr(e.ID.String()),
				Outcome:        "success",
				RequestID:      strPtr(intmid.RequestIDFromContext(ctx)),
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

func strPtr(s string) *string {
	return &s
}

func recordAuditEventInTx(ctx context.Context, tx *sql.Tx, auditor auditdomain.EventRecorder, params auditdomain.RecordEventParams) error {
	if txRecorder, ok := auditor.(txEventRecorder); ok {
		return txRecorder.RecordEventInTx(ctx, tx, params)
	}
	return auditor.RecordEvent(ctx, params)
}
