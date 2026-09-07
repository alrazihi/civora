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
		{"Created to Open", CaseStatusCreated, CaseStatusOpen, true},
		{"Open to InReview", CaseStatusOpen, CaseStatusInReview, true},
		{"InReview to Resolved", CaseStatusInReview, CaseStatusResolved, true},
		{"Resolved to Closed", CaseStatusResolved, CaseStatusClosed, true},
		{"InReview back to Open", CaseStatusInReview, CaseStatusOpen, true},
		{"Resolved back to InReview", CaseStatusResolved, CaseStatusInReview, true},
		{"Created to Closed (invalid)", CaseStatusCreated, CaseStatusClosed, false},
		{"Open to Resolved (invalid)", CaseStatusOpen, CaseStatusResolved, false},
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
		{CaseStatusCreated, []CaseStatus{CaseStatusOpen}},
		{CaseStatusOpen, []CaseStatus{CaseStatusInReview}},
		{CaseStatusInReview, []CaseStatus{CaseStatusOpen, CaseStatusResolved}},
		{CaseStatusResolved, []CaseStatus{CaseStatusClosed, CaseStatusInReview}},
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
		c, err := NewCase(uuid.New(), uuid.New(), "Test Case", "Description")
		require.NoError(t, err)
		err = c.TransitionTo(CaseStatusOpen)
		require.NoError(t, err)
		assert.Equal(t, CaseStatusOpen, c.Status)
	})

	t.Run("invalid transition", func(t *testing.T) {
		c, err := NewCase(uuid.New(), uuid.New(), "Test Case", "Description")
		require.NoError(t, err)
		err = c.TransitionTo(CaseStatusClosed)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidStateTransition)
	})

	t.Run("closed sets closed_at", func(t *testing.T) {
		c, err := NewCase(uuid.New(), uuid.New(), "Test Case", "Description")
		require.NoError(t, err)
		c.Status = CaseStatusResolved
		err = c.TransitionTo(CaseStatusClosed)
		require.NoError(t, err)
		assert.NotNil(t, c.ClosedAt)
	})
}

func TestCase_AssignTo(t *testing.T) {
	c, err := NewCase(uuid.New(), uuid.New(), "Test Case", "Description")
	require.NoError(t, err)
	user := uuid.New()
	c.AssignTo(user)
	require.NotNil(t, c.AssignedToID)
	assert.Equal(t, user, *c.AssignedToID)
}

func TestGenerateCaseNumber(t *testing.T) {
	c, err := NewCase(uuid.New(), uuid.New(), "Test Case", "Description")
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
	c, err := NewCase(uuid.New(), uuid.New(), "Test Case", "Description")
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
		_, err := NewCase(uuid.New(), uuid.New(), longTitle, "description")
		assert.ErrorIs(t, err, ErrCaseInvalidInput)
	})

	t.Run("description too long", func(t *testing.T) {
		longDesc := strings.Repeat("x", maxDescriptionLength+1)
		_, err := NewCase(uuid.New(), uuid.New(), "title", longDesc)
		assert.ErrorIs(t, err, ErrCaseInvalidInput)
	})

	t.Run("valid input", func(t *testing.T) {
		c, err := NewCase(uuid.New(), uuid.New(), "Valid Title", "Valid description")
		require.NoError(t, err)
		assert.NotEmpty(t, c.CaseNumber)
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
