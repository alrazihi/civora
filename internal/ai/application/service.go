package application

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/alrazihi/civora/internal/ai/domain"
	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/database"
	evidencedomain "github.com/alrazihi/civora/internal/evidence/domain"
	formsubdomain "github.com/alrazihi/civora/internal/form_submission/domain"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/google/uuid"
)

var (
	ErrObservationNotFound   = errors.New("observation not found")
	ErrEvidenceNotFound      = errors.New("evidence not found")
	ErrAIProviderUnavailable = errors.New("AI provider unavailable")
	ErrInvalidInput          = errors.New("invalid input")
)

type EvidenceRepository interface {
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*evidencedomain.Evidence, error)
	FindByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID, limit, offset int) ([]*evidencedomain.Evidence, error)
	ListDocuments(ctx context.Context, orgID, evidenceID uuid.UUID, limit, offset int) ([]*evidencedomain.Document, int, error)
	SaveTx(ctx context.Context, tx *sql.Tx, e *evidencedomain.Evidence) error
}

type FormSubmissionRepository interface {
	ListByCase(ctx context.Context, tenantID, caseID uuid.UUID) ([]*formsubdomain.FormSubmission, error)
}

// DocumentContentProvider retrieves the raw content of a single document.
// It is the infrastructural port the AI service uses to read actual document
// bytes instead of metadata only. Implementations must enforce tenant scoping.
type DocumentContentProvider interface {
	GetDocumentContent(ctx context.Context, orgID, evidenceID, documentID, actorID uuid.UUID) (io.ReadCloser, error)
}

// PIISanitizer optionally redacts personally identifiable information from
// document content before it is sent to a provider.
type PIISanitizer interface {
	SanitizeDocumentContent(content []byte, contentType string, fileName string) ([]byte, error)
}

type AIProvider interface {
	GenerateObservations(ctx context.Context, req ProviderRequest) ([]ObservationResult, error)
	ProviderInfo() domain.ModelInfo
}

type ProviderRequest struct {
	OrganizationID uuid.UUID
	EvidenceID     uuid.UUID
	Documents      []domain.DocumentContent
	CaseFacts      []map[string]any
	Options        domain.ProviderOptions
}

type ObservationResult struct {
	Type       domain.ObservationType
	Content    map[string]any
	Confidence *float64
	InputHash  string
	OutputHash string
	Model      domain.ModelInfo
}

type AIService struct {
	repo             domain.ObservationRepository
	evidenceRepo     EvidenceRepository
	formRepo         FormSubmissionRepository
	userChecker      shared.UserChecker
	auditor          auditdomain.EventRecorder
	provider         AIProvider
	db               *sql.DB
	documents        DocumentContentProvider
	sanitizer        PIISanitizer
	verifiedFactRepo domain.VerifiedFactRepository
}

func NewAIService(repo domain.ObservationRepository, evidenceRepo EvidenceRepository, formRepo FormSubmissionRepository, userChecker shared.UserChecker, auditor auditdomain.EventRecorder, provider AIProvider) *AIService {
	return &AIService{
		repo:         repo,
		evidenceRepo: evidenceRepo,
		formRepo:     formRepo,
		userChecker:  userChecker,
		auditor:      auditor,
		provider:     provider,
	}
}

// WithTransaction sets the database connection used to make audit and domain
// writes atomic. Must be called in production wiring; tests may omit it.
func (s *AIService) WithTransaction(db *sql.DB) *AIService {
	s.db = db
	return s
}

// WithDocumentContent wires the port used to retrieve actual document bytes.
func (s *AIService) WithDocumentContent(documents DocumentContentProvider) *AIService {
	s.documents = documents
	return s
}

// WithPIISanitizer wires mandatory PII redaction applied to documents before
// they are sent to a provider. A sanitizer must always be configured.
func (s *AIService) WithPIISanitizer(sanitizer PIISanitizer) *AIService {
	if sanitizer == nil {
		panic("PII sanitizer is required and must not be nil")
	}
	s.sanitizer = sanitizer
	return s
}

