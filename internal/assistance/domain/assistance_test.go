package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAssistance(t *testing.T) {
	t.Run("valid assistance", func(t *testing.T) {
		orgID := uuid.New()
		srID := uuid.New()
		staff := uuid.New()

		a, err := NewAssistance(orgID, srID, staff, AssistanceTypeFood, "Emergency food package")
		require.NoError(t, err)
		assert.NotEmpty(t, a.ID)
		assert.Equal(t, orgID, a.OrganizationID)
		assert.Equal(t, srID, a.ServiceRequestID)
		assert.Equal(t, AssistanceTypeFood, a.Type)
		assert.Equal(t, "Emergency food package", a.Description)
		assert.Equal(t, AssistanceStatusPlanned, a.Status)
		assert.Equal(t, staff, a.ResponsibleStaff)
		require.Nil(t, a.StartedAt)
		require.Nil(t, a.CompletedAt)
	})

	t.Run("empty description", func(t *testing.T) {
		_, err := NewAssistance(uuid.New(), uuid.New(), uuid.New(), AssistanceTypeOther, "")
		assert.ErrorIs(t, err, ErrAssistanceInvalidInput)
	})
}

func TestAssistanceLifecycle(t *testing.T) {
	a, err := NewAssistance(uuid.New(), uuid.New(), uuid.New(), AssistanceTypeShelter, "30 days shelter placement")
	require.NoError(t, err)

	a.Start()
	assert.Equal(t, AssistanceStatusInProgress, a.Status)
	require.NotNil(t, a.StartedAt)

	a.Complete()
	assert.Equal(t, AssistanceStatusCompleted, a.Status)
	require.NotNil(t, a.CompletedAt)

	a2, _ := NewAssistance(uuid.New(), uuid.New(), uuid.New(), AssistanceTypeMedical, "Medical checkup")
	a2.Start()
	a2.Cancel()
	assert.Equal(t, AssistanceStatusCancelled, a2.Status)
}

func TestAssistanceTimestamps(t *testing.T) {
	a, err := NewAssistance(uuid.New(), uuid.New(), uuid.New(), AssistanceTypeTransport, "Bus pass for 30 days")
	require.NoError(t, err)

	before := time.Now().UTC()
	a.Start()
	after := time.Now().UTC()

	assert.True(t, !a.StartedAt.Before(before))
	assert.True(t, !a.StartedAt.After(after))
}
