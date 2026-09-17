package application

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/casecontext/application"
	casecontextdomain "github.com/alrazihi/civora/internal/casecontext/domain"
	"github.com/alrazihi/civora/internal/casesummary/domain"
	"github.com/alrazihi/civora/internal/database"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/google/uuid"
)

var (
	ErrSummaryNotFound       = errors.New("case summary not found")
	ErrCaseNotFound          = errors.New("case not found")
	ErrAIProviderUnavailable = errors.New("AI provider unavailable")
	ErrInvalidInput          = errors.New("invalid input")
)

type CaseSummaryProvider interface {
	GenerateCaseSummary(ctx context.Context, req CaseSummaryRequest) (*CaseSummaryResult, error)
	ProviderInfo() domain.ModelInfo
}

type CaseSummaryRequest struct {
	OrganizationID uuid.UUID
	CaseID         uuid.UUID
	Context        *casecontextdomain.CaseContext
	Options        CaseSummaryOptions
}

type CaseSummaryOptions struct {
	Types           []domain.SummaryType
	MaxTokens       int
	IncludeSections []string
}

type CaseSummaryResult struct {
	Content       domain.SummaryContent
	Confidence    *float64
	InputHash     string
	OutputHash    string
	Model         domain.ModelInfo
	TokenEstimate int
}

type CaseSummaryService struct {
	repo           domain.CaseSummaryRepository
	caseRepo       shared.CaseFinder
	contextService BuildContextService
	userChecker    shared.UserChecker
	auditor        auditdomain.EventRecorder
	provider       CaseSummaryProvider
	db             *sql.DB
}

type BuildContextService interface {
	BuildContext(ctx context.Context, params application.BuildContextParams) (*application.BuildContextResult, error)
}

func NewCaseSummaryService(
	repo domain.CaseSummaryRepository,
	caseRepo shared.CaseFinder,
	contextService BuildContextService,
	userChecker shared.UserChecker,
	auditor auditdomain.EventRecorder,
) *CaseSummaryService {
	return &CaseSummaryService{
		repo:           repo,
		caseRepo:       caseRepo,
		contextService: contextService,
		userChecker:    userChecker,
		auditor:        auditor,
	}
}

func (s *CaseSummaryService) WithProvider(provider CaseSummaryProvider) *CaseSummaryService {
	s.provider = provider
	return s
}

func (s *CaseSummaryService) WithTransaction(db *sql.DB) *CaseSummaryService {
	s.db = db
	return s
}

type GenerateCaseSummaryParams struct {
	OrganizationID  uuid.UUID
	CaseID          uuid.UUID
	ActorID         uuid.UUID
	Types           []domain.SummaryType
	MaxTokens       int
	IncludeSections []string
}

type GenerateCaseSummaryResult struct {
	Summary *domain.CaseSummary `json:"summary"`
}

