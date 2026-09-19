package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewObservation_Success(t *testing.T) {
	orgID := uuid.New()
	caseID := uuid.New()

	obs, err := NewObservation(ObservationParams{
		OrganizationID:   orgID,
		CaseID:           &caseID,
		Type:             ObservationTypeSummary,
		Content:          map[string]any{"text": "sample"},
		InputHash:        "abc123",
		OutputHash:       "def456",
		Statement:        "Case summary statement",
		SourceReferences: []SourceReference{{Type: "evidence", ID: "uuid"}},
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
	if obs.CaseID == nil || *obs.CaseID != caseID {
		t.Errorf("expected case ID %s, got %v", caseID, obs.CaseID)
	}
	if obs.Type != ObservationTypeSummary {
		t.Errorf("expected type %s, got %s", ObservationTypeSummary, obs.Type)
	}
	if obs.Source != ObservationSourceAIModel {
		t.Errorf("expected source %s, got %s", ObservationSourceAIModel, obs.Source)
	}
	if obs.Status != ObservationStatusOpen {
		t.Errorf("expected status %s, got %s", ObservationStatusOpen, obs.Status)
	}
	if obs.Statement != "Case summary statement" {
		t.Errorf("expected statement 'Case summary statement', got %s", obs.Statement)
	}
	if len(obs.SourceReferences) != 1 || obs.SourceReferences[0].Type != "evidence" {
		t.Errorf("expected source references, got %v", obs.SourceReferences)
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
			params:  ObservationParams{CaseID: uuidPtr(uuid.New())},
			wantErr: true,
		},
		{
			name:    "missing case and evidence ID",
			params:  ObservationParams{OrganizationID: uuid.New()},
			wantErr: true,
		},
		{
			name:    "invalid observation type",
			params:  ObservationParams{OrganizationID: uuid.New(), CaseID: uuidPtr(uuid.New()), Type: "INVALID_TYPE"},
			wantErr: true,
		},
		{
			name: "confidence out of range",
			params: ObservationParams{
				OrganizationID: uuid.New(),
				CaseID:         uuidPtr(uuid.New()),
				Confidence:     floatPtr(1.5),
			},
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
		CaseID:         uuidPtr(uuid.New()),
		Status:         ObservationStatusOpen,
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
		CaseID:         uuidPtr(uuid.New()),
		Status:         ObservationStatusOpen,
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
		CaseID:         uuidPtr(uuid.New()),
		Status:         ObservationStatusOpen,
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

func TestObservation_Correct(t *testing.T) {
	obs := &Observation{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		CaseID:         uuidPtr(uuid.New()),
		Status:         ObservationStatusOpen,
	}

	reviewerID := uuid.New()
	err := obs.Correct(reviewerID, "corrected statement")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if obs.Status != ObservationStatusCorrected {
		t.Errorf("expected status %s, got %s", ObservationStatusCorrected, obs.Status)
	}
	if obs.ReviewNotes != "corrected statement" {
		t.Errorf("expected notes 'corrected statement', got %s", obs.ReviewNotes)
	}
}

func TestObservation_Dismiss(t *testing.T) {
	obs := &Observation{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		CaseID:         uuidPtr(uuid.New()),
		Status:         ObservationStatusOpen,
	}

	reviewerID := uuid.New()
	err := obs.Dismiss(reviewerID, "not applicable")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if obs.Status != ObservationStatusDismissed {
		t.Errorf("expected status %s, got %s", ObservationStatusDismissed, obs.Status)
	}
	if obs.ReviewNotes != "not applicable" {
		t.Errorf("expected notes 'not applicable', got %s", obs.ReviewNotes)
	}
}

func TestObservation_Defaults(t *testing.T) {
	obs, err := NewObservation(ObservationParams{
		OrganizationID: uuid.New(),
		CaseID:         uuidPtr(uuid.New()),
	})
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	if obs.Source != ObservationSourceAIModel {
		t.Errorf("expected default source %s, got %s", ObservationSourceAIModel, obs.Source)
	}
	if obs.Status != ObservationStatusOpen {
		t.Errorf("expected default status %s, got %s", ObservationStatusOpen, obs.Status)
	}
	if obs.Statement != "" {
		t.Errorf("expected empty statement, got %s", obs.Statement)
	}
	if obs.SourceReferences != nil {
		t.Errorf("expected nil source references, got %v", obs.SourceReferences)
	}
	_ = obs
}

func TestIsValidObservationType(t *testing.T) {
	tests := []struct {
		t     ObservationType
		valid bool
	}{
		{ObservationTypeSummary, true},
		{ObservationTypeEntityExtraction, true},
		{ObservationTypeClassification, true},
		{ObservationTypeInconsistency, true},
		{ObservationTypeMissingInformation, true},
		{ObservationTypeRelevantEvidence, true},
		{ObservationTypeIncompleteDoc, true},
		{"INVALID", false},
		{"", false},
	}

	for _, tt := range tests {
		got := isValidObservationType(tt.t)
		if got != tt.valid {
			t.Errorf("isValidObservationType(%q) = %v, want %v", tt.t, got, tt.valid)
		}
	}
}

func TestIsValidObservationStatus(t *testing.T) {
	tests := []struct {
		s     ObservationStatus
		valid bool
	}{
		{ObservationStatusOpen, true},
		{ObservationStatusPendingReview, true},
		{ObservationStatusAccepted, true},
		{ObservationStatusRejected, true},
		{ObservationStatusCorrected, true},
		{ObservationStatusDismissed, true},
		{"INVALID", false},
		{"", false},
	}

	for _, tt := range tests {
		got := isValidObservationStatus(tt.s)
		if got != tt.valid {
			t.Errorf("isValidObservationStatus(%q) = %v, want %v", tt.s, got, tt.valid)
		}
	}
}

func uuidPtr(u uuid.UUID) *uuid.UUID {
	return &u
}

func floatPtr(f float64) *float64 {
	return &f
}
