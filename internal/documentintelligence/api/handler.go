package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/alrazihi/civora/internal/documentintelligence/application"
	"github.com/alrazihi/civora/internal/documentintelligence/domain"
	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	service *application.DocumentAnalysisService
}

func NewHandler(service *application.DocumentAnalysisService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/documents/{documentId}/analysis", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.RequireSameTenant)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAnyRole("admin", "staff"))
			r.Post("/generate", h.GenerateAnalyses)
			r.Get("/", h.ListAnalyses)
			r.Post("/{analysisId}/accept", h.AcceptAnalysis)
			r.Post("/{analysisId}/reject", h.RejectAnalysis)
		})
	})
}

func (h *Handler) GenerateAnalyses(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		Types     []string `json:"types"`
		MaxTokens int      `json:"max_tokens"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid JSON body")
		return
	}

	var types []domain.AnalysisType
	for _, t := range req.Types {
		types = append(types, domain.AnalysisType(t))
	}

	params := application.GenerateAnalysesParams{
		OrganizationID: orgID,
		DocumentID:     documentID,
		ActorID:        actorID,
		Types:          types,
		MaxTokens:      req.MaxTokens,
	}

	result, err := h.service.GenerateAnalyses(r.Context(), params)
	if err != nil {
		writeAnalysisError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, result, nil)
}

func (h *Handler) ListAnalyses(w http.ResponseWriter, r *http.Request) {
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

	evidenceIDStr := r.URL.Query().Get("evidence_id")
	var evidenceID *uuid.UUID
	if evidenceIDStr != "" {
		eid, err := uuid.Parse(evidenceIDStr)
		if err != nil {
			shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid evidence ID")
			return
		}
		evidenceID = &eid
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	params := application.ListAnalysesParams{
		OrganizationID: orgID,
		DocumentID:     documentID,
		EvidenceID:     evidenceID,
		ActorID:        actorID,
		Limit:          limit,
		Offset:         offset,
	}

	analyses, total, err := h.service.ListAnalyses(r.Context(), params)
	if err != nil {
		writeAnalysisError(w, err)
		return
	}

	dtos := make([]application.AnalysisDTO, len(analyses))
	for i, a := range analyses {
		dtos[i] = serializeAnalysis(a)
	}

	shared.WritePaginatedSuccess(w, http.StatusOK, dtos, total, limit, offset)
}

func (h *Handler) AcceptAnalysis(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	_, ok = parseUUID(r, "documentId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid document ID")
		return
	}

	analysisID, ok := parseUUID(r, "analysisId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid analysis ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		Notes string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid JSON body")
		return
	}

	params := application.ReviewAnalysisParams{
		OrganizationID: orgID,
		AnalysisID:     analysisID,
		ReviewerID:     actorID,
		Action:         domain.AnalysisStatusAccepted,
		Notes:          req.Notes,
	}

	analysis, err := h.service.ReviewAnalysis(r.Context(), params)
	if err != nil {
		writeAnalysisError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeAnalysis(analysis), nil)
}

func (h *Handler) RejectAnalysis(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	_, ok = parseUUID(r, "documentId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid document ID")
		return
	}

	analysisID, ok := parseUUID(r, "analysisId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid analysis ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		Notes string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid JSON body")
		return
	}

	params := application.ReviewAnalysisParams{
		OrganizationID: orgID,
		AnalysisID:     analysisID,
		ReviewerID:     actorID,
		Action:         domain.AnalysisStatusRejected,
		Notes:          req.Notes,
	}

	analysis, err := h.service.ReviewAnalysis(r.Context(), params)
	if err != nil {
		writeAnalysisError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeAnalysis(analysis), nil)
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

func serializeAnalysis(a *domain.DocumentAnalysis) application.AnalysisDTO {
	return application.AnalysisDTO{
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

func writeAnalysisError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrDocumentNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "document not found")
	case errors.Is(err, application.ErrEvidenceNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "evidence not found")
	case errors.Is(err, application.ErrAnalysisNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "analysis not found")
	case errors.Is(err, application.ErrAIProviderUnavailable):
		shared.WriteError(w, http.StatusServiceUnavailable, shared.CodeInternalError, "AI provider unavailable")
	case errors.Is(err, application.ErrInvalidInput):
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid input")
	default:
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "internal server error")
	}
}
