package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	casesdomain "github.com/alrazihi/civora/internal/cases/domain"
	"github.com/alrazihi/civora/internal/database"
	followupdomain "github.com/alrazihi/civora/internal/followup/domain"
	intmid "github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/google/uuid"
)

var (
	ErrFollowUpNotFound = errors.New("follow-up not found")
	ErrFollowUpInput    = errors.New("invalid follow-up input")
	ErrCaseNotFound     = errors.New("case not found")
	ErrCaseClosed       = errors.New("case is closed")
	ErrUserNotFound     = errors.New("user not found")
)

type FollowUpService struct {
	repo        followupdomain.FollowUpRepository
	caseRepo    shared.CaseFinder
	userChecker shared.UserChecker
	auditor     auditdomain.EventRecorder
}

func NewFollowUpService(repo followupdomain.FollowUpRepository, caseRepo shared.CaseFinder, userChecker shared.UserChecker, auditor auditdomain.EventRecorder) *FollowUpService {
	return &FollowUpService{repo: repo, caseRepo: caseRepo, userChecker: userChecker, auditor: auditor}
}

type CreateFollowUpParams struct {
	OrganizationID   uuid.UUID
	ServiceRequestID uuid.UUID
	ScheduledDate    time.Time
	Outcome          string
	Notes            string
	ActorID          uuid.UUID
}

func (s *FollowUpService) CreateFollowUp(ctx context.Context, params CreateFollowUpParams) (*followupdomain.FollowUp, error) {
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

	if !followupdomain.IsValidCaseStatusForFollowUp(string(c.Status)) {
		return nil, fmt.Errorf("%w: case status %s does not allow follow-up", ErrFollowUpInput, c.Status)
	}

	valid, err := s.userChecker.BelongsToOrganization(ctx, params.OrganizationID, params.ActorID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate actor: %w", err)
	}
	if !valid {
		return nil, ErrUserNotFound
	}

	f, err := followupdomain.NewFollowUp(params.OrganizationID, params.ServiceRequestID, params.ActorID, params.ScheduledDate, params.Outcome, params.Notes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFollowUpInput, err)
	}

	var result *followupdomain.FollowUp
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.SaveTx(ctx, tx, f); err != nil {
			return fmt.Errorf("failed to save follow-up: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: f.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "follow_up.scheduled",
				Resource:       "follow_up",
				ResourceID:     shared.StrPtr(f.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"service_request_id": f.ServiceRequestID.String(),
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		result = f
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *FollowUpService) CompleteFollowUp(ctx context.Context, orgID, id uuid.UUID, completedDate time.Time, actorID uuid.UUID) (*followupdomain.FollowUp, error) {
	f, err := s.repo.FindByID(ctx, orgID, id)
	if err != nil {
		return nil, ErrFollowUpNotFound
	}

	f.Complete(completedDate)

	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.SaveTx(ctx, tx, f); err != nil {
			return fmt.Errorf("failed to update follow-up: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: f.OrganizationID,
				ActorID:        &actorID,
				Action:         "follow_up.completed",
				Resource:       "follow_up",
				ResourceID:     shared.StrPtr(f.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"service_request_id": f.ServiceRequestID.String(),
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

	return f, nil
}

func (s *FollowUpService) GetFollowUp(ctx context.Context, orgID, id uuid.UUID) (*followupdomain.FollowUp, error) {
	f, err := s.repo.FindByID(ctx, orgID, id)
	if err != nil {
		return nil, ErrFollowUpNotFound
	}
	return f, nil
}

func (s *FollowUpService) ListFollowUps(ctx context.Context, orgID, serviceRequestID uuid.UUID, limit, offset int) ([]*followupdomain.FollowUp, int, error) {
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
		return nil, 0, fmt.Errorf("failed to count follow-ups: %w", err)
	}

	items, err := s.repo.FindByServiceRequest(ctx, orgID, serviceRequestID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list follow-ups: %w", err)
	}

	return items, total, nil
}
