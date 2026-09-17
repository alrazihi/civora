package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewCaseSummary_Valid(t *testing.T) {
	params := CaseSummaryParams{
		OrganizationID: uuid.New(),
		CaseID:         uuid.New(),
		Type:           SummaryTypeFull,
		Content:        SummaryContent{},
	}

	summary, err := NewCaseSummary(params)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if summary.OrganizationID != params.OrganizationID {
		t.Errorf("organization ID mismatch")
	}
	if summary.Status != SummaryStatusPendingReview {
		t.Errorf("expected pending review status, got %s", summary.Status)
	}
	if summary.Source != AnalysisSourceAIModel {
		t.Errorf("expected AI_MODEL source, got %s", summary.Source)
	}
}

func TestNewCaseSummary_Defaults(t *testing.T) {
	params := CaseSummaryParams{
		OrganizationID: uuid.New(),
		CaseID:         uuid.New(),
		CreatedAt:      time.Now().UTC(),
	}

	summary, err := NewCaseSummary(params)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if summary.Type != "" {
		t.Errorf("expected empty type (unspecified), got %s", summary.Type)
	}
	if summary.SchemaVersion != "1.0" {
		t.Errorf("expected schema version 1.0, got %s", summary.SchemaVersion)
	}
}

func TestNewCaseSummary_MissingOrgID(t *testing.T) {
	params := CaseSummaryParams{
		CaseID: uuid.New(),
	}

	_, err := NewCaseSummary(params)
	if err == nil {
		t.Fatal("expected error for missing organization ID")
	}
}

func TestNewCaseSummary_MissingCaseID(t *testing.T) {
	params := CaseSummaryParams{
		OrganizationID: uuid.New(),
	}

	_, err := NewCaseSummary(params)
	if err == nil {
		t.Fatal("expected error for missing case ID")
	}
}

func TestNewCaseSummary_InvalidType(t *testing.T) {
	params := CaseSummaryParams{
		OrganizationID: uuid.New(),
		CaseID:         uuid.New(),
		Type:           SummaryType("INVALID"),
	}

	_, err := NewCaseSummary(params)
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestNewCaseSummary_ConfidenceOutOfRange(t *testing.T) {
	lowConf := -0.1
	highConf := 1.5

	params := CaseSummaryParams{
		OrganizationID: uuid.New(),
		CaseID:         uuid.New(),
		Confidence:     &lowConf,
	}
	_, err := NewCaseSummary(params)
	if err == nil {
		t.Fatal("expected error for negative confidence")
	}

	params.Confidence = &highConf
	_, err = NewCaseSummary(params)
	if err == nil {
		t.Fatal("expected error for confidence > 1")
	}
}

func TestNewCaseSummary_ContentTooLarge(t *testing.T) {
	params := CaseSummaryParams{
		OrganizationID: uuid.New(),
		CaseID:         uuid.New(),
	}

	largeText := make([]byte, MaxSummaryContentSize+1)
	for i := range largeText {
		largeText[i] = 'a'
	}

	params.Content = SummaryContent{
		Situation: []SummaryClaim{
			{Text: string(largeText)},
		},
	}

	_, err := NewCaseSummary(params)
	if err == nil {
		t.Fatal("expected error for content exceeding max size")
	}
}

func TestCaseSummary_Accept(t *testing.T) {
	params := CaseSummaryParams{
		OrganizationID: uuid.New(),
		CaseID:         uuid.New(),
	}

	summary, err := NewCaseSummary(params)
	if err != nil {
		t.Fatalf("failed to create summary: %v", err)
	}

	reviewerID := uuid.New()
	notes := "Confirmed accurate"
	if err := summary.Accept(reviewerID, notes); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Status != SummaryStatusAccepted {
		t.Errorf("expected accepted status, got %s", summary.Status)
	}
	if summary.ReviewNotes != notes {
		t.Errorf("review notes not set correctly")
	}
	if summary.ReviewedBy == nil || *summary.ReviewedBy != reviewerID {
		t.Errorf("reviewed by not set correctly")
	}
}

func TestCaseSummary_Accept_MissingReviewer(t *testing.T) {
	params := CaseSummaryParams{
		OrganizationID: uuid.New(),
		CaseID:         uuid.New(),
	}

	summary, err := NewCaseSummary(params)
	if err != nil {
		t.Fatalf("failed to create summary: %v", err)
	}

	if err := summary.Accept(uuid.Nil, "notes"); err == nil {
		t.Fatal("expected error for missing reviewer")
	}
}

func TestCaseSummary_Reject(t *testing.T) {
	params := CaseSummaryParams{
		OrganizationID: uuid.New(),
		CaseID:         uuid.New(),
	}

	summary, err := NewCaseSummary(params)
	if err != nil {
		t.Fatalf("failed to create summary: %v", err)
	}

	reviewerID := uuid.New()
	notes := "Found inaccuracies"
	if err := summary.Reject(reviewerID, notes); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Status != SummaryStatusRejected {
		t.Errorf("expected rejected status, got %s", summary.Status)
	}
}

func TestCaseSummary_Reject_MissingReviewer(t *testing.T) {
	params := CaseSummaryParams{
		OrganizationID: uuid.New(),
		CaseID:         uuid.New(),
	}

	summary, err := NewCaseSummary(params)
	if err != nil {
		t.Fatalf("failed to create summary: %v", err)
	}

	if err := summary.Reject(uuid.Nil, "notes"); err == nil {
		t.Fatal("expected error for missing reviewer")
	}
}

func TestIsValidSummaryType(t *testing.T) {
	validTypes := []SummaryType{
		SummaryTypeFull,
		SummaryTypeSituation,
		SummaryTypeRelevantInformation,
		SummaryTypeImportantEvidence,
		SummaryTypeMissingInformation,
		SummaryTypePotentialInconsistencies,
		SummaryTypeRuleEvaluationResults,
		SummaryTypeWorkflowHistory,
		SummaryTypePreviousActions,
		SummaryTypeAIObservations,
	}

	for _, typ := range validTypes {
		if !isValidSummaryType(typ) {
			t.Errorf("expected %s to be valid", typ)
		}
	}

	if isValidSummaryType("INVALID") {
		t.Error("expected INVALID to be invalid")
	}
}

func TestSummaryClaim_Provenance(t *testing.T) {
	validProvenances := []ClaimProvenance{
		ClaimProvenanceFact,
		ClaimProvenanceSource,
		ClaimProvenanceAIObservation,
		ClaimProvenanceRuleResult,
		ClaimProvenanceHumanDecision,
		ClaimProvenanceWorkflowHistory,
	}

	for _, prov := range validProvenances {
		claim := SummaryClaim{
			Text:       "Test claim",
			Provenance: prov,
		}
		if claim.Provenance != prov {
			t.Errorf("provenance not set correctly")
		}
	}
}
