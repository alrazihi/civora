package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/alrazihi/civora/internal/ai/application"
	"github.com/alrazihi/civora/internal/ai/domain"
	intmid "github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc AIService
}

type AIService interface {
	GenerateObservations(ctx context.Context, params application.GenerateObservationsParams) (*application.GenerateObservationsResult, error)
	GenerateCaseObservations(ctx context.Context, params application.GenerateCaseObservationsParams) (*application.GenerateCaseObservationsResult, error)
	ListObservations(ctx context.Context, params application.ListObservationsParams) ([]*domain.Observation, int, error)
	ListCaseObservations(ctx context.Context, params application.ListCaseObservationsParams) ([]*domain.Observation, int, error)
	ReviewObservation(ctx context.Context, params application.ReviewObservationParams) (*domain.Observation, error)
	CreateVerifiedFact(ctx context.Context, params application.CreateVerifiedFactParams) (*domain.VerifiedFact, error)
}

func NewHandler(svc AIService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/organizations/{orgId}/evidence/{evidenceId}/ai", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(intmid.RequireSameTenant)
		r.Group(func(r chi.Router) {
			r.Use(intmid.RequireAnyRole("admin", "staff"))
			r.Post("/observations/generate", h.GenerateObservations)
			r.Get("/observations", h.ListObservations)
			r.Post("/observations/{observationId}/accept", h.AcceptObservation)
			r.Post("/observations/{observationId}/reject", h.RejectObservation)
			r.Post("/observations/{observationId}/verified-facts", h.CreateVerifiedFact)
		})
	})

	r.Route("/api/v1/organizations/{orgId}/cases/{caseId}/ai", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(intmid.RequireSameTenant)
		r.Group(func(r chi.Router) {
			r.Use(intmid.RequireAnyRole("admin", "staff"))
			r.Post("/observations/generate", h.GenerateCaseObservations)
			r.Get("/observations", h.ListCaseObservations)
			r.Post("/observations/{observationId}/accept", h.AcceptObservation)
			r.Post("/observations/{observationId}/reject", h.RejectObservation)
			r.Post("/observations/{observationId}/correct", h.CorrectObservation)
			r.Post("/observations/{observationId}/dismiss", h.DismissObservation)
			r.Post("/observations/{observationId}/verified-facts", h.CreateVerifiedFact)
		})
	})
}

func (h *Handler) GenerateObservations(w http.ResponseWriter, r *http.Request) {
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
		Types     []string `json:"types,omitempty"`
		MaxTokens int      `json:"max_tokens,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	var obsTypes []domain.ObservationType
	for _, t := range req.Types {
		obsTypes = append(obsTypes, domain.ObservationType(t))
	}

	result, err := h.svc.GenerateObservations(r.Context(), application.GenerateObservationsParams{
		OrganizationID: orgID,
		EvidenceID:     evidenceID,
		ActorID:        actorID,
		Types:          obsTypes,
		MaxTokens:      req.MaxTokens,
	})
	if err != nil {
		writeAIError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, result, nil)
}

func (h *Handler) GenerateCaseObservations(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	caseID, ok := parseUUID(r, "caseId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid case ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		Types     []string `json:"types,omitempty"`
		MaxTokens int      `json:"max_tokens,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	var obsTypes []domain.ObservationType
	for _, t := range req.Types {
		obsTypes = append(obsTypes, domain.ObservationType(t))
	}

	result, err := h.svc.GenerateCaseObservations(r.Context(), application.GenerateCaseObservationsParams{
		OrganizationID: orgID,
		CaseID:         caseID,
		ActorID:        actorID,
		Types:          obsTypes,
		MaxTokens:      req.MaxTokens,
	})
	if err != nil {
		writeAIError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, result, nil)
}

func (h *Handler) ListObservations(w http.ResponseWriter, r *http.Request) {
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

	items, total, err := h.svc.ListObservations(r.Context(), application.ListObservationsParams{
		OrganizationID: orgID,
		EvidenceID:     evidenceID,
		ActorID:        actorID,
		Limit:          perPage,
		Offset:         offset,
	})
	if err != nil {
		writeAIError(w, err)
		return
	}

	result := make([]map[string]interface{}, len(items))
	for i, o := range items {
		result[i] = serializeObservation(o)
	}

	shared.WritePaginatedSuccess(w, http.StatusOK, result, page, perPage, total)
}

func (h *Handler) ListCaseObservations(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	caseID, ok := parseUUID(r, "caseId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid case ID")
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

	items, total, err := h.svc.ListCaseObservations(r.Context(), application.ListCaseObservationsParams{
		OrganizationID: orgID,
		CaseID:         caseID,
		ActorID:        actorID,
		Limit:          perPage,
		Offset:         offset,
	})
	if err != nil {
		writeAIError(w, err)
		return
	}

	result := make([]map[string]interface{}, len(items))
	for i, o := range items {
		result[i] = serializeObservation(o)
	}

	shared.WritePaginatedSuccess(w, http.StatusOK, result, page, perPage, total)
}

