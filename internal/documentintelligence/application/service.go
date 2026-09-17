package application

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/database"
	"github.com/alrazihi/civora/internal/documentintelligence/domain"
	evidencedomain "github.com/alrazihi/civora/internal/evidence/domain"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/google/uuid"
)

var (
	ErrAnalysisNotFound      = errors.New("analysis not found")
	ErrDocumentNotFound      = errors.New("document not found")
	ErrEvidenceNotFound      = errors.New("evidence not found")
	ErrAIProviderUnavailable = errors.New("AI provider unavailable")
	ErrInvalidInput          = errors.New("invalid input")
)

type EvidenceRepository interface {
	FindByID(ctx context.Context, orgID, id uuid.UUID) (*evidencedomain.Evidence, error)
	FindDocumentByID(ctx context.Context, orgID, documentID uuid.UUID) (*evidencedomain.Document, error)
	ListDocuments(ctx context.Context, orgID, evidenceID uuid.UUID, limit, offset int) ([]*evidencedomain.Document, int, error)
	SaveTx(ctx context.Context, tx *sql.Tx, e *evidencedomain.Evidence) error
}

type DocumentContentProvider interface {
	GetDocumentContent(ctx context.Context, orgID, evidenceID, documentID, actorID uuid.UUID) ([]byte, error)
}

type PIISanitizer interface {
	SanitizeDocumentContent(content []byte, contentType string, fileName string) ([]byte, error)
}

type AIProvider interface {
	GenerateDocumentAnalyses(ctx context.Context, req ProviderRequest) ([]AnalysisResult, error)
	ProviderInfo() domain.ModelInfo
}

type ProviderRequest struct {
	OrganizationID uuid.UUID
	EvidenceID     uuid.UUID
	Documents      []domain.DocumentContent
	Options        domain.ProviderOptions
}

type AnalysisResult struct {
	Type       domain.AnalysisType
	Content    map[string]any
	Confidence *float64
	InputHash  string
	OutputHash string
	Model      domain.ModelInfo
}

type DocumentAnalysisService struct {
	repo         domain.DocumentAnalysisRepository
	evidenceRepo EvidenceRepository
	userChecker  shared.UserChecker
	auditor      auditdomain.EventRecorder
	provider     AIProvider
	db           *sql.DB
	documents    DocumentContentProvider
	sanitizer    PIISanitizer
}

func NewDocumentAnalysisService(repo domain.DocumentAnalysisRepository, evidenceRepo EvidenceRepository, userChecker shared.UserChecker, auditor auditdomain.EventRecorder, provider AIProvider) *DocumentAnalysisService {
	return &DocumentAnalysisService{
		repo:         repo,
		evidenceRepo: evidenceRepo,
		userChecker:  userChecker,
		auditor:      auditor,
		provider:     provider,
	}
}

func (s *DocumentAnalysisService) WithTransaction(db *sql.DB) *DocumentAnalysisService {
	s.db = db
	return s
}

func (s *DocumentAnalysisService) WithDocumentContent(documents DocumentContentProvider) *DocumentAnalysisService {
	s.documents = documents
	return s
}

func (s *DocumentAnalysisService) WithPIISanitizer(sanitizer PIISanitizer) *DocumentAnalysisService {
	s.sanitizer = sanitizer
	return s
}

func (s *DocumentAnalysisService) withTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	if s.db == nil {
		return fn(nil)
	}
	return database.InTransaction(ctx, s.db, fn)
}

type GenerateAnalysesParams struct {
	OrganizationID uuid.UUID
	DocumentID     uuid.UUID
	ActorID        uuid.UUID
	Types          []domain.AnalysisType
	MaxTokens      int
}

type AnalysisDTO struct {
	ID             uuid.UUID             `json:"id"`
	OrganizationID uuid.UUID             `json:"organization_id"`
	DocumentID     uuid.UUID             `json:"document_id"`
	EvidenceID     uuid.UUID             `json:"evidence_id"`
	Type           domain.AnalysisType   `json:"type"`
	Source         domain.AnalysisSource `json:"source"`
	Status         domain.AnalysisStatus `json:"status"`
	Model          *domain.ModelInfo     `json:"model,omitempty"`
	Content        map[string]any        `json:"content"`
	Confidence     *float64              `json:"confidence,omitempty"`
	InputHash      string                `json:"input_hash"`
	OutputHash     string                `json:"output_hash"`
	SchemaVersion  string                `json:"schema_version"`
	CreatedAt      time.Time             `json:"created_at"`
	CreatedBy      *uuid.UUID            `json:"created_by,omitempty"`
	ReviewedAt     *time.Time            `json:"reviewed_at,omitempty"`
	ReviewedBy     *uuid.UUID            `json:"reviewed_by,omitempty"`
	ReviewNotes    string                `json:"review_notes,omitempty"`
}

