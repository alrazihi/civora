package domain

import (
	"testing"
	"time"

	"github.com/alrazihi/civora/internal/cases/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewReviewQueueEntry(t *testing.T) {
	t.Run("creates entry with defaults", func(t *testing.T) {
		orgID := uuid.New()
		caseID := uuid.New()
		wfInstanceID := uuid.New()
		evalID := uuid.New()

		entry := NewReviewQueueEntry(orgID, caseID, wfInstanceID, "DECISION_PENDING", domain.PriorityHigh, []uuid.UUID{evalID}, nil, nil)

		assert.Equal(t, orgID, entry.OrganizationID)
		assert.Equal(t, caseID, entry.CaseID)
		assert.Equal(t, wfInstanceID, entry.WorkflowInstanceID)
		assert.Equal(t, ReviewStatusPending, entry.Status)
		assert.Equal(t, domain.PriorityHigh, entry.Priority)
		assert.Equal(t, "DECISION_PENDING", entry.WorkflowState)
		assert.Equal(t, []uuid.UUID{evalID}, entry.RuleEvaluationIDs)
		assert.Empty(t, entry.EvidenceIDs)
		assert.Nil(t, entry.FormSubmissionID)
		assert.Empty(t, entry.MissingInformation)
		assert.Nil(t, entry.AssignedToID)
		assert.Nil(t, entry.CompletedAt)
		assert.NotZero(t, entry.CreatedAt)
	})
}

func TestReviewQueueEntry_CanTransition(t *testing.T) {
	tests := []struct {
		name     string
		from     ReviewStatus
		to       ReviewStatus
		expected bool
	}{
		{"pending to assigned", ReviewStatusPending, ReviewStatusAssigned, true},
		{"pending to in_review", ReviewStatusPending, ReviewStatusInReview, false},
		{"pending to completed", ReviewStatusPending, ReviewStatusCompleted, false},
		{"assigned to in_review", ReviewStatusAssigned, ReviewStatusInReview, true},
		{"assigned to pending", ReviewStatusAssigned, ReviewStatusPending, true},
		{"assigned to completed", ReviewStatusAssigned, ReviewStatusCompleted, false},
		{"in_review to completed", ReviewStatusInReview, ReviewStatusCompleted, true},
		{"in_review to escalated", ReviewStatusInReview, ReviewStatusEscalated, true},
		{"in_review to waiting_info", ReviewStatusInReview, ReviewStatusWaitingInfo, true},
		{"in_review to pending", ReviewStatusInReview, ReviewStatusPending, false},
		{"waiting_info to in_review", ReviewStatusWaitingInfo, ReviewStatusInReview, true},
		{"waiting_info to pending", ReviewStatusWaitingInfo, ReviewStatusPending, true},
		{"completed to assigned", ReviewStatusCompleted, ReviewStatusAssigned, false},
		{"escalated to assigned", ReviewStatusEscalated, ReviewStatusAssigned, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &ReviewQueueEntry{Status: tt.from}
			assert.Equal(t, tt.expected, entry.CanTransition(tt.to))
		})
	}
}

func TestReviewQueueEntry_Transition(t *testing.T) {
	t.Run("valid transition", func(t *testing.T) {
		entry := &ReviewQueueEntry{Status: ReviewStatusPending}
		reviewerID := uuid.New()
		err := entry.Transition(ReviewStatusAssigned, &reviewerID)
		require.NoError(t, err)
		assert.Equal(t, ReviewStatusAssigned, entry.Status)
		assert.Equal(t, &reviewerID, entry.AssignedToID)
		assert.NotZero(t, entry.UpdatedAt)
	})

	t.Run("invalid transition", func(t *testing.T) {
		entry := &ReviewQueueEntry{Status: ReviewStatusPending}
		err := entry.Transition(ReviewStatusCompleted, nil)
		assert.Error(t, err)
	})

	t.Run("completed sets completed_at", func(t *testing.T) {
		reviewerID := uuid.New()
		entry := &ReviewQueueEntry{Status: ReviewStatusInReview, AssignedToID: &reviewerID}
		before := time.Now().UTC()
		err := entry.Transition(ReviewStatusCompleted, entry.AssignedToID)
		require.NoError(t, err)
		assert.NotNil(t, entry.CompletedAt)
		assert.True(t, entry.CompletedAt.After(before) || entry.CompletedAt.Equal(before))
	})
}

func TestReviewQueueEntry_IsFinal(t *testing.T) {
	assert.True(t, (&ReviewQueueEntry{Status: ReviewStatusCompleted}).IsFinal())
	assert.True(t, (&ReviewQueueEntry{Status: ReviewStatusEscalated}).IsFinal())
	assert.False(t, (&ReviewQueueEntry{Status: ReviewStatusPending}).IsFinal())
	assert.False(t, (&ReviewQueueEntry{Status: ReviewStatusInReview}).IsFinal())
}

func TestValidateReviewStatus(t *testing.T) {
	assert.NoError(t, ValidateReviewStatus(ReviewStatusPending))
	assert.NoError(t, ValidateReviewStatus(ReviewStatusAssigned))
	assert.NoError(t, ValidateReviewStatus(ReviewStatusInReview))
	assert.NoError(t, ValidateReviewStatus(ReviewStatusCompleted))
	assert.NoError(t, ValidateReviewStatus(ReviewStatusEscalated))
	assert.NoError(t, ValidateReviewStatus(ReviewStatusWaitingInfo))
	assert.Error(t, ValidateReviewStatus("INVALID"))
}