func (h *Handler) AcceptObservation(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	observationID, ok := parseUUID(r, "observationId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid observation ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		Notes string `json:"notes,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	obs, err := h.svc.ReviewObservation(r.Context(), application.ReviewObservationParams{
		OrganizationID: orgID,
		ObservationID:  observationID,
		ReviewerID:     actorID,
		Action:         domain.ObservationStatusAccepted,
		Notes:          req.Notes,
	})
	if err != nil {
		writeAIError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeObservation(obs), nil)
}

func (h *Handler) RejectObservation(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	observationID, ok := parseUUID(r, "observationId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid observation ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		Notes string `json:"notes,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	obs, err := h.svc.ReviewObservation(r.Context(), application.ReviewObservationParams{
		OrganizationID: orgID,
		ObservationID:  observationID,
		ReviewerID:     actorID,
		Action:         domain.ObservationStatusRejected,
		Notes:          req.Notes,
	})
	if err != nil {
		writeAIError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeObservation(obs), nil)
}

func (h *Handler) CorrectObservation(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	observationID, ok := parseUUID(r, "observationId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid observation ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		Notes string `json:"notes,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	obs, err := h.svc.ReviewObservation(r.Context(), application.ReviewObservationParams{
		OrganizationID: orgID,
		ObservationID:  observationID,
		ReviewerID:     actorID,
		Action:         domain.ObservationStatusCorrected,
		Notes:          req.Notes,
	})
	if err != nil {
		writeAIError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeObservation(obs), nil)
}

func (h *Handler) DismissObservation(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	observationID, ok := parseUUID(r, "observationId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid observation ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		Notes string `json:"notes,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	obs, err := h.svc.ReviewObservation(r.Context(), application.ReviewObservationParams{
		OrganizationID: orgID,
		ObservationID:  observationID,
		ReviewerID:     actorID,
		Action:         domain.ObservationStatusDismissed,
		Notes:          req.Notes,
	})
	if err != nil {
		writeAIError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeObservation(obs), nil)
}

func (h *Handler) CreateVerifiedFact(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	observationID, ok := parseUUID(r, "observationId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid observation ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		Action string `json:"action,omitempty"`
		Notes  string `json:"notes,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	action := domain.ObservationStatus(req.Action)
	if action == "" {
		action = domain.ObservationStatusAccepted
	}
	if err := validateVerifiedFactAction(action); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, err.Error())
		return
	}

	fact, err := h.svc.CreateVerifiedFact(r.Context(), application.CreateVerifiedFactParams{
		OrganizationID: orgID,
		ObservationID:  observationID,
		ReviewerID:     actorID,
		ReviewAction:   action,
		ReviewNotes:    req.Notes,
	})
	if err != nil {
		writeAIError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusCreated, serializeVerifiedFact(fact), nil)
}

func validateVerifiedFactAction(action domain.ObservationStatus) error {
	switch action {
	case domain.ObservationStatusAccepted,
		domain.ObservationStatusRejected,
		domain.ObservationStatusCorrected,
		domain.ObservationStatusDismissed:
		return nil
	default:
		return fmt.Errorf("invalid action %q", action)
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
	idStr := intmid.GetUserID(r)
	if idStr == "" {
		return uuid.Nil
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil
	}
	return id
}

func serializeObservation(o *domain.Observation) map[string]interface{} {
	result := map[string]interface{}{
		"id":                o.ID,
		"organization_id":   o.OrganizationID,
		"type":              o.Type,
		"source":            o.Source,
		"status":            o.Status,
		"content":           o.Content,
		"statement":         o.Statement,
		"source_references": o.SourceReferences,
		"input_hash":        o.InputHash,
		"output_hash":       o.OutputHash,
		"created_at":        o.CreatedAt,
		"reviewed_at":       o.ReviewedAt,
		"review_notes":      o.ReviewNotes,
	}
	if o.CaseID != nil {
		result["case_id"] = *o.CaseID
	}
	if o.EvidenceID != nil {
		result["evidence_id"] = *o.EvidenceID
	}
	if o.Model != nil {
		result["model"] = o.Model
	}
	if o.Confidence != nil {
		result["confidence"] = o.Confidence
	}
	if o.CreatedBy != nil {
		result["created_by"] = o.CreatedBy
	}
	if o.ReviewedBy != nil {
		result["reviewed_by"] = o.ReviewedBy
	}
	return result
}

func serializeVerifiedFact(f *domain.VerifiedFact) map[string]interface{} {
	result := map[string]interface{}{
		"id":               f.ID,
		"organization_id":  f.OrganizationID,
		"observation_id":   f.ObservationID,
		"observation_type": f.Type,
		"value":            f.Value,
		"original_value":   f.OriginalValue,
		"corrected_value":  f.CorrectedValue,
		"provenance":       f.Provenance,
		"review_action":    f.ReviewAction,
		"reviewer_id":      f.ReviewerID,
		"review_notes":     f.ReviewNotes,
		"created_at":       f.CreatedAt,
		"verified_at":      f.VerifiedAt,
		"source":           f.Source,
		"input_hash":       f.InputHash,
		"output_hash":      f.OutputHash,
	}
	if f.CaseID != nil {
		result["case_id"] = *f.CaseID
	}
	if f.Model != nil {
		result["model"] = f.Model
	}
	return result
}

func writeAIError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrEvidenceNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "evidence not found")
	case errors.Is(err, application.ErrObservationNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "observation not found")
	case errors.Is(err, application.ErrAIProviderUnavailable):
		shared.WriteError(w, http.StatusServiceUnavailable, shared.CodeInternalError, "AI provider unavailable")
	case errors.Is(err, application.ErrInvalidInput):
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid input")
	default:
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "internal server error")
	}
}
