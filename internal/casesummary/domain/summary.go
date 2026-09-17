package domain

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type SummaryType string

const (
	SummaryTypeFull                     SummaryType = "FULL_SUMMARY"
	SummaryTypeSituation                SummaryType = "SITUATION"
	SummaryTypeRelevantInformation      SummaryType = "RELEVANT_INFORMATION"
	SummaryTypeImportantEvidence        SummaryType = "IMPORTANT_EVIDENCE"
	SummaryTypeMissingInformation       SummaryType = "MISSING_INFORMATION"
	SummaryTypePotentialInconsistencies SummaryType = "POTENTIAL_INCONSISTENCIES"
	SummaryTypeRuleEvaluationResults    SummaryType = "RULE_EVALUATION_RESULTS"
	SummaryTypeWorkflowHistory          SummaryType = "WORKFLOW_HISTORY"
	SummaryTypePreviousActions          SummaryType = "PREVIOUS_ACTIONS"
	SummaryTypeAIObservations           SummaryType = "AI_OBSERVATIONS"
)

type SummaryStatus string

const (
	SummaryStatusPendingReview SummaryStatus = "PENDING_REVIEW"
	SummaryStatusAccepted      SummaryStatus = "ACCEPTED"
	SummaryStatusRejected      SummaryStatus = "REJECTED"
)

type AnalysisSource string

const (
	AnalysisSourceAIModel AnalysisSource = "AI_MODEL"
	AnalysisSourceHuman   AnalysisSource = "HUMAN"
)

type ClaimProvenance string

const (
	ClaimProvenanceFact            ClaimProvenance = "FACT"
	ClaimProvenanceSource          ClaimProvenance = "SOURCE"
	ClaimProvenanceAIObservation   ClaimProvenance = "AI_OBSERVATION"
	ClaimProvenanceRuleResult      ClaimProvenance = "RULE_RESULT"
	ClaimProvenanceHumanDecision   ClaimProvenance = "HUMAN_DECISION"
	ClaimProvenanceWorkflowHistory ClaimProvenance = "WORKFLOW_HISTORY"
)

type SourceReferenceType string

const (
	SourceTypeEvidence           SourceReferenceType = "evidence"
	SourceTypeDocument           SourceReferenceType = "document"
	SourceTypeFormSubmission     SourceReferenceType = "form_submission"
	SourceTypeRuleEvaluation     SourceReferenceType = "rule_evaluation"
	SourceTypeDecision           SourceReferenceType = "decision"
	SourceTypeWorkflowTransition SourceReferenceType = "workflow_transition"
	SourceTypeAIObservation      SourceReferenceType = "ai_observation"
	SourceTypeCaseMetadata       SourceReferenceType = "case_metadata"
	SourceTypePerson             SourceReferenceType = "person"
)

type SourceReference struct {
	Type  SourceReferenceType `json:"type"`
	ID    string              `json:"id"`
	Label string              `json:"label,omitempty"`
}

type SummaryClaim struct {
	Text       string            `json:"text"`
	Provenance ClaimProvenance   `json:"provenance"`
	References []SourceReference `json:"references"`
	Confidence *float64          `json:"confidence,omitempty"`
}

type SummaryContent struct {
	Situation                []SummaryClaim `json:"situation"`
	RelevantInformation      []SummaryClaim `json:"relevant_information"`
	ImportantEvidence        []SummaryClaim `json:"important_evidence"`
	MissingInformation       []SummaryClaim `json:"missing_information"`
	PotentialInconsistencies []SummaryClaim `json:"potential_inconsistencies"`
	RuleEvaluationResults    []SummaryClaim `json:"rule_evaluation_results"`
	WorkflowHistory          []SummaryClaim `json:"workflow_history"`
	PreviousActions          []SummaryClaim `json:"previous_actions"`
	AIObservations           []SummaryClaim `json:"ai_observations"`
}

