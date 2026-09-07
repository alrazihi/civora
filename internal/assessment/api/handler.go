package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/alrazihi/civora/internal/assessment/application"
	"github.com/alrazihi/civora/internal/assessment/domain"
	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc AssessmentService
}

func NewHandler(svc AssessmentService) *Handler {
	return &Handler{svc: svc}
}

type AssessmentService interface {
	CreateAssessment(ctx context.Context, params application.CreateAssessmentParams) (*domain.Assessment, error)
	GetAssessment(ctx context.Context, orgID, id uuid.UUID) (*domain.Assessment, error)
	GetAssessmentByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) (*domain.Assessment, error)
	ListAssessments(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Assessment, int, error)
}

func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/organizations/{orgId}/assessments", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.RequireSameTenant)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAnyRole("admin", "staff"))
			r.Post("/", h.CreateAssessment)
			r.Get("/", h.ListAssessments)
		})
		r.Get("/{assessmentId}", h.GetAssessment)
		r.Get("/service-request/{serviceRequestId}", h.GetByServiceRequest)
	})
}

func (h *Handler) CreateAssessment(w http.ResponseWriter, r *http.Request) {
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
		Findings         string `json:"findings"`
		NeedsIdentified  string `json:"needs_identified"`
		Recommendation   string `json:"recommendation"`
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

	a, err := h.svc.CreateAssessment(r.Context(), application.CreateAssessmentParams{
		OrganizationID:   orgID,
		ServiceRequestID: serviceRequestID,
		Findings:         req.Findings,
		NeedsIdentified:  req.NeedsIdentified,
		Recommendation:   req.Recommendation,
		ActorID:          actorID,
	})
	if err != nil {
		writeAssessmentError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusCreated, serializeAssessment(a), nil)
}

func (h *Handler) GetAssessment(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	assessmentID, ok := parseUUID(r, "assessmentId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid assessment ID")
		return
	}

	a, err := h.svc.GetAssessment(r.Context(), orgID, assessmentID)
	if err != nil {
		writeAssessmentError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeAssessment(a), nil)
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

	a, err := h.svc.GetAssessmentByServiceRequest(r.Context(), orgID, serviceRequestID)
	if err != nil {
		writeAssessmentError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeAssessment(a), nil)
}

func (h *Handler) ListAssessments(w http.ResponseWriter, r *http.Request) {
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

	items, total, err := h.svc.ListAssessments(r.Context(), orgID, perPage, offset)
	if err != nil {
		writeAssessmentError(w, err)
		return
	}

	result := make([]map[string]interface{}, len(items))
	for i, a := range items {
		result[i] = serializeAssessment(a)
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

func serializeAssessment(a *domain.Assessment) map[string]interface{} {
	return map[string]interface{}{
		"id":                 a.ID,
		"organization_id":    a.OrganizationID,
		"service_request_id": a.ServiceRequestID,
		"findings":           a.Findings,
		"needs_identified":   a.NeedsIdentified,
		"recommendation":     a.Recommendation,
		"assessor":           a.Assessor,
		"assessed_at":        a.AssessedAt,
		"created_at":         a.CreatedAt,
	}
}

func writeAssessmentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrAssessmentNotFound), errors.Is(err, domain.ErrAssessmentNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "assessment not found")
	case errors.Is(err, application.ErrAssessmentInput), errors.Is(err, domain.ErrAssessmentInvalidInput):
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid input")
	default:
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "internal server error")
	}
}
