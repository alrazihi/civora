package domain

import (
	"context"
	"errors"
	"fmt"
	"time"

	"database/sql"

	"github.com/google/uuid"
)

type ObservationType string

const (
	ObservationTypeSummary          ObservationType = "SUMMARY"
	ObservationTypeEntityExtraction ObservationType = "ENTITY_EXTRACTION"
	ObservationTypeClassification   ObservationType = "CLASSIFICATION"
	ObservationTypeInconsistency    ObservationType = "INCONSISTENCY"
)

type ObservationStatus string

const (
	ObservationStatusPendingReview ObservationStatus = "PENDING_REVIEW"
	ObservationStatusAccepted      ObservationStatus = "ACCEPTED"
	ObservationStatusRejected      ObservationStatus = "REJECTED"
)

type ObservationSource string

const (
	ObservationSourceAIModel ObservationSource = "AI_MODEL"
	ObservationSourceHuman   ObservationSource = "HUMAN"
)

var (
	ErrObservationNotFound     = errors.New("observation not found")
	ErrObservationInvalidInput = errors.New("invalid observation input")
)

type ModelInfo struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Provider string `json:"provider"`
}

type Observation struct {
	ID             uuid.UUID         `json:"id"`
	OrganizationID uuid.UUID         `json:"organization_id"`
	EvidenceID     uuid.UUID         `json:"evidence_id"`
	Type           ObservationType   `json:"type"`
	Source         ObservationSource `json:"source"`
	Status         ObservationStatus `json:"status"`
	Model          *ModelInfo        `json:"model,omitempty"`
	Content        map[string]any    `json:"content"`
	Confidence     *float64          `json:"confidence,omitempty"`
	InputHash      string            `json:"input_hash"`
	OutputHash     string            `json:"output_hash"`
	CreatedAt      time.Time         `json:"created_at"`
	CreatedBy      *uuid.UUID        `json:"created_by,omitempty"`
	ReviewedAt     *time.Time        `json:"reviewed_at,omitempty"`
	ReviewedBy     *uuid.UUID        `json:"reviewed_by,omitempty"`
	ReviewNotes    string            `json:"review_notes,omitempty"`
}

type ObservationParams struct {
	OrganizationID uuid.UUID
	EvidenceID     uuid.UUID
	Type           ObservationType
	Source         ObservationSource
	Status         ObservationStatus
	Model          *ModelInfo
	Content        map[string]any
	Confidence     *float64
	InputHash      string
	OutputHash     string
	CreatedBy      *uuid.UUID
	CreatedAt      time.Time
}

const MaxObservationContentSize = 100_000

func NewObservation(params ObservationParams) (*Observation, error) {
	if params.OrganizationID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization ID is required", ErrObservationInvalidInput)
	}
	if params.EvidenceID == uuid.Nil {
		return nil, fmt.Errorf("%w: evidence ID is required", ErrObservationInvalidInput)
	}
	if params.Source == "" {
		params.Source = ObservationSourceAIModel
	}
	if params.Status == "" {
		params.Status = ObservationStatusPendingReview
	}
	if params.CreatedAt.IsZero() {
		params.CreatedAt = time.Now().UTC()
	}

	obs := &Observation{
		ID:             uuid.New(),
		OrganizationID: params.OrganizationID,
		EvidenceID:     params.EvidenceID,
		Type:           params.Type,
		Source:         params.Source,
		Status:         params.Status,
		Model:          params.Model,
		Content:        copyContent(params.Content),
		Confidence:     params.Confidence,
		InputHash:      params.InputHash,
		OutputHash:     params.OutputHash,
		CreatedAt:      params.CreatedAt,
		CreatedBy:      params.CreatedBy,
	}

	return obs, nil
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

func (o *Observation) Accept(reviewerID uuid.UUID, notes string) error {
	if reviewerID == uuid.Nil {
		return fmt.Errorf("%w: reviewer ID is required", ErrObservationInvalidInput)
	}
	now := time.Now().UTC()
	o.Status = ObservationStatusAccepted
	o.ReviewedAt = &now
	o.ReviewedBy = &reviewerID
	o.ReviewNotes = notes
	return nil
}

func (o *Observation) Reject(reviewerID uuid.UUID, notes string) error {
	if reviewerID == uuid.Nil {
		return fmt.Errorf("%w: reviewer ID is required", ErrObservationInvalidInput)
	}
	now := time.Now().UTC()
	o.Status = ObservationStatusRejected
	o.ReviewedAt = &now
	o.ReviewedBy = &reviewerID
	o.ReviewNotes = notes
	return nil
}

type DocumentContent struct {
	DocumentID  uuid.UUID
	FileName    string
	ContentType string
	Content     []byte
	Checksum    string
}

type ProviderOptions struct {
	Types     []ObservationType
	MaxTokens int
}

type ObservationRepository interface {
	Save(ctx context.Context, o *Observation) error
	SaveTx(ctx context.Context, tx *sql.Tx, o *Observation) error
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*Observation, error)
	FindByEvidence(ctx context.Context, orgID, evidenceID uuid.UUID, limit, offset int) ([]*Observation, int, error)
	UpdateStatus(ctx context.Context, orgID, observationID uuid.UUID, status ObservationStatus, reviewerID *uuid.UUID, notes string) error
	CountByEvidence(ctx context.Context, orgID, evidenceID uuid.UUID) (int, error)
}
