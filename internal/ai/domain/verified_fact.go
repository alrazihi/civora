package domain

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrVerifiedFactNotFound     = errors.New("verified fact not found")
	ErrVerifiedFactInvalidInput = errors.New("verified fact invalid input")
)

type VerifiedFact struct {
	ID              uuid.UUID         `json:"id"`
	OrganizationID  uuid.UUID         `json:"organization_id"`
	CaseID          *uuid.UUID        `json:"case_id,omitempty"`
	ObservationID   uuid.UUID         `json:"observation_id"`
	Type            ObservationType   `json:"type"`
	Value           map[string]any    `json:"value"`
	OriginalValue   map[string]any    `json:"original_value"`
	CorrectedValue  map[string]any    `json:"corrected_value,omitempty"`
	Provenance      *Provenance       `json:"provenance,omitempty"`
	ReviewAction    ObservationStatus `json:"review_action"`
	ReviewerID      uuid.UUID         `json:"reviewer_id"`
	ReviewNotes     string            `json:"review_notes"`
	CreatedAt       time.Time         `json:"created_at"`
	VerifiedAt      time.Time         `json:"verified_at"`
	Source          string            `json:"source"`
	Model           *ModelInfo        `json:"model,omitempty"`
	InputHash       string            `json:"input_hash"`
	OutputHash      string            `json:"output_hash"`
}

type Provenance struct {
	Source         string            `json:"source"`
	OriginalSource string            `json:"original_source"`
	VerifiedBy     uuid.UUID         `json:"verified_by"`
	VerifiedAt     time.Time         `json:"verified_at"`
	Model          *ModelInfo        `json:"model,omitempty"`
	References     []SourceReference `json:"references,omitempty"`
	Metadata       map[string]any    `json:"metadata,omitempty"`
}

type VerifiedFactParams struct {
	OrganizationID  uuid.UUID
	CaseID          *uuid.UUID
	ObservationID   uuid.UUID
	Type            ObservationType
	Value           map[string]any
	OriginalValue   map[string]any
	CorrectedValue  map[string]any
	ReviewAction    ObservationStatus
	ReviewerID      uuid.UUID
	ReviewNotes     string
	Source          string
	Model           *ModelInfo
	InputHash       string
	OutputHash      string
}

const MaxVerifiedFactValueSize = 100_000

func NewVerifiedFact(params VerifiedFactParams) (*VerifiedFact, error) {
	if params.OrganizationID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization ID is required", ErrVerifiedFactInvalidInput)
	}
	if params.ObservationID == uuid.Nil {
		return nil, fmt.Errorf("%w: observation ID is required", ErrVerifiedFactInvalidInput)
	}
	if params.ReviewerID == uuid.Nil {
		return nil, fmt.Errorf("%w: reviewer ID is required", ErrVerifiedFactInvalidInput)
	}
	if params.Type != "" && !isValidObservationType(params.Type) {
		return nil, fmt.Errorf("%w: invalid observation type %q", ErrVerifiedFactInvalidInput, params.Type)
	}
	if params.Source == "" {
		params.Source = "AI_OBSERVATION"
	}
	if params.InputHash == "" {
		params.InputHash = verifiedFactHash(params.Value)
	}
	if params.OutputHash == "" {
		params.OutputHash = verifiedFactHash(params.Value)
	}
	if params.ReviewAction == "" {
		params.ReviewAction = ObservationStatusAccepted
	}

	source := "AI_OBSERVATION"
	if params.ReviewAction == ObservationStatusCorrected {
		source = "HUMAN_CORRECTION"
	} else if params.ReviewAction == ObservationStatusAccepted {
		source = "HUMAN_ACCEPTED"
	}

	valueSize := 0
	for _, v := range params.Value {
		switch s := v.(type) {
		case string:
			valueSize += len(s)
		case []byte:
			valueSize += len(s)
		case map[string]any:
			b, _ := json.Marshal(s)
			valueSize += len(b)
		}
	}
	if valueSize > MaxVerifiedFactValueSize {
		return nil, fmt.Errorf("%w: verified fact value exceeds max size of %d bytes", ErrVerifiedFactInvalidInput, MaxVerifiedFactValueSize)
	}

	now := time.Now().UTC()

	provenance := &Provenance{
		Source:         source,
		OriginalSource: params.Source,
		VerifiedBy:     params.ReviewerID,
		VerifiedAt:     now,
		Model:          params.Model,
		References:     nil,
		Metadata: map[string]any{
			"review_action": string(params.ReviewAction),
			"review_notes":  params.ReviewNotes,
		},
	}

	correctedValue := copyMap(params.CorrectedValue)
	if correctedValue == nil {
		correctedValue = nil
	}

	return &VerifiedFact{
		ID:              uuid.New(),
		OrganizationID:  params.OrganizationID,
		CaseID:          params.CaseID,
		ObservationID:   params.ObservationID,
		Type:            params.Type,
		Value:           copyMap(params.Value),
		OriginalValue:   copyMap(params.OriginalValue),
		CorrectedValue:  correctedValue,
		Provenance:      provenance,
		ReviewAction:    params.ReviewAction,
		ReviewerID:      params.ReviewerID,
		ReviewNotes:     params.ReviewNotes,
		CreatedAt:       now,
		VerifiedAt:      now,
		Source:          source,
		Model:           params.Model,
		InputHash:       params.InputHash,
		OutputHash:      params.OutputHash,
	}, nil
}

func copyMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func verifiedFactHash(data any) string {
	b, _ := json.Marshal(data)
	if len(b) == 0 {
		return hex.EncodeToString([]byte(fmt.Sprintf("%v", data)))
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (v *VerifiedFact) ValidateAction(action ObservationStatus) error {
	switch action {
	case ObservationStatusAccepted,
		ObservationStatusRejected,
		ObservationStatusCorrected,
		ObservationStatusDismissed:
		return nil
	default:
		return fmt.Errorf("%w: invalid review action %q", ErrVerifiedFactInvalidInput, action)
	}
}

type VerifiedFactRepository interface {
	Save(ctx context.Context, f *VerifiedFact) error
	SaveTx(ctx context.Context, tx *sql.Tx, f *VerifiedFact) error
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*VerifiedFact, error)
	FindByObservation(ctx context.Context, orgID, observationID uuid.UUID) (*VerifiedFact, error)
	FindByCase(ctx context.Context, orgID, caseID uuid.UUID, limit, offset int) ([]*VerifiedFact, int, error)
	CountByCase(ctx context.Context, orgID, caseID uuid.UUID) (int, error)
	Delete(ctx context.Context, orgID, id uuid.UUID) error
}