type ModelInfo struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Provider string `json:"provider"`
}

var (
	ErrSummaryNotFound     = errors.New("case summary not found")
	ErrSummaryInvalidInput = errors.New("invalid case summary input")
)

type CaseSummary struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	CaseID         uuid.UUID
	Type           SummaryType
	Source         AnalysisSource
	Status         SummaryStatus
	Model          *ModelInfo
	Content        SummaryContent
	Confidence     *float64
	InputHash      string
	OutputHash     string
	SchemaVersion  string
	TokenEstimate  int
	CreatedAt      time.Time
	CreatedBy      *uuid.UUID
	ReviewedAt     *time.Time
	ReviewedBy     *uuid.UUID
	ReviewNotes    string
}

type CaseSummaryParams struct {
	OrganizationID uuid.UUID
	CaseID         uuid.UUID
	Type           SummaryType
	Source         AnalysisSource
	Status         SummaryStatus
	Model          *ModelInfo
	Content        SummaryContent
	Confidence     *float64
	InputHash      string
	OutputHash     string
	SchemaVersion  string
	TokenEstimate  int
	CreatedAt      time.Time
	CreatedBy      *uuid.UUID
}

const (
	MaxSummaryContentSize = 500_000
	MaxReviewNotesLength  = 5000
)

func NewCaseSummary(params CaseSummaryParams) (*CaseSummary, error) {
	if params.OrganizationID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization ID is required", ErrSummaryInvalidInput)
	}
	if params.CaseID == uuid.Nil {
		return nil, fmt.Errorf("%w: case ID is required", ErrSummaryInvalidInput)
	}
	if params.Type != "" && !isValidSummaryType(params.Type) {
		return nil, fmt.Errorf("%w: invalid summary type %q", ErrSummaryInvalidInput, params.Type)
	}
	if params.Confidence != nil {
		if *params.Confidence < 0 || *params.Confidence > 1 {
			return nil, fmt.Errorf("%w: confidence must be between 0 and 1", ErrSummaryInvalidInput)
		}
	}
	if params.Source == "" {
		params.Source = AnalysisSourceAIModel
	}
	if params.Status == "" {
		params.Status = SummaryStatusPendingReview
	}
	if params.SchemaVersion == "" {
		params.SchemaVersion = "1.0"
	}
	if params.CreatedAt.IsZero() {
		params.CreatedAt = time.Now().UTC()
	}

	contentSize := estimateContentSize(params.Content)
	if contentSize > MaxSummaryContentSize {
		return nil, fmt.Errorf("%w: content exceeds max size of %d bytes", ErrSummaryInvalidInput, MaxSummaryContentSize)
	}

	return &CaseSummary{
		ID:             uuid.New(),
		OrganizationID: params.OrganizationID,
		CaseID:         params.CaseID,
		Type:           params.Type,
		Source:         params.Source,
		Status:         params.Status,
		Model:          params.Model,
		Content:        params.Content,
		Confidence:     params.Confidence,
		InputHash:      params.InputHash,
		OutputHash:     params.OutputHash,
		SchemaVersion:  params.SchemaVersion,
		TokenEstimate:  params.TokenEstimate,
		CreatedAt:      params.CreatedAt,
		CreatedBy:      params.CreatedBy,
	}, nil
}

