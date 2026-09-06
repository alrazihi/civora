package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type CaseStatus string

const (
	CaseStatusCreated  CaseStatus = "CREATED"
	CaseStatusOpen     CaseStatus = "OPEN"
	CaseStatusInReview CaseStatus = "IN_REVIEW"
	CaseStatusResolved CaseStatus = "RESOLVED"
	CaseStatusClosed   CaseStatus = "CLOSED"
)

type Case struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	CaseNumber     string     `json:"case_number"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	Status         CaseStatus `json:"status"`
	CreatedByID    uuid.UUID  `json:"created_by"`
	AssignedToID   *uuid.UUID `json:"assigned_to"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ClosedAt       *time.Time `json:"closed_at"`
}

func NewCase(orgID, createdByID uuid.UUID, title, description string) *Case {
	now := time.Now().UTC()
	return &Case{
		ID:             uuid.New(),
		OrganizationID: orgID,
		CaseNumber:     generateCaseNumber(now),
		Title:          title,
		Description:    description,
		Status:         CaseStatusCreated,
		CreatedByID:    createdByID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
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

var transitionRules = map[CaseStatus][]CaseStatus{
	CaseStatusCreated:  {CaseStatusOpen},
	CaseStatusOpen:     {CaseStatusInReview},
	CaseStatusInReview: {CaseStatusOpen, CaseStatusResolved},
	CaseStatusResolved: {CaseStatusClosed, CaseStatusInReview},
	CaseStatusClosed:   {},
}

func IsValidTransition(from, to CaseStatus) bool {
	validStatuses := map[CaseStatus]bool{
		CaseStatusCreated:  true,
		CaseStatusOpen:     true,
		CaseStatusInReview: true,
		CaseStatusResolved: true,
		CaseStatusClosed:   true,
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

func generateCaseNumber(t time.Time) string {
	return fmt.Sprintf("CAS-%s-%08d-%s", t.Format("20060102"), t.Nanosecond()%100000000, uuid.NewString()[:8])
}

var (
	ErrInvalidStateTransition = errors.New("invalid state transition")
	ErrCaseNotFound           = errors.New("case not found")
	ErrCaseInvalidInput       = errors.New("invalid case input")
)