// WithVerifiedFactRepo wires the repository used to persist verified facts
// produced when AI observations are accepted or corrected.
func (s *AIService) WithVerifiedFactRepo(repo domain.VerifiedFactRepository) *AIService {
	s.verifiedFactRepo = repo
	return s
}

// withTx runs fn atomically in a database transaction when a connection is
// configured. When db is nil (e.g. unit tests) fn runs inline so the
// non-transactional persistence path remains testable.
func (s *AIService) withTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	if s.db == nil {
		return fn(nil)
	}
	return database.InTransaction(ctx, s.db, fn)
}

type GenerateObservationsParams struct {
	OrganizationID uuid.UUID
	EvidenceID     uuid.UUID
	ActorID        uuid.UUID
	Types          []domain.ObservationType
	MaxTokens      int
}

type ObservationDTO struct {
	ID               uuid.UUID                `json:"id"`
	OrganizationID   uuid.UUID                `json:"organization_id"`
	CaseID           *uuid.UUID               `json:"case_id,omitempty"`
	EvidenceID       *uuid.UUID               `json:"evidence_id,omitempty"`
	Type             domain.ObservationType   `json:"type"`
	Source           domain.ObservationSource `json:"source"`
	Status           domain.ObservationStatus `json:"status"`
	Model            *domain.ModelInfo        `json:"model,omitempty"`
	Content          map[string]any           `json:"content"`
	Confidence       *float64                 `json:"confidence,omitempty"`
	Statement        string                   `json:"statement,omitempty"`
	SourceReferences []domain.SourceReference `json:"source_references,omitempty"`
	InputHash        string                   `json:"input_hash"`
	OutputHash       string                   `json:"output_hash"`
	CreatedAt        time.Time                `json:"created_at"`
	CreatedBy        *uuid.UUID               `json:"created_by,omitempty"`
	ReviewedAt       *time.Time               `json:"reviewed_at,omitempty"`
	ReviewedBy       *uuid.UUID               `json:"reviewed_by,omitempty"`
	ReviewNotes      string                   `json:"review_notes,omitempty"`
}

type GenerateObservationsResult struct {
	Observations []ObservationDTO `json:"observations"`
}

