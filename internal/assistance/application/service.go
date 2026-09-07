package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	assistancedomain "github.com/alrazihi/civora/internal/assistance/domain"
	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/database"
	intmid "github.com/alrazihi/civora/internal/middleware"
	"github.com/google/uuid"
)

var (
	ErrAssistanceNotFound = errors.New("assistance not found")
	ErrAssistanceInput    = errors.New("invalid assistance input")
)

type AssistanceService struct {
	repo    assistancedomain.AssistanceRepository
	auditor auditdomain.EventRecorder
}

type txEventRecorder interface {
	RecordEventInTx(ctx context.Context, tx *sql.Tx, params auditdomain.RecordEventParams) error
}

func NewAssistanceService(repo assistancedomain.AssistanceRepository, auditor auditdomain.EventRecorder) *AssistanceService {
	return &AssistanceService{repo: repo, auditor: auditor}
}

type CreateAssistanceParams struct {
	OrganizationID   uuid.UUID
	ServiceRequestID uuid.UUID
	Type             assistancedomain.AssistanceType
	Description      string
	ResponsibleStaff uuid.UUID
	ActorID          uuid.UUID
}

func (s *AssistanceService) CreateAssistance(ctx context.Context, params CreateAssistanceParams) (*assistancedomain.Assistance, error) {
	a, err := assistancedomain.NewAssistance(params.OrganizationID, params.ServiceRequestID, params.ResponsibleStaff, params.Type, params.Description)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAssistanceInput, err)
	}

	var result *assistancedomain.Assistance
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.SaveTx(ctx, tx, a); err != nil {
			return fmt.Errorf("failed to save assistance: %w", err)
		}

		if s.auditor != nil {
			if err := recordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: a.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "assistance.created",
				Resource:       "assistance",
				ResourceID:     strPtr(a.ID.String()),
				Outcome:        "success",
				RequestID:      strPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"service_request_id": a.ServiceRequestID.String(),
					"assistance_type":    string(a.Type),
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		result = a
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *AssistanceService) GetAssistance(ctx context.Context, orgID, id uuid.UUID) (*assistancedomain.Assistance, error) {
	a, err := s.repo.FindByID(ctx, orgID, id)
	if err != nil {
		return nil, ErrAssistanceNotFound
	}
	return a, nil
}

func (s *AssistanceService) ListAssistances(ctx context.Context, orgID, serviceRequestID uuid.UUID, limit, offset int) ([]*assistancedomain.Assistance, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	total, err := s.repo.CountByServiceRequest(ctx, orgID, serviceRequestID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count assistances: %w", err)
	}

	items, err := s.repo.FindByServiceRequest(ctx, orgID, serviceRequestID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list assistances: %w", err)
	}

	return items, total, nil
}

func (s *AssistanceService) UpdateAssistanceStatus(ctx context.Context, orgID, id uuid.UUID, action string, actorID uuid.UUID) (*assistancedomain.Assistance, error) {
	a, err := s.repo.FindByID(ctx, orgID, id)
	if err != nil {
		return nil, ErrAssistanceNotFound
	}

	switch action {
	case "start":
		a.Start()
	case "complete":
		a.Complete()
	case "cancel":
		a.Cancel()
	default:
		return nil, fmt.Errorf("%w: invalid action %s", ErrAssistanceInput, action)
	}

	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.SaveTx(ctx, tx, a); err != nil {
			return fmt.Errorf("failed to update assistance: %w", err)
		}

		if s.auditor != nil {
			auditAction := "assistance." + action + "ed"
			if err := recordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: a.OrganizationID,
				ActorID:        &actorID,
				Action:         auditAction,
				Resource:       "assistance",
				ResourceID:     strPtr(a.ID.String()),
				Outcome:        "success",
				RequestID:      strPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"service_request_id": a.ServiceRequestID.String(),
					"status":             string(a.Status),
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return a, nil
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
