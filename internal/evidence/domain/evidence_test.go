package domain

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEvidence(t *testing.T) {
	t.Run("valid evidence", func(t *testing.T) {
		orgID := uuid.New()
		srID := uuid.New()
		uploadedBy := uuid.New()

		e, err := NewEvidence(orgID, srID, uploadedBy, EvidenceTypeIdentityDocument, "Passport", "s3://bucket/passport.pdf")
		require.NoError(t, err)
		assert.NotEmpty(t, e.ID)
		assert.Equal(t, orgID, e.OrganizationID)
		assert.Equal(t, EvidenceTypeIdentityDocument, e.Type)
		assert.Equal(t, "Passport", e.Description)
		assert.Equal(t, "s3://bucket/passport.pdf", e.StorageReference)
		assert.Equal(t, uploadedBy, e.UploadedBy)
	})

	t.Run("empty description", func(t *testing.T) {
		_, err := NewEvidence(uuid.New(), uuid.New(), uuid.New(), EvidenceTypeStaffNote, "", "ref")
		assert.ErrorIs(t, err, ErrEvidenceInvalidInput)
	})

	t.Run("empty storage reference", func(t *testing.T) {
		_, err := NewEvidence(uuid.New(), uuid.New(), uuid.New(), EvidenceTypeStaffNote, "Note", "")
		assert.ErrorIs(t, err, ErrEvidenceInvalidInput)
	})

	t.Run("description too long", func(t *testing.T) {
		_, err := NewEvidence(uuid.New(), uuid.New(), uuid.New(), EvidenceTypeStaffNote, strings.Repeat("x", 5001), "ref")
		assert.ErrorIs(t, err, ErrEvidenceInvalidInput)
	})

	t.Run("storage reference too long", func(t *testing.T) {
		_, err := NewEvidence(uuid.New(), uuid.New(), uuid.New(), EvidenceTypeStaffNote, "Note", strings.Repeat("x", 501))
		assert.ErrorIs(t, err, ErrEvidenceInvalidInput)
	})
}
