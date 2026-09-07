package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/database"
	decisionsdomain "github.com/alrazihi/civora/internal/decisions/domain"
	intmid "github.com/alrazihi/civora/internal/middleware"
	"github.com/google/uuid"
)

var (
	ErrDecisionNotFound = errors.New("decision not found")
	ErrDecisionInput    = errors.New("invalid decision input")
)

type DecisionService struct {
	repo    decisionsdomain.DecisionRepository
	auditor auditdomain.EventRecorder
}

type txEventRecorder interface {
	RecordEventInTx(ctx context.Context, tx *sql.Tx, params auditdomain.RecordEventParams) error
}

func NewDecisionService(repo decisionsdomain.DecisionRepository, auditor auditdomain.EventRecorder) *DecisionService {
	return &DecisionService{repo: repo, auditor: auditor}
}

type MakeDecisionParams struct {
	OrganizationID   uuid.UUID
	ServiceRequestID uuid.UUID
	Decision         decisionsdomain.DecisionType
	Reason           string
	ActorID          uuid.UUID
}

func (s *DecisionService) MakeDecision(ctx context.Context, params MakeDecisionParams) (*decisionsdomain.Decision, error) {
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
			if err := recordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: d.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "decision.made",
				Resource:       "decision",
				ResourceID:     strPtr(d.ID.String()),
				Outcome:        "success",
				RequestID:      strPtr(intmid.RequestIDFromContext(ctx)),
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

func strPtr(s string) *string {
	return &s
}

func recordAuditEventInTx(ctx context.Context, tx *sql.Tx, auditor auditdomain.EventRecorder, params auditdomain.RecordEventParams) error {
	if txRecorder, ok := auditor.(txEventRecorder); ok {
		return txRecorder.RecordEventInTx(ctx, tx, params)
	}
	return auditor.RecordEvent(ctx, params)
}
