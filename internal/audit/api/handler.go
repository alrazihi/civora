package api

import (
	"net/http"
	"strconv"

	auditapp "github.com/alrazihi/civora/internal/audit/application"
	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc *auditapp.AuditService
}

func NewHandler(svc *auditapp.AuditService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/organizations/{orgId}/audit", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.RequireSameTenant)
		r.Get("/", h.ListEvents)
	})
}

func (h *Handler) ListEvents(w http.ResponseWriter, r *http.Request) {
	orgIDStr := chi.URLParam(r, "orgId")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
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

	events, err := h.svc.FindByOrganization(r.Context(), orgID, perPage, offset)
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "failed to list audit events")
		return
	}

	result := make([]map[string]interface{}, len(events))
	for i, ev := range events {
		result[i] = map[string]interface{}{
			"id":              ev.ID,
			"organization_id": ev.OrganizationID,
			"actor_id":        ev.ActorID,
			"action":          ev.Action,
			"resource":        ev.Resource,
			"resource_id":     ev.ResourceID,
			"outcome":         ev.Outcome,
			"request_id":      ev.RequestID,
			"timestamp":       ev.Timestamp,
			"hash":            ev.Hash,
			"previous_hash":   ev.PreviousHash,
		}
	}

	shared.WriteSuccess(w, http.StatusOK, result, &shared.PaginationMeta{
		Page:    page,
		PerPage: perPage,
		Total:   len(result),
	})
}
