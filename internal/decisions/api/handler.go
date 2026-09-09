package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/alrazihi/civora/internal/decisions/application"
	"github.com/alrazihi/civora/internal/decisions/domain"
	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc DecisionService
}

func NewHandler(svc DecisionService) *Handler {
	return &Handler{svc: svc}
}

type DecisionService interface {
	MakeDecision(ctx context.Context, params application.MakeDecisionParams) (*domain.Decision, error)
	GetDecision(ctx context.Context, orgID, id uuid.UUID) (*domain.Decision, error)
	GetDecisionByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) (*domain.Decision, error)
	ListDecisions(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Decision, int, error)
}

func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/organizations/{orgId}/decisions", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.RequireSameTenant)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAnyRole("admin", "staff"))
			r.Post("/", h.MakeDecision)
			r.Get("/", h.ListDecisions)
		})
		r.Get("/{decisionId}", h.GetDecision)
		r.Get("/by-service-request/{serviceRequestId}", h.GetByServiceRequest)
	})
}

func (h *Handler) MakeDecision(w http.ResponseWriter, r *http.Request) {
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
		Decision         domain.DecisionType `json:"decision"`
		Reason           string              `json:"reason"`
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

	d, err := h.svc.MakeDecision(r.Context(), application.MakeDecisionParams{
		OrganizationID:   orgID,
		ServiceRequestID: serviceRequestID,
		Decision:         req.Decision,
		Reason:           req.Reason,
		ActorID:          actorID,
	})
	if err != nil {
		writeDecisionError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusCreated, serializeDecision(d), nil)
}

func (h *Handler) GetDecision(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	decisionID, ok := parseUUID(r, "decisionId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid decision ID")
		return
	}

	d, err := h.svc.GetDecision(r.Context(), orgID, decisionID)
	if err != nil {
		writeDecisionError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeDecision(d), nil)
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

	d, err := h.svc.GetDecisionByServiceRequest(r.Context(), orgID, serviceRequestID)
	if err != nil {
		if errors.Is(err, application.ErrDecisionNotFound) || errors.Is(err, domain.ErrDecisionNotFound) {
			shared.WriteSuccess(w, http.StatusOK, nil, nil)
			return
		}
		writeDecisionError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeDecision(d), nil)
}

func (h *Handler) ListDecisions(w http.ResponseWriter, r *http.Request) {
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

	items, total, err := h.svc.ListDecisions(r.Context(), orgID, perPage, offset)
	if err != nil {
		writeDecisionError(w, err)
		return
	}

	result := make([]map[string]interface{}, len(items))
	for i, d := range items {
		result[i] = serializeDecision(d)
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

func serializeDecision(d *domain.Decision) map[string]interface{} {
	return map[string]interface{}{
		"id":                 d.ID,
		"organization_id":    d.OrganizationID,
		"service_request_id": d.ServiceRequestID,
		"decision":           d.Decision,
		"reason":             d.Reason,
		"decision_maker":     d.DecisionMaker,
		"decided_at":         d.DecidedAt,
		"created_at":         d.CreatedAt,
	}
}

func writeDecisionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrDecisionNotFound), errors.Is(err, domain.ErrDecisionNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "decision not found")
	case errors.Is(err, application.ErrDecisionInput), errors.Is(err, domain.ErrDecisionInvalidInput):
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid input")
	default:
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "internal server error")
	}
}
