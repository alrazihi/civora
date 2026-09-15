package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/alrazihi/civora/internal/cases/domain"
	"github.com/google/uuid"
)

type ReviewStatus string

const (
	ReviewStatusPending     ReviewStatus = "PENDING"
	ReviewStatusAssigned    ReviewStatus = "ASSIGNED"
	ReviewStatusInReview    ReviewStatus = "IN_REVIEW"
	ReviewStatusCompleted   ReviewStatus = "COMPLETED"
	ReviewStatusEscalated   ReviewStatus = "ESCALATED"
	ReviewStatusWaitingInfo ReviewStatus = "WAITING_INFORMATION"
)

var (
	ErrReviewNotFound     = errors.New("review queue entry not found")
	ErrReviewInvalidInput = errors.New("invalid review input")
	ErrReviewNotPending   = errors.New("review is not pending")
	ErrReviewNotAssigned  = errors.New("review is not assigned to the caller")
	ErrReviewNotInReview  = errors.New("review is not in review")
	ErrReviewAlreadyFinal = errors.New("review is already in a final state")
)

type ReviewQueueEntry struct {
	ID                 uuid.UUID
	OrganizationID     uuid.UUID
	CaseID             uuid.UUID
	WorkflowInstanceID uuid.UUID
	Status             ReviewStatus
	AssignedToID       *uuid.UUID
	Priority           domain.Priority
	WorkflowState      string
	RuleEvaluationIDs  []uuid.UUID
	EvidenceIDs        []uuid.UUID
	MissingInformation []string
	FormSubmissionID   *uuid.UUID
	CreatedAt          time.Time
	UpdatedAt          time.Time
	CompletedAt        *time.Time
	Metadata           map[string]interface{}
}

func NewReviewQueueEntry(orgID, caseID, workflowInstanceID uuid.UUID, workflowState string, priority domain.Priority, ruleEvalIDs []uuid.UUID, evidenceIDs []uuid.UUID, formSubmissionID *uuid.UUID) *ReviewQueueEntry {
	now := time.Now().UTC()
	if ruleEvalIDs == nil {
		ruleEvalIDs = []uuid.UUID{}
	}
	if evidenceIDs == nil {
		evidenceIDs = []uuid.UUID{}
	}
	return &ReviewQueueEntry{
		ID:                 uuid.New(),
		OrganizationID:     orgID,
		CaseID:             caseID,
		WorkflowInstanceID: workflowInstanceID,
		Status:             ReviewStatusPending,
		Priority:           priority,
		WorkflowState:      workflowState,
		RuleEvaluationIDs:  ruleEvalIDs,
		EvidenceIDs:        evidenceIDs,
		MissingInformation: []string{},
		FormSubmissionID:   formSubmissionID,
		CreatedAt:          now,
		UpdatedAt:          now,
		Metadata:           map[string]interface{}{},
	}
}

func (r *ReviewQueueEntry) CanTransition(to ReviewStatus) bool {
	switch r.Status {
	case ReviewStatusPending:
		return to == ReviewStatusAssigned
	case ReviewStatusAssigned:
		return to == ReviewStatusInReview || to == ReviewStatusPending
	case ReviewStatusInReview:
		return to == ReviewStatusCompleted || to == ReviewStatusEscalated || to == ReviewStatusWaitingInfo || to == ReviewStatusAssigned
	case ReviewStatusWaitingInfo:
		return to == ReviewStatusInReview || to == ReviewStatusAssigned || to == ReviewStatusPending
	case ReviewStatusCompleted, ReviewStatusEscalated:
		return false
	}
	return false
}

func (r *ReviewQueueEntry) Transition(to ReviewStatus, assignedToID *uuid.UUID) error {
	if !r.CanTransition(to) {
		return fmt.Errorf("%w: cannot transition from %s to %s", ErrReviewInvalidInput, r.Status, to)
	}
	r.Status = to
	r.UpdatedAt = time.Now().UTC()
	if assignedToID != nil {
		r.AssignedToID = assignedToID
	}
	if to == ReviewStatusCompleted || to == ReviewStatusEscalated {
		now := time.Now().UTC()
		r.CompletedAt = &now
	}
	return nil
}

func (r *ReviewQueueEntry) IsFinal() bool {
	return r.Status == ReviewStatusCompleted || r.Status == ReviewStatusEscalated
}

func ValidateReviewStatus(s ReviewStatus) error {
	switch s {
	case ReviewStatusPending, ReviewStatusAssigned, ReviewStatusInReview,
		ReviewStatusCompleted, ReviewStatusEscalated, ReviewStatusWaitingInfo:
		return nil
	}
	return fmt.Errorf("%w: invalid review status %s", ErrReviewInvalidInput, s)
}
