package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDocumentAnalysis_Valid(t *testing.T) {
	params := DocumentAnalysisParams{
		OrganizationID: uuid.New(),
		DocumentID:     uuid.New(),
		EvidenceID:     uuid.New(),
		Type:           AnalysisTypeClassification,
		Source:         AnalysisSourceAIModel,
		Status:         AnalysisStatusPendingReview,
		Content:        map[string]any{"key": "value"},
		Confidence:     float64Ptr(0.85),
		InputHash:      "input-hash",
		OutputHash:     "output-hash",
		SchemaVersion:  "1.0",
		CreatedAt:      time.Now().UTC(),
	}

	analysis, err := NewDocumentAnalysis(params)

	require.NoError(t, err)
	assert.NotNil(t, analysis)
	assert.Equal(t, params.OrganizationID, analysis.OrganizationID)
	assert.Equal(t, params.DocumentID, analysis.DocumentID)
	assert.Equal(t, params.EvidenceID, analysis.EvidenceID)
	assert.Equal(t, params.Type, analysis.Type)
	assert.Equal(t, params.Source, analysis.Source)
	assert.Equal(t, params.Status, analysis.Status)
	assert.Equal(t, params.Content, analysis.Content)
	assert.Equal(t, params.Confidence, analysis.Confidence)
	assert.Equal(t, params.InputHash, analysis.InputHash)
	assert.Equal(t, params.OutputHash, analysis.OutputHash)
	assert.Equal(t, params.SchemaVersion, analysis.SchemaVersion)
}

func TestNewDocumentAnalysis_Defaults(t *testing.T) {
	params := DocumentAnalysisParams{
		OrganizationID: uuid.New(),
		DocumentID:     uuid.New(),
		EvidenceID:     uuid.New(),
		Type:           AnalysisTypeSummarization,
		Content:        map[string]any{"summary": "test"},
		InputHash:      "input-hash",
		OutputHash:     "output-hash",
		CreatedAt:      time.Now().UTC(),
	}

	analysis, err := NewDocumentAnalysis(params)

	require.NoError(t, err)
	assert.Equal(t, AnalysisSourceAIModel, analysis.Source)
	assert.Equal(t, AnalysisStatusPendingReview, analysis.Status)
	assert.Equal(t, "1.0", analysis.SchemaVersion)
}

func TestNewDocumentAnalysis_MissingOrgID(t *testing.T) {
	params := DocumentAnalysisParams{
		DocumentID: uuid.New(),
		EvidenceID: uuid.New(),
		Type:       AnalysisTypeClassification,
		Content:    map[string]any{"key": "value"},
		InputHash:  "input-hash",
		OutputHash: "output-hash",
		CreatedAt:  time.Now().UTC(),
	}

	_, err := NewDocumentAnalysis(params)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")
}

func TestNewDocumentAnalysis_MissingDocumentID(t *testing.T) {
	params := DocumentAnalysisParams{
		OrganizationID: uuid.New(),
		EvidenceID:     uuid.New(),
		Type:           AnalysisTypeClassification,
		Content:        map[string]any{"key": "value"},
		InputHash:      "input-hash",
		OutputHash:     "output-hash",
		CreatedAt:      time.Now().UTC(),
	}

	_, err := NewDocumentAnalysis(params)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "document ID is required")
}

func TestNewDocumentAnalysis_MissingEvidenceID(t *testing.T) {
	params := DocumentAnalysisParams{
		OrganizationID: uuid.New(),
		DocumentID:     uuid.New(),
		Type:           AnalysisTypeClassification,
		Content:        map[string]any{"key": "value"},
		InputHash:      "input-hash",
		OutputHash:     "output-hash",
		CreatedAt:      time.Now().UTC(),
	}

	_, err := NewDocumentAnalysis(params)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "evidence ID is required")
}