func (s *CaseSummaryService) GenerateCaseSummary(ctx context.Context, params GenerateCaseSummaryParams) (*GenerateCaseSummaryResult, error) {
	if params.ActorID == uuid.Nil {
		return nil, fmt.Errorf("%w: actor ID is required", ErrInvalidInput)
	}

	valid, err := s.userChecker.BelongsToOrganization(ctx, params.OrganizationID, params.ActorID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate actor: %w", err)
	}
	if !valid {
		return nil, fmt.Errorf("%w: user does not belong to organization", ErrInvalidInput)
	}

	_, err = s.caseRepo.FindByID(ctx, params.OrganizationID, params.CaseID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCaseNotFound, err)
	}

	options := casecontextdomain.ContextOptions{
		IncludeCaseMetadata:     true,
		IncludePerson:           true,
		IncludeEvidence:         true,
		IncludeVerifiedEvidence: true,
		IncludeDocuments:        false,
		IncludeFormSubmissions:  true,
		IncludeRuleEvaluations:  true,
		IncludeWorkflowHistory:  true,
		IncludeDecisions:        true,
		IncludeAIObservations:   true,
		MaxTokens:               8000,
		MaxFacts:                200,
		OnlyHumanVerified:       false,
	}
	if params.MaxTokens > 0 {
		options.MaxTokens = params.MaxTokens
	}

	ctxResult, err := s.contextService.BuildContext(ctx, application.BuildContextParams{
		OrganizationID: params.OrganizationID,
		CaseID:         params.CaseID,
		ActorID:        params.ActorID,
		Options:        options,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build case context: %w", err)
	}

	factKeys := make(map[string]bool)
	for _, claim := range params.IncludeSections {
		factKeys[claim] = true
	}

	req := CaseSummaryRequest{
		OrganizationID: params.OrganizationID,
		CaseID:         params.CaseID,
		Context:        ctxResult.Context,
		Options: CaseSummaryOptions{
			Types:           params.Types,
			MaxTokens:       params.MaxTokens,
			IncludeSections: params.IncludeSections,
		},
	}

	providerInfo := s.provider.ProviderInfo()
	result, err := s.provider.GenerateCaseSummary(ctx, req)
	if err != nil {
		s.recordAudit(ctx, params.OrganizationID, &params.ActorID, "case_summary.failed", "case", shared.StrPtr(params.CaseID.String()), "failure", map[string]interface{}{
			"reason": err.Error(),
			"model":  providerInfo.Name,
		})
		return nil, fmt.Errorf("%w: %v", ErrAIProviderUnavailable, err)
	}

	inputHash := computeInputHash(ctxResult.Context)
	outputHash := computeSHA256(result.Content)

	summary, err := domain.NewCaseSummary(domain.CaseSummaryParams{
		OrganizationID: params.OrganizationID,
		CaseID:         params.CaseID,
		Type:           domain.SummaryTypeFull,
		Source:         domain.AnalysisSourceAIModel,
		Status:         domain.SummaryStatusPendingReview,
		Model:          &result.Model,
		Content:        result.Content,
		Confidence:     result.Confidence,
		InputHash:      inputHash,
		OutputHash:     outputHash,
		SchemaVersion:  "1.0",
		TokenEstimate:  result.TokenEstimate,
		CreatedBy:      nil,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	err = s.withTx(ctx, func(tx *sql.Tx) error {
		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: params.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "case_summary.generated",
				Resource:       "case",
				ResourceID:     shared.StrPtr(params.CaseID.String()),
				Outcome:        "success",
				Metadata: map[string]interface{}{
					"count": len(result.Content.Situation) + len(result.Content.RelevantInformation) +
						len(result.Content.ImportantEvidence) + len(result.Content.MissingInformation) +
						len(result.Content.PotentialInconsistencies) + len(result.Content.RuleEvaluationResults) +
						len(result.Content.WorkflowHistory) + len(result.Content.PreviousActions) +
						len(result.Content.AIObservations),
					"model":           result.Model.Name,
					"model_version":   result.Model.Version,
					"provider":        result.Model.Provider,
					"types_requested": params.Types,
					"token_estimate":  result.TokenEstimate,
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		if err := s.repo.SaveTx(ctx, tx, summary); err != nil {
			return fmt.Errorf("failed to save summary: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &GenerateCaseSummaryResult{Summary: summary}, nil
}

type ListCaseSummariesParams struct {
	OrganizationID uuid.UUID
	CaseID         uuid.UUID
	ActorID        uuid.UUID
	Limit          int
	Offset         int
}

func (s *CaseSummaryService) ListCaseSummaries(ctx context.Context, params ListCaseSummariesParams) ([]*domain.CaseSummary, int, error) {
	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.Limit > 200 {
		params.Limit = 200
	}
	if params.Offset < 0 {
		params.Offset = 0
	}

	_, err := s.caseRepo.FindByID(ctx, params.OrganizationID, params.CaseID)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrCaseNotFound, err)
	}

	items, total, err := s.repo.FindByCase(ctx, params.OrganizationID, params.CaseID, params.Limit, params.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list summaries: %w", err)
	}

	return items, total, nil
}

type ReviewCaseSummaryParams struct {
	OrganizationID uuid.UUID
	SummaryID      uuid.UUID
	ReviewerID     uuid.UUID
	Action         domain.SummaryStatus
	Notes          string
}

func (s *CaseSummaryService) ReviewCaseSummary(ctx context.Context, params ReviewCaseSummaryParams) (*domain.CaseSummary, error) {
	if params.ReviewerID == uuid.Nil {
		return nil, fmt.Errorf("%w: reviewer ID is required", ErrInvalidInput)
	}

	valid, err := s.userChecker.BelongsToOrganization(ctx, params.OrganizationID, params.ReviewerID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate reviewer: %w", err)
	}
	if !valid {
		return nil, fmt.Errorf("%w: user does not belong to organization", ErrInvalidInput)
	}

	summary, err := s.repo.FindByID(ctx, params.OrganizationID, params.SummaryID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSummaryNotFound, err)
	}

	switch params.Action {
	case domain.SummaryStatusAccepted:
		if err := summary.Accept(params.ReviewerID, params.Notes); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
	case domain.SummaryStatusRejected:
		if err := summary.Reject(params.ReviewerID, params.Notes); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
	default:
		return nil, fmt.Errorf("%w: invalid action %q", ErrInvalidInput, params.Action)
	}

	auditAction := "case_summary.rejected"
	if params.Action == domain.SummaryStatusAccepted {
		auditAction = "case_summary.accepted"
	}

	err = s.withTx(ctx, func(tx *sql.Tx) error {
		if err := s.repo.UpdateStatusTx(ctx, tx, params.OrganizationID, params.SummaryID, params.Action, &params.ReviewerID, params.Notes); err != nil {
			return fmt.Errorf("failed to update summary status: %w", err)
		}
		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: params.OrganizationID,
				ActorID:        &params.ReviewerID,
				Action:         auditAction,
				Resource:       "case_summary",
				ResourceID:     shared.StrPtr(params.SummaryID.String()),
				Outcome:        "success",
				Metadata: map[string]interface{}{
					"case_id": summary.CaseID.String(),
					"status":  string(params.Action),
					"notes":   params.Notes,
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return summary, nil
}

func (s *CaseSummaryService) withTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	if s.db == nil {
		return fn(nil)
	}
	return database.InTransaction(ctx, s.db, fn)
}

func (s *CaseSummaryService) recordAudit(ctx context.Context, orgID uuid.UUID, actorID *uuid.UUID, action, resource string, resourceID *string, outcome string, metadata map[string]interface{}) {
	if s.auditor == nil {
		return
	}
	_ = s.auditor.RecordEvent(ctx, auditdomain.RecordEventParams{
		OrganizationID: orgID,
		ActorID:        actorID,
		Action:         action,
		Resource:       resource,
		ResourceID:     resourceID,
		Outcome:        outcome,
		Metadata:       metadata,
	})
}

func computeInputHash(context *casecontextdomain.CaseContext) string {
	data, err := json.Marshal(context)
	if err != nil {
		return "error"
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func computeSHA256(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		data = []byte(fmt.Sprintf("%v", v))
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
