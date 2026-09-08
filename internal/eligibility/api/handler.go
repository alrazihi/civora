package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/alrazihi/civora/internal/eligibility/application"
	"github.com/alrazihi/civora/internal/eligibility/domain"
	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc EligibilityService
}

func NewHandler(svc EligibilityService) *Handler {
	return &Handler{svc: svc}
}

type EligibilityService interface {
	CreateEligibility(ctx context.Context, params application.CreateEligibilityParams) (*domain.Eligibility, error)
	GetEligibility(ctx context.Context, orgID, id uuid.UUID) (*domain.Eligibility, error)
	GetEligibilityByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) (*domain.Eligibility, error)
	ListEligibilities(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Eligibility, int, error)
	UpdateEligibilityResult(ctx context.Context, orgID, id uuid.UUID, result domain.EligibilityResult, actorID uuid.UUID) (*domain.Eligibility, error)
}

func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/organizations/{orgId}/eligibilities", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.RequireSameTenant)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAnyRole("admin", "staff"))
			r.Post("/", h.CreateEligibility)
			r.Get("/", h.ListEligibilities)
		})
		r.Get("/{eligibilityId}", h.GetEligibility)
		r.Get("/by-service-request/{serviceRequestId}", h.GetByServiceRequest)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAnyRole("admin", "staff"))
			r.Patch("/{eligibilityId}/result", h.UpdateEligibilityResult)
		})
	})
}

func (h *Handler) CreateEligibility(w http.ResponseWriter, r *http.Request) {
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
		Criteria         map[string]interface{} `json:"criteria"`
		Explanation      string                 `json:"explanation"`
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

	e, err := h.svc.CreateEligibility(r.Context(), application.CreateEligibilityParams{
		OrganizationID:   orgID,
		ServiceRequestID: serviceRequestID,
		Criteria:         req.Criteria,
		Explanation:      req.Explanation,
		ActorID:          actorID,
	})
	if err != nil {
		writeEligibilityError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusCreated, serializeEligibility(e), nil)
}

func (h *Handler) GetEligibility(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	eligibilityID, ok := parseUUID(r, "eligibilityId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid eligibility ID")
		return
	}

	e, err := h.svc.GetEligibility(r.Context(), orgID, eligibilityID)
	if err != nil {
		writeEligibilityError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeEligibility(e), nil)
}

func (h *Handler) GetByServiceRequest(w http.ResponseWriter, r *http.Request) {
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

	e, err := h.svc.GetEligibilityByServiceRequest(r.Context(), orgID, serviceRequestID)
	if err != nil {
		writeEligibilityError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeEligibility(e), nil)
}

func (h *Handler) ListEligibilities(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
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

	items, total, err := h.svc.ListEligibilities(r.Context(), orgID, perPage, offset)
	if err != nil {
		writeEligibilityError(w, err)
		return
	}

	result := make([]map[string]interface{}, len(items))
	for i, e := range items {
		result[i] = serializeEligibility(e)
	}

	shared.WritePaginatedSuccess(w, http.StatusOK, result, page, perPage, total)
}

func (h *Handler) UpdateEligibilityResult(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	eligibilityID, ok := parseUUID(r, "eligibilityId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid eligibility ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	e, err := h.svc.UpdateEligibilityResult(r.Context(), orgID, eligibilityID, domain.EligibilityResult(req.Result), actorID)
	if err != nil {
		writeEligibilityError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeEligibility(e), nil)
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

func serializeEligibility(e *domain.Eligibility) map[string]interface{} {
	return map[string]interface{}{
		"id":                 e.ID,
		"organization_id":    e.OrganizationID,
		"service_request_id": e.ServiceRequestID,
		"criteria":           e.Criteria,
		"result":             e.Result,
		"explanation":        e.Explanation,
		"assessed_by":        e.AssessedBy,
		"assessed_at":        e.AssessedAt,
		"created_at":         e.CreatedAt,
	}
}

func writeEligibilityError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrEligibilityNotFound), errors.Is(err, domain.ErrEligibilityNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "eligibility not found")
	case errors.Is(err, application.ErrEligibilityInput), errors.Is(err, domain.ErrEligibilityInvalidInput):
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid input")
	default:
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "internal server error")
	}
}
