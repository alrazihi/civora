package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/organizations/application"
	"github.com/alrazihi/civora/internal/organizations/domain"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type OrganizationService interface {
	CreateOrganization(ctx context.Context, params application.CreateOrganizationParams) (*domain.Organization, error)
	GetOrganization(ctx context.Context, id uuid.UUID) (*domain.Organization, error)
	GetBySlug(ctx context.Context, slug string) (*domain.Organization, error)
}

type Handler struct {
	svc OrganizationService
}

func NewHandler(svc OrganizationService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/organizations", h.CreateOrganization)
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware)
			r.Use(middleware.RequireSameTenant)
			r.Get("/organizations/{orgId}", h.GetOrganization)
		})
	})
}

func (h *Handler) CreateOrganization(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Slug        string `json:"slug"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	org, err := h.svc.CreateOrganization(r.Context(), application.CreateOrganizationParams{
		Name:        req.Name,
		Description: req.Description,
		Slug:        req.Slug,
	})
	if err != nil {
		writeOrgError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusCreated, map[string]interface{}{
		"id":          org.ID,
		"name":        org.Name,
		"description": org.Description,
		"slug":        org.Slug,
		"created_at":  org.CreatedAt,
	}, nil)
}

func (h *Handler) GetOrganization(w http.ResponseWriter, r *http.Request) {
	orgIDStr := chi.URLParam(r, "orgId")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	org, err := h.svc.GetOrganization(r.Context(), orgID)
	if err != nil {
		writeOrgError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, map[string]interface{}{
		"id":          org.ID,
		"name":        org.Name,
		"description": org.Description,
		"slug":        org.Slug,
		"created_at":  org.CreatedAt,
		"updated_at":  org.UpdatedAt,
	}, nil)
}

func writeOrgError(w http.ResponseWriter, err error) {
	switch err {
	case application.ErrOrgNotFound:
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "organization not found")
	case application.ErrOrgInvalidInput:
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid input")
	case application.ErrOrgSlugTaken:
		shared.WriteError(w, http.StatusConflict, shared.CodeConflict, "organization slug already taken")
	default:
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "internal server error")
	}
}
