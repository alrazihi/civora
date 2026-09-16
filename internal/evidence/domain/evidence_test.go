package domain

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func baseParams() EvidenceParams {
	return EvidenceParams{
		OrganizationID:   uuid.New(),
		ServiceRequestID: uuid.New(),
		UploadedBy:       uuid.New(),
		Type:             EvidenceTypeIdentityDocument,
		Description:      "Passport",
		StorageReference: "s3://bucket/passport.pdf",
	}
}

func TestNewEvidence(t *testing.T) {
	t.Run("valid evidence", func(t *testing.T) {
		params := baseParams()
		e, err := NewEvidence(params)
		require.NoError(t, err)
		assert.NotEmpty(t, e.ID)
		assert.Equal(t, params.OrganizationID, e.OrganizationID)
		assert.Equal(t, EvidenceTypeIdentityDocument, e.Type)
		assert.Equal(t, "Passport", e.Description)
		assert.Equal(t, "s3://bucket/passport.pdf", e.StorageReference)
		assert.Equal(t, params.UploadedBy, e.UploadedBy)
		assert.Equal(t, VerificationStatusUnverified, e.VerificationStatus)
		assert.Nil(t, e.VerifiedBy)
		assert.Nil(t, e.VerifiedAt)
	})

	t.Run("valid evidence with all fields", func(t *testing.T) {
		params := baseParams()
		params.Source = EvidenceSourceExternalReference
		params.CustomType = "UTILITY_BILL"
		params.Type = EvidenceTypeOther
		personID := uuid.New()
		params.PersonID = &personID
		params.Metadata = map[string]any{"document_type": "utility_bill", "pages": 2}

		e, err := NewEvidence(params)
		require.NoError(t, err)
		assert.Equal(t, EvidenceSourceExternalReference, e.Source)
		require.NotNil(t, e.CustomType)
		assert.Equal(t, "UTILITY_BILL", *e.CustomType)
		require.NotNil(t, e.PersonID)
		assert.Equal(t, personID, *e.PersonID)
		assert.Equal(t, "utility_bill", e.Metadata["document_type"])
	})

	t.Run("default source is upload", func(t *testing.T) {
		params := baseParams()
		params.Source = ""
		e, err := NewEvidence(params)
		require.NoError(t, err)
		assert.Equal(t, EvidenceSourceUpload, e.Source)
	})

	t.Run("empty description", func(t *testing.T) {
		params := baseParams()
		params.Description = ""
		_, err := NewEvidence(params)
		assert.ErrorIs(t, err, ErrEvidenceInvalidInput)
	})

	t.Run("empty storage reference", func(t *testing.T) {
		params := baseParams()
		params.StorageReference = ""
		_, err := NewEvidence(params)
		assert.ErrorIs(t, err, ErrEvidenceInvalidInput)
	})

	t.Run("description too long", func(t *testing.T) {
		params := baseParams()
		params.Description = strings.Repeat("x", 5001)
		_, err := NewEvidence(params)
		assert.ErrorIs(t, err, ErrEvidenceInvalidInput)
	})

	t.Run("storage reference too long", func(t *testing.T) {
		params := baseParams()
		params.StorageReference = strings.Repeat("x", 501)
		_, err := NewEvidence(params)
		assert.ErrorIs(t, err, ErrEvidenceInvalidInput)
	})

	t.Run("storage reference rejects local paths", func(t *testing.T) {
		params := baseParams()
		params.StorageReference = "../../etc/passwd"
		_, err := NewEvidence(params)
		assert.ErrorIs(t, err, ErrEvidenceInvalidInput)
	})

	t.Run("storage reference rejects encoded traversal", func(t *testing.T) {
		params := baseParams()
		params.StorageReference = "s3://bucket/%2e%2e/secret"
		_, err := NewEvidence(params)
		assert.ErrorIs(t, err, ErrEvidenceInvalidInput)
	})

	t.Run("storage reference rejects unsupported schemes", func(t *testing.T) {
		params := baseParams()
		params.StorageReference = "file:///etc/passwd"
		_, err := NewEvidence(params)
		assert.ErrorIs(t, err, ErrEvidenceInvalidInput)
	})

	t.Run("zero organization ID", func(t *testing.T) {
		params := baseParams()
		params.OrganizationID = uuid.Nil
		_, err := NewEvidence(params)
		assert.ErrorIs(t, err, ErrEvidenceInvalidInput)
	})

	t.Run("zero uploaded by", func(t *testing.T) {
		params := baseParams()
		params.UploadedBy = uuid.Nil
		_, err := NewEvidence(params)
		assert.ErrorIs(t, err, ErrEvidenceInvalidInput)
	})

	t.Run("custom type too long", func(t *testing.T) {
		params := baseParams()
		params.Type = EvidenceTypeOther
		params.CustomType = strings.Repeat("x", 201)
		_, err := NewEvidence(params)
		assert.ErrorIs(t, err, ErrEvidenceInvalidInput)
	})

	t.Run("custom type with control characters", func(t *testing.T) {
		params := baseParams()
		params.Type = EvidenceTypeOther
		params.CustomType = "type\x00name"
		_, err := NewEvidence(params)
		assert.ErrorIs(t, err, ErrEvidenceInvalidInput)
	})
}

