package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrAssessmentNotFound     = errors.New("assessment not found")
	ErrAssessmentInvalidInput = errors.New("invalid assessment input")
)

type Assessment struct {
	ID               uuid.UUID `json:"id"`
	OrganizationID   uuid.UUID `json:"organization_id"`
	ServiceRequestID uuid.UUID `json:"service_request_id"`
	Findings         string    `json:"findings"`
	NeedsIdentified  string    `json:"needs_identified"`
	Recommendation   string    `json:"recommendation"`
	Assessor         uuid.UUID `json:"assessor"`
	AssessedAt       time.Time `json:"assessed_at"`
	CreatedAt        time.Time `json:"created_at"`
}

func NewAssessment(orgID, serviceRequestID, assessor uuid.UUID, findings, needsIdentified, recommendation string) (*Assessment, error) {
	if findings == "" {
		return nil, fmt.Errorf("%w: findings are required", ErrAssessmentInvalidInput)
	}
	if recommendation == "" {
		return nil, fmt.Errorf("%w: recommendation is required", ErrAssessmentInvalidInput)
	}
	now := time.Now().UTC()
	return &Assessment{
		ID:               uuid.New(),
		OrganizationID:   orgID,
		ServiceRequestID: serviceRequestID,
		Findings:         findings,
		NeedsIdentified:  needsIdentified,
		Recommendation:   recommendation,
		Assessor:         assessor,
		AssessedAt:       now,
		CreatedAt:        now,
	}, nil
}
