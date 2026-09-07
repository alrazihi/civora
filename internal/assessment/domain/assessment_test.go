package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAssessment(t *testing.T) {
	t.Run("valid assessment", func(t *testing.T) {
		orgID := uuid.New()
		srID := uuid.New()
		assessor := uuid.New()

		a, err := NewAssessment(orgID, srID, assessor, "All findings documented", "Food, shelter needs", "Recommend approval")
		require.NoError(t, err)
		assert.NotEmpty(t, a.ID)
		assert.Equal(t, orgID, a.OrganizationID)
		assert.Equal(t, srID, a.ServiceRequestID)
		assert.Equal(t, "All findings documented", a.Findings)
		assert.Equal(t, "Food, shelter needs", a.NeedsIdentified)
		assert.Equal(t, "Recommend approval", a.Recommendation)
		assert.Equal(t, assessor, a.Assessor)
	})

	t.Run("empty findings", func(t *testing.T) {
		_, err := NewAssessment(uuid.New(), uuid.New(), uuid.New(), "", "need", "recommend")
		assert.ErrorIs(t, err, ErrAssessmentInvalidInput)
	})

	t.Run("empty recommendation", func(t *testing.T) {
		_, err := NewAssessment(uuid.New(), uuid.New(), uuid.New(), "findings", "need", "")
		assert.ErrorIs(t, err, ErrAssessmentInvalidInput)
	})
}
