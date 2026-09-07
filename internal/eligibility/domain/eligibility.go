package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type EligibilityResult string

const (
	EligibilityResultEligible                EligibilityResult = "ELIGIBLE"
	EligibilityResultNotEligible             EligibilityResult = "NOT_ELIGIBLE"
	EligibilityResultRequiresMoreInformation EligibilityResult = "REQUIRES_MORE_INFORMATION"
)

var (
	ErrEligibilityNotFound     = errors.New("eligibility assessment not found")
	ErrEligibilityInvalidInput = errors.New("invalid eligibility input")
)

type Eligibility struct {
	ID               uuid.UUID              `json:"id"`
	OrganizationID   uuid.UUID              `json:"organization_id"`
	ServiceRequestID uuid.UUID              `json:"service_request_id"`
	Criteria         map[string]interface{} `json:"criteria"`
	Result           EligibilityResult      `json:"result"`
	Explanation      string                 `json:"explanation"`
	AssessedBy       uuid.UUID              `json:"assessed_by"`
	AssessedAt       time.Time              `json:"assessed_at"`
	CreatedAt        time.Time              `json:"created_at"`
}

func NewEligibility(orgID, serviceRequestID, assessedBy uuid.UUID, criteria map[string]interface{}, explanation string) (*Eligibility, error) {
	if criteria == nil {
		return nil, fmt.Errorf("%w: criteria are required", ErrEligibilityInvalidInput)
	}
	if explanation == "" {
		return nil, fmt.Errorf("%w: explanation is required", ErrEligibilityInvalidInput)
	}
	now := time.Now().UTC()
	return &Eligibility{
		ID:               uuid.New(),
		OrganizationID:   orgID,
		ServiceRequestID: serviceRequestID,
		Criteria:         criteria,
		Result:           EligibilityResultRequiresMoreInformation,
		Explanation:      explanation,
		AssessedBy:       assessedBy,
		AssessedAt:       now,
		CreatedAt:        now,
	}, nil
}

func (e *Eligibility) SetResult(result EligibilityResult) {
	e.Result = result
	e.AssessedAt = time.Now().UTC()
}
