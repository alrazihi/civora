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

	t.Run("empty reason", func(t *testing.T) {
		_, err := NewDecision(uuid.New(), uuid.New(), uuid.New(), DecisionTypeApproved, "")
		assert.ErrorIs(t, err, ErrDecisionInvalidInput)
	})

	t.Run("reason too long", func(t *testing.T) {
		_, err := NewDecision(uuid.New(), uuid.New(), uuid.New(), DecisionTypeApproved, strings.Repeat("x", 5001))
		assert.ErrorIs(t, err, ErrDecisionInvalidInput)
	})
}
