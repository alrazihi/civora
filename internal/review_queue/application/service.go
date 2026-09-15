package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/cases/domain"
	"github.com/alrazihi/civora/internal/database"
	decisionsdomain "github.com/alrazihi/civora/internal/decisions/domain"
	reviewdomain "github.com/alrazihi/civora/internal/review_queue/domain"
	"github.com/alrazihi/civora/internal/shared"
	workflowapp "github.com/alrazihi/civora/internal/workflow/application"
	workflowdomain "github.com/alrazihi/civora/internal/workflow/domain"
	"github.com/google/uuid"
)

var (
	ErrReviewNotFound     = errors.New("review queue entry not found")
	ErrReviewInvalidInput = errors.New("invalid review input")
	ErrReviewNotPending   = errors.New("review is not pending")
	ErrReviewNotAssigned  = errors.New("review is not assigned to the caller")
	ErrReviewNotInReview  = errors.New("review is not in review")
	ErrReviewAlreadyFinal = errors.New("review is already in a final state")
	ErrCaseNotFound       = errors.New("case not found")
	ErrUserNotFound       = errors.New("user not found")
)

type UserChecker interface {
	BelongsToOrganization(ctx context.Context, orgID, userID uuid.UUID) (bool, error)
}

type CaseFinder interface {
	FindByID(ctx context.Context, orgID, caseID uuid.UUID) (*domain.Case, error)
}

type WorkflowExecutor interface {
	GetInstanceByCaseID(ctx context.Context, tenantID, caseID uuid.UUID) (*workflowdomain.WorkflowInstance, error)
	GetValidTransitions(ctx context.Context, tenantID, instanceID uuid.UUID) ([]workflowdomain.WorkflowTransition, error)
	ExecuteTransitionInTx(ctx context.Context, tx *sql.Tx, params workflowapp.ExecuteTransitionParams) (*workflowdomain.WorkflowInstance, error)
}

type DecisionCreator interface {
	CreateDecisionTx(ctx context.Context, tx *sql.Tx, orgID, serviceRequestID, decisionMaker uuid.UUID, decision decisionsdomain.DecisionType, reason, workflowState string, ruleEvalIDs, evidenceIDs []uuid.UUID, formSubmissionID *uuid.UUID) (*decisionsdomain.Decision, error)
}

type ReviewQueueService struct {
	repo        reviewdomain.ReviewQueueRepository
	caseFinder  CaseFinder
	userChecker UserChecker
	auditor     auditdomain.EventRecorder
	workflowSvc WorkflowExecutor
	decisionSvc DecisionCreator
}

func NewReviewQueueService(
	repo reviewdomain.ReviewQueueRepository,
	caseFinder CaseFinder,
	userChecker UserChecker,
	auditor auditdomain.EventRecorder,
	workflowSvc WorkflowExecutor,
	decisionSvc DecisionCreator,
) *ReviewQueueService {
	return &ReviewQueueService{
		repo:        repo,
		caseFinder:  caseFinder,
		userChecker: userChecker,
		auditor:     auditor,
		workflowSvc: workflowSvc,
		decisionSvc: decisionSvc,
	}
}

type GetQueueParams struct {
	OrganizationID uuid.UUID
	Status         *reviewdomain.ReviewStatus
	AssignedToID   *uuid.UUID
	Limit          int
	Offset         int
}

func (s *ReviewQueueService) GetQueue(ctx context.Context, params GetQueueParams) ([]*reviewdomain.ReviewQueueEntry, int, error) {
	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.Limit > 200 {
		params.Limit = 200
	}
	if params.Offset < 0 {
		params.Offset = 0
	}

	items, total, err := s.repo.ListByOrganization(ctx, params.OrganizationID, params.Status, params.AssignedToID, params.Limit, params.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list review queue: %w", err)
	}
	return items, total, nil
}

