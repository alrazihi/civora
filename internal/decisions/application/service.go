package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	casesdomain "github.com/alrazihi/civora/internal/cases/domain"
	"github.com/alrazihi/civora/internal/database"
	decisionsdomain "github.com/alrazihi/civora/internal/decisions/domain"
	intmid "github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/google/uuid"
)

var (
	ErrDecisionNotFound  = errors.New("decision not found")
	ErrDecisionInput     = errors.New("invalid decision input")
	ErrCaseNotFound      = errors.New("case not found")
	ErrUserNotFound      = errors.New("user not found")
	ErrInvalidCaseStatus = errors.New("case is not in DECISION_PENDING status")
)

type DecisionService struct {
	repo        decisionsdomain.DecisionRepository
	caseRepo    shared.CaseFinder
	userChecker shared.UserChecker
	auditor     auditdomain.EventRecorder
}

func NewDecisionService(repo decisionsdomain.DecisionRepository, caseRepo shared.CaseFinder, userChecker shared.UserChecker, auditor auditdomain.EventRecorder) *DecisionService {
	return &DecisionService{repo: repo, caseRepo: caseRepo, userChecker: userChecker, auditor: auditor}
}

type MakeDecisionParams struct {
	OrganizationID   uuid.UUID
	ServiceRequestID uuid.UUID
	Decision         decisionsdomain.DecisionType
	Reason           string
	ActorID          uuid.UUID
}

func (s *DecisionService) MakeDecision(ctx context.Context, params MakeDecisionParams) (*decisionsdomain.Decision, error) {
	c, err := s.caseRepo.FindByID(ctx, params.OrganizationID, params.ServiceRequestID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCaseNotFound, err)
	}
	if c.OrganizationID != params.OrganizationID {
		return nil, ErrCaseNotFound
	}

	existing, err := s.repo.FindByServiceRequest(ctx, params.OrganizationID, params.ServiceRequestID)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("%w: decision already exists for this service request", ErrDecisionInput)
	}

	valid, err := s.userChecker.BelongsToOrganization(ctx, params.OrganizationID, params.ActorID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate actor: %w", err)
	}
	if !valid {
		return nil, ErrUserNotFound
	}

	if c.Status != casesdomain.CaseStatusDecisionPending {
		return nil, ErrInvalidCaseStatus
	}

	d, err := decisionsdomain.NewDecision(params.OrganizationID, params.ServiceRequestID, params.ActorID, params.Decision, params.Reason)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecisionInput, err)
	}

	var result *decisionsdomain.Decision
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.SaveTx(ctx, tx, d); err != nil {
			return fmt.Errorf("failed to save decision: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: d.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "decision.made",
				Resource:       "decision",
				ResourceID:     shared.StrPtr(d.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"service_request_id": d.ServiceRequestID.String(),
					"decision":           string(d.Decision),
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		result = d
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *DecisionService) GetDecision(ctx context.Context, orgID, id uuid.UUID) (*decisionsdomain.Decision, error) {
	d, err := s.repo.FindByID(ctx, orgID, id)
	if err != nil {
		return nil, ErrDecisionNotFound
	}
	return d, nil
}

func (s *DecisionService) GetDecisionByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) (*decisionsdomain.Decision, error) {
	d, err := s.repo.FindByServiceRequest(ctx, orgID, serviceRequestID)
	if err != nil {
		return nil, ErrDecisionNotFound
	}
	return d, nil
}

func (s *DecisionService) ListDecisions(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*decisionsdomain.Decision, int, error) {
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
		return nil, 0, fmt.Errorf("failed to count decisions: %w", err)
	}

	items, err := s.repo.FindByOrganization(ctx, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list decisions: %w", err)
	}

	return items, total, nil
}
