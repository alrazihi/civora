package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type AssistanceType string

const (
	AssistanceTypeFinancial AssistanceType = "FINANCIAL"
	AssistanceTypeFood      AssistanceType = "FOOD"
	AssistanceTypeShelter   AssistanceType = "SHELTER"
	AssistanceTypeMedical   AssistanceType = "MEDICAL"
	AssistanceTypeEducation AssistanceType = "EDUCATION"
	AssistanceTypeTransport AssistanceType = "TRANSPORT"
	AssistanceTypeOther     AssistanceType = "OTHER"
)

type AssistanceStatus string

const (
	AssistanceStatusPlanned    AssistanceStatus = "PLANNED"
	AssistanceStatusInProgress AssistanceStatus = "IN_PROGRESS"
	AssistanceStatusCompleted  AssistanceStatus = "COMPLETED"
	AssistanceStatusCancelled  AssistanceStatus = "CANCELLED"
)

var (
	ErrAssistanceNotFound     = errors.New("assistance not found")
	ErrAssistanceInvalidInput = errors.New("invalid assistance input")
)

type Assistance struct {
	ID               uuid.UUID        `json:"id"`
	OrganizationID   uuid.UUID        `json:"organization_id"`
	ServiceRequestID uuid.UUID        `json:"service_request_id"`
	Type             AssistanceType   `json:"type"`
	Description      string           `json:"description"`
	Status           AssistanceStatus `json:"status"`
	ResponsibleStaff uuid.UUID        `json:"responsible_staff"`
	StartedAt        *time.Time       `json:"started_at"`
	CompletedAt      *time.Time       `json:"completed_at"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
}

func NewAssistance(orgID, serviceRequestID, responsibleStaff uuid.UUID, assistanceType AssistanceType, description string) (*Assistance, error) {
	if len(description) > 5000 {
		return nil, fmt.Errorf("%w: description exceeds maximum length of 5000 characters", ErrAssistanceInvalidInput)
	}
	if description == "" {
		return nil, fmt.Errorf("%w: description is required", ErrAssistanceInvalidInput)
	}
	now := time.Now().UTC()
	return &Assistance{
		ID:               uuid.New(),
		OrganizationID:   orgID,
		ServiceRequestID: serviceRequestID,
		Type:             assistanceType,
		Description:      description,
		Status:           AssistanceStatusPlanned,
		ResponsibleStaff: responsibleStaff,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

func (a *Assistance) Start() {
	a.Status = AssistanceStatusInProgress
	now := time.Now().UTC()
	a.StartedAt = &now
	a.UpdatedAt = now
}

func (a *Assistance) Complete() {
	a.Status = AssistanceStatusCompleted
	now := time.Now().UTC()
	a.CompletedAt = &now
	a.UpdatedAt = now
}

func (a *Assistance) Cancel() {
	a.Status = AssistanceStatusCancelled
	a.UpdatedAt = time.Now().UTC()
}

func IsValidStatusTransition(from AssistanceStatus, action string) bool {
	switch from {
	case AssistanceStatusPlanned:
		return action == "start" || action == "cancel"
	case AssistanceStatusInProgress:
		return action == "complete" || action == "cancel"
	case AssistanceStatusCompleted, AssistanceStatusCancelled:
		return false
	default:
		return false
	}
}
