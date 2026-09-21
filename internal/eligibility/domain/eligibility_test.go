package domain

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEligibility(t *testing.T) {
	t.Run("valid eligibility", func(t *testing.T) {
		orgID := uuid.New()
		srID := uuid.New()
		assessedBy := uuid.New()
		criteria := map[string]interface{}{
			"income_verified": true,
			"residency":       "verified",
		}

		e, err := NewEligibility(orgID, srID, assessedBy, criteria, "All criteria met")
		require.NoError(t, err)
		assert.NotEmpty(t, e.ID)
		assert.Equal(t, orgID, e.OrganizationID)
		assert.Equal(t, srID, e.ServiceRequestID)
		assert.Equal(t, assessedBy, e.AssessedBy)
		assert.Equal(t, "All criteria met", e.Explanation)
		assert.Equal(t, EligibilityResultRequiresMoreInformation, e.Result)
	})

	t.Run("nil criteria", func(t *testing.T) {
		_, err := NewEligibility(uuid.New(), uuid.New(), uuid.New(), nil, "Explanation")
		assert.ErrorIs(t, err, ErrEligibilityInvalidInput)
	})

	t.Run("empty explanation", func(t *testing.T) {
		_, err := NewEligibility(uuid.New(), uuid.New(), uuid.New(), map[string]interface{}{}, "")
		assert.ErrorIs(t, err, ErrEligibilityInvalidInput)
	})

	t.Run("explanation too long", func(t *testing.T) {
		_, err := NewEligibility(uuid.New(), uuid.New(), uuid.New(), map[string]interface{}{}, strings.Repeat("x", 5001))
		assert.ErrorIs(t, err, ErrEligibilityInvalidInput)
	})
}

func TestEligibilitySetResult(t *testing.T) {
	e, err := NewEligibility(uuid.New(), uuid.New(), uuid.New(), map[string]interface{}{"verified": true}, "Initial explanation")
	require.NoError(t, err)

	e.SetResult(EligibilityResultEligible)
	assert.Equal(t, EligibilityResultEligible, e.Result)

	e.SetResult(EligibilityResultNotEligible)
	assert.Equal(t, EligibilityResultNotEligible, e.Result)
}
