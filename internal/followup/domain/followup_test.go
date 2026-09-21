package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFollowUp(t *testing.T) {
	t.Run("valid follow-up", func(t *testing.T) {
		orgID := uuid.New()
		srID := uuid.New()
		performedBy := uuid.New()
		scheduled := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)

		f, err := NewFollowUp(orgID, srID, performedBy, scheduled, "Family stably housed", "Weekly check-ins scheduled")
		require.NoError(t, err)
		assert.NotEmpty(t, f.ID)
		assert.Equal(t, orgID, f.OrganizationID)
		assert.Equal(t, srID, f.ServiceRequestID)
		assert.Equal(t, "Family stably housed", f.Outcome)
		assert.Equal(t, "Weekly check-ins scheduled", f.Notes)
		assert.Equal(t, performedBy, f.PerformedBy)
		assert.Equal(t, scheduled, f.ScheduledDate)
		require.Nil(t, f.CompletedDate)
	})

	t.Run("empty outcome", func(t *testing.T) {
		_, err := NewFollowUp(uuid.New(), uuid.New(), uuid.New(), time.Now(), "", "notes")
		assert.ErrorIs(t, err, ErrFollowUpInvalidInput)
	})

	t.Run("empty notes", func(t *testing.T) {
		_, err := NewFollowUp(uuid.New(), uuid.New(), uuid.New(), time.Now(), "outcome", "")
		assert.ErrorIs(t, err, ErrFollowUpInvalidInput)
	})

	t.Run("outcome too long", func(t *testing.T) {
		_, err := NewFollowUp(uuid.New(), uuid.New(), uuid.New(), time.Now(), strings.Repeat("x", 501), "notes")
		assert.ErrorIs(t, err, ErrFollowUpInvalidInput)
	})

	t.Run("notes too long", func(t *testing.T) {
		_, err := NewFollowUp(uuid.New(), uuid.New(), uuid.New(), time.Now(), "outcome", strings.Repeat("x", 5001))
		assert.ErrorIs(t, err, ErrFollowUpInvalidInput)
	})
}

func TestFollowUpComplete(t *testing.T) {
	f, err := NewFollowUp(uuid.New(), uuid.New(), uuid.New(), time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC), "Housed", "Notes")
	require.NoError(t, err)

	completed := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	f.Complete(completed)

	require.NotNil(t, f.CompletedDate)
	assert.Equal(t, completed, *f.CompletedDate)
}
