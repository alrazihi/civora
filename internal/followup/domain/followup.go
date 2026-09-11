package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrFollowUpNotFound     = errors.New("follow-up not found")
	ErrFollowUpInvalidInput = errors.New("invalid follow-up input")
)

type FollowUp struct {
	ID               uuid.UUID  `json:"id"`
	OrganizationID   uuid.UUID  `json:"organization_id"`
	ServiceRequestID uuid.UUID  `json:"service_request_id"`
	ScheduledDate    time.Time  `json:"scheduled_date"`
	CompletedDate    *time.Time `json:"completed_date"`
	Outcome          string     `json:"outcome"`
	Notes            string     `json:"notes"`
	PerformedBy      uuid.UUID  `json:"performed_by"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func NewFollowUp(orgID, serviceRequestID, performedBy uuid.UUID, scheduledDate time.Time, outcome, notes string) (*FollowUp, error) {
	if len(outcome) > 500 {
		return nil, fmt.Errorf("%w: outcome exceeds maximum length of 500 characters", ErrFollowUpInvalidInput)
	}
	if outcome == "" {
		return nil, fmt.Errorf("%w: outcome is required", ErrFollowUpInvalidInput)
	}
	if len(notes) > 5000 {
		return nil, fmt.Errorf("%w: notes exceed maximum length of 5000 characters", ErrFollowUpInvalidInput)
	}
	if notes == "" {
		return nil, fmt.Errorf("%w: notes are required", ErrFollowUpInvalidInput)
	}
	now := time.Now().UTC()
	return &FollowUp{
		ID:               uuid.New(),
		OrganizationID:   orgID,
		ServiceRequestID: serviceRequestID,
		ScheduledDate:    scheduledDate,
		Outcome:          outcome,
		Notes:            notes,
		PerformedBy:      performedBy,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

func (f *FollowUp) Complete(completedDate time.Time) {
	f.CompletedDate = &completedDate
	f.UpdatedAt = time.Now().UTC()
}
