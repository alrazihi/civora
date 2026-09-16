package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type DecisionType string

const (
	DecisionTypeApproved             DecisionType = "APPROVED"
	DecisionTypeRejected             DecisionType = "REJECTED"
	DecisionTypeNeedsMoreInformation DecisionType = "NEEDS_MORE_INFORMATION"
	DecisionTypeEscalate             DecisionType = "ESCALATE"
)

var (
	ErrDecisionNotFound     = errors.New("decision not found")
	ErrDecisionInvalidInput = errors.New("invalid decision input")
)

type Decision struct {
	ID                 uuid.UUID    `json:"id"`
	OrganizationID     uuid.UUID    `json:"organization_id"`
	ServiceRequestID   uuid.UUID    `json:"service_request_id"`
	Decision           DecisionType `json:"decision"`
	Reason             string       `json:"reason"`
	DecisionMaker      uuid.UUID    `json:"decision_maker"`
	DecidedAt          time.Time    `json:"decided_at"`
	CreatedAt          time.Time    `json:"created_at"`
	SupersededByID     *uuid.UUID   `json:"superseded_by_id,omitempty"`
	WorkflowState      string       `json:"workflow_state"`
	RuleEvaluationIDs  []uuid.UUID  `json:"rule_evaluation_ids,omitempty"`
	EvidenceIDs        []uuid.UUID  `json:"evidence_ids,omitempty"`
	FormSubmissionID   *uuid.UUID   `json:"form_submission_id,omitempty"`
	ReviewQueueEntryID *uuid.UUID   `json:"review_queue_entry_id,omitempty"`
	Version            int          `json:"version"`
}

func NewDecision(orgID, serviceRequestID, decisionMaker uuid.UUID, decision DecisionType, reason string) (*Decision, error) {
	return NewDecisionWithContext(orgID, serviceRequestID, decisionMaker, decision, reason, "", nil, nil, nil, nil, 1)
}

type DecisionOptions struct {
	WorkflowState     string
	RuleEvaluationIDs []uuid.UUID
	EvidenceIDs       []uuid.UUID
	FormSubmissionID  *uuid.UUID
	Version           int
}

func NewDecisionWithContext(orgID, serviceRequestID, decisionMaker uuid.UUID, decision DecisionType, reason string, workflowState string, ruleEvalIDs []uuid.UUID, evidenceIDs []uuid.UUID, formSubmissionID *uuid.UUID, reviewQueueEntryID *uuid.UUID, version int) (*Decision, error) {
	if len(reason) > 5000 {
		return nil, fmt.Errorf("%w: reason exceeds maximum length of 5000 characters", ErrDecisionInvalidInput)
	}
	if ReasonRequired(decision) && reason == "" {
		return nil, fmt.Errorf("%w: reason is required for decision type %s", ErrDecisionInvalidInput, decision)
	}
	now := time.Now().UTC()
	if version < 1 {
		version = 1
	}
	if ruleEvalIDs == nil {
		ruleEvalIDs = []uuid.UUID{}
	}
	if evidenceIDs == nil {
		evidenceIDs = []uuid.UUID{}
	}
	return &Decision{
		ID:                 uuid.New(),
		OrganizationID:     orgID,
		ServiceRequestID:   serviceRequestID,
		Decision:           decision,
		Reason:             reason,
		DecisionMaker:      decisionMaker,
		DecidedAt:          now,
		CreatedAt:          now,
		WorkflowState:      workflowState,
		RuleEvaluationIDs:  ruleEvalIDs,
		EvidenceIDs:        evidenceIDs,
		FormSubmissionID:   formSubmissionID,
		ReviewQueueEntryID: reviewQueueEntryID,
		Version:            version,
	}, nil
}

func ReasonRequired(decision DecisionType) bool {
	switch decision {
	case DecisionTypeRejected, DecisionTypeEscalate, DecisionTypeNeedsMoreInformation:
		return true
	case DecisionTypeApproved:
		return false
	default:
		return true
	}
}

func NewSupersedingDecision(orgID, serviceRequestID, decisionMaker uuid.UUID, decision DecisionType, reason string, supersededDecision *Decision, workflowState string, ruleEvalIDs []uuid.UUID, evidenceIDs []uuid.UUID, formSubmissionID *uuid.UUID, reviewQueueEntryID *uuid.UUID) (*Decision, error) {
	version := 1
	if supersededDecision != nil {
		version = supersededDecision.Version + 1
	}
	d, err := NewDecisionWithContext(orgID, serviceRequestID, decisionMaker, decision, reason, workflowState, ruleEvalIDs, evidenceIDs, formSubmissionID, reviewQueueEntryID, version)
	if err != nil {
		return nil, err
	}
	supersededByID := d.ID
	d.SupersededByID = &supersededByID
	return d, nil
}
