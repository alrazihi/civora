package postgres

import (
	"context"
	"database/sql"
	"testing"

	"github.com/alrazihi/civora/internal/ai/domain"
	"github.com/google/uuid"
)

func setupVerifiedFactRepo(t *testing.T) *PostgresVerifiedFactRepository {
	t.Helper()

	db, err := sql.Open("postgres", "postgres://postgres:postgres@localhost:5432/civora_test?sslmode=disable")
	if err != nil {
		t.Skipf("skipping integration test: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS ai_verified_facts (
		id UUID PRIMARY KEY,
		organization_id UUID NOT NULL,
		case_id UUID,
		observation_id UUID NOT NULL,
		observation_type TEXT NOT NULL,
		value JSONB NOT NULL,
		original_value JSONB NOT NULL,
		corrected_value JSONB,
		provenance JSONB NOT NULL,
		review_action TEXT NOT NULL,
		reviewer_id UUID NOT NULL,
		review_notes TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		verified_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		source TEXT NOT NULL,
		model JSONB,
		input_hash TEXT NOT NULL,
		output_hash TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	return NewPostgresVerifiedFactRepository(db)
}

func TestPostgresVerifiedFactRepo_SaveAndFindByID(t *testing.T) {
	repo := setupVerifiedFactRepo(t)
	if repo == nil {
		return
	}

	ctx := context.Background()
	orgID := uuid.New()
	caseID := uuid.New()
	observationID := uuid.New()
	reviewerID := uuid.New()

	fact, err := domain.NewVerifiedFact(domain.VerifiedFactParams{
		OrganizationID: orgID,
		CaseID:         &caseID,
		ObservationID:  observationID,
		Type:           domain.ObservationTypeSummary,
		Value:          map[string]any{"text": "sample"},
		OriginalValue:  map[string]any{"text": "sample"},
		ReviewAction:   domain.ObservationStatusAccepted,
		ReviewerID:     reviewerID,
		ReviewNotes:    "reviewer notes",
		Source:         "AI_OBSERVATION",
		Model:          &domain.ModelInfo{Name: "test-model", Version: "1.0", Provider: "test"},
		InputHash:      "abc123",
		OutputHash:     "def456",
	})
	if err != nil {
		t.Fatalf("failed to create fact: %v", err)
	}

	if err := repo.Save(ctx, fact); err != nil {
		t.Fatalf("failed to save fact: %v", err)
	}

	found, err := repo.FindByID(ctx, orgID, fact.ID)
	if err != nil {
		t.Fatalf("failed to find fact: %v", err)
	}

	if found.ID != fact.ID {
		t.Errorf("expected ID %s, got %s", fact.ID, found.ID)
	}
	if found.OrganizationID != orgID {
		t.Errorf("expected org ID %s, got %s", orgID, found.OrganizationID)
	}
	if found.ObservationID != observationID {
		t.Errorf("expected observation ID %s, got %s", observationID, found.ObservationID)
	}
	if found.ReviewAction != domain.ObservationStatusAccepted {
		t.Errorf("expected review action %s, got %s", domain.ObservationStatusAccepted, found.ReviewAction)
	}
	if found.Source != "AI_OBSERVATION" {
		t.Errorf("expected source AI_OBSERVATION, got %s", found.Source)
	}
	if found.Provenance == nil {
		t.Error("expected non-nil provenance")
	} else {
		if found.Provenance.VerifiedBy != reviewerID {
			t.Errorf("expected provenance verified by %s, got %s", reviewerID, found.Provenance.VerifiedBy)
		}
	}
}

func TestPostgresVerifiedFactRepo_FindByObservation(t *testing.T) {
	repo := setupVerifiedFactRepo(t)
	if repo == nil {
		return
	}

	ctx := context.Background()
	orgID := uuid.New()
	observationID := uuid.New()
	reviewerID := uuid.New()

	fact, err := domain.NewVerifiedFact(domain.VerifiedFactParams{
		OrganizationID: orgID,
		ObservationID:  observationID,
		Type:           domain.ObservationTypeSummary,
		Value:          map[string]any{"text": "sample"},
		OriginalValue:  map[string]any{"text": "sample"},
		ReviewAction:   domain.ObservationStatusAccepted,
		ReviewerID:     reviewerID,
		ReviewNotes:    "reviewer notes",
		Source:         "AI_OBSERVATION",
		InputHash:      "abc123",
		OutputHash:     "def456",
	})
	if err != nil {
		t.Fatalf("failed to create fact: %v", err)
	}

	if err := repo.Save(ctx, fact); err != nil {
		t.Fatalf("failed to save fact: %v", err)
	}

	found, err := repo.FindByObservation(ctx, orgID, observationID)
	if err != nil {
		t.Fatalf("failed to find fact by observation: %v", err)
	}

	if found == nil {
		t.Fatal("expected non-nil fact")
	}
	if found.ObservationID != observationID {
		t.Errorf("expected observation ID %s, got %s", observationID, found.ObservationID)
	}
}

func TestPostgresVerifiedFactRepo_UniqueObservation(t *testing.T) {
	repo := setupVerifiedFactRepo(t)
	if repo == nil {
		return
	}

	ctx := context.Background()
	orgID := uuid.New()
	observationID := uuid.New()
	reviewerID := uuid.New()

	fact1, _ := domain.NewVerifiedFact(domain.VerifiedFactParams{
		OrganizationID: orgID,
		ObservationID:  observationID,
		Type:           domain.ObservationTypeSummary,
		Value:          map[string]any{"text": "sample"},
		OriginalValue:  map[string]any{"text": "sample"},
		ReviewAction:   domain.ObservationStatusAccepted,
		ReviewerID:     reviewerID,
		ReviewNotes:    "first",
		Source:         "AI_OBSERVATION",
		InputHash:      "abc123",
		OutputHash:     "def456",
	})

	fact2, _ := domain.NewVerifiedFact(domain.VerifiedFactParams{
		OrganizationID: orgID,
		ObservationID:  observationID,
		Type:           domain.ObservationTypeSummary,
		Value:          map[string]any{"text": "sample"},
		OriginalValue:  map[string]any{"text": "sample"},
		ReviewAction:   domain.ObservationStatusAccepted,
		ReviewerID:     reviewerID,
		ReviewNotes:    "second",
		Source:         "AI_OBSERVATION",
		InputHash:      "abc123",
		OutputHash:     "def456",
	})

	if err := repo.Save(ctx, fact1); err != nil {
		t.Fatalf("failed to save fact1: %v", err)
	}

	if err := repo.Save(ctx, fact2); err != nil {
		t.Logf("got expected error on duplicate observation_id: %v", err)
	} else {
		t.Error("expected error when saving duplicate observation_id")
	}
}
