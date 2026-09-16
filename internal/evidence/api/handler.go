package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/alrazihi/civora/internal/evidence/application"
	"github.com/alrazihi/civora/internal/evidence/domain"
	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc EvidenceService
}

func NewHandler(svc EvidenceService) *Handler {
	return &Handler{svc: svc}
}

type EvidenceService interface {
	AddEvidence(ctx context.Context, params application.AddEvidenceParams) (*domain.Evidence, error)
	GetEvidence(ctx context.Context, orgID, id uuid.UUID) (*domain.Evidence, error)
	ListEvidence(ctx context.Context, orgID, serviceRequestID uuid.UUID, limit, offset int) ([]*domain.Evidence, int, error)
	VerifyEvidence(ctx context.Context, params application.VerifyEvidenceParams) (*domain.Evidence, *domain.VerificationRecord, error)
	RejectEvidence(ctx context.Context, params application.VerifyEvidenceParams) (*domain.Evidence, *domain.VerificationRecord, error)
	MarkEvidenceForReview(ctx context.Context, params application.VerifyEvidenceParams) (*domain.Evidence, *domain.VerificationRecord, error)
	UpdateEvidenceMetadata(ctx context.Context, params application.UpdateEvidenceMetadataParams) (*domain.Evidence, error)
	ListVerificationHistory(ctx context.Context, params application.ListVerificationHistoryParams) ([]*domain.VerificationRecord, int, error)
	UploadDocument(ctx context.Context, params application.UploadDocumentParams) (*application.UploadDocumentResult, error)
	GetDocumentStream(ctx context.Context, orgID, evidenceID, downloaderID uuid.UUID) (*application.DownloadDocumentResult, error)
	ListDocuments(ctx context.Context, params application.ListDocumentsParams) ([]*domain.Document, int, error)
	DeleteDocument(ctx context.Context, params application.DeleteDocumentParams) error
}

func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/organizations/{orgId}/evidence", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.RequireSameTenant)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAnyRole("admin", "staff"))
			r.Post("/", h.AddEvidence)
			r.Get("/", h.ListEvidence)
			r.Get("/by-service-request/{serviceRequestId}", h.ListEvidence)
			r.Get("/{evidenceId}", h.GetEvidence)
			r.Patch("/{evidenceId}", h.UpdateEvidenceMetadata)
			r.Route("/{evidenceId}", func(r chi.Router) {
				r.Post("/upload", h.UploadDocument)
				r.Get("/documents", h.ListDocuments)
				r.Get("/document", h.DownloadDocument)
				r.Delete("/documents/{documentId}", h.DeleteDocument)
				r.Post("/verify", h.VerifyEvidence)
				r.Post("/reject", h.RejectEvidence)
				r.Post("/review", h.MarkEvidenceForReview)
				r.Get("/history", h.ListVerificationHistory)
			})
		})
	})
}

func (h *Handler) AddEvidence(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		ServiceRequestID string                 `json:"service_request_id"`
		Type             domain.EvidenceType    `json:"type"`
		CustomType       string                 `json:"custom_type,omitempty"`
		Description      string                 `json:"description"`
		StorageReference string                 `json:"storage_reference"`
		PersonID         *string                `json:"person_id,omitempty"`
		Source           string                 `json:"source,omitempty"`
		Metadata         map[string]interface{} `json:"metadata,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	serviceRequestID, err := uuid.Parse(req.ServiceRequestID)
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid service request ID")
		return
	}

	var personID *uuid.UUID
	if req.PersonID != nil {
		p, err := uuid.Parse(*req.PersonID)
		if err != nil {
			shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid person ID")
			return
		}
		personID = &p
	}

	source := domain.EvidenceSourceUpload
	if req.Source != "" {
		source = domain.EvidenceSource(req.Source)
	}

	e, err := h.svc.AddEvidence(r.Context(), application.AddEvidenceParams{
		OrganizationID:   orgID,
		ServiceRequestID: serviceRequestID,
		Type:             req.Type,
		CustomType:       req.CustomType,
		Description:      req.Description,
		StorageReference: req.StorageReference,
		UploadedBy:       actorID,
		PersonID:         personID,
		Source:           source,
		Metadata:         req.Metadata,
	})
	if err != nil {
		writeEvidenceError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusCreated, serializeEvidence(e), nil)
}

func (h *Handler) GetEvidence(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	evidenceID, ok := parseUUID(r, "evidenceId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid evidence ID")
		return
	}

	e, err := h.svc.GetEvidence(r.Context(), orgID, evidenceID)
	if err != nil {
		writeEvidenceError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeEvidence(e), nil)
}

func (h *Handler) ListEvidence(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	serviceRequestID, ok := parseUUID(r, "serviceRequestId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid service request ID")
		return
	}

	page, parseErr := strconv.Atoi(r.URL.Query().Get("page"))
	if parseErr != nil {
		page = 1
	}
	perPage, parseErr := strconv.Atoi(r.URL.Query().Get("per_page"))
	if parseErr != nil {
		perPage = 20
	}
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 200 {
		perPage = 200
	}
	offset := (page - 1) * perPage

	items, total, err := h.svc.ListEvidence(r.Context(), orgID, serviceRequestID, perPage, offset)
	if err != nil {
		writeEvidenceError(w, err)
		return
	}

	result := make([]map[string]interface{}, len(items))
	for i, e := range items {
		result[i] = serializeEvidence(e)
	}

	shared.WritePaginatedSuccess(w, http.StatusOK, result, page, perPage, total)
}

func (h *Handler) VerifyEvidence(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	evidenceID, ok := parseUUID(r, "evidenceId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid evidence ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		Reason string `json:"reason,omitempty"`
		Method string `json:"method,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	e, rec, err := h.svc.VerifyEvidence(r.Context(), application.VerifyEvidenceParams{
		OrganizationID: orgID,
		EvidenceID:     evidenceID,
		VerifierID:     actorID,
		Reason:         req.Reason,
		Method:         req.Method,
	})
	if err != nil {
		writeEvidenceError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeEvidenceWithHistory(e, rec), nil)
}