type ClaimReviewParams struct {
	OrganizationID uuid.UUID
	ReviewID       uuid.UUID
	ReviewerID     uuid.UUID
}

func (s *ReviewQueueService) ClaimReview(ctx context.Context, params ClaimReviewParams) (*reviewdomain.ReviewQueueEntry, error) {
	entry, err := s.repo.FindByID(ctx, params.OrganizationID, params.ReviewID)
	if err != nil {
		return nil, ErrReviewNotFound
	}

	c, err := s.caseFinder.FindByID(ctx, params.OrganizationID, entry.CaseID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCaseNotFound, err)
	}
	if c.OrganizationID != params.OrganizationID {
		return nil, ErrCaseNotFound
	}

	valid, err := s.userChecker.BelongsToOrganization(ctx, params.OrganizationID, params.ReviewerID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate reviewer: %w", err)
	}
	if !valid {
		return nil, ErrUserNotFound
	}

	var result *reviewdomain.ReviewQueueEntry
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		entry, err := s.repo.FindByIDTx(ctx, tx, params.OrganizationID, params.ReviewID)
		if err != nil {
			return err
		}

		if entry.Status != reviewdomain.ReviewStatusPending {
			return ErrReviewNotPending
		}

		if err := entry.Transition(reviewdomain.ReviewStatusAssigned, &params.ReviewerID); err != nil {
			return err
		}

		if err := s.repo.UpdateStatusTx(ctx, tx, params.OrganizationID, entry.ID, reviewdomain.ReviewStatusAssigned, &params.ReviewerID); err != nil {
			return fmt.Errorf("failed to update review status: %w", err)
		}

		if s.auditor != nil {
			reviewIDStr := entry.ID.String()
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: params.OrganizationID,
				ActorID:        &params.ReviewerID,
				Action:         "review.claimed",
				Resource:       "review_queue",
				ResourceID:     &reviewIDStr,
				Outcome:        "success",
				Metadata: map[string]interface{}{
					"case_id": entry.CaseID.String(),
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		result = entry
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

type StartReviewParams struct {
	OrganizationID uuid.UUID
	ReviewID       uuid.UUID
	ReviewerID     uuid.UUID
}

func (s *ReviewQueueService) StartReview(ctx context.Context, params StartReviewParams) (*reviewdomain.ReviewQueueEntry, error) {
	entry, err := s.repo.FindByID(ctx, params.OrganizationID, params.ReviewID)
	if err != nil {
		return nil, ErrReviewNotFound
	}

	if entry.AssignedToID == nil || *entry.AssignedToID != params.ReviewerID {
		return nil, ErrReviewNotAssigned
	}

	if err := entry.Transition(reviewdomain.ReviewStatusInReview, entry.AssignedToID); err != nil {
		return nil, err
	}

	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.UpdateStatusTx(ctx, tx, params.OrganizationID, entry.ID, reviewdomain.ReviewStatusInReview, entry.AssignedToID); err != nil {
			return fmt.Errorf("failed to update review status: %w", err)
		}

		if s.auditor != nil {
			reviewIDStr := entry.ID.String()
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: params.OrganizationID,
				ActorID:        &params.ReviewerID,
				Action:         "review.started",
				Resource:       "review_queue",
				ResourceID:     &reviewIDStr,
				Outcome:        "success",
				Metadata: map[string]interface{}{
					"case_id": entry.CaseID.String(),
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

	return entry, nil
}

type CompleteReviewParams struct {
	OrganizationID uuid.UUID
	ReviewID       uuid.UUID
	ReviewerID     uuid.UUID
	Decision       string
	Reason         string
}

func (s *ReviewQueueService) CompleteReview(ctx context.Context, params CompleteReviewParams) (*reviewdomain.ReviewQueueEntry, error) {
	entry, err := s.repo.FindByID(ctx, params.OrganizationID, params.ReviewID)
	if err != nil {
		return nil, ErrReviewNotFound
	}

	if entry.AssignedToID == nil || *entry.AssignedToID != params.ReviewerID {
		return nil, ErrReviewNotAssigned
	}

	if entry.Status != reviewdomain.ReviewStatusInReview && entry.Status != reviewdomain.ReviewStatusAssigned {
		return nil, ErrReviewNotInReview
	}

	if err := entry.Transition(reviewdomain.ReviewStatusCompleted, entry.AssignedToID); err != nil {
		return nil, err
	}

	workflowState := entry.WorkflowState
	if s.workflowSvc != nil {
		if inst, wfErr := s.workflowSvc.GetInstanceByCaseID(ctx, params.OrganizationID, entry.CaseID); wfErr == nil && inst != nil {
			workflowState = inst.CurrentState
		}
	}

	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.UpdateStatusTx(ctx, tx, params.OrganizationID, entry.ID, reviewdomain.ReviewStatusCompleted, entry.AssignedToID); err != nil {
			return fmt.Errorf("failed to update review status: %w", err)
		}

		if s.decisionSvc != nil {
			decisionType := decisionsdomain.DecisionType(params.Decision)
			if err := validateDecisionType(decisionType); err != nil {
				return err
			}
			if _, err := s.decisionSvc.CreateDecisionTx(ctx, tx, params.OrganizationID, entry.CaseID, params.ReviewerID, decisionType, params.Reason, workflowState, entry.RuleEvaluationIDs, nil, nil); err != nil {
				return fmt.Errorf("failed to create decision: %w", err)
			}
		}

		if s.auditor != nil {
			reviewIDStr := entry.ID.String()
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: params.OrganizationID,
				ActorID:        &params.ReviewerID,
				Action:         "review.completed",
				Resource:       "review_queue",
				ResourceID:     &reviewIDStr,
				Outcome:        "success",
				Metadata: map[string]interface{}{
					"case_id":  entry.CaseID.String(),
					"decision": params.Decision,
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

	return entry, nil
}

func validateDecisionType(d decisionsdomain.DecisionType) error {
	switch d {
	case decisionsdomain.DecisionTypeApproved, decisionsdomain.DecisionTypeRejected,
		decisionsdomain.DecisionTypeNeedsMoreInformation, decisionsdomain.DecisionTypeEscalate:
		return nil
	default:
		return fmt.Errorf("%w: invalid decision type %q", ErrReviewInvalidInput, d)
	}
}

type EscalateReviewParams struct {
	OrganizationID uuid.UUID
	ReviewID       uuid.UUID
	ReviewerID     uuid.UUID
	Reason         string
}

func (s *ReviewQueueService) EscalateReview(ctx context.Context, params EscalateReviewParams) (*reviewdomain.ReviewQueueEntry, error) {
	entry, err := s.repo.FindByID(ctx, params.OrganizationID, params.ReviewID)
	if err != nil {
		return nil, ErrReviewNotFound
	}

	if entry.AssignedToID == nil || *entry.AssignedToID != params.ReviewerID {
		return nil, ErrReviewNotAssigned
	}

	if entry.Status != reviewdomain.ReviewStatusInReview && entry.Status != reviewdomain.ReviewStatusAssigned {
		return nil, ErrReviewNotInReview
	}

	if err := entry.Transition(reviewdomain.ReviewStatusEscalated, entry.AssignedToID); err != nil {
		return nil, err
	}

	workflowState := entry.WorkflowState
	if s.workflowSvc != nil {
		if inst, wfErr := s.workflowSvc.GetInstanceByCaseID(ctx, params.OrganizationID, entry.CaseID); wfErr == nil && inst != nil {
			workflowState = inst.CurrentState
		}
	}

	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.UpdateStatusTx(ctx, tx, params.OrganizationID, entry.ID, reviewdomain.ReviewStatusEscalated, entry.AssignedToID); err != nil {
			return fmt.Errorf("failed to update review status: %w", err)
		}

		if s.decisionSvc != nil {
			if _, err := s.decisionSvc.CreateDecisionTx(ctx, tx, params.OrganizationID, entry.CaseID, params.ReviewerID, decisionsdomain.DecisionTypeEscalate, params.Reason, workflowState, entry.RuleEvaluationIDs, nil, nil); err != nil {
				return fmt.Errorf("failed to create decision: %w", err)
			}
		}

		if s.auditor != nil {
			reviewIDStr := entry.ID.String()
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: params.OrganizationID,
				ActorID:        &params.ReviewerID,
				Action:         "review.escalated",
				Resource:       "review_queue",
				ResourceID:     &reviewIDStr,
				Outcome:        "success",
				Metadata: map[string]interface{}{
					"case_id": entry.CaseID.String(),
					"reason":  params.Reason,
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

	return entry, nil
}

type RequestInformationParams struct {
	OrganizationID uuid.UUID
	ReviewID       uuid.UUID
	ReviewerID     uuid.UUID
	MissingFields  []string
	Reason         string
}

func (s *ReviewQueueService) RequestInformation(ctx context.Context, params RequestInformationParams) (*reviewdomain.ReviewQueueEntry, error) {
	entry, err := s.repo.FindByID(ctx, params.OrganizationID, params.ReviewID)
	if err != nil {
		return nil, ErrReviewNotFound
	}

	if entry.AssignedToID == nil || *entry.AssignedToID != params.ReviewerID {
		return nil, ErrReviewNotAssigned
	}

	if entry.Status != reviewdomain.ReviewStatusInReview && entry.Status != reviewdomain.ReviewStatusAssigned {
		return nil, ErrReviewNotInReview
	}

	if err := entry.Transition(reviewdomain.ReviewStatusWaitingInfo, entry.AssignedToID); err != nil {
		return nil, err
	}
	entry.MissingInformation = params.MissingFields

	workflowState := entry.WorkflowState
	if s.workflowSvc != nil {
		if inst, wfErr := s.workflowSvc.GetInstanceByCaseID(ctx, params.OrganizationID, entry.CaseID); wfErr == nil && inst != nil {
			workflowState = inst.CurrentState
		}
	}

	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.UpdateStatusTx(ctx, tx, params.OrganizationID, entry.ID, reviewdomain.ReviewStatusWaitingInfo, entry.AssignedToID); err != nil {
			return fmt.Errorf("failed to update review status: %w", err)
		}

		if s.decisionSvc != nil {
			if _, err := s.decisionSvc.CreateDecisionTx(ctx, tx, params.OrganizationID, entry.CaseID, params.ReviewerID, decisionsdomain.DecisionTypeNeedsMoreInformation, params.Reason, workflowState, entry.RuleEvaluationIDs, nil, nil); err != nil {
				return fmt.Errorf("failed to create decision: %w", err)
			}
		}

		if s.auditor != nil {
			reviewIDStr := entry.ID.String()
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: params.OrganizationID,
				ActorID:        &params.ReviewerID,
				Action:         "review.requested_information",
				Resource:       "review_queue",
				ResourceID:     &reviewIDStr,
				Outcome:        "success",
				Metadata: map[string]interface{}{
					"case_id":        entry.CaseID.String(),
					"missing_fields": params.MissingFields,
					"reason":         params.Reason,
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

	return entry, nil
}

func (s *ReviewQueueService) GetReview(ctx context.Context, orgID, reviewID uuid.UUID) (*reviewdomain.ReviewQueueEntry, error) {
	entry, err := s.repo.FindByID(ctx, orgID, reviewID)
	if err != nil {
		return nil, ErrReviewNotFound
	}
	return entry, nil
}

func (s *ReviewQueueService) Save(ctx context.Context, entry *reviewdomain.ReviewQueueEntry) error {
	return s.repo.Save(ctx, entry)
}