func (s *AIService) GenerateObservations(ctx context.Context, params GenerateObservationsParams) (*GenerateObservationsResult, error) {
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

	e, err := s.evidenceRepo.FindByID(ctx, params.OrganizationID, params.EvidenceID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEvidenceNotFound, err)
	}
	_ = e

	var docs []*evidencedomain.Document
	docs, _, err = s.evidenceRepo.ListDocuments(ctx, params.OrganizationID, params.EvidenceID, 50, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve documents: %w", err)
	}

	var docContents []domain.DocumentContent
	for _, doc := range docs {
		dc := domain.DocumentContent{
			DocumentID:  doc.ID,
			FileName:    doc.FileName,
			ContentType: doc.ContentType,
			Checksum:    doc.Checksum,
		}
		if s.documents != nil {
			content, err := s.loadDocumentContent(ctx, params.OrganizationID, params.EvidenceID, params.ActorID, doc)
			if err != nil {
				return nil, fmt.Errorf("failed to retrieve document %s: %w", doc.ID, err)
			}
			dc.Content = content
			if err := validateDocumentChecksum(dc); err != nil {
				return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
			}
		}
		docContents = append(docContents, dc)
	}

	providerOpts := domain.ProviderOptions{
		Types:     params.Types,
		MaxTokens: params.MaxTokens,
	}
	if providerOpts.MaxTokens <= 0 {
		providerOpts.MaxTokens = 4096
	}

	req := ProviderRequest{
		OrganizationID: params.OrganizationID,
		EvidenceID:     params.EvidenceID,
		Documents:      docContents,
		Options:        providerOpts,
	}

	results, err := s.provider.GenerateObservations(ctx, req)
	if err != nil {
		s.recordAudit(ctx, params.OrganizationID, &params.ActorID, "ai.observation.failed", "evidence", shared.StrPtr(params.EvidenceID.String()), "failure", map[string]interface{}{
			"reason": err.Error(),
			"model":  s.provider.ProviderInfo().Name,
		})
		return nil, fmt.Errorf("%w: %v", ErrAIProviderUnavailable, err)
	}

	providerInfo := s.provider.ProviderInfo()

	result := &GenerateObservationsResult{
		Observations: make([]ObservationDTO, 0, len(results)),
	}

	if err := s.withTx(ctx, func(tx *sql.Tx) error {
		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: params.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "ai.observations_generated",
				Resource:       "evidence",
				ResourceID:     shared.StrPtr(params.EvidenceID.String()),
				Outcome:        "success",
				Metadata: map[string]interface{}{
					"count":           len(results),
					"model":           providerInfo.Name,
					"model_version":   providerInfo.Version,
					"provider":        providerInfo.Provider,
					"types_requested": params.Types,
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		for _, res := range results {
			inputHash := res.InputHash
			if inputHash == "" {
				inputHash = computeInputHash(docContents, params.Types, providerOpts.MaxTokens)
			}
			outputHash := res.OutputHash
			if outputHash == "" {
				outputHash = computeSHA256(res.Content)
			}

			obs, err := domain.NewObservation(domain.ObservationParams{
				OrganizationID: params.OrganizationID,
				EvidenceID:     &params.EvidenceID,
				Type:           res.Type,
				Source:         domain.ObservationSourceAIModel,
				Status:         domain.ObservationStatusOpen,
				Model:          &res.Model,
				Content:        res.Content,
				Confidence:     res.Confidence,
				InputHash:      inputHash,
				OutputHash:     outputHash,
				CreatedBy:      nil,
			})
			if err != nil {
				return fmt.Errorf("%w: %v", ErrInvalidInput, err)
			}

			if s.auditor != nil {
				if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
					OrganizationID: params.OrganizationID,
					ActorID:        &params.ActorID,
					Action:         "ai.observation.created",
					Resource:       "ai_observation",
					ResourceID:     shared.StrPtr(obs.ID.String()),
					Outcome:        "success",
					Metadata: map[string]interface{}{
						"evidence_id":      obs.EvidenceID.String(),
						"observation_type": string(obs.Type),
						"model":            obs.Model.Name,
						"confidence":       obs.Confidence,
						"input_hash":       obs.InputHash,
						"output_hash":      obs.OutputHash,
					},
				}); err != nil {
					return fmt.Errorf("failed to record audit event: %w", err)
				}
			}

			if err := s.repo.SaveTx(ctx, tx, obs); err != nil {
				return fmt.Errorf("failed to save observation: %w", err)
			}

			result.Observations = append(result.Observations, serializeObservation(obs))
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

type GenerateCaseObservationsParams struct {
	OrganizationID uuid.UUID
	CaseID         uuid.UUID
	ActorID        uuid.UUID
	Types          []domain.ObservationType
	MaxTokens      int
}

type GenerateCaseObservationsResult struct {
	Observations []ObservationDTO `json:"observations"`
}

func (s *AIService) GenerateCaseObservations(ctx context.Context, params GenerateCaseObservationsParams) (*GenerateCaseObservationsResult, error) {
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

	evidenceList, err := s.evidenceRepo.FindByServiceRequest(ctx, params.OrganizationID, params.CaseID, 100, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve evidence: %w", err)
	}

	formSubmissions, err := s.formRepo.ListByCase(ctx, params.OrganizationID, params.CaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve form submissions: %w", err)
	}

	caseFacts := s.buildCaseFacts(evidenceList, formSubmissions)

	providerOpts := domain.ProviderOptions{
		Types:     params.Types,
		MaxTokens: params.MaxTokens,
	}
	if providerOpts.MaxTokens <= 0 {
		providerOpts.MaxTokens = 4096
	}

	req := ProviderRequest{
		OrganizationID: params.OrganizationID,
		EvidenceID:     uuid.Nil,
		Documents:      nil,
		CaseFacts:      caseFacts,
		Options:        providerOpts,
	}

	results, err := s.provider.GenerateObservations(ctx, req)
	if err != nil {
		s.recordAudit(ctx, params.OrganizationID, &params.ActorID, "ai.observation.failed", "case", shared.StrPtr(params.CaseID.String()), "failure", map[string]interface{}{
			"reason": err.Error(),
			"model":  s.provider.ProviderInfo().Name,
		})
		return nil, fmt.Errorf("%w: %v", ErrAIProviderUnavailable, err)
	}

	providerInfo := s.provider.ProviderInfo()

	result := &GenerateCaseObservationsResult{
		Observations: make([]ObservationDTO, 0, len(results)),
	}

	if err := s.withTx(ctx, func(tx *sql.Tx) error {
		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: params.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "ai.observations_generated",
				Resource:       "case",
				ResourceID:     shared.StrPtr(params.CaseID.String()),
				Outcome:        "success",
				Metadata: map[string]interface{}{
					"count":           len(results),
					"model":           providerInfo.Name,
					"model_version":   providerInfo.Version,
					"provider":        providerInfo.Provider,
					"types_requested": params.Types,
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		for _, res := range results {
			inputHash := res.InputHash
			if inputHash == "" {
				inputHash = computeCaseInputHash(caseFacts, params.Types, providerOpts.MaxTokens)
			}
			outputHash := res.OutputHash
			if outputHash == "" {
				outputHash = computeSHA256(res.Content)
			}

			sourceRefs := extractSourceReferences(res.Content)
			statement := extractStatement(res.Content)

			obs, err := domain.NewObservation(domain.ObservationParams{
				OrganizationID:   params.OrganizationID,
				CaseID:           &params.CaseID,
				Type:             res.Type,
				Source:           domain.ObservationSourceAIModel,
				Status:           domain.ObservationStatusOpen,
				Model:            &res.Model,
				Content:          res.Content,
				Confidence:       res.Confidence,
				Statement:        statement,
				SourceReferences: sourceRefs,
				InputHash:        inputHash,
				OutputHash:       outputHash,
				CreatedBy:        nil,
			})
			if err != nil {
				return fmt.Errorf("%w: %v", ErrInvalidInput, err)
			}

			if s.auditor != nil {
				if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
					OrganizationID: params.OrganizationID,
					ActorID:        &params.ActorID,
					Action:         "ai.observation.created",
					Resource:       "ai_observation",
					ResourceID:     shared.StrPtr(obs.ID.String()),
					Outcome:        "success",
					Metadata: map[string]interface{}{
						"case_id":          obs.CaseID.String(),
						"observation_type": string(obs.Type),
						"model":            obs.Model.Name,
						"confidence":       obs.Confidence,
						"input_hash":       obs.InputHash,
						"output_hash":      obs.OutputHash,
					},
				}); err != nil {
					return fmt.Errorf("failed to record audit event: %w", err)
				}
			}

			if err := s.repo.SaveTx(ctx, tx, obs); err != nil {
				return fmt.Errorf("failed to save observation: %w", err)
			}

			result.Observations = append(result.Observations, serializeObservation(obs))
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func (s *AIService) buildCaseFacts(evidenceList []*evidencedomain.Evidence, formSubmissions []*formsubdomain.FormSubmission) []map[string]any {
	var facts []map[string]any

	for _, e := range evidenceList {
		fact := map[string]any{
			"type":                "evidence",
			"id":                  e.ID.String(),
			"evidence_type":       e.Type,
			"description":         e.Description,
			"verification_status": e.VerificationStatus,
			"source":              e.Source,
			"metadata":            e.Metadata,
			"created_at":          e.CreatedAt,
		}
		facts = append(facts, fact)
	}

	for _, sub := range formSubmissions {
		fact := map[string]any{
			"type":         "form_submission",
			"id":           sub.ID.String(),
			"form_id":      sub.FormID.String(),
			"form_version": sub.FormVersionID.String(),
			"status":       sub.Status,
			"data":         sub.Data,
			"submitted_at": sub.SubmittedAt,
		}
		facts = append(facts, fact)
	}

	return facts
}

func extractSourceReferences(content map[string]any) []domain.SourceReference {
	raw, ok := content["source_references"]
	if !ok {
		return nil
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	var refs []domain.SourceReference
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		refType, _ := m["type"].(string)
		refID, _ := m["id"].(string)
		refs = append(refs, domain.SourceReference{Type: refType, ID: refID})
	}
	return refs
}

func extractStatement(content map[string]any) string {
	stmt, _ := content["statement"].(string)
	return stmt
}

func (s *AIService) recordAudit(ctx context.Context, orgID uuid.UUID, actorID *uuid.UUID, action, resource string, resourceID *string, outcome string, metadata map[string]interface{}) {
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

type ListObservationsParams struct {
	OrganizationID uuid.UUID
	EvidenceID     uuid.UUID
	ActorID        uuid.UUID
	Limit          int
	Offset         int
}

func (s *AIService) ListObservations(ctx context.Context, params ListObservationsParams) ([]*domain.Observation, int, error) {
	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.Limit > 200 {
		params.Limit = 200
	}
	if params.Offset < 0 {
		params.Offset = 0
	}

	_, err := s.evidenceRepo.FindByID(ctx, params.OrganizationID, params.EvidenceID)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrEvidenceNotFound, err)
	}

	items, total, err := s.repo.FindByEvidence(ctx, params.OrganizationID, params.EvidenceID, params.Limit, params.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list observations: %w", err)
	}

	return items, total, nil
}

type ListCaseObservationsParams struct {
	OrganizationID uuid.UUID
	CaseID         uuid.UUID
	ActorID        uuid.UUID
	Limit          int
	Offset         int
}

func (s *AIService) ListCaseObservations(ctx context.Context, params ListCaseObservationsParams) ([]*domain.Observation, int, error) {
	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.Limit > 200 {
		params.Limit = 200
	}
	if params.Offset < 0 {
		params.Offset = 0
	}

	evidenceList, err := s.evidenceRepo.FindByServiceRequest(ctx, params.OrganizationID, params.CaseID, 1, 0)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: case not found or has no evidence", ErrEvidenceNotFound)
	}
	if len(evidenceList) == 0 {
		return nil, 0, fmt.Errorf("%w: case has no evidence", ErrEvidenceNotFound)
	}

	items, total, err := s.repo.FindByCase(ctx, params.OrganizationID, params.CaseID, params.Limit, params.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list observations: %w", err)
	}

	return items, total, nil
}

type ReviewObservationParams struct {
	OrganizationID uuid.UUID
	ObservationID  uuid.UUID
	ReviewerID     uuid.UUID
	Action         domain.ObservationStatus
	Notes          string
}

type CreateVerifiedFactParams struct {
	OrganizationID uuid.UUID
	ObservationID  uuid.UUID
	ReviewerID     uuid.UUID
	ReviewAction   domain.ObservationStatus
	ReviewNotes    string
}

func (s *AIService) ReviewObservation(ctx context.Context, params ReviewObservationParams) (*domain.Observation, error) {
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

	obs, err := s.repo.FindByID(ctx, params.OrganizationID, params.ObservationID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrObservationNotFound, err)
	}

	switch params.Action {
	case domain.ObservationStatusAccepted:
		if err := obs.Accept(params.ReviewerID, params.Notes); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
	case domain.ObservationStatusRejected:
		if err := obs.Reject(params.ReviewerID, params.Notes); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
	case domain.ObservationStatusCorrected:
		if err := obs.Correct(params.ReviewerID, params.Notes); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
	case domain.ObservationStatusDismissed:
		if err := obs.Dismiss(params.ReviewerID, params.Notes); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
	default:
		return nil, fmt.Errorf("%w: invalid action %q", ErrInvalidInput, params.Action)
	}

	auditAction := "ai.observation.rejected"
	if params.Action == domain.ObservationStatusAccepted {
		auditAction = "ai.observation.accepted"
	} else if params.Action == domain.ObservationStatusCorrected {
		auditAction = "ai.observation.corrected"
	} else if params.Action == domain.ObservationStatusDismissed {
		auditAction = "ai.observation.dismissed"
	}

	if err := s.withTx(ctx, func(tx *sql.Tx) error {
		if err := s.repo.UpdateStatusTx(ctx, tx, params.OrganizationID, params.ObservationID, params.Action, &params.ReviewerID, params.Notes); err != nil {
			return fmt.Errorf("failed to update observation status: %w", err)
		}
		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: params.OrganizationID,
				ActorID:        &params.ReviewerID,
				Action:         auditAction,
				Resource:       "ai_observation",
				ResourceID:     shared.StrPtr(params.ObservationID.String()),
				Outcome:        "success",
				Metadata: map[string]interface{}{
					"evidence_id": obs.EvidenceID.String(),
					"status":      string(params.Action),
					"notes":       params.Notes,
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return obs, nil
}

func (s *AIService) CreateVerifiedFact(ctx context.Context, params CreateVerifiedFactParams) (*domain.VerifiedFact, error) {
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

	obs, err := s.repo.FindByID(ctx, params.OrganizationID, params.ObservationID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrObservationNotFound, err)
	}

	var caseID *uuid.UUID
	if obs.CaseID != nil {
		caseID = obs.CaseID
	} else if obs.EvidenceID != nil {
		sr := &evidencedomain.Evidence{}
		_ = sr
	}

	value := make(map[string]any)
	for k, v := range obs.Content {
		value[k] = v
	}
	if obs.Statement != "" {
		value["statement"] = obs.Statement
	}

	originalValue := make(map[string]any)
	for k, v := range obs.Content {
		originalValue[k] = v
	}
	if obs.Statement != "" {
		originalValue["statement"] = obs.Statement
	}

	var correctedValue map[string]any
	if params.ReviewAction == domain.ObservationStatusCorrected {
		correctedValue = make(map[string]any)
		for k, v := range originalValue {
			correctedValue[k] = v
		}
		if len(params.ReviewNotes) > 0 {
			correctedValue["human_correction"] = params.ReviewNotes
		}
	}

	model := &domain.ModelInfo{}
	if obs.Model != nil {
		model = obs.Model
	}

	inputHash := obs.InputHash
	if inputHash == "" {
		inputHash = computePlaceholderHash(originalValue)
	}
	outputHash := obs.OutputHash
	if outputHash == "" {
		outputHash = computePlaceholderHash(originalValue)
	}

	source := "AI_OBSERVATION"
	if params.ReviewAction == domain.ObservationStatusCorrected {
		source = "HUMAN_CORRECTION"
	} else if params.ReviewAction == domain.ObservationStatusAccepted {
		source = "HUMAN_ACCEPTED"
	}

	fact, err := domain.NewVerifiedFact(domain.VerifiedFactParams{
		OrganizationID: params.OrganizationID,
		CaseID:         caseID,
		ObservationID:  params.ObservationID,
		Type:           obs.Type,
		Value:          value,
		OriginalValue:  originalValue,
		CorrectedValue: correctedValue,
		ReviewAction:   params.ReviewAction,
		ReviewerID:     params.ReviewerID,
		ReviewNotes:    params.ReviewNotes,
		Source:         source,
		Model:          model,
		InputHash:      inputHash,
		OutputHash:     outputHash,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	if s.verifiedFactRepo == nil {
		return nil, fmt.Errorf("%w: verified fact repository not configured", ErrInvalidInput)
	}

	if err := s.withTx(ctx, func(tx *sql.Tx) error {
		if s.auditor != nil {
			auditAction := "ai.verified_fact.created"
			auditOutcome := "success"
			auditResource := "ai_verified_fact"
			auditResourceID := shared.StrPtr(fact.ID.String())

			if s.auditor != nil {
				if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
					OrganizationID: params.OrganizationID,
					ActorID:        &params.ReviewerID,
					Action:         auditAction,
					Resource:       auditResource,
					ResourceID:     auditResourceID,
					Outcome:        auditOutcome,
					Metadata: map[string]interface{}{
						"observation_id":   fact.ObservationID.String(),
						"observation_type": string(fact.Type),
						"review_action":    string(fact.ReviewAction),
						"source":           fact.Source,
						"model":            model.Name,
						"reviewer_id":      fact.ReviewerID.String(),
					},
				}); err != nil {
					return fmt.Errorf("failed to record verified fact audit event: %w", err)
				}
			}
		}

		if err := s.verifiedFactRepo.SaveTx(ctx, tx, fact); err != nil {
			return fmt.Errorf("failed to save verified fact: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return fact, nil
}

func serializeObservation(o *domain.Observation) ObservationDTO {
	return ObservationDTO{
		ID:               o.ID,
		OrganizationID:   o.OrganizationID,
		CaseID:           o.CaseID,
		EvidenceID:       o.EvidenceID,
		Type:             o.Type,
		Source:           o.Source,
		Status:           o.Status,
		Model:            o.Model,
		Content:          o.Content,
		Confidence:       o.Confidence,
		Statement:        o.Statement,
		SourceReferences: o.SourceReferences,
		InputHash:        o.InputHash,
		OutputHash:       o.OutputHash,
		CreatedAt:        o.CreatedAt,
		CreatedBy:        o.CreatedBy,
		ReviewedAt:       o.ReviewedAt,
		ReviewedBy:       o.ReviewedBy,
		ReviewNotes:      o.ReviewNotes,
	}
}

func computePlaceholderHash(data any) string {
	h := hex.EncodeToString([]byte(fmt.Sprintf("%v", data)))
	if len(h) > 16 {
		return h[:16]
	}
	return h
}

// loadDocumentContent retrieves the actual document bytes through the
// DocumentContentProvider port, enforcing the size cap and optional PII
// sanitization before returning them to a provider.
func (s *AIService) loadDocumentContent(ctx context.Context, orgID, evidenceID, actorID uuid.UUID, doc *evidencedomain.Document) ([]byte, error) {
	rc, err := s.documents.GetDocumentContent(ctx, orgID, evidenceID, doc.ID, actorID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve document content: %w", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(io.LimitReader(rc, int64(domain.MaxDocumentBytes)+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read document content: %w", err)
	}
	if len(data) > domain.MaxDocumentBytes {
		data = data[:domain.MaxDocumentBytes]
	}

	sanitized, err := s.sanitizer.SanitizeDocumentContent(data, doc.ContentType, doc.FileName)
	if err != nil {
		return nil, fmt.Errorf("failed to sanitize document content: %w", err)
	}
	data = sanitized

	return data, nil
}

// validateDocumentChecksum confirms the retrieved content matches the checksum
// recorded on the document, guarding against tampering in the retrieval path.
func validateDocumentChecksum(dc domain.DocumentContent) error {
	if dc.Checksum == "" {
		return nil
	}
	sum := sha256.Sum256(dc.Content)
	if hex.EncodeToString(sum[:]) != dc.Checksum {
		return ErrInvalidInput
	}
	return nil
}

// computeInputHash produces a SHA-256 over the actual document content
// (not just metadata) plus the requested types and token budget, so
// observations are reproducible from their inputs.
func computeInputHash(docs []domain.DocumentContent, types []domain.ObservationType, maxTokens int) string {
	h := sha256.New()
	for _, doc := range docs {
		h.Write([]byte(doc.DocumentID.String()))
		h.Write([]byte(doc.FileName))
		h.Write([]byte(doc.ContentType))
		h.Write([]byte(doc.Checksum))
		h.Write(doc.Content)
	}
	for _, t := range types {
		h.Write([]byte(t))
	}
	h.Write([]byte(fmt.Sprintf("%d", maxTokens)))
	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}

// computeCaseInputHash produces a SHA-256 over case facts plus the requested
// types and token budget.
func computeCaseInputHash(facts []map[string]any, types []domain.ObservationType, maxTokens int) string {
	h := sha256.New()
	for _, fact := range facts {
		h.Write([]byte(fmt.Sprintf("%v", fact)))
	}
	for _, t := range types {
		h.Write([]byte(t))
	}
	h.Write([]byte(fmt.Sprintf("%d", maxTokens)))
	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}

// computeSHA256 returns the hex SHA-256 of a value, JSON-marshalled.
func computeSHA256(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		data = []byte(fmt.Sprintf("%v", v))
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
