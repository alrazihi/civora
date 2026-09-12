package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrSubmissionNotFound     = errors.New("submission not found")
	ErrSubmissionExists       = errors.New("submission already exists for this case and form version")
	ErrInvalidSubmissionData  = errors.New("invalid submission data")
	ErrUnknownFields          = errors.New("submission contains unknown fields")
	ErrFieldValidationFailed  = errors.New("field validation failed")
	ErrOptionValidationFailed = errors.New("option validation failed")
	ErrFormNotAssigned        = errors.New("form is not assigned to the current workflow state")
	ErrFormNotPublished       = errors.New("form version is not published")
	ErrFormArchived           = errors.New("form is archived")
	ErrTenantMismatch         = errors.New("tenant mismatch")
	ErrInvalidStatus          = errors.New("invalid submission status")
	ErrConcurrentModification = errors.New("concurrent modification detected")
)

type SubmissionStatus string

const (
	SubmissionStatusSubmitted SubmissionStatus = "submitted"
	SubmissionStatusRejected  SubmissionStatus = "rejected"
	SubmissionStatusCorrected SubmissionStatus = "corrected"
	SubmissionStatusApproved  SubmissionStatus = "approved"
)

type FormSubmission struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	CaseID        uuid.UUID
	FormID        uuid.UUID
	FormVersionID uuid.UUID
	SubmittedBy   uuid.UUID
	Status        SubmissionStatus
	Data          map[string]interface{}
	SubmittedAt   time.Time
	UpdatedAt     time.Time
}

func NewFormSubmission(
	tenantID, caseID, formID, formVersionID, submittedBy uuid.UUID,
	data map[string]interface{},
	status SubmissionStatus,
) (*FormSubmission, error) {
	if tenantID == uuid.Nil {
		return nil, errors.New("tenant ID is required")
	}
	if caseID == uuid.Nil {
		return nil, errors.New("case ID is required")
	}
	if formID == uuid.Nil {
		return nil, errors.New("form ID is required")
	}
	if formVersionID == uuid.Nil {
		return nil, errors.New("form version ID is required")
	}
	if submittedBy == uuid.Nil {
		return nil, errors.New("submitted by is required")
	}
	if data == nil {
		return nil, errors.New("submission data is required")
	}
	if status == "" {
		status = SubmissionStatusSubmitted
	}

	now := time.Now().UTC()
	return &FormSubmission{
		ID:            uuid.New(),
		TenantID:      tenantID,
		CaseID:        caseID,
		FormID:        formID,
		FormVersionID: formVersionID,
		SubmittedBy:   submittedBy,
		Status:        status,
		Data:          data,
		SubmittedAt:   now,
		UpdatedAt:     now,
	}, nil
}
