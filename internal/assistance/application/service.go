package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	assistancedomain "github.com/alrazihi/civora/internal/assistance/domain"
	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	casesdomain "github.com/alrazihi/civora/internal/cases/domain"
	"github.com/alrazihi/civora/internal/database"
	intmid "github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/google/uuid"
)

var (
	ErrAssistanceNotFound = errors.New("assistance not found")
	ErrAssistanceInput    = errors.New("invalid assistance input")
	ErrCaseNotFound       = errors.New("case not found")
	ErrCaseClosed         = errors.New("case is closed")
	ErrUserNotFound       = errors.New("user not found")
)

type AssistanceService struct {
	repo        assistancedomain.AssistanceRepository
	caseRepo    shared.CaseFinder
	userChecker shared.UserChecker
	auditor     auditdomain.EventRecorder
}

func NewAssistanceService(repo assistancedomain.AssistanceRepository, caseRepo shared.CaseFinder, userChecker shared.UserChecker, auditor auditdomain.EventRecorder) *AssistanceService {
	return &AssistanceService{repo: repo, caseRepo: caseRepo, userChecker: userChecker, auditor: auditor}
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

	valid, err := s.userChecker.BelongsToOrganization(ctx, params.OrganizationID, params.ResponsibleStaff)
	if err != nil {
		return nil, fmt.Errorf("failed to validate responsible staff: %w", err)
	}
	if !valid {
		return nil, ErrUserNotFound
	}

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
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: a.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "assistance.created",
				Resource:       "assistance",
				ResourceID:     shared.StrPtr(a.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
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

	c, err := s.caseRepo.FindByID(ctx, orgID, a.ServiceRequestID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCaseNotFound, err)
	}
	if c.OrganizationID != orgID {
		return nil, ErrCaseNotFound
	}
	if casesdomain.IsClosed(c.Status) {
		return nil, ErrCaseClosed
	}

	if !assistancedomain.IsValidStatusTransition(a.Status, action) {
		return nil, fmt.Errorf("%w: invalid action %s for status %s", ErrAssistanceInput, action, a.Status)
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
			// The audit action vocabulary (see migrations/0007) only allows
			// 'assistance.created' and 'assistance.status_changed', so all
			// status transitions map to the latter.
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: a.OrganizationID,
				ActorID:        &actorID,
				Action:         "assistance.status_changed",
				Resource:       "assistance",
				ResourceID:     shared.StrPtr(a.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"service_request_id": a.ServiceRequestID.String(),
					"action":             action,
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
