package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/alrazihi/civora/internal/followup/application"
	"github.com/alrazihi/civora/internal/followup/domain"
	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc FollowUpService
}

func NewHandler(svc FollowUpService) *Handler {
	return &Handler{svc: svc}
}

type FollowUpService interface {
	CreateFollowUp(ctx context.Context, params application.CreateFollowUpParams) (*domain.FollowUp, error)
	CompleteFollowUp(ctx context.Context, orgID, id uuid.UUID, completedDate time.Time, actorID uuid.UUID) (*domain.FollowUp, error)
	GetFollowUp(ctx context.Context, orgID, id uuid.UUID) (*domain.FollowUp, error)
	ListFollowUps(ctx context.Context, orgID, serviceRequestID uuid.UUID, limit, offset int) ([]*domain.FollowUp, int, error)
}

func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/organizations/{orgId}/follow-ups", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.RequireSameTenant)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAnyRole("admin", "staff"))
			r.Post("/", h.CreateFollowUp)
			r.Get("/service-request/{serviceRequestId}", h.ListFollowUps)
		})
		r.Get("/{followUpId}", h.GetFollowUp)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAnyRole("admin", "staff"))
			r.Patch("/{followUpId}/complete", h.CompleteFollowUp)
		})
	})
}

func (h *Handler) CreateFollowUp(w http.ResponseWriter, r *http.Request) {
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
		ServiceRequestID string `json:"service_request_id"`
		ScheduledDate    string `json:"scheduled_date"`
		Outcome          string `json:"outcome"`
		Notes            string `json:"notes"`
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

	scheduledDate, err := time.Parse("2006-01-02", req.ScheduledDate)
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid scheduled_date format, use YYYY-MM-DD")
		return
	}

	f, err := h.svc.CreateFollowUp(r.Context(), application.CreateFollowUpParams{
		OrganizationID:   orgID,
		ServiceRequestID: serviceRequestID,
		ScheduledDate:    scheduledDate,
		Outcome:          req.Outcome,
		Notes:            req.Notes,
		ActorID:          actorID,
	})
	if err != nil {
		writeFollowUpError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusCreated, serializeFollowUp(f), nil)
}

func (h *Handler) CompleteFollowUp(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	followUpID, ok := parseUUID(r, "followUpId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid follow-up ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		CompletedDate string `json:"completed_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	completedDate, err := time.Parse("2006-01-02", req.CompletedDate)
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid completed_date format, use YYYY-MM-DD")
		return
	}

	f, err := h.svc.CompleteFollowUp(r.Context(), orgID, followUpID, completedDate, actorID)
	if err != nil {
		writeFollowUpError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeFollowUp(f), nil)
}

func (h *Handler) GetFollowUp(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	followUpID, ok := parseUUID(r, "followUpId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid follow-up ID")
		return
	}

	f, err := h.svc.GetFollowUp(r.Context(), orgID, followUpID)
	if err != nil {
		writeFollowUpError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeFollowUp(f), nil)
}

func (h *Handler) ListFollowUps(w http.ResponseWriter, r *http.Request) {
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

	items, total, err := h.svc.ListFollowUps(r.Context(), orgID, serviceRequestID, perPage, offset)
	if err != nil {
		writeFollowUpError(w, err)
		return
	}

	result := make([]map[string]interface{}, len(items))
	for i, f := range items {
		result[i] = serializeFollowUp(f)
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

func serializeFollowUp(f *domain.FollowUp) map[string]interface{} {
	result := map[string]interface{}{
		"id":                 f.ID,
		"organization_id":    f.OrganizationID,
		"service_request_id": f.ServiceRequestID,
		"scheduled_date":     f.ScheduledDate,
		"outcome":            f.Outcome,
		"notes":              f.Notes,
		"performed_by":       f.PerformedBy,
		"created_at":         f.CreatedAt,
		"updated_at":         f.UpdatedAt,
	}
	if f.CompletedDate != nil {
		result["completed_date"] = f.CompletedDate
	}
	return result
}

func writeFollowUpError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrFollowUpNotFound), errors.Is(err, domain.ErrFollowUpNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "follow-up not found")
	case errors.Is(err, application.ErrFollowUpInput), errors.Is(err, domain.ErrFollowUpInvalidInput):
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid input")
	default:
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "internal server error")
	}
}