func TestMetadataValidation(t *testing.T) {
	t.Run("empty metadata is valid", func(t *testing.T) {
		assert.NoError(t, validateMetadata(nil))
		assert.NoError(t, validateMetadata(map[string]any{}))
	})

	t.Run("metadata with values is valid", func(t *testing.T) {
		assert.NoError(t, validateMetadata(map[string]any{"key": "value"}))
	})

	t.Run("metadata keys with dots are rejected", func(t *testing.T) {
		err := validateMetadata(map[string]any{"key.sub": "value"})
		assert.Error(t, err)
	})

	t.Run("metadata keys with quotes are rejected", func(t *testing.T) {
		err := validateMetadata(map[string]any{`key"sub`: "value"})
		assert.Error(t, err)
	})
}

func TestEvidence_Verify(t *testing.T) {
	t.Run("verify transitions to VERIFIED", func(t *testing.T) {
		params := baseParams()
		e, err := NewEvidence(params)
		require.NoError(t, err)

		rec, err := e.Verify(uuid.New(), "manual review", "manual")
		require.NoError(t, err)
		assert.Equal(t, VerificationStatusVerified, e.VerificationStatus)
		require.NotNil(t, e.VerifiedBy)
		require.NotNil(t, e.VerifiedAt)
		assert.NotNil(t, rec)
		assert.Equal(t, VerificationStatusVerified, rec.Status)
	})

	t.Run("verify without verifier ID fails", func(t *testing.T) {
		params := baseParams()
		e, err := NewEvidence(params)
		require.NoError(t, err)

		_, err = e.Verify(uuid.Nil, "reason", "method")
		assert.ErrorIs(t, err, ErrVerificationInvalid)
	})
}

func TestEvidence_Reject(t *testing.T) {
	t.Run("reject transitions to REJECTED", func(t *testing.T) {
		params := baseParams()
		e, err := NewEvidence(params)
		require.NoError(t, err)

		rec, err := e.Reject(uuid.New(), "fake document", "manual")
		require.NoError(t, err)
		assert.Equal(t, VerificationStatusRejected, e.VerificationStatus)
		require.NotNil(t, e.VerifiedBy)
		require.NotNil(t, e.VerifiedAt)
		assert.Equal(t, "fake document", e.VerificationReason)
		assert.Equal(t, "manual", e.VerificationMethod)
		assert.NotNil(t, rec)
		assert.Equal(t, VerificationStatusRejected, rec.Status)
	})
}

func TestEvidence_MarkForReview(t *testing.T) {
	t.Run("mark for review transitions to NEEDS_REVIEW", func(t *testing.T) {
		params := baseParams()
		e, err := NewEvidence(params)
		require.NoError(t, err)

		rec, err := e.MarkForReview(uuid.New(), "needs additional documents", "manual")
		require.NoError(t, err)
		assert.Equal(t, VerificationStatusNeedsReview, e.VerificationStatus)
		require.NotNil(t, e.VerifiedBy)
		require.NotNil(t, e.VerifiedAt)
		assert.Equal(t, "needs additional documents", e.VerificationReason)
		assert.NotNil(t, rec)
		assert.Equal(t, VerificationStatusNeedsReview, rec.Status)
	})
}

