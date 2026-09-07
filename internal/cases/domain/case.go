package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type CaseStatus string

const (
	CaseStatusNew             CaseStatus = "NEW"
	CaseStatusOpen            CaseStatus = "OPEN"
	CaseStatusInReview        CaseStatus = "IN_REVIEW"
	CaseStatusAssessment      CaseStatus = "ASSESSMENT"
	CaseStatusDecisionPending CaseStatus = "DECISION_PENDING"
	CaseStatusApproved        CaseStatus = "APPROVED"
	CaseStatusRejected        CaseStatus = "REJECTED"
	CaseStatusInProgress      CaseStatus = "IN_PROGRESS"
	CaseStatusFollowUp        CaseStatus = "FOLLOW_UP"
	CaseStatusClosed          CaseStatus = "CLOSED"
)

type ServiceType string

const (
	ServiceTypeGeneral   ServiceType = "GENERAL"
	ServiceTypeEmergency ServiceType = "EMERGENCY"
	ServiceTypeFinancial ServiceType = "FINANCIAL"
	ServiceTypeFood      ServiceType = "FOOD"
	ServiceTypeShelter   ServiceType = "SHELTER"
	ServiceTypeMedical   ServiceType = "MEDICAL"
	ServiceTypeEducation ServiceType = "EDUCATION"
	ServiceTypeTransport ServiceType = "TRANSPORT"
)

type Priority string

const (
	PriorityLow    Priority = "LOW"
	PriorityNormal Priority = "NORMAL"
	PriorityHigh   Priority = "HIGH"
	PriorityUrgent Priority = "URGENT"
)

var (
	ErrInvalidStateTransition = errors.New("invalid state transition")
	ErrCaseNotFound           = errors.New("case not found")
	ErrCaseInvalidInput       = errors.New("invalid case input")
	ErrCaseNumberConflict     = errors.New("case number conflict")
	ErrCaseTenantViolation    = errors.New("person does not belong to organization")
)

type Case struct {
	ID             uuid.UUID   `json:"id"`
	OrganizationID uuid.UUID   `json:"organization_id"`
	CaseNumber     string      `json:"case_number"`
	Title          string      `json:"title"`
	Description    string      `json:"description"`
	Status         CaseStatus  `json:"status"`
	ServiceType    ServiceType `json:"service_type"`
	Priority       Priority    `json:"priority"`
	PersonID       *uuid.UUID  `json:"person_id"`
	CreatedByID    uuid.UUID   `json:"created_by"`
	AssignedToID   *uuid.UUID  `json:"assigned_to"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
	ClosedAt       *time.Time  `json:"closed_at"`
	Version        int         `json:"version"`
}

const (
	maxTitleLength       = 200
	maxDescriptionLength = 10000
)

func validateCaseInput(title, description string) error {
	if len(title) > maxTitleLength {
		return fmt.Errorf("%w: title exceeds maximum length of %d characters", ErrCaseInvalidInput, maxTitleLength)
	}
	if len(description) > maxDescriptionLength {
		return fmt.Errorf("%w: description exceeds maximum length of %d characters", ErrCaseInvalidInput, maxDescriptionLength)
	}
	return nil
}

func NewCase(orgID, createdByID uuid.UUID, title, description string, serviceType ServiceType, priority Priority, personID *uuid.UUID) (*Case, error) {
	if err := validateCaseInput(title, description); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	return &Case{
		ID:             uuid.New(),
		OrganizationID: orgID,
		CaseNumber:     GenerateCaseNumber(now),
		Title:          title,
		Description:    description,
		Status:         CaseStatusNew,
		ServiceType:    serviceType,
		Priority:       priority,
		PersonID:       personID,
		CreatedByID:    createdByID,
		CreatedAt:      now,
		UpdatedAt:      now,
		Version:        1,
	}, nil
}

func (c *Case) TransitionTo(status CaseStatus) error {
	if !IsValidTransition(c.Status, status) {
		return fmt.Errorf(
			"%w: cannot transition from %s to %s",
			ErrInvalidStateTransition, c.Status, status,
		)
	}
	c.Status = status
	c.UpdatedAt = time.Now().UTC()
	if status == CaseStatusClosed {
		closedAt := time.Now().UTC()
		c.ClosedAt = &closedAt
	}
	return nil
}

func (c *Case) AssignTo(userID uuid.UUID) {
	c.AssignedToID = &userID
	c.UpdatedAt = time.Now().UTC()
}

func (c *Case) RegenerateCaseNumber() {
	c.CaseNumber = GenerateCaseNumber(time.Now().UTC())
}

var transitionRules = map[CaseStatus][]CaseStatus{
	CaseStatusNew:             {CaseStatusOpen, CaseStatusInReview},
	CaseStatusOpen:            {CaseStatusInReview},
	CaseStatusInReview:        {CaseStatusAssessment, CaseStatusOpen},
	CaseStatusAssessment:      {CaseStatusDecisionPending},
	CaseStatusDecisionPending: {CaseStatusApproved, CaseStatusRejected},
	CaseStatusApproved:        {CaseStatusInProgress},
	CaseStatusRejected:        {CaseStatusClosed},
	CaseStatusInProgress:      {CaseStatusFollowUp},
	CaseStatusFollowUp:        {CaseStatusClosed},
	CaseStatusClosed:          {},
}

func IsValidTransition(from, to CaseStatus) bool {
	validStatuses := map[CaseStatus]bool{
		CaseStatusNew:             true,
		CaseStatusOpen:            true,
		CaseStatusInReview:        true,
		CaseStatusAssessment:      true,
		CaseStatusDecisionPending: true,
		CaseStatusApproved:        true,
		CaseStatusRejected:        true,
		CaseStatusInProgress:      true,
		CaseStatusFollowUp:        true,
		CaseStatusClosed:          true,
	}
	if !validStatuses[from] || !validStatuses[to] {
		return false
	}
	if from == to {
		return false
	}
	allowed, ok := transitionRules[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

func ValidTransitionsFrom(status CaseStatus) []CaseStatus {
	return transitionRules[status]
}

func IsValidStatus(status string) bool {
	switch status {
	case string(CaseStatusNew), string(CaseStatusOpen), string(CaseStatusInReview),
		string(CaseStatusAssessment), string(CaseStatusDecisionPending),
		string(CaseStatusApproved), string(CaseStatusRejected),
		string(CaseStatusInProgress), string(CaseStatusFollowUp), string(CaseStatusClosed):
		return true
	default:
		return false
	}
}

func GenerateCaseNumber(t time.Time) string {
	return fmt.Sprintf("CAS-%s-%08d-%s", t.Format("20060102"), t.Nanosecond()%100000000, uuid.NewString()[:8])
}