type GenerateAnalysesResult struct {
	Analyses []AnalysisDTO `json:"analyses"`
}

func (s *DocumentAnalysisService) GenerateAnalyses(ctx context.Context, params GenerateAnalysesParams) (*GenerateAnalysesResult, error) {
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

	doc, err := s.evidenceRepo.FindDocumentByID(ctx, params.OrganizationID, params.DocumentID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDocumentNotFound, err)
	}

	e, err := s.evidenceRepo.FindByID(ctx, params.OrganizationID, doc.EvidenceID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEvidenceNotFound, err)
	}
	_ = e

	var docContents []domain.DocumentContent
	dc := domain.DocumentContent{
		DocumentID:  doc.ID,
		FileName:    doc.FileName,
		ContentType: doc.ContentType,
		Checksum:    doc.Checksum,
	}
	if s.documents != nil {
		content, err := s.loadDocumentContent(ctx, params.OrganizationID, doc.EvidenceID, params.ActorID, doc)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve document %s: %w", doc.ID, err)
		}
		dc.Content = content
		if err := validateDocumentChecksum(dc); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
	}
	docContents = append(docContents, dc)

	providerOpts := domain.ProviderOptions{
		Types:     params.Types,
		MaxTokens: params.MaxTokens,
	}
	if providerOpts.MaxTokens <= 0 {
		providerOpts.MaxTokens = 4096
	}

	req := ProviderRequest{
		OrganizationID: params.OrganizationID,
		EvidenceID:     doc.EvidenceID,
		Documents:      docContents,
		Options:        providerOpts,
	}

	results, err := s.provider.GenerateDocumentAnalyses(ctx, req)
	if err != nil {
		s.recordAudit(ctx, params.OrganizationID, &params.ActorID, "document_analysis.failed", "document", shared.StrPtr(params.DocumentID.String()), "failure", map[string]interface{}{
			"reason": err.Error(),
			"model":  s.provider.ProviderInfo().Name,
		})
		return nil, fmt.Errorf("%w: %v", ErrAIProviderUnavailable, err)
	}

	providerInfo := s.provider.ProviderInfo()

	result := &GenerateAnalysesResult{
		Analyses: make([]AnalysisDTO, 0, len(results)),
	}

	if err := s.withTx(ctx, func(tx *sql.Tx) error {
		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: params.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "document_analyses_generated",
				Resource:       "document",
				ResourceID:     shared.StrPtr(params.DocumentID.String()),
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

			analysis, err := domain.NewDocumentAnalysis(domain.DocumentAnalysisParams{
				OrganizationID: params.OrganizationID,
				DocumentID:     params.DocumentID,
				EvidenceID:     doc.EvidenceID,
				Type:           res.Type,
				Source:         domain.AnalysisSourceAIModel,
				Status:         domain.AnalysisStatusPendingReview,
				Model:          &res.Model,
				Content:        res.Content,
				Confidence:     res.Confidence,
				InputHash:      inputHash,
				OutputHash:     outputHash,
				SchemaVersion:  "1.0",
				CreatedBy:      nil,
			})
			if err != nil {
				return fmt.Errorf("%w: %v", ErrInvalidInput, err)
			}

			if s.auditor != nil {
				if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
					OrganizationID: params.OrganizationID,
					ActorID:        &params.ActorID,
					Action:         "document_analysis.created",
					Resource:       "document_analysis",
					ResourceID:     shared.StrPtr(analysis.ID.String()),
					Outcome:        "success",
					Metadata: map[string]interface{}{
						"document_id":    analysis.DocumentID.String(),
						"evidence_id":    analysis.EvidenceID.String(),
						"analysis_type":  string(analysis.Type),
						"model":          analysis.Model.Name,
						"confidence":     analysis.Confidence,
						"input_hash":     analysis.InputHash,
						"output_hash":    analysis.OutputHash,
						"schema_version": analysis.SchemaVersion,
					},
				}); err != nil {
					return fmt.Errorf("failed to record audit event: %w", err)
				}
			}

			if err := s.repo.SaveTx(ctx, tx, analysis); err != nil {
				return fmt.Errorf("failed to save analysis: %w", err)
			}

			result.Analyses = append(result.Analyses, serializeAnalysis(analysis))
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func (s *DocumentAnalysisService) recordAudit(ctx context.Context, orgID uuid.UUID, actorID *uuid.UUID, action, resource string, resourceID *string, outcome string, metadata map[string]interface{}) {
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

type ListAnalysesParams struct {
	OrganizationID uuid.UUID
	DocumentID     uuid.UUID
	EvidenceID     *uuid.UUID
	ActorID        uuid.UUID
	Limit          int
	Offset         int
}

func (s *DocumentAnalysisService) ListAnalyses(ctx context.Context, params ListAnalysesParams) ([]*domain.DocumentAnalysis, int, error) {
	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.Limit > 200 {
		params.Limit = 200
	}
	if params.Offset < 0 {
		params.Offset = 0
	}

	_, err := s.evidenceRepo.FindByID(ctx, params.OrganizationID, func() uuid.UUID {
		if params.EvidenceID != nil {
			return *params.EvidenceID
		}
		doc, err := s.evidenceRepo.FindDocumentByID(ctx, params.OrganizationID, params.DocumentID)
		if err != nil {
			return uuid.Nil
		}
		return doc.EvidenceID
	}())
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrEvidenceNotFound, err)
	}

	if params.EvidenceID != nil {
		items, total, err := s.repo.FindByEvidence(ctx, params.OrganizationID, *params.EvidenceID, params.Limit, params.Offset)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to list analyses: %w", err)
		}
		return items, total, nil
	}

	items, total, err := s.repo.FindByDocument(ctx, params.OrganizationID, params.DocumentID, params.Limit, params.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list analyses: %w", err)
	}

	return items, total, nil
}