func TestEvidence_VerificationStateTransitions(t *testing.T) {
	t.Run("reject then verify is allowed", func(t *testing.T) {
		params := baseParams()
		e, err := NewEvidence(params)
		require.NoError(t, err)

		_, err = e.Reject(uuid.New(), "bad", "manual")
		require.NoError(t, err)
		assert.Equal(t, VerificationStatusRejected, e.VerificationStatus)

		_, err = e.Verify(uuid.New(), "recheck passed", "manual")
		require.NoError(t, err)
		assert.Equal(t, VerificationStatusVerified, e.VerificationStatus)
	})

	t.Run("verify then reject is allowed", func(t *testing.T) {
		params := baseParams()
		e, err := NewEvidence(params)
		require.NoError(t, err)

		_, err = e.Verify(uuid.New(), "all good", "manual")
		require.NoError(t, err)
		assert.Equal(t, VerificationStatusVerified, e.VerificationStatus)

		_, err = e.Reject(uuid.New(), "actually bad", "manual")
		require.NoError(t, err)
		assert.Equal(t, VerificationStatusRejected, e.VerificationStatus)
	})

	t.Run("mark for review then verify is allowed", func(t *testing.T) {
		params := baseParams()
		e, err := NewEvidence(params)
		require.NoError(t, err)

		_, err = e.MarkForReview(uuid.New(), "need more", "manual")
		require.NoError(t, err)
		assert.Equal(t, VerificationStatusNeedsReview, e.VerificationStatus)

		_, err = e.Verify(uuid.New(), "got more", "manual")
		require.NoError(t, err)
		assert.Equal(t, VerificationStatusVerified, e.VerificationStatus)
	})
}

func TestEvidence_UpdateMetadata(t *testing.T) {
	t.Run("update metadata succeeds", func(t *testing.T) {
		params := baseParams()
		e, err := NewEvidence(params)
		require.NoError(t, err)

		err = e.UpdateMetadata(uuid.New(), map[string]any{"key": "value"}, "")
		require.NoError(t, err)
		assert.Equal(t, "value", e.Metadata["key"])
	})

	t.Run("update metadata with nil actor fails", func(t *testing.T) {
		params := baseParams()
		e, err := NewEvidence(params)
		require.NoError(t, err)

		err = e.UpdateMetadata(uuid.Nil, map[string]any{"key": "value"}, "")
		assert.Error(t, err)
	})
}

func TestEvidence_SetPerson(t *testing.T) {
	t.Run("set person", func(t *testing.T) {
		params := baseParams()
		e, err := NewEvidence(params)
		require.NoError(t, err)

		personID := uuid.New()
		err = e.SetPerson(personID)
		require.NoError(t, err)
		require.NotNil(t, e.PersonID)
		assert.Equal(t, personID, *e.PersonID)
	})

	t.Run("set person with nil ID fails", func(t *testing.T) {
		params := baseParams()
		e, err := NewEvidence(params)
		require.NoError(t, err)

		err = e.SetPerson(uuid.Nil)
		assert.Error(t, err)
	})
}

func TestVerificationRecord_Creation(t *testing.T) {
	params := baseParams()
	e, err := NewEvidence(params)
	require.NoError(t, err)

	verifierID := uuid.New()
	rec, err := e.Verify(verifierID, "verified via manual check", "manual")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, rec.ID)
	assert.Equal(t, e.ID, rec.EvidenceID)
	assert.Equal(t, e.OrganizationID, rec.OrganizationID)
	assert.Equal(t, VerificationStatusVerified, rec.Status)
	require.NotNil(t, rec.VerifierID)
	assert.Equal(t, verifierID, *rec.VerifierID)
	assert.Equal(t, "verified via manual check", rec.Reason)
	assert.Equal(t, "manual", rec.Method)
	assert.False(t, rec.VerifiedAt.IsZero())
	assert.False(t, rec.CreatedAt.IsZero())
}
