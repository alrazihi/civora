package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

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
}

func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/organizations/{orgId}/evidence", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.RequireSameTenant)
		r.Post("/", h.AddEvidence)
		r.Get("/service-request/{serviceRequestId}", h.ListEvidence)
		r.Get("/{evidenceId}", h.GetEvidence)
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
		ServiceRequestID string              `json:"service_request_id"`
		Type             domain.EvidenceType `json:"type"`
		Description      string              `json:"description"`
		StorageReference string              `json:"storage_reference"`
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

	e, err := h.svc.AddEvidence(r.Context(), application.AddEvidenceParams{
		OrganizationID:   orgID,
		ServiceRequestID: serviceRequestID,
		Type:             req.Type,
		Description:      req.Description,
		StorageReference: req.StorageReference,
		ActorID:          actorID,
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

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
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
	return map[string]interface{}{
		"id":                 e.ID,
		"organization_id":    e.OrganizationID,
		"service_request_id": e.ServiceRequestID,
		"type":               e.Type,
		"description":        e.Description,
		"storage_reference":  e.StorageReference,
		"uploaded_by":        e.UploadedBy,
		"created_at":         e.CreatedAt,
	}
}

func writeEvidenceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrEvidenceNotFound), errors.Is(err, domain.ErrEvidenceNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "evidence not found")
	case errors.Is(err, application.ErrEvidenceInput), errors.Is(err, domain.ErrEvidenceInvalidInput):
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid input")
	default:
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "internal server error")
	}
}
