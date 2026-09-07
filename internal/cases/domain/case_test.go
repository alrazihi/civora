package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsValidTransition(t *testing.T) {
	tests := []struct {
		name string
		from CaseStatus
		to   CaseStatus
		want bool
	}{
		{"New to Open", CaseStatusNew, CaseStatusOpen, true},
		{"New to InReview", CaseStatusNew, CaseStatusInReview, true},
		{"Open to InReview", CaseStatusOpen, CaseStatusInReview, true},
		{"InReview to Assessment", CaseStatusInReview, CaseStatusAssessment, true},
		{"InReview back to Open", CaseStatusInReview, CaseStatusOpen, true},
		{"Assessment to DecisionPending", CaseStatusAssessment, CaseStatusDecisionPending, true},
		{"DecisionPending to Approved", CaseStatusDecisionPending, CaseStatusApproved, true},
		{"DecisionPending to Rejected", CaseStatusDecisionPending, CaseStatusRejected, true},
		{"Approved to InProgress", CaseStatusApproved, CaseStatusInProgress, true},
		{"Rejected to Closed", CaseStatusRejected, CaseStatusClosed, true},
		{"InProgress to FollowUp", CaseStatusInProgress, CaseStatusFollowUp, true},
		{"FollowUp to Closed", CaseStatusFollowUp, CaseStatusClosed, true},
		{"New to Closed (invalid)", CaseStatusNew, CaseStatusClosed, false},
		{"Open to Approved (invalid)", CaseStatusOpen, CaseStatusApproved, false},
		{"Closed to anything (invalid)", CaseStatusClosed, CaseStatusOpen, false},
		{"Same status (invalid)", CaseStatusOpen, CaseStatusOpen, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidTransition(tt.from, tt.to)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidTransitionsFrom(t *testing.T) {
	tests := []struct {
		status   CaseStatus
		expected []CaseStatus
	}{
		{CaseStatusNew, []CaseStatus{CaseStatusOpen, CaseStatusInReview}},
		{CaseStatusOpen, []CaseStatus{CaseStatusInReview}},
		{CaseStatusInReview, []CaseStatus{CaseStatusAssessment, CaseStatusOpen}},
		{CaseStatusAssessment, []CaseStatus{CaseStatusDecisionPending}},
		{CaseStatusDecisionPending, []CaseStatus{CaseStatusApproved, CaseStatusRejected}},
		{CaseStatusApproved, []CaseStatus{CaseStatusInProgress}},
		{CaseStatusRejected, []CaseStatus{CaseStatusClosed}},
		{CaseStatusInProgress, []CaseStatus{CaseStatusFollowUp}},
		{CaseStatusFollowUp, []CaseStatus{CaseStatusClosed}},
		{CaseStatusClosed, []CaseStatus{}},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			result := ValidTransitionsFrom(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCase_TransitionTo(t *testing.T) {
	t.Run("valid transition", func(t *testing.T) {
		c, err := NewCase(uuid.New(), uuid.New(), "Test Case", "Description", ServiceTypeGeneral, PriorityNormal, nil)
		require.NoError(t, err)
		err = c.TransitionTo(CaseStatusOpen)
		require.NoError(t, err)
		assert.Equal(t, CaseStatusOpen, c.Status)
	})

	t.Run("invalid transition", func(t *testing.T) {
		c, err := NewCase(uuid.New(), uuid.New(), "Test Case", "Description", ServiceTypeGeneral, PriorityNormal, nil)
		require.NoError(t, err)
		err = c.TransitionTo(CaseStatusClosed)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidStateTransition)
	})

	t.Run("closed sets closed_at", func(t *testing.T) {
		c, err := NewCase(uuid.New(), uuid.New(), "Test Case", "Description", ServiceTypeGeneral, PriorityUrgent, nil)
		require.NoError(t, err)
		c.Status = CaseStatusFollowUp
		err = c.TransitionTo(CaseStatusClosed)
		require.NoError(t, err)
		assert.NotNil(t, c.ClosedAt)
	})
}

func TestCase_AssignTo(t *testing.T) {
	c, err := NewCase(uuid.New(), uuid.New(), "Test Case", "Description", ServiceTypeGeneral, PriorityNormal, nil)
	require.NoError(t, err)
	user := uuid.New()
	c.AssignTo(user)
	require.NotNil(t, c.AssignedToID)
	assert.Equal(t, user, *c.AssignedToID)
}

func TestGenerateCaseNumber(t *testing.T) {
	c, err := NewCase(uuid.New(), uuid.New(), "Test Case", "Description", ServiceTypeGeneral, PriorityNormal, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, c.CaseNumber)
	assert.Contains(t, c.CaseNumber, "CAS-")
}

func TestCaseNumberCollision_Uniqueness(t *testing.T) {
	now := time.Now().UTC()
	seen := make(map[string]bool)
	for i := 0; i < 10000; i++ {
		num := GenerateCaseNumber(now)
		assert.NotContains(t, seen, num, "case number collision at iteration %d", i)
		seen[num] = true
	}
}

func TestCase_RegenerateCaseNumber(t *testing.T) {
	c, err := NewCase(uuid.New(), uuid.New(), "Test Case", "Description", ServiceTypeGeneral, PriorityNormal, nil)
	require.NoError(t, err)
	original := c.CaseNumber
	c.RegenerateCaseNumber()
	assert.NotEmpty(t, c.CaseNumber)
	assert.NotEqual(t, original, c.CaseNumber, "regenerated case number should differ from original")
	assert.Contains(t, c.CaseNumber, "CAS-")
}

func TestNewCase_InputValidation(t *testing.T) {
	t.Run("title too long", func(t *testing.T) {
		longTitle := strings.Repeat("x", maxTitleLength+1)
		_, err := NewCase(uuid.New(), uuid.New(), longTitle, "description", ServiceTypeGeneral, PriorityNormal, nil)
		assert.ErrorIs(t, err, ErrCaseInvalidInput)
	})

	t.Run("description too long", func(t *testing.T) {
		longDesc := strings.Repeat("x", maxDescriptionLength+1)
		_, err := NewCase(uuid.New(), uuid.New(), "title", longDesc, ServiceTypeGeneral, PriorityNormal, nil)
		assert.ErrorIs(t, err, ErrCaseInvalidInput)
	})

	t.Run("valid input", func(t *testing.T) {
		c, err := NewCase(uuid.New(), uuid.New(), "Valid Title", "Valid description", ServiceTypeEmergency, PriorityHigh, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, c.CaseNumber)
		assert.Equal(t, ServiceTypeEmergency, c.ServiceType)
		assert.Equal(t, PriorityHigh, c.Priority)
		assert.Equal(t, CaseStatusNew, c.Status)
	})

	t.Run("with person", func(t *testing.T) {
		personID := uuid.New()
		c, err := NewCase(uuid.New(), uuid.New(), "Title", "Desc", ServiceTypeGeneral, PriorityNormal, &personID)
		require.NoError(t, err)
		assert.NotNil(t, c.PersonID)
		assert.Equal(t, personID, *c.PersonID)
	})
}

func TestGenerateCaseNumber_Unique(t *testing.T) {
	now := time.Now().UTC()
	numbers := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		n := GenerateCaseNumber(now.Add(time.Duration(i) * time.Microsecond))
		assert.NotContains(t, numbers, n, "case number should be unique")
		numbers[n] = true
	}
	assert.Len(t, numbers, 1000, "all 1000 generated case numbers should be unique")
}
