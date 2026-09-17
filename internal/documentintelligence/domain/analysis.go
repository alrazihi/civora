package domain

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type AnalysisType string

const (
	AnalysisTypeClassification       AnalysisType = "CLASSIFICATION"
	AnalysisTypeTextExtraction       AnalysisType = "TEXT_EXTRACTION"
	AnalysisTypeStructuredExtraction AnalysisType = "STRUCTURED_EXTRACTION"
	AnalysisTypeSummarization        AnalysisType = "SUMMARIZATION"
	AnalysisTypeRelevanceDetection   AnalysisType = "RELEVANCE_DETECTION"
)

type AnalysisStatus string

const (
	AnalysisStatusPendingReview AnalysisStatus = "PENDING_REVIEW"
	AnalysisStatusAccepted      AnalysisStatus = "ACCEPTED"
	AnalysisStatusRejected      AnalysisStatus = "REJECTED"
)

type AnalysisSource string

const (
	AnalysisSourceAIModel AnalysisSource = "AI_MODEL"
	AnalysisSourceHuman   AnalysisSource = "HUMAN"
)

var (
	ErrAnalysisNotFound     = errors.New("analysis not found")
	ErrAnalysisInvalidInput = errors.New("invalid analysis input")
)

type ModelInfo struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Provider string `json:"provider"`
}

type DocumentAnalysis struct {
	ID             uuid.UUID      `json:"id"`
	OrganizationID uuid.UUID      `json:"organization_id"`
	DocumentID     uuid.UUID      `json:"document_id"`
	EvidenceID     uuid.UUID      `json:"evidence_id"`
	Type           AnalysisType   `json:"type"`
	Source         AnalysisSource `json:"source"`
	Status         AnalysisStatus `json:"status"`
	Model          *ModelInfo     `json:"model,omitempty"`
	Content        map[string]any `json:"content"`
	Confidence     *float64       `json:"confidence,omitempty"`
	InputHash      string         `json:"input_hash"`
	OutputHash     string         `json:"output_hash"`
	SchemaVersion  string         `json:"schema_version"`
	CreatedAt      time.Time      `json:"created_at"`
	CreatedBy      *uuid.UUID     `json:"created_by,omitempty"`
	ReviewedAt     *time.Time     `json:"reviewed_at,omitempty"`
	ReviewedBy     *uuid.UUID     `json:"reviewed_by,omitempty"`
	ReviewNotes    string         `json:"review_notes,omitempty"`
}

type DocumentAnalysisParams struct {
	OrganizationID uuid.UUID
	DocumentID     uuid.UUID
	EvidenceID     uuid.UUID
	Type           AnalysisType
	Source         AnalysisSource
	Status         AnalysisStatus
	Model          *ModelInfo
	Content        map[string]any
	Confidence     *float64
	InputHash      string
	OutputHash     string
	SchemaVersion  string
	CreatedBy      *uuid.UUID
	CreatedAt      time.Time
}

const MaxAnalysisContentSize = 200_000

const MaxDocumentBytes = 100_000

