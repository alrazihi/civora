package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/alrazihi/civora/internal/cases/application"
	"github.com/alrazihi/civora/internal/cases/domain"
	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc CaseService
}

func NewHandler(svc CaseService) *Handler {
	return &Handler{svc: svc}
}

type CaseService interface {
	CreateCase(ctx context.Context, params application.CreateCaseParams) (*domain.Case, error)
	ListCases(ctx context.Context, orgID uuid.UUID, limit, offset int, filter domain.CaseFilter) ([]*domain.Case, int, error)
	GetCase(ctx context.Context, orgID, id uuid.UUID) (*domain.Case, error)
	ChangeStatus(ctx context.Context, params application.ChangeCaseStatusParams) (*domain.Case, error)
	AssignCase(ctx context.Context, params application.AssignCaseParams) (*domain.Case, error)
}

func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/organizations/{orgId}/cases", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.RequireSameTenant)
		r.Post("/", h.CreateCase)
		r.Get("/", h.ListCases)
		r.Get("/{caseId}", h.GetCase)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAnyRole("admin", "staff"))
			r.Post("/{caseId}/transitions", h.ChangeCaseStatus)
			r.Post("/{caseId}/assign", h.AssignCase)
		})
	})
}

func (h *Handler) CreateCase(w http.ResponseWriter, r *http.Request) {
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
		Title       string  `json:"title"`
		Description string  `json:"description"`
		ServiceType string  `json:"service_type"`
		Priority    string  `json:"priority"`
		PersonID    *string `json:"person_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	if req.ServiceType == "" {
		req.ServiceType = "GENERAL"
	}
	if req.Priority == "" {
		req.Priority = "NORMAL"
	}

	var personID *uuid.UUID
	if req.PersonID != nil && *req.PersonID != "" {
		pid, err := uuid.Parse(*req.PersonID)
		if err != nil {
			shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid person ID")
			return
		}
		personID = &pid
	}

	c, err := h.svc.CreateCase(r.Context(), application.CreateCaseParams{
		OrganizationID: orgID,
		Title:          req.Title,
		Description:    req.Description,
		ServiceType:    domain.ServiceType(req.ServiceType),
		Priority:       domain.Priority(req.Priority),
		PersonID:       personID,
		CreatedByID:    actorID,
	})
	if err != nil {
		writeCaseError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusCreated, serializeCase(c), nil)
}

func (h *Handler) GetCase(w http.ResponseWriter, r *http.Request) {
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

	c, err := h.svc.GetCase(r.Context(), orgID, caseID)
	if err != nil {
		writeCaseError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeCase(c), nil)
}

func (h *Handler) ListCases(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
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

	filter := domain.CaseFilter{}
	if statusStr := r.URL.Query().Get("status"); statusStr != "" {
		filter.Status = domain.CaseStatus(statusStr)
	}
	if personIDStr := r.URL.Query().Get("person_id"); personIDStr != "" {
		if pid, err := uuid.Parse(personIDStr); err == nil {
			filter.PersonID = &pid
		}
	}

	cases, total, err := h.svc.ListCases(r.Context(), orgID, perPage, offset, filter)
	if err != nil {
		writeCaseError(w, err)
		return
	}

	result := make([]map[string]interface{}, len(cases))
	for i, c := range cases {
		result[i] = serializeCase(c)
	}

	shared.WritePaginatedSuccess(w, http.StatusOK, result, page, perPage, total)
}

func (h *Handler) ChangeCaseStatus(w http.ResponseWriter, r *http.Request) {
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
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	c, err := h.svc.ChangeStatus(r.Context(), application.ChangeCaseStatusParams{
		OrganizationID: orgID,
		CaseID:         caseID,
		Status:         domain.CaseStatus(req.Status),
		ActorID:        actorID,
	})
	if err != nil {
		writeCaseError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeCase(c), nil)
}

func (h *Handler) AssignCase(w http.ResponseWriter, r *http.Request) {
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
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid user ID")
		return
	}

	c, err := h.svc.AssignCase(r.Context(), application.AssignCaseParams{
		OrganizationID: orgID,
		CaseID:         caseID,
		UserID:         userID,
		ActorID:        actorID,
	})
	if err != nil {
		writeCaseError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeCase(c), nil)
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

func serializeCase(c *domain.Case) map[string]interface{} {
	result := map[string]interface{}{
		"id":              c.ID,
		"organization_id": c.OrganizationID,
		"case_number":     c.CaseNumber,
		"title":           c.Title,
		"description":     c.Description,
		"status":          c.Status,
		"service_type":    c.ServiceType,
		"priority":        c.Priority,
		"created_by":      c.CreatedByID,
		"assigned_to":     c.AssignedToID,
		"created_at":      c.CreatedAt,
		"updated_at":      c.UpdatedAt,
		"closed_at":       c.ClosedAt,
	}
	if c.PersonID != nil {
		result["person_id"] = c.PersonID
	} else {
		result["person_id"] = nil
	}
	return result
}

func writeCaseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrCaseNotFound), errors.Is(err, domain.ErrCaseNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "case not found")
	case errors.Is(err, application.ErrCaseInvalidInput), errors.Is(err, domain.ErrCaseInvalidInput):
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid input")
	case errors.Is(err, application.ErrCaseTransition), errors.Is(err, domain.ErrInvalidStateTransition):
		shared.WriteError(w, http.StatusConflict, shared.CodeStateTransition, "invalid state transition")
	default:
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "internal server error")
	}
}
