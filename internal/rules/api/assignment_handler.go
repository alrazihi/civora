package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/rules/application"
	rulesdomain "github.com/alrazihi/civora/internal/rules/domain"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AssignmentHandler struct {
	svc AssignmentService
}

type AssignmentService interface {
	CreateRuleAssignment(ctx context.Context, params CreateRuleAssignmentParams) (*rulesdomain.WorkflowStateRuleAssignment, error)
	GetRuleAssignment(ctx context.Context, orgID, id uuid.UUID) (*rulesdomain.WorkflowStateRuleAssignment, error)
	ListRuleAssignments(ctx context.Context, orgID, workflowDefID uuid.UUID) ([]*rulesdomain.WorkflowStateRuleAssignment, error)
	UpdateRuleAssignment(ctx context.Context, params UpdateRuleAssignmentParams) (*rulesdomain.WorkflowStateRuleAssignment, error)
	DeleteRuleAssignment(ctx context.Context, orgID, id, actorID uuid.UUID) error
}

func NewAssignmentHandler(svc AssignmentService) *AssignmentHandler {
	return &AssignmentHandler{svc: svc}
}

func (h *AssignmentHandler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/organizations/{orgId}/rules/workflow-state-assignments", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.RequireSameTenant)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAnyRole("admin", "staff"))
			r.Get("/", h.ListAssignments)
			r.Get("/{assignmentId}", h.GetAssignment)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAnyRole("admin"))
			r.Post("/", h.CreateAssignment)
			r.Patch("/{assignmentId}", h.UpdateAssignment)
			r.Delete("/{assignmentId}", h.DeleteAssignment)
		})
	})
}

type CreateRuleAssignmentParams = application.CreateRuleAssignmentParams
type UpdateRuleAssignmentParams = application.UpdateRuleAssignmentParams

func (h *AssignmentHandler) CreateAssignment(w http.ResponseWriter, r *http.Request) {
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
		WorkflowDefID    *string `json:"workflow_definition_id"`
		WorkflowStateKey string  `json:"workflow_state_key"`
		RuleSetID        *string `json:"rule_set_id"`
		Required         *bool   `json:"required"`
		Active           *bool   `json:"active"`
		DisplayOrder     *int    `json:"display_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	var workflowDefID, ruleSetID uuid.UUID
	if req.WorkflowDefID == nil || *req.WorkflowDefID == "" {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "workflow_definition_id is required")
		return
	}
	workflowDefID, err := uuid.Parse(*req.WorkflowDefID)
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid workflow_definition_id")
		return
	}
	if req.RuleSetID == nil || *req.RuleSetID == "" {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "rule_set_id is required")
		return
	}
	ruleSetID, err = uuid.Parse(*req.RuleSetID)
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid rule_set_id")
		return
	}

	required := req.Required != nil && *req.Required
	active := req.Active == nil || *req.Active
	displayOrder := 0
	if req.DisplayOrder != nil {
		displayOrder = *req.DisplayOrder
	}

	assignment, err := h.svc.CreateRuleAssignment(r.Context(), application.CreateRuleAssignmentParams{
		OrganizationID:   orgID,
		WorkflowDefID:    workflowDefID,
		WorkflowStateKey: req.WorkflowStateKey,
		RuleSetID:        ruleSetID,
		Required:         required,
		Active:           active,
		DisplayOrder:     displayOrder,
		ActorID:          actorID,
	})
	if err != nil {
		writeAssignmentError(w, err)
		return
	}
	shared.WriteSuccess(w, http.StatusCreated, serializeRuleAssignment(assignment), nil)
}

func (h *AssignmentHandler) ListAssignments(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}
	workflowIDStr := r.URL.Query().Get("workflow_id")
	if workflowIDStr == "" {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "workflow_id is required")
		return
	}
	workflowDefID, err := uuid.Parse(workflowIDStr)
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid workflow_id")
		return
	}
	items, err := h.svc.ListRuleAssignments(r.Context(), orgID, workflowDefID)
	if err != nil {
		writeAssignmentError(w, err)
		return
	}
	result := make([]map[string]interface{}, len(items))
	for i, a := range items {
		result[i] = serializeRuleAssignment(a)
	}
	shared.WriteSuccess(w, http.StatusOK, result, nil)
}

func (h *AssignmentHandler) GetAssignment(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}
	id, ok := parseUUID(r, "assignmentId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid assignment ID")
		return
	}
	a, err := h.svc.GetRuleAssignment(r.Context(), orgID, id)
	if err != nil {
		writeAssignmentError(w, err)
		return
	}
	shared.WriteSuccess(w, http.StatusOK, serializeRuleAssignment(a), nil)
}

func (h *AssignmentHandler) UpdateAssignment(w http.ResponseWriter, r *http.Request) {
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
	id, ok := parseUUID(r, "assignmentId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid assignment ID")
		return
	}

	var req struct {
		RuleSetID    *string `json:"rule_set_id"`
		Required     *bool   `json:"required"`
		Active       *bool   `json:"active"`
		DisplayOrder *int    `json:"display_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	params := application.UpdateRuleAssignmentParams{
		OrganizationID: orgID,
		ID:             id,
		ActorID:        actorID,
	}
	if req.RuleSetID != nil && *req.RuleSetID != "" {
		rulesetID, err := uuid.Parse(*req.RuleSetID)
		if err != nil {
			shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid rule_set_id")
			return
		}
		params.RuleSetID = &rulesetID
	}
	if req.Required != nil {
		params.Required = req.Required
	}
	if req.Active != nil {
		params.Active = req.Active
	}
	if req.DisplayOrder != nil {
		params.DisplayOrder = req.DisplayOrder
	}

	a, err := h.svc.UpdateRuleAssignment(r.Context(), params)
	if err != nil {
		writeAssignmentError(w, err)
		return
	}
	shared.WriteSuccess(w, http.StatusOK, serializeRuleAssignment(a), nil)
}

func (h *AssignmentHandler) DeleteAssignment(w http.ResponseWriter, r *http.Request) {
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
	id, ok := parseUUID(r, "assignmentId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid assignment ID")
		return
	}
	if err := h.svc.DeleteRuleAssignment(r.Context(), orgID, id, actorID); err != nil {
		writeAssignmentError(w, err)
		return
	}
	shared.WriteSuccess(w, http.StatusOK, map[string]interface{}{"deleted": true}, nil)
}

func serializeRuleAssignment(a *rulesdomain.WorkflowStateRuleAssignment) map[string]interface{} {
	return map[string]interface{}{
		"id":                     a.ID,
		"organization_id":        a.OrganizationID,
		"workflow_definition_id": a.WorkflowDefID,
		"workflow_state_key":     a.WorkflowStateKey,
		"rule_set_id":            a.RuleSetID,
		"required":               a.Required,
		"active":                 a.Active,
		"display_order":          a.DisplayOrder,
		"created_at":             a.CreatedAt,
		"updated_at":             a.UpdatedAt,
	}
}

func writeAssignmentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrRuleAssignmentNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "rule assignment not found")
	case errors.Is(err, application.ErrRuleAssignmentConflict):
		shared.WriteError(w, http.StatusConflict, shared.CodeConflict, "rule assignment conflict")
	case errors.Is(err, application.ErrRuleSetNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "rule set not found")
	case errors.Is(err, application.ErrRuleSetInvalid):
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, err.Error())
	default:
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "internal server error")
	}
}
