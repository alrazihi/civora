package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewVerifiedFact_Success(t *testing.T) {
	orgID := uuid.New()
	caseID := uuid.New()
	observationID := uuid.New()
	reviewerID := uuid.New()

	fact, err := NewVerifiedFact(VerifiedFactParams{
		OrganizationID: orgID,
		CaseID:         &caseID,
		ObservationID:  observationID,
		Type:           ObservationTypeSummary,
		Value:          map[string]any{"text": "sample"},
		OriginalValue:  map[string]any{"text": "sample"},
		ReviewAction:   ObservationStatusAccepted,
		ReviewerID:     reviewerID,
		ReviewNotes:    "reviewer notes",
		Source:         "AI_OBSERVATION",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fact.ID == uuid.Nil {
		t.Fatal("expected non-nil ID")
	}
	if fact.OrganizationID != orgID {
		t.Errorf("expected org ID %s, got %s", orgID, fact.OrganizationID)
	}
	if fact.ObservationID != observationID {
		t.Errorf("expected observation ID %s, got %s", observationID, fact.ObservationID)
	}
	if fact.ReviewAction != ObservationStatusAccepted {
		t.Errorf("expected review action %s, got %s", ObservationStatusAccepted, fact.ReviewAction)
	}
	if fact.ReviewerID != reviewerID {
		t.Errorf("expected reviewer ID %s, got %s", reviewerID, fact.ReviewerID)
	}
	if fact.ReviewNotes != "reviewer notes" {
		t.Errorf("expected notes 'reviewer notes', got %s", fact.ReviewNotes)
	}
	if fact.Provenance == nil {
		t.Fatal("expected non-nil provenance")
	}
	if fact.Provenance.VerifiedBy != reviewerID {
		t.Errorf("expected provenance verified by %s, got %s", reviewerID, fact.Provenance.VerifiedBy)
	}
	if fact.Source != "HUMAN_ACCEPTED" {
		t.Errorf("expected source HUMAN_ACCEPTED, got %s", fact.Source)
	}
	if fact.Provenance.Source != "HUMAN_ACCEPTED" {
		t.Errorf("expected provenance source HUMAN_ACCEPTED, got %s", fact.Provenance.Source)
	}
	if fact.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if fact.VerifiedAt.IsZero() {
		t.Error("expected non-zero VerifiedAt")
	}
}

func TestNewVerifiedFact_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		params  VerifiedFactParams
		wantErr bool
	}{
		{
			name:    "missing organization ID",
			params:  VerifiedFactParams{ObservationID: uuid.New(), ReviewerID: uuid.New()},
			wantErr: true,
		},
		{
			name:    "missing observation ID",
			params:  VerifiedFactParams{OrganizationID: uuid.New(), ReviewerID: uuid.New()},
			wantErr: true,
		},
		{
			name:    "missing reviewer ID",
			params:  VerifiedFactParams{OrganizationID: uuid.New(), ObservationID: uuid.New()},
			wantErr: true,
		},
		{
			name: "invalid observation type",
			params: VerifiedFactParams{
				OrganizationID: uuid.New(),
				ObservationID:  uuid.New(),
				ReviewerID:     uuid.New(),
				Type:           "INVALID_TYPE",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewVerifiedFact(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("expected error: %v, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestVerifiedFact_Defaults(t *testing.T) {
	fact, err := NewVerifiedFact(VerifiedFactParams{
		OrganizationID: uuid.New(),
		ObservationID:  uuid.New(),
		ReviewerID:     uuid.New(),
	})
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	if fact.Source != "HUMAN_ACCEPTED" {
		t.Errorf("expected default source HUMAN_ACCEPTED, got %s", fact.Source)
	}
	if fact.ReviewAction != ObservationStatusAccepted {
		t.Errorf("expected default review action ACCEPTED, got %s", fact.ReviewAction)
	}
	if fact.Provenance == nil {
		t.Fatal("expected non-nil provenance")
	}
	if fact.Provenance.Source != "HUMAN_ACCEPTED" {
		t.Errorf("expected provenance source HUMAN_ACCEPTED, got %s", fact.Provenance.Source)
	}
}

func TestVerifiedFact_CorrectedSource(t *testing.T) {
	fact, err := NewVerifiedFact(VerifiedFactParams{
		OrganizationID: uuid.New(),
		ObservationID:  uuid.New(),
		ReviewerID:     uuid.New(),
		ReviewAction:   ObservationStatusCorrected,
		ReviewNotes:    "corrected",
	})
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	if fact.Source != "HUMAN_CORRECTION" {
		t.Errorf("expected source HUMAN_CORRECTION, got %s", fact.Source)
	}
	if fact.Provenance == nil {
		t.Fatal("expected non-nil provenance")
	}
	if fact.Provenance.Source != "HUMAN_CORRECTION" {
		t.Errorf("expected provenance source HUMAN_CORRECTION, got %s", fact.Provenance.Source)
	}
}

func TestVerifiedFact_AcceptedSource(t *testing.T) {
	fact, err := NewVerifiedFact(VerifiedFactParams{
		OrganizationID: uuid.New(),
		ObservationID:  uuid.New(),
		ReviewerID:     uuid.New(),
		ReviewAction:   ObservationStatusAccepted,
		ReviewNotes:    "accepted",
	})
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	if fact.Source != "HUMAN_ACCEPTED" {
		t.Errorf("expected source HUMAN_ACCEPTED, got %s", fact.Source)
	}
	if fact.Provenance == nil {
		t.Fatal("expected non-nil provenance")
	}
	if fact.Provenance.Source != "HUMAN_ACCEPTED" {
		t.Errorf("expected provenance source HUMAN_ACCEPTED, got %s", fact.Provenance.Source)
	}
}

func TestVerifiedFact_ValidateAction(t *testing.T) {
	tests := []struct {
		action ObservationStatus
		want   bool
	}{
		{ObservationStatusAccepted, true},
		{ObservationStatusRejected, true},
		{ObservationStatusCorrected, true},
		{ObservationStatusDismissed, true},
		{ObservationStatusOpen, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			fact := &VerifiedFact{}
			err := fact.ValidateAction(tt.action)
			if tt.want && err != nil {
				t.Errorf("expected no error for action %s, got: %v", tt.action, err)
			}
			if !tt.want && err == nil {
				t.Errorf("expected error for action %s, got nil", tt.action)
			}
		})
	}
}