func estimateContentSize(content SummaryContent) int {
	size := 0
	for _, claim := range content.Situation {
		size += len(claim.Text)
		for _, ref := range claim.References {
			size += len(ref.Type) + len(ref.ID) + len(ref.Label)
		}
	}
	for _, claim := range content.RelevantInformation {
		size += len(claim.Text)
		for _, ref := range claim.References {
			size += len(ref.Type) + len(ref.ID) + len(ref.Label)
		}
	}
	for _, claim := range content.ImportantEvidence {
		size += len(claim.Text)
		for _, ref := range claim.References {
			size += len(ref.Type) + len(ref.ID) + len(ref.Label)
		}
	}
	for _, claim := range content.MissingInformation {
		size += len(claim.Text)
		for _, ref := range claim.References {
			size += len(ref.Type) + len(ref.ID) + len(ref.Label)
		}
	}
	for _, claim := range content.PotentialInconsistencies {
		size += len(claim.Text)
		for _, ref := range claim.References {
			size += len(ref.Type) + len(ref.ID) + len(ref.Label)
		}
	}
	for _, claim := range content.RuleEvaluationResults {
		size += len(claim.Text)
		for _, ref := range claim.References {
			size += len(ref.Type) + len(ref.ID) + len(ref.Label)
		}
	}
	for _, claim := range content.WorkflowHistory {
		size += len(claim.Text)
		for _, ref := range claim.References {
			size += len(ref.Type) + len(ref.ID) + len(ref.Label)
		}
	}
	for _, claim := range content.PreviousActions {
		size += len(claim.Text)
		for _, ref := range claim.References {
			size += len(ref.Type) + len(ref.ID) + len(ref.Label)
		}
	}
	for _, claim := range content.AIObservations {
		size += len(claim.Text)
		for _, ref := range claim.References {
			size += len(ref.Type) + len(ref.ID) + len(ref.Label)
		}
	}
	return size
}

func (s *CaseSummary) Accept(reviewerID uuid.UUID, notes string) error {
	if reviewerID == uuid.Nil {
		return fmt.Errorf("%w: reviewer ID is required", ErrSummaryInvalidInput)
	}
	if len(notes) > MaxReviewNotesLength {
		return fmt.Errorf("%w: review notes exceed maximum length of %d characters", ErrSummaryInvalidInput, MaxReviewNotesLength)
	}
	now := time.Now().UTC()
	s.Status = SummaryStatusAccepted
	s.ReviewedAt = &now
	s.ReviewedBy = &reviewerID
	s.ReviewNotes = notes
	return nil
}

func (s *CaseSummary) Reject(reviewerID uuid.UUID, notes string) error {
	if reviewerID == uuid.Nil {
		return fmt.Errorf("%w: reviewer ID is required", ErrSummaryInvalidInput)
	}
	if len(notes) > MaxReviewNotesLength {
		return fmt.Errorf("%w: review notes exceed maximum length of %d characters", ErrSummaryInvalidInput, MaxReviewNotesLength)
	}
	now := time.Now().UTC()
	s.Status = SummaryStatusRejected
	s.ReviewedAt = &now
	s.ReviewedBy = &reviewerID
	s.ReviewNotes = notes
	return nil
}

func isValidSummaryType(t SummaryType) bool {
	switch t {
	case SummaryTypeFull,
		SummaryTypeSituation,
		SummaryTypeRelevantInformation,
		SummaryTypeImportantEvidence,
		SummaryTypeMissingInformation,
		SummaryTypePotentialInconsistencies,
		SummaryTypeRuleEvaluationResults,
		SummaryTypeWorkflowHistory,
		SummaryTypePreviousActions,
		SummaryTypeAIObservations:
		return true
	default:
		return false
	}
}

type CaseSummaryRepository interface {
	Save(ctx context.Context, s *CaseSummary) error
	SaveTx(ctx context.Context, tx *sql.Tx, s *CaseSummary) error
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*CaseSummary, error)
	FindByCase(ctx context.Context, orgID, caseID uuid.UUID, limit, offset int) ([]*CaseSummary, int, error)
	UpdateStatus(ctx context.Context, orgID, summaryID uuid.UUID, status SummaryStatus, reviewerID *uuid.UUID, notes string) error
	UpdateStatusTx(ctx context.Context, tx *sql.Tx, orgID, summaryID uuid.UUID, status SummaryStatus, reviewerID *uuid.UUID, notes string) error
	CountByCase(ctx context.Context, orgID, caseID uuid.UUID) (int, error)
}
