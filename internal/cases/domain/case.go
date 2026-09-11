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
	ErrInvalidStateTransition  = errors.New("invalid state transition")
	ErrCaseNotFound            = errors.New("case not found")
	ErrCaseInvalidInput        = errors.New("invalid case input")
	ErrCaseNumberConflict      = errors.New("case number conflict")
	ErrCaseTenantViolation     = errors.New("person does not belong to organization")
	ErrCaseStatusContradiction = errors.New("case status contradicts workflow state")
)

type Case struct {
	ID                 uuid.UUID   `json:"id"`
	OrganizationID     uuid.UUID   `json:"organization_id"`
	CaseNumber         string      `json:"case_number"`
	Title              string      `json:"title"`
	Description        string      `json:"description"`
	Status             CaseStatus  `json:"status"`
	WorkflowState      string      `json:"workflow_state,omitempty"`
	ServiceType        ServiceType `json:"service_type"`
	Priority           Priority    `json:"priority"`
	PersonID           *uuid.UUID  `json:"person_id"`
	AssignedToID       *uuid.UUID  `json:"assigned_to_id"`
	CreatedByID        uuid.UUID   `json:"created_by"`
	CreatedAt          time.Time   `json:"created_at"`
	UpdatedAt          time.Time   `json:"updated_at"`
	ClosedAt           *time.Time  `json:"closed_at"`
	WorkflowInstanceID *uuid.UUID  `json:"workflow_instance_id"`
	Version            int         `json:"version"`
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

// ValidateConsistency verifies that the Case status matches the linked
// workflow instance's current state. When no workflow instance is linked,
// the check is skipped.
// The authoritative lifecycle state is the workflow current state. The
// Case.Status field is a denormalized mirror that must always agree.
// This function returns ErrCaseStatusContradiction when they disagree.
func (c *Case) ValidateConsistency(workflowState string) error {
	if c.WorkflowInstanceID == nil {
		return nil
	}
	if workflowState == "" {
		return nil
	}
	if string(c.Status) != workflowState {
		return fmt.Errorf("%w: case status %s does not match workflow state %s",
			ErrCaseStatusContradiction, c.Status, workflowState)
	}
	return nil
}

// StatusFromWorkflowState returns the CaseStatus derived from a workflow
// state key. Any non-empty state key is accepted so custom workflow
// definitions are not limited to a fixed legacy state set.
func StatusFromWorkflowState(state string) (CaseStatus, error) {
	if state == "" {
		return "", fmt.Errorf("workflow state cannot be empty")
	}
	return CaseStatus(state), nil
}

func (c *Case) AssignTo(userID uuid.UUID) {
	c.AssignedToID = &userID
	c.UpdatedAt = time.Now().UTC()
}

func (c *Case) RegenerateCaseNumber() {
	c.CaseNumber = GenerateCaseNumber(time.Now().UTC())
}

// SyncStatusFromWorkflow updates the Case status to match the given workflow
// state. This is called after a workflow transition to keep the denormalized
// status field in sync with the authoritative workflow state.
func (c *Case) SyncStatusFromWorkflow(state string) error {
	if state == "" {
		return nil
	}
	c.Status = CaseStatus(state)
	c.UpdatedAt = time.Now().UTC()
	if CaseStatus(state) == CaseStatusClosed {
		closedAt := time.Now().UTC()
		c.ClosedAt = &closedAt
	}
	return nil
}

func IsClosed(status CaseStatus) bool {
	return status == CaseStatusClosed
}

func IsValidStatus(status string) bool {
	return status != ""
}

func GenerateCaseNumber(t time.Time) string {
	return fmt.Sprintf("CAS-%s-%08d-%s", t.Format("20060102"), t.Nanosecond()%100000000, uuid.NewString()[:8])
}

// WorkflowKeyForServiceType maps a Case.ServiceType to the authoritative
// workflow definition key that governs its lifecycle. This is the single
// place where service types are bound to workflow definitions, ensuring
// service-type-aware workflows are selected consistently at case creation.
func WorkflowKeyForServiceType(serviceType ServiceType) string {
	switch serviceType {
	case ServiceTypeEmergency:
		return "emergency_assistance"
	case ServiceTypeMedical:
		return "medical_assistance"
	case ServiceTypeFinancial:
		return "financial_assistance"
	case ServiceTypeFood:
		return "food_assistance"
	case ServiceTypeShelter:
		return "shelter_assistance"
	case ServiceTypeEducation:
		return "education_assistance"
	case ServiceTypeTransport:
		return "transport_assistance"
	case ServiceTypeGeneral:
		return "general_assistance"
	default:
		return "general_assistance"
	}
}
