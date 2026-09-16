package domain

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDecision(t *testing.T) {
	t.Run("valid decision - approved", func(t *testing.T) {
		orgID := uuid.New()
		srID := uuid.New()
		decisionMaker := uuid.New()

		d, err := NewDecision(orgID, srID, decisionMaker, DecisionTypeApproved, "Meets all criteria")
		require.NoError(t, err)
		assert.NotEmpty(t, d.ID)
		assert.Equal(t, orgID, d.OrganizationID)
		assert.Equal(t, DecisionTypeApproved, d.Decision)
		assert.Equal(t, decisionMaker, d.DecisionMaker)
		assert.Equal(t, 1, d.Version)
		assert.Empty(t, d.WorkflowState)
		assert.Empty(t, d.RuleEvaluationIDs)
		assert.Empty(t, d.EvidenceIDs)
		assert.Nil(t, d.FormSubmissionID)
		assert.Nil(t, d.SupersededByID)
	})

	t.Run("valid decision - rejected", func(t *testing.T) {
		d, err := NewDecision(uuid.New(), uuid.New(), uuid.New(), DecisionTypeRejected, "Missing documentation")
		require.NoError(t, err)
		assert.Equal(t, DecisionTypeRejected, d.Decision)
	})

	t.Run("valid decision - needs more info", func(t *testing.T) {
		d, err := NewDecision(uuid.New(), uuid.New(), uuid.New(), DecisionTypeNeedsMoreInformation, "More info required")
		require.NoError(t, err)
		assert.Equal(t, DecisionTypeNeedsMoreInformation, d.Decision)
	})

	t.Run("valid decision - escalate", func(t *testing.T) {
		d, err := NewDecision(uuid.New(), uuid.New(), uuid.New(), DecisionTypeEscalate, "Requires senior review")
		require.NoError(t, err)
		assert.Equal(t, DecisionTypeEscalate, d.Decision)
	})

	t.Run("empty reason", func(t *testing.T) {
		_, err := NewDecision(uuid.New(), uuid.New(), uuid.New(), DecisionTypeRejected, "")
		assert.ErrorIs(t, err, ErrDecisionInvalidInput)
	})

	t.Run("reason too long", func(t *testing.T) {
		_, err := NewDecision(uuid.New(), uuid.New(), uuid.New(), DecisionTypeApproved, strings.Repeat("x", 5001))
		assert.ErrorIs(t, err, ErrDecisionInvalidInput)
	})
}

func TestNewDecisionWithContext(t *testing.T) {
	t.Run("with context fields", func(t *testing.T) {
		orgID := uuid.New()
		srID := uuid.New()
		actorID := uuid.New()
		evalID := uuid.New()
		evidenceID := uuid.New()
		submissionID := uuid.New()

		d, err := NewDecisionWithContext(
			orgID, srID, actorID, DecisionTypeApproved, "Approved",
			"DECISION_PENDING", []uuid.UUID{evalID}, []uuid.UUID{evidenceID},
			&submissionID, nil, 1,
		)
		require.NoError(t, err)
		assert.Equal(t, "DECISION_PENDING", d.WorkflowState)
		assert.Equal(t, []uuid.UUID{evalID}, d.RuleEvaluationIDs)
		assert.Equal(t, []uuid.UUID{evidenceID}, d.EvidenceIDs)
		assert.Equal(t, &submissionID, d.FormSubmissionID)
		assert.Equal(t, 1, d.Version)
	})

	t.Run("defaults empty slices", func(t *testing.T) {
		d, err := NewDecisionWithContext(uuid.New(), uuid.New(), uuid.New(), DecisionTypeApproved, "ok", "", nil, nil, nil, nil, 1)
		require.NoError(t, err)
		assert.Empty(t, d.RuleEvaluationIDs)
		assert.Empty(t, d.EvidenceIDs)
		assert.Nil(t, d.FormSubmissionID)
		assert.Nil(t, d.ReviewQueueEntryID)
	})
}

func TestNewSupersedingDecision(t *testing.T) {
	t.Run("supersedes previous decision", func(t *testing.T) {
		orgID := uuid.New()
		srID := uuid.New()
		actorID := uuid.New()
		prev := &Decision{
			ID:               uuid.New(),
			OrganizationID:   orgID,
			ServiceRequestID: srID,
			Decision:         DecisionTypeApproved,
			Version:          1,
		}

		d, err := NewSupersedingDecision(
			orgID, srID, actorID, DecisionTypeRejected, "Reversed",
			prev, "DECISION_PENDING", nil, nil, nil, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, 2, d.Version)
		assert.NotNil(t, d.SupersededByID)
		assert.Equal(t, d.ID, *d.SupersededByID)
	})

	t.Run("no previous decision defaults version to 1", func(t *testing.T) {
		d, err := NewSupersedingDecision(
			uuid.New(), uuid.New(), uuid.New(), DecisionTypeApproved, "ok",
			nil, "", nil, nil, nil, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, 1, d.Version)
		assert.NotNil(t, d.SupersededByID)
		assert.Equal(t, d.ID, *d.SupersededByID)
	})
}
