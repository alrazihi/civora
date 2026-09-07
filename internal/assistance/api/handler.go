package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/alrazihi/civora/internal/assistance/application"
	"github.com/alrazihi/civora/internal/assistance/domain"
	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc AssistanceService
}

func NewHandler(svc AssistanceService) *Handler {
	return &Handler{svc: svc}
}

type AssistanceService interface {
	CreateAssistance(ctx context.Context, params application.CreateAssistanceParams) (*domain.Assistance, error)
	GetAssistance(ctx context.Context, orgID, id uuid.UUID) (*domain.Assistance, error)
	ListAssistances(ctx context.Context, orgID, serviceRequestID uuid.UUID, limit, offset int) ([]*domain.Assistance, int, error)
	UpdateAssistanceStatus(ctx context.Context, orgID, id uuid.UUID, action string, actorID uuid.UUID) (*domain.Assistance, error)
}

func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/organizations/{orgId}/assistance", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.RequireSameTenant)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAnyRole("admin", "staff"))
			r.Post("/", h.CreateAssistance)
			r.Get("/service-request/{serviceRequestId}", h.ListAssistances)
		})
		r.Get("/{assistanceId}", h.GetAssistance)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAnyRole("admin", "staff"))
			r.Patch("/{assistanceId}/status", h.UpdateAssistanceStatus)
		})
	})
}

func (h *Handler) CreateAssistance(w http.ResponseWriter, r *http.Request) {
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
		ServiceRequestID string                `json:"service_request_id"`
		Type             domain.AssistanceType `json:"type"`
		Description      string                `json:"description"`
		ResponsibleStaff string                `json:"responsible_staff"`
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

	responsibleStaff, err := uuid.Parse(req.ResponsibleStaff)
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid responsible staff ID")
		return
	}

	a, err := h.svc.CreateAssistance(r.Context(), application.CreateAssistanceParams{
		OrganizationID:   orgID,
		ServiceRequestID: serviceRequestID,
		Type:             req.Type,
		Description:      req.Description,
		ResponsibleStaff: responsibleStaff,
		ActorID:          actorID,
	})
	if err != nil {
		writeAssistanceError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusCreated, serializeAssistance(a), nil)
}

func (h *Handler) GetAssistance(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	assistanceID, ok := parseUUID(r, "assistanceId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid assistance ID")
		return
	}

	a, err := h.svc.GetAssistance(r.Context(), orgID, assistanceID)
	if err != nil {
		writeAssistanceError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeAssistance(a), nil)
}

func (h *Handler) ListAssistances(w http.ResponseWriter, r *http.Request) {
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

	items, total, err := h.svc.ListAssistances(r.Context(), orgID, serviceRequestID, perPage, offset)
	if err != nil {
		writeAssistanceError(w, err)
		return
	}

	result := make([]map[string]interface{}, len(items))
	for i, a := range items {
		result[i] = serializeAssistance(a)
	}

	shared.WritePaginatedSuccess(w, http.StatusOK, result, page, perPage, total)
}

func (h *Handler) UpdateAssistanceStatus(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	assistanceID, ok := parseUUID(r, "assistanceId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid assistance ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	a, err := h.svc.UpdateAssistanceStatus(r.Context(), orgID, assistanceID, req.Action, actorID)
	if err != nil {
		writeAssistanceError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeAssistance(a), nil)
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

func serializeAssistance(a *domain.Assistance) map[string]interface{} {
	result := map[string]interface{}{
		"id":                 a.ID,
		"organization_id":    a.OrganizationID,
		"service_request_id": a.ServiceRequestID,
		"type":               a.Type,
		"description":        a.Description,
		"status":             a.Status,
		"responsible_staff":  a.ResponsibleStaff,
		"created_at":         a.CreatedAt,
		"updated_at":         a.UpdatedAt,
	}
	if a.StartedAt != nil {
		result["started_at"] = a.StartedAt
	}
	if a.CompletedAt != nil {
		result["completed_at"] = a.CompletedAt
	}
	return result
}

func writeAssistanceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrAssistanceNotFound), errors.Is(err, domain.ErrAssistanceNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "assistance not found")
	case errors.Is(err, application.ErrAssistanceInput), errors.Is(err, domain.ErrAssistanceInvalidInput):
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid input")
	default:
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "internal server error")
	}
}