func (h *Handler) RejectEvidence(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	evidenceID, ok := parseUUID(r, "evidenceId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid evidence ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		Reason string `json:"reason,omitempty"`
		Method string `json:"method,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	e, rec, err := h.svc.RejectEvidence(r.Context(), application.VerifyEvidenceParams{
		OrganizationID: orgID,
		EvidenceID:     evidenceID,
		VerifierID:     actorID,
		Reason:         req.Reason,
		Method:         req.Method,
	})
	if err != nil {
		writeEvidenceError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeEvidenceWithHistory(e, rec), nil)
}

func (h *Handler) MarkEvidenceForReview(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	evidenceID, ok := parseUUID(r, "evidenceId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid evidence ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		Reason string `json:"reason,omitempty"`
		Method string `json:"method,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	e, rec, err := h.svc.MarkEvidenceForReview(r.Context(), application.VerifyEvidenceParams{
		OrganizationID: orgID,
		EvidenceID:     evidenceID,
		VerifierID:     actorID,
		Reason:         req.Reason,
		Method:         req.Method,
	})
	if err != nil {
		writeEvidenceError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeEvidenceWithHistory(e, rec), nil)
}

func (h *Handler) UpdateEvidenceMetadata(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	evidenceID, ok := parseUUID(r, "evidenceId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid evidence ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	e, err := h.svc.UpdateEvidenceMetadata(r.Context(), application.UpdateEvidenceMetadataParams{
		OrganizationID: orgID,
		EvidenceID:     evidenceID,
		Metadata:       req.Metadata,
		ActorID:        actorID,
	})
	if err != nil {
		writeEvidenceError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeEvidence(e), nil)
}

func (h *Handler) ListVerificationHistory(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	evidenceID, ok := parseUUID(r, "evidenceId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid evidence ID")
		return
	}

	page, parseErr := strconv.Atoi(r.URL.Query().Get("page"))
	if parseErr != nil {
		page = 1
	}
	perPage, parseErr := strconv.Atoi(r.URL.Query().Get("per_page"))
	if parseErr != nil {
		perPage = 20
	}
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 200 {
		perPage = 200
	}
	offset := (page - 1) * perPage

	items, total, err := h.svc.ListVerificationHistory(r.Context(), application.ListVerificationHistoryParams{
		OrganizationID: orgID,
		EvidenceID:     evidenceID,
		Limit:          perPage,
		Offset:         offset,
	})
	if err != nil {
		writeEvidenceError(w, err)
		return
	}

	result := make([]map[string]interface{}, len(items))
	for i, rec := range items {
		result[i] = serializeVerificationRecord(rec)
	}

	shared.WritePaginatedSuccess(w, http.StatusOK, result, page, perPage, total)
}

func (h *Handler) UploadDocument(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	evidenceID, ok := parseUUID(r, "evidenceId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid evidence ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "request too large or invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "no file provided")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	result, err := h.svc.UploadDocument(r.Context(), application.UploadDocumentParams{
		OrganizationID: orgID,
		EvidenceID:     evidenceID,
		FileName:       header.Filename,
		ContentType:    contentType,
		Size:           header.Size,
		Reader:         file,
		UploadedBy:     actorID,
	})
	if err != nil {
		writeEvidenceError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusCreated, serializeDocument(result.Document), nil)
}

func (h *Handler) DownloadDocument(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	evidenceID, ok := parseUUID(r, "evidenceId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid evidence ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	result, err := h.svc.GetDocumentStream(r.Context(), orgID, evidenceID, actorID)
	if err != nil {
		writeEvidenceError(w, err)
		return
	}
	defer result.Content.Close()

	w.Header().Set("Content-Type", result.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, sanitizeFilename(result.FileName)))
	w.Header().Set("Content-Length", strconv.FormatInt(result.Document.SizeBytes, 10))
	w.Header().Set("X-Checksum-Sha256", result.Document.Checksum)

	if _, err := io.Copy(w, result.Content); err != nil {
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "failed to stream document")
		return
	}
}

