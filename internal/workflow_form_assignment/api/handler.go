package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/alrazihi/civora/internal/workflow_form_assignment/application"
	"github.com/alrazihi/civora/internal/workflow_form_assignment/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc *application.WorkflowStateFormAssignmentService
}

func NewHandler(svc *application.WorkflowStateFormAssignmentService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/organizations/{orgId}/workflows/{workflowId}/form-assignments", func(r chi.Router) {
		r.Use(authMiddleware, middleware.RequireSameTenant)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAnyRole("admin", "staff"))
			r.Get("/", h.ListAssignments)
			r.Get("/state/{stateKey}", h.GetAssignmentsForState)
			r.Get("/{assignmentId}", h.GetAssignment)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAnyRole("admin"))
			r.Post("/", h.CreateAssignment)
			r.Put("/{assignmentId}", h.UpdateAssignment)
			r.Delete("/{assignmentId}", h.DeleteAssignment)
		})
	})
}

func (h *Handler) CreateAssignment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, err := getOrgID(r)
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}
	workflowID, err := uuid.Parse(chi.URLParam(r, "workflowId"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid workflow ID")
		return
	}
	actorID, err := uuid.Parse(middleware.GetUserID(r))
	if err != nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "invalid user ID")
		return
	}

	var req CreateAssignmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	assignment, err := h.svc.CreateAssignment(ctx, application.CreateAssignmentParams{
		TenantID:             orgID,
		WorkflowDefinitionID: workflowID,
		WorkflowStateKey:     req.WorkflowStateKey,
		FormID:               req.FormID,
		FormVersionID:        req.FormVersionID,
		Required:             req.Required,
		DisplayOrder:         req.DisplayOrder,
		Active:               req.Active,
		CreatedBy:            actorID,
	})
	if err != nil {
		writeAssignmentError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusCreated, serializeAssignment(assignment), nil)
}

func (h *Handler) GetAssignment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, err := getOrgID(r)
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}
	assignmentID, err := uuid.Parse(chi.URLParam(r, "assignmentId"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid assignment ID")
		return
	}

	assignment, err := h.svc.GetAssignment(ctx, orgID, assignmentID)
	if err != nil {
		writeAssignmentError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeAssignment(assignment), nil)
}

func (h *Handler) ListAssignments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, err := getOrgID(r)
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}
	workflowID, err := uuid.Parse(chi.URLParam(r, "workflowId"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid workflow ID")
		return
	}

	assignments, err := h.svc.ListAssignmentsByWorkflow(ctx, orgID, workflowID)
	if err != nil {
		writeAssignmentError(w, err)
		return
	}

	data := make([]map[string]interface{}, len(assignments))
	for i, a := range assignments {
		data[i] = serializeAssignment(a)
	}
	shared.WriteSuccess(w, http.StatusOK, data, nil)
}

func (h *Handler) GetAssignmentsForState(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, err := getOrgID(r)
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}
	workflowID, err := uuid.Parse(chi.URLParam(r, "workflowId"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid workflow ID")
		return
	}
	stateKey := chi.URLParam(r, "stateKey")

	assignments, err := h.svc.GetAssignmentsForState(ctx, orgID, workflowID, stateKey)
	if err != nil {
		writeAssignmentError(w, err)
		return
	}

	data := make([]map[string]interface{}, len(assignments))
	for i, a := range assignments {
		data[i] = serializeAssignment(a)
	}
	shared.WriteSuccess(w, http.StatusOK, data, nil)
}

func (h *Handler) UpdateAssignment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, err := getOrgID(r)
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}
	assignmentID, err := uuid.Parse(chi.URLParam(r, "assignmentId"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid assignment ID")
		return
	}
	actorID, err := uuid.Parse(middleware.GetUserID(r))
	if err != nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "invalid user ID")
		return
	}

	var req UpdateAssignmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	assignment, err := h.svc.UpdateAssignment(ctx, application.UpdateAssignmentParams{
		TenantID:     orgID,
		AssignmentID: assignmentID,
		Required:     req.Required,
		DisplayOrder: req.DisplayOrder,
		Active:       req.Active,
		ActorID:      actorID,
	})
	if err != nil {
		writeAssignmentError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeAssignment(assignment), nil)
}

func (h *Handler) DeleteAssignment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, err := getOrgID(r)
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}
	assignmentID, err := uuid.Parse(chi.URLParam(r, "assignmentId"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid assignment ID")
		return
	}
	actorID, err := uuid.Parse(middleware.GetUserID(r))
	if err != nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "invalid user ID")
		return
	}

	if err := h.svc.DeleteAssignment(ctx, orgID, assignmentID, actorID); err != nil {
		writeAssignmentError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type CreateAssignmentRequest struct {
	WorkflowStateKey string    `json:"workflow_state_key"`
	FormID           uuid.UUID `json:"form_id"`
	FormVersionID    uuid.UUID `json:"form_version_id"`
	Required         bool      `json:"required"`
	DisplayOrder     int       `json:"display_order"`
	Active           bool      `json:"active"`
}

type UpdateAssignmentRequest struct {
	Required     *bool `json:"required"`
	DisplayOrder *int  `json:"display_order"`
	Active       *bool `json:"active"`
}

func serializeAssignment(a *domain.WorkflowStateFormAssignment) map[string]interface{} {
	return map[string]interface{}{
		"id":                     a.ID.String(),
		"tenant_id":              a.TenantID.String(),
		"workflow_definition_id": a.WorkflowDefinitionID.String(),
		"workflow_state_key":     a.WorkflowStateKey,
		"form_id":                a.FormID.String(),
		"form_version_id":        a.FormVersionID.String(),
		"required":               a.Required,
		"display_order":          a.DisplayOrder,
		"active":                 a.Active,
		"created_by":             a.CreatedBy.String(),
		"created_at":             a.CreatedAt,
		"updated_at":             a.UpdatedAt,
	}
}

func writeAssignmentError(w http.ResponseWriter, err error) {
	switch {
	case err == domain.ErrAssignmentNotFound:
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "assignment not found")
	case errors.Is(err, application.ErrWorkflowNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "workflow definition not found")
	case err == application.ErrAssignmentAlreadyExists:
		shared.WriteError(w, http.StatusConflict, shared.CodeConflict, "assignment already exists for this state and form version")
	case err == application.ErrInvalidWorkflowState:
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid workflow state key")
	case err == application.ErrInvalidFormVersion:
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid form version (not found or not published)")
	case err == application.ErrFormArchived:
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "form is archived and cannot be assigned")
	case err == application.ErrDuplicateDisplayOrder:
		shared.WriteError(w, http.StatusConflict, shared.CodeConflict, "display order already in use for this state")
	case errors.Is(err, domain.ErrTenantMismatch):
		shared.WriteError(w, http.StatusForbidden, shared.CodeForbidden, "tenant mismatch")
	default:
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "internal server error")
	}
}

func getOrgID(r *http.Request) (uuid.UUID, error) {
	orgIDStr := chi.URLParam(r, "orgId")
	return uuid.Parse(orgIDStr)
}