func TestNewDocumentAnalysis_InvalidType(t *testing.T) {
	params := DocumentAnalysisParams{
		OrganizationID: uuid.New(),
		DocumentID:     uuid.New(),
		EvidenceID:     uuid.New(),
		Type:           AnalysisType("INVALID"),
		Content:        map[string]any{"key": "value"},
		InputHash:      "input-hash",
		OutputHash:     "output-hash",
		CreatedAt:      time.Now().UTC(),
	}

	_, err := NewDocumentAnalysis(params)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid analysis type")
}

func TestNewDocumentAnalysis_ConfidenceOutOfRange(t *testing.T) {
	confidence := 1.5
	params := DocumentAnalysisParams{
		OrganizationID: uuid.New(),
		DocumentID:     uuid.New(),
		EvidenceID:     uuid.New(),
		Type:           AnalysisTypeClassification,
		Content:        map[string]any{"key": "value"},
		Confidence:     &confidence,
		InputHash:      "input-hash",
		OutputHash:     "output-hash",
		CreatedAt:      time.Now().UTC(),
	}

	_, err := NewDocumentAnalysis(params)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "confidence must be between 0 and 1")
}

func TestNewDocumentAnalysis_ContentTooLarge(t *testing.T) {
	params := DocumentAnalysisParams{
		OrganizationID: uuid.New(),
		DocumentID:     uuid.New(),
		EvidenceID:     uuid.New(),
		Type:           AnalysisTypeClassification,
		Content:        map[string]any{"large": string(make([]byte, MaxAnalysisContentSize+1))},
		InputHash:      "input-hash",
		OutputHash:     "output-hash",
		CreatedAt:      time.Now().UTC(),
	}

	_, err := NewDocumentAnalysis(params)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds max size")
}

func TestDocumentAnalysis_Accept(t *testing.T) {
	analysis := createTestAnalysis(t)
	reviewerID := uuid.New()

	err := analysis.Accept(reviewerID, "looks good")

	assert.NoError(t, err)
	assert.Equal(t, AnalysisStatusAccepted, analysis.Status)
	assert.Equal(t, reviewerID, *analysis.ReviewedBy)
	assert.Equal(t, "looks good", analysis.ReviewNotes)
	assert.NotNil(t, analysis.ReviewedAt)
}

func TestDocumentAnalysis_Accept_MissingReviewer(t *testing.T) {
	analysis := createTestAnalysis(t)

	err := analysis.Accept(uuid.Nil, "notes")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reviewer ID is required")
}

func TestDocumentAnalysis_Reject(t *testing.T) {
	analysis := createTestAnalysis(t)
	reviewerID := uuid.New()

	err := analysis.Reject(reviewerID, "not accurate")

	assert.NoError(t, err)
	assert.Equal(t, AnalysisStatusRejected, analysis.Status)
	assert.Equal(t, reviewerID, *analysis.ReviewedBy)
	assert.Equal(t, "not accurate", analysis.ReviewNotes)
	assert.NotNil(t, analysis.ReviewedAt)
}

func TestIsValidAnalysisType(t *testing.T) {
	assert.True(t, isValidAnalysisType(AnalysisTypeClassification))
	assert.True(t, isValidAnalysisType(AnalysisTypeTextExtraction))
	assert.True(t, isValidAnalysisType(AnalysisTypeStructuredExtraction))
	assert.True(t, isValidAnalysisType(AnalysisTypeSummarization))
	assert.True(t, isValidAnalysisType(AnalysisTypeRelevanceDetection))
	assert.False(t, isValidAnalysisType(AnalysisType("INVALID")))
}

func createTestAnalysis(t *testing.T) *DocumentAnalysis {
	t.Helper()
	analysis, err := NewDocumentAnalysis(DocumentAnalysisParams{
		OrganizationID: uuid.New(),
		DocumentID:     uuid.New(),
		EvidenceID:     uuid.New(),
		Type:           AnalysisTypeClassification,
		Content:        map[string]any{"key": "value"},
		InputHash:      "input-hash",
		OutputHash:     "output-hash",
		CreatedAt:      time.Now().UTC(),
	})
	require.NoError(t, err)
	return analysis
}

func float64Ptr(f float64) *float64 {
	return &f
}
