package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestIsClosed(t *testing.T) {
	t.Run("closed case", func(t *testing.T) {
		assert.True(t, IsClosed(CaseStatusClosed))
	})
	t.Run("rejected case", func(t *testing.T) {
		assert.True(t, IsClosed(CaseStatusRejected))
	})
	t.Run("open case", func(t *testing.T) {
		assert.False(t, IsClosed(CaseStatusOpen))
	})
	t.Run("new case", func(t *testing.T) {
		assert.False(t, IsClosed(CaseStatusNew))
	})
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
		assert.Equal(t, 1, c.Version)
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

func TestSyncStatusFromWorkflow(t *testing.T) {
	t.Run("syncs each valid workflow state", func(t *testing.T) {
		states := []CaseStatus{
			CaseStatusNew, CaseStatusOpen, CaseStatusInReview, CaseStatusAssessment,
			CaseStatusDecisionPending, CaseStatusApproved, CaseStatusRejected,
			CaseStatusInProgress, CaseStatusFollowUp, CaseStatusClosed,
		}
		for _, st := range states {
			c := &Case{Status: CaseStatusOpen}
			err := c.SyncStatusFromWorkflow(string(st))
			assert.NoError(t, err, "state %s should sync", st)
			assert.Equal(t, st, c.Status, "case status should match workflow state %s", st)
			if st == CaseStatusClosed {
				assert.NotNil(t, c.ClosedAt, "closed state should set closed_at")
			}
		}
	})

	t.Run("custom workflow state is accepted", func(t *testing.T) {
		c := &Case{Status: CaseStatusOpen}
		err := c.SyncStatusFromWorkflow("GRANT_PROCESSING")
		assert.NoError(t, err)
		assert.Equal(t, CaseStatus("GRANT_PROCESSING"), c.Status)
	})

	t.Run("empty state is a no-op", func(t *testing.T) {
		c := &Case{Status: CaseStatusOpen}
		err := c.SyncStatusFromWorkflow("")
		assert.NoError(t, err)
		assert.Equal(t, CaseStatusOpen, c.Status)
	})
}

func TestValidateConsistency(t *testing.T) {
	t.Run("no workflow instance is consistent", func(t *testing.T) {
		c := &Case{Status: CaseStatusOpen, WorkflowInstanceID: nil}
		assert.NoError(t, c.ValidateConsistency("OPEN"))
	})

	t.Run("empty workflow state is a no-op", func(t *testing.T) {
		c := &Case{Status: CaseStatusOpen, WorkflowInstanceID: &uuid.UUID{}}
		assert.NoError(t, c.ValidateConsistency(""))
	})

	t.Run("matching state is consistent", func(t *testing.T) {
		c := &Case{Status: CaseStatusClosed, WorkflowInstanceID: &uuid.UUID{}}
		assert.NoError(t, c.ValidateConsistency("CLOSED"))
	})

	t.Run("mismatched state is a contradiction", func(t *testing.T) {
		c := &Case{Status: CaseStatusOpen, WorkflowInstanceID: &uuid.UUID{}}
		err := c.ValidateConsistency("CLOSED")
		assert.ErrorIs(t, err, ErrCaseStatusContradiction)
	})
}

func TestStatusFromWorkflowState(t *testing.T) {
	for _, st := range []CaseStatus{
		CaseStatusNew, CaseStatusOpen, CaseStatusInReview, CaseStatusAssessment,
		CaseStatusDecisionPending, CaseStatusApproved, CaseStatusRejected,
		CaseStatusInProgress, CaseStatusFollowUp, CaseStatusClosed,
	} {
		got, err := StatusFromWorkflowState(string(st))
		assert.NoError(t, err)
		assert.Equal(t, st, got)
	}
	got, err := StatusFromWorkflowState("BOGUS")
	assert.NoError(t, err)
	assert.Equal(t, CaseStatus("BOGUS"), got)

	got, err = StatusFromWorkflowState("GRANT_PROCESSING")
	assert.NoError(t, err)
	assert.Equal(t, CaseStatus("GRANT_PROCESSING"), got)

	_, err = StatusFromWorkflowState("")
	assert.Error(t, err)
}
