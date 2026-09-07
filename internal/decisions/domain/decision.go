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
)

var (
	ErrDecisionNotFound     = errors.New("decision not found")
	ErrDecisionInvalidInput = errors.New("invalid decision input")
)

type Decision struct {
	ID               uuid.UUID    `json:"id"`
	OrganizationID   uuid.UUID    `json:"organization_id"`
	ServiceRequestID uuid.UUID    `json:"service_request_id"`
	Decision         DecisionType `json:"decision"`
	Reason           string       `json:"reason"`
	DecisionMaker    uuid.UUID    `json:"decision_maker"`
	DecidedAt        time.Time    `json:"decided_at"`
	CreatedAt        time.Time    `json:"created_at"`
}

func NewDecision(orgID, serviceRequestID, decisionMaker uuid.UUID, decision DecisionType, reason string) (*Decision, error) {
	if reason == "" {
		return nil, fmt.Errorf("%w: reason is required", ErrDecisionInvalidInput)
	}
	now := time.Now().UTC()
	return &Decision{
		ID:               uuid.New(),
		OrganizationID:   orgID,
		ServiceRequestID: serviceRequestID,
		Decision:         decision,
		Reason:           reason,
		DecisionMaker:    decisionMaker,
		DecidedAt:        now,
		CreatedAt:        now,
	}, nil
}