func (h *Handler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	evidenceID, ok := parseUUID(r, "evidenceId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid evidence ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	page, parseErr := strconv.Atoi(r.URL.Query().Get("page"))
	if parseErr != nil {
		page = 1
	}
	perPage, parseErr := strconv.Atoi(r.URL.Query().Get("per_page"))
	if parseErr != nil {
		perPage = 50
	}
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 50
	}
	if perPage > 200 {
		perPage = 200
	}
	offset := (page - 1) * perPage

	items, total, err := h.svc.ListDocuments(r.Context(), application.ListDocumentsParams{
		OrganizationID: orgID,
		EvidenceID:     evidenceID,
		DownloaderID:   actorID,
		Limit:          perPage,
		Offset:         offset,
	})
	if err != nil {
		writeEvidenceError(w, err)
		return
	}

	result := make([]map[string]interface{}, len(items))
	for i, d := range items {
		result[i] = serializeDocument(d)
	}

	shared.WritePaginatedSuccess(w, http.StatusOK, result, page, perPage, total)
}

func (h *Handler) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	documentID, ok := parseUUID(r, "documentId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid document ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	err := h.svc.DeleteDocument(r.Context(), application.DeleteDocumentParams{
		OrganizationID: orgID,
		DocumentID:     documentID,
		DeletedBy:      actorID,
	})
	if err != nil {
		writeEvidenceError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusNoContent, nil, nil)
}

func sanitizeFilename(name string) string {
	for _, c := range name {
		if c == '"' || c == '\\' || c == '\r' || c == '\n' {
			name = strings.ReplaceAll(name, string(c), "_")
		}
	}
	return name
}

func serializeDocument(d *domain.Document) map[string]interface{} {
	return map[string]interface{}{
		"id":           d.ID,
		"evidence_id":  d.EvidenceID,
		"file_name":    d.FileName,
		"content_type": d.ContentType,
		"size_bytes":   d.SizeBytes,
		"checksum":     d.Checksum,
		"uploaded_by":  d.UploadedBy,
		"uploaded_at":  d.UploadedAt,
		"created_at":   d.CreatedAt,
	}
}

func parseUUID(r *http.Request, name string) (uuid.UUID, bool) {
	v := chi.URLParam(r, name)
	id, err := uuid.Parse(v)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

func getUserID(r *http.Request) uuid.UUID {
	idStr := middleware.GetUserID(r)
	if idStr == "" {
		return uuid.Nil
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil
	}
	return id
}

func serializeEvidence(e *domain.Evidence) map[string]interface{} {
	result := map[string]interface{}{
		"id":                  e.ID,
		"organization_id":     e.OrganizationID,
		"service_request_id":  e.ServiceRequestID,
		"type":                e.Type,
		"description":         e.Description,
		"uploaded_by":         e.UploadedBy,
		"created_at":          e.CreatedAt,
		"verification_status": e.VerificationStatus,
		"evidence_source":     e.Source,
	}
	if e.CustomType != nil {
		result["custom_type"] = *e.CustomType
	}
	if e.PersonID != nil {
		result["person_id"] = *e.PersonID
	}
	if e.Metadata != nil {
		result["metadata"] = e.Metadata
	}
	if e.VerifiedBy != nil {
		result["verified_by"] = *e.VerifiedBy
	}
	if e.VerifiedAt != nil {
		result["verified_at"] = *e.VerifiedAt
	}
	if e.VerificationReason != "" {
		result["verification_reason"] = e.VerificationReason
	}
	if e.VerificationMethod != "" {
		result["verification_method"] = e.VerificationMethod
	}
	return result
}

func serializeEvidenceWithHistory(e *domain.Evidence, rec *domain.VerificationRecord) map[string]interface{} {
	result := serializeEvidence(e)
	result["verification_record"] = serializeVerificationRecord(rec)
	return result
}

func serializeVerificationRecord(rec *domain.VerificationRecord) map[string]interface{} {
	result := map[string]interface{}{
		"id":          rec.ID,
		"evidence_id": rec.EvidenceID,
		"status":      rec.Status,
		"verified_at": rec.VerifiedAt,
		"created_at":  rec.CreatedAt,
	}
	if rec.VerifierID != nil {
		result["verifier_id"] = *rec.VerifierID
	}
	if rec.Reason != "" {
		result["reason"] = rec.Reason
	}
	if rec.Method != "" {
		result["method"] = rec.Method
	}
	if rec.Notes != "" {
		result["notes"] = rec.Notes
	}
	return result
}

func writeEvidenceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrEvidenceNotFound), errors.Is(err, domain.ErrEvidenceNotFound),
		errors.Is(err, domain.ErrDocumentNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "resource not found")
	case errors.Is(err, application.ErrEvidenceInput), errors.Is(err, domain.ErrEvidenceInvalidInput):
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid input")
	case errors.Is(err, application.ErrCaseNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "case not found")
	case errors.Is(err, application.ErrCaseClosed):
		shared.WriteError(w, http.StatusConflict, shared.CodeConflict, "case is closed")
	case errors.Is(err, application.ErrUserNotFound), errors.Is(err, application.ErrVerifierNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "user not found")
	default:
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "internal server error")
	}
}