func NewDocumentAnalysis(params DocumentAnalysisParams) (*DocumentAnalysis, error) {
	if params.OrganizationID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization ID is required", ErrAnalysisInvalidInput)
	}
	if params.DocumentID == uuid.Nil {
		return nil, fmt.Errorf("%w: document ID is required", ErrAnalysisInvalidInput)
	}
	if params.EvidenceID == uuid.Nil {
		return nil, fmt.Errorf("%w: evidence ID is required", ErrAnalysisInvalidInput)
	}
	if params.Type != "" && !isValidAnalysisType(params.Type) {
		return nil, fmt.Errorf("%w: invalid analysis type %q", ErrAnalysisInvalidInput, params.Type)
	}
	if params.Confidence != nil {
		if *params.Confidence < 0 || *params.Confidence > 1 {
			return nil, fmt.Errorf("%w: confidence must be between 0 and 1", ErrAnalysisInvalidInput)
		}
	}
	if params.Source == "" {
		params.Source = AnalysisSourceAIModel
	}
	if params.Status == "" {
		params.Status = AnalysisStatusPendingReview
	}
	if params.CreatedAt.IsZero() {
		params.CreatedAt = time.Now().UTC()
	}
	if params.SchemaVersion == "" {
		params.SchemaVersion = "1.0"
	}

	contentSize := 0
	for _, v := range params.Content {
		switch s := v.(type) {
		case string:
			contentSize += len(s)
		case []byte:
			contentSize += len(s)
		}
	}
	if contentSize > MaxAnalysisContentSize {
		return nil, fmt.Errorf("%w: analysis content exceeds max size of %d bytes", ErrAnalysisInvalidInput, MaxAnalysisContentSize)
	}

	analysis := &DocumentAnalysis{
		ID:             uuid.New(),
		OrganizationID: params.OrganizationID,
		DocumentID:     params.DocumentID,
		EvidenceID:     params.EvidenceID,
		Type:           params.Type,
		Source:         params.Source,
		Status:         params.Status,
		Model:          params.Model,
		Content:        copyContent(params.Content),
		Confidence:     params.Confidence,
		InputHash:      params.InputHash,
		OutputHash:     params.OutputHash,
		SchemaVersion:  params.SchemaVersion,
		CreatedAt:      params.CreatedAt,
		CreatedBy:      params.CreatedBy,
	}

	return analysis, nil
}

func copyContent(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (a *DocumentAnalysis) Accept(reviewerID uuid.UUID, notes string) error {
	if reviewerID == uuid.Nil {
		return fmt.Errorf("%w: reviewer ID is required", ErrAnalysisInvalidInput)
	}
	now := time.Now().UTC()
	a.Status = AnalysisStatusAccepted
	a.ReviewedAt = &now
	a.ReviewedBy = &reviewerID
	a.ReviewNotes = notes
	return nil
}

func (a *DocumentAnalysis) Reject(reviewerID uuid.UUID, notes string) error {
	if reviewerID == uuid.Nil {
		return fmt.Errorf("%w: reviewer ID is required", ErrAnalysisInvalidInput)
	}
	now := time.Now().UTC()
	a.Status = AnalysisStatusRejected
	a.ReviewedAt = &now
	a.ReviewedBy = &reviewerID
	a.ReviewNotes = notes
	return nil
}

func isValidAnalysisType(t AnalysisType) bool {
	switch t {
	case AnalysisTypeClassification,
		AnalysisTypeTextExtraction,
		AnalysisTypeStructuredExtraction,
		AnalysisTypeSummarization,
		AnalysisTypeRelevanceDetection:
		return true
	default:
		return false
	}
}

type DocumentContent struct {
	DocumentID  uuid.UUID
	FileName    string
	ContentType string
	Content     []byte
	Checksum    string
}

type ProviderOptions struct {
	Types     []AnalysisType
	MaxTokens int
}

type DocumentAnalysisRepository interface {
	Save(ctx context.Context, a *DocumentAnalysis) error
	SaveTx(ctx context.Context, tx *sql.Tx, a *DocumentAnalysis) error
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*DocumentAnalysis, error)
	FindByDocument(ctx context.Context, orgID, documentID uuid.UUID, limit, offset int) ([]*DocumentAnalysis, int, error)
	FindByEvidence(ctx context.Context, orgID, evidenceID uuid.UUID, limit, offset int) ([]*DocumentAnalysis, int, error)
	UpdateStatus(ctx context.Context, orgID, analysisID uuid.UUID, status AnalysisStatus, reviewerID *uuid.UUID, notes string) error
	UpdateStatusTx(ctx context.Context, tx *sql.Tx, orgID, analysisID uuid.UUID, status AnalysisStatus, reviewerID *uuid.UUID, notes string) error
	CountByDocument(ctx context.Context, orgID, documentID uuid.UUID) (int, error)
}

type DocumentContentProvider interface {
	GetDocumentContent(ctx context.Context, orgID, evidenceID, documentID, actorID uuid.UUID) ([]byte, error)
}
