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
	ListDocuments(ctx context.Context, orgID, evidenceID uuid.UUID, limit, offset int) ([]*evidencedomain.Document, int, error)
	SaveTx(ctx context.Context, tx *sql.Tx, e *evidencedomain.Evidence) error
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
	repo         domain.ObservationRepository
	evidenceRepo EvidenceRepository
	userChecker  shared.UserChecker
	auditor      auditdomain.EventRecorder
	provider     AIProvider
	db           *sql.DB
	documents    DocumentContentProvider
	sanitizer    PIISanitizer
}

func NewAIService(repo domain.ObservationRepository, evidenceRepo EvidenceRepository, userChecker shared.UserChecker, auditor auditdomain.EventRecorder, provider AIProvider) *AIService {
	return &AIService{
		repo:         repo,
		evidenceRepo: evidenceRepo,
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

// WithPIISanitizer wires optional PII redaction applied to documents before
// they are sent to a provider.
func (s *AIService) WithPIISanitizer(sanitizer PIISanitizer) *AIService {
	s.sanitizer = sanitizer
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
	ID             uuid.UUID                `json:"id"`
	OrganizationID uuid.UUID                `json:"organization_id"`
	EvidenceID     uuid.UUID                `json:"evidence_id"`
	Type           domain.ObservationType   `json:"type"`
	Source         domain.ObservationSource `json:"source"`
	Status         domain.ObservationStatus `json:"status"`
	Model          *domain.ModelInfo        `json:"model,omitempty"`
	Content        map[string]any           `json:"content"`
	Confidence     *float64                 `json:"confidence,omitempty"`
	InputHash      string                   `json:"input_hash"`
	OutputHash     string                   `json:"output_hash"`
	CreatedAt      time.Time                `json:"created_at"`
	CreatedBy      *uuid.UUID               `json:"created_by,omitempty"`
	ReviewedAt     *time.Time               `json:"reviewed_at,omitempty"`
	ReviewedBy     *uuid.UUID               `json:"reviewed_by,omitempty"`
	ReviewNotes    string                   `json:"review_notes,omitempty"`
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

	// Capture inputs before persisting so the result can be returned even if
	// the provider call later fails in a way we don't expect.
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
				EvidenceID:     params.EvidenceID,
				Type:           res.Type,
				Source:         domain.ObservationSourceAIModel,
				Status:         domain.ObservationStatusPendingReview,
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

type ReviewObservationParams struct {
	OrganizationID uuid.UUID
	ObservationID  uuid.UUID
	ReviewerID     uuid.UUID
	Action         domain.ObservationStatus
	Notes          string
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
	default:
		return nil, fmt.Errorf("%w: invalid action %q", ErrInvalidInput, params.Action)
	}

	auditAction := "ai.observation.rejected"
	if params.Action == domain.ObservationStatusAccepted {
		auditAction = "ai.observation.accepted"
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

func serializeObservation(o *domain.Observation) ObservationDTO {
	return ObservationDTO{
		ID:             o.ID,
		OrganizationID: o.OrganizationID,
		EvidenceID:     o.EvidenceID,
		Type:           o.Type,
		Source:         o.Source,
		Status:         o.Status,
		Model:          o.Model,
		Content:        o.Content,
		Confidence:     o.Confidence,
		InputHash:      o.InputHash,
		OutputHash:     o.OutputHash,
		CreatedAt:      o.CreatedAt,
		CreatedBy:      o.CreatedBy,
		ReviewedAt:     o.ReviewedAt,
		ReviewedBy:     o.ReviewedBy,
		ReviewNotes:    o.ReviewNotes,
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

	if s.sanitizer != nil {
		sanitized, err := s.sanitizer.SanitizeDocumentContent(data, doc.ContentType, doc.FileName)
		if err != nil {
			return nil, fmt.Errorf("failed to sanitize document content: %w", err)
		}
		data = sanitized
	}

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

// computeSHA256 returns the hex SHA-256 of a value, JSON-marshalled.
func computeSHA256(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		data = []byte(fmt.Sprintf("%v", v))
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