type ReviewAnalysisParams struct {
	OrganizationID uuid.UUID
	AnalysisID     uuid.UUID
	ReviewerID     uuid.UUID
	Action         domain.AnalysisStatus
	Notes          string
}

func (s *DocumentAnalysisService) ReviewAnalysis(ctx context.Context, params ReviewAnalysisParams) (*domain.DocumentAnalysis, error) {
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

	analysis, err := s.repo.FindByID(ctx, params.OrganizationID, params.AnalysisID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAnalysisNotFound, err)
	}

	switch params.Action {
	case domain.AnalysisStatusAccepted:
		if err := analysis.Accept(params.ReviewerID, params.Notes); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
	case domain.AnalysisStatusRejected:
		if err := analysis.Reject(params.ReviewerID, params.Notes); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
	default:
		return nil, fmt.Errorf("%w: invalid action %q", ErrInvalidInput, params.Action)
	}

	auditAction := "document_analysis.rejected"
	if params.Action == domain.AnalysisStatusAccepted {
		auditAction = "document_analysis.accepted"
	}

	if err := s.withTx(ctx, func(tx *sql.Tx) error {
		if err := s.repo.UpdateStatusTx(ctx, tx, params.OrganizationID, params.AnalysisID, params.Action, &params.ReviewerID, params.Notes); err != nil {
			return fmt.Errorf("failed to update analysis status: %w", err)
		}
		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: params.OrganizationID,
				ActorID:        &params.ReviewerID,
				Action:         auditAction,
				Resource:       "document_analysis",
				ResourceID:     shared.StrPtr(params.AnalysisID.String()),
				Outcome:        "success",
				Metadata: map[string]interface{}{
					"document_id": analysis.DocumentID.String(),
					"evidence_id": analysis.EvidenceID.String(),
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

	return analysis, nil
}

func serializeAnalysis(a *domain.DocumentAnalysis) AnalysisDTO {
	return AnalysisDTO{
		ID:             a.ID,
		OrganizationID: a.OrganizationID,
		DocumentID:     a.DocumentID,
		EvidenceID:     a.EvidenceID,
		Type:           a.Type,
		Source:         a.Source,
		Status:         a.Status,
		Model:          a.Model,
		Content:        a.Content,
		Confidence:     a.Confidence,
		InputHash:      a.InputHash,
		OutputHash:     a.OutputHash,
		SchemaVersion:  a.SchemaVersion,
		CreatedAt:      a.CreatedAt,
		CreatedBy:      a.CreatedBy,
		ReviewedAt:     a.ReviewedAt,
		ReviewedBy:     a.ReviewedBy,
		ReviewNotes:    a.ReviewNotes,
	}
}

func (s *DocumentAnalysisService) loadDocumentContent(ctx context.Context, orgID, evidenceID, actorID uuid.UUID, doc *evidencedomain.Document) ([]byte, error) {
	data, err := s.documents.GetDocumentContent(ctx, orgID, evidenceID, doc.ID, actorID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve document content: %w", err)
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

func computeInputHash(docs []domain.DocumentContent, types []domain.AnalysisType, maxTokens int) string {
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

func computeSHA256(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		data = []byte(fmt.Sprintf("%v", v))
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
