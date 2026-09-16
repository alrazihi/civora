package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewObservation_Success(t *testing.T) {
	orgID := uuid.New()
	evidenceID := uuid.New()

	obs, err := NewObservation(ObservationParams{
		OrganizationID: orgID,
		EvidenceID:     evidenceID,
		Type:           ObservationTypeSummary,
		Content:        map[string]any{"text": "sample"},
		InputHash:      "abc123",
		OutputHash:     "def456",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if obs.ID == uuid.Nil {
		t.Fatal("expected non-nil ID")
	}
	if obs.OrganizationID != orgID {
		t.Errorf("expected org ID %s, got %s", orgID, obs.OrganizationID)
	}
	if obs.EvidenceID != evidenceID {
		t.Errorf("expected evidence ID %s, got %s", evidenceID, obs.EvidenceID)
	}
	if obs.Type != ObservationTypeSummary {
		t.Errorf("expected type %s, got %s", ObservationTypeSummary, obs.Type)
	}
	if obs.Source != ObservationSourceAIModel {
		t.Errorf("expected source %s, got %s", ObservationSourceAIModel, obs.Source)
	}
	if obs.Status != ObservationStatusPendingReview {
		t.Errorf("expected status %s, got %s", ObservationStatusPendingReview, obs.Status)
	}
	if obs.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestNewObservation_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		params  ObservationParams
		wantErr bool
	}{
		{
			name:    "missing organization ID",
			params:  ObservationParams{EvidenceID: uuid.New()},
			wantErr: true,
		},
		{
			name:    "missing evidence ID",
			params:  ObservationParams{OrganizationID: uuid.New()},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewObservation(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("expected error: %v, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestObservation_Accept(t *testing.T) {
	obs := &Observation{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		EvidenceID:     uuid.New(),
		Status:         ObservationStatusPendingReview,
	}

	reviewerID := uuid.New()
	err := obs.Accept(reviewerID, "reviewer notes")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if obs.Status != ObservationStatusAccepted {
		t.Errorf("expected status %s, got %s", ObservationStatusAccepted, obs.Status)
	}
	if obs.ReviewedBy == nil || *obs.ReviewedBy != reviewerID {
		t.Error("expected reviewed_by to be set")
	}
	if obs.ReviewedAt == nil {
		t.Error("expected reviewed_at to be set")
	}
	if obs.ReviewNotes != "reviewer notes" {
		t.Errorf("expected notes 'reviewer notes', got %s", obs.ReviewNotes)
	}
}

func TestObservation_Accept_NilReviewerID(t *testing.T) {
	obs := &Observation{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		EvidenceID:     uuid.New(),
		Status:         ObservationStatusPendingReview,
	}

	err := obs.Accept(uuid.Nil, "test")
	if err == nil {
		t.Fatal("expected error for nil reviewer ID")
	}
	if obs.Status == ObservationStatusAccepted {
		t.Error("status should not have changed")
	}
}

func TestObservation_Reject(t *testing.T) {
	obs := &Observation{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		EvidenceID:     uuid.New(),
		Status:         ObservationStatusPendingReview,
	}

	reviewerID := uuid.New()
	err := obs.Reject(reviewerID, "incomplete")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if obs.Status != ObservationStatusRejected {
		t.Errorf("expected status %s, got %s", ObservationStatusRejected, obs.Status)
	}
	if obs.ReviewNotes != "incomplete" {
		t.Errorf("expected notes 'incomplete', got %s", obs.ReviewNotes)
	}
}

func TestObservation_Defaults(t *testing.T) {
	now := time.Now().UTC()
	obs := &Observation{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		EvidenceID:     uuid.New(),
		Type:           "",
		Source:         "",
		Status:         "",
		CreatedAt:      now,
	}

	// Verify defaults are applied in NewObservation
	obs2, err := NewObservation(ObservationParams{
		OrganizationID: uuid.New(),
		EvidenceID:     uuid.New(),
	})
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	if obs2.Source != ObservationSourceAIModel {
		t.Errorf("expected default source %s, got %s", ObservationSourceAIModel, obs2.Source)
	}
	if obs2.Status != ObservationStatusPendingReview {
		t.Errorf("expected default status %s, got %s", ObservationStatusPendingReview, obs2.Status)
	}

	// Verify obs is untouched by defaults test
	_ = obs
}
