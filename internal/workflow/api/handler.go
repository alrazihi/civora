package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/alrazihi/civora/internal/workflow/application"
	"github.com/alrazihi/civora/internal/workflow/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc WorkflowService
}

func NewHandler(svc WorkflowService) *Handler {
	return &Handler{svc: svc}
}

type WorkflowService interface {
	CreateWorkflowDefinition(ctx context.Context, params application.CreateWorkflowDefinitionParams) (*domain.WorkflowDefinition, error)
	ActivateWorkflowDefinition(ctx context.Context, tenantID, id, actorID uuid.UUID) error
	ArchiveWorkflowDefinition(ctx context.Context, tenantID, id, actorID uuid.UUID) error
	GetWorkflowDefinition(ctx context.Context, tenantID, id uuid.UUID) (*domain.WorkflowDefinition, error)
	ListWorkflowDefinitions(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*domain.WorkflowDefinition, int, error)
	FindLatestActiveByKey(ctx context.Context, tenantID uuid.UUID, key string) (*domain.WorkflowDefinition, error)
	CreateInstanceForCase(ctx context.Context, tenantID, caseID uuid.UUID, workflowDefKey string, actorID uuid.UUID) (*domain.WorkflowInstance, error)
	ExecuteTransition(ctx context.Context, params application.ExecuteTransitionParams) (*domain.WorkflowInstance, error)
	ExecuteTransitionInTx(ctx context.Context, tx *sql.Tx, params application.ExecuteTransitionParams) (*domain.WorkflowInstance, error)
	GetInstanceByCaseID(ctx context.Context, tenantID, caseID uuid.UUID) (*domain.WorkflowInstance, error)
	GetValidTransitions(ctx context.Context, tenantID, instanceID uuid.UUID) ([]domain.WorkflowTransition, error)
	GetWorkflowHistoryByCaseID(ctx context.Context, tenantID, caseID uuid.UUID) ([]domain.WorkflowTransitionHistory, error)
}

func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/organizations/{orgId}/workflows", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.RequireSameTenant)
		r.Get("/", h.ListWorkflowDefinitions)
		r.Get("/{workflowId}", h.GetWorkflowDefinition)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAnyRole("admin"))
			r.Post("/", h.CreateWorkflowDefinition)
			r.Post("/{workflowId}/activate", h.ActivateWorkflowDefinition)
			r.Post("/{workflowId}/archive", h.ArchiveWorkflowDefinition)
		})
	})

	r.Route("/api/v1/organizations/{orgId}/cases/{caseId}/workflow", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.RequireSameTenant)
		r.Get("/", h.GetCaseWorkflow)
		r.Get("/transitions", h.GetValidTransitions)
		r.Post("/transitions/{transitionKey}", h.ExecuteTransition)
		r.Get("/history", h.GetWorkflowHistory)
	})
}

func (h *Handler) CreateWorkflowDefinition(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	var req struct {
		Key          string                      `json:"key"`
		Name         string                      `json:"name"`
		Description  string                      `json:"description"`
		Version      int                         `json:"version"`
		InitialState string                      `json:"initial_state"`
		States       []domain.WorkflowState      `json:"states"`
		Transitions  []domain.WorkflowTransition `json:"transitions"`
		Metadata     map[string]interface{}      `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	if req.Version < 1 {
		req.Version = 1
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	def, err := h.svc.CreateWorkflowDefinition(r.Context(), application.CreateWorkflowDefinitionParams{
		TenantID:     orgID,
		ActorID:      actorID,
		Key:          req.Key,
		Name:         req.Name,
		Description:  req.Description,
		Version:      req.Version,
		InitialState: req.InitialState,
		States:       req.States,
		Transitions:  req.Transitions,
		Metadata:     req.Metadata,
	})
	if err != nil {
		writeWorkflowError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusCreated, serializeWorkflowDefinition(def), nil)
}

func (h *Handler) GetWorkflowDefinition(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}
	workflowID, ok := parseUUID(r, "workflowId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid workflow ID")
		return
	}

	def, err := h.svc.GetWorkflowDefinition(r.Context(), orgID, workflowID)
	if err != nil {
		writeWorkflowError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeWorkflowDefinition(def), nil)
}

func (h *Handler) ListWorkflowDefinitions(w http.ResponseWriter, r *http.Request) {
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

	definitions, total, err := h.svc.ListWorkflowDefinitions(r.Context(), orgID, perPage, offset)
	if err != nil {
		writeWorkflowError(w, err)
		return
	}

	result := make([]map[string]interface{}, len(definitions))
	for i, def := range definitions {
		result[i] = serializeWorkflowDefinition(def)
	}
	shared.WritePaginatedSuccess(w, http.StatusOK, result, page, perPage, total)
}

func (h *Handler) ActivateWorkflowDefinition(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}
	workflowID, ok := parseUUID(r, "workflowId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid workflow ID")
		return
	}

	actorID := getUserID(r)

	if err := h.svc.ActivateWorkflowDefinition(r.Context(), orgID, workflowID, actorID); err != nil {
		writeWorkflowError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, map[string]interface{}{"status": "activated"}, nil)
}

func (h *Handler) ArchiveWorkflowDefinition(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}
	workflowID, ok := parseUUID(r, "workflowId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid workflow ID")
		return
	}

	actorID := getUserID(r)

	if err := h.svc.ArchiveWorkflowDefinition(r.Context(), orgID, workflowID, actorID); err != nil {
		writeWorkflowError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, map[string]interface{}{"status": "archived"}, nil)
}

func (h *Handler) GetCaseWorkflow(w http.ResponseWriter, r *http.Request) {
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

	instance, err := h.svc.GetInstanceByCaseID(r.Context(), orgID, caseID)
	if err != nil {
		writeWorkflowError(w, err)
		return
	}

	def, err := h.svc.GetWorkflowDefinition(r.Context(), orgID, instance.WorkflowDefID)
	if err != nil {
		writeWorkflowError(w, err)
		return
	}

	result := map[string]interface{}{
		"instance":   serializeWorkflowInstance(instance),
		"definition": serializeWorkflowDefinition(def),
	}

	shared.WriteSuccess(w, http.StatusOK, result, nil)
}

func (h *Handler) GetValidTransitions(w http.ResponseWriter, r *http.Request) {
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

	instance, err := h.svc.GetInstanceByCaseID(r.Context(), orgID, caseID)
	if err != nil {
		writeWorkflowError(w, err)
		return
	}

	transitions, err := h.svc.GetValidTransitions(r.Context(), orgID, instance.ID)
	if err != nil {
		writeWorkflowError(w, err)
		return
	}

	result := make([]map[string]interface{}, len(transitions))
	for i, t := range transitions {
		result[i] = map[string]interface{}{
			"key":        t.Key,
			"name":       t.Name,
			"from_state": t.FromState,
			"to_state":   t.ToState,
			"active":     t.Active,
		}
	}
	shared.WriteSuccess(w, http.StatusOK, result, nil)
}

func (h *Handler) ExecuteTransition(w http.ResponseWriter, r *http.Request) {
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
	transitionKey := chi.URLParam(r, "transitionKey")
	if transitionKey == "" {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "transition key is required")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}
	actorRole := middleware.GetUserRole(r)

	var req struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	instance, err := h.svc.GetInstanceByCaseID(r.Context(), orgID, caseID)
	if err != nil {
		writeWorkflowError(w, err)
		return
	}

	_, err = h.svc.ExecuteTransition(r.Context(), application.ExecuteTransitionParams{
		TenantID:      orgID,
		InstanceID:    instance.ID,
		TransitionKey: transitionKey,
		ActorID:       actorID,
		ActorRole:     actorRole,
		Reason:        req.Reason,
	})
	if err != nil {
		writeWorkflowError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, map[string]interface{}{
		"case_id":    caseID,
		"transition": transitionKey,
		"status":     "success",
	}, nil)
}

func (h *Handler) GetWorkflowHistory(w http.ResponseWriter, r *http.Request) {
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

	histories, err := h.svc.GetWorkflowHistoryByCaseID(r.Context(), orgID, caseID)
	if err != nil {
		writeWorkflowError(w, err)
		return
	}

	result := make([]map[string]interface{}, len(histories))
	for i, h := range histories {
		result[i] = map[string]interface{}{
			"id":                   h.ID,
			"workflow_instance_id": h.WorkflowInstanceID,
			"case_id":              h.CaseID,
			"from_state":           h.FromState,
			"to_state":             h.ToState,
			"transition_key":       h.TransitionKey,
			"actor_id":             h.ActorID,
			"occurred_at":          h.OccurredAt,
			"reason":               h.Reason,
			"metadata":             h.Metadata,
		}
	}
	shared.WriteSuccess(w, http.StatusOK, result, nil)
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

func serializeWorkflowDefinition(def *domain.WorkflowDefinition) map[string]interface{} {
	states := make([]map[string]interface{}, len(def.States))
	for i, s := range def.States {
		states[i] = map[string]interface{}{
			"id":               s.ID,
			"key":              s.Key,
			"name":             s.Name,
			"description":      s.Description,
			"category":         s.Category,
			"terminal":         s.Terminal,
			"display_order":    s.DisplayOrder,
			"responsible_role": s.ResponsibleRole,
		}
	}
	transitions := make([]map[string]interface{}, len(def.Transitions))
	for i, t := range def.Transitions {
		transitions[i] = map[string]interface{}{
			"id":            t.ID,
			"key":           t.Key,
			"name":          t.Name,
			"from_state":    t.FromState,
			"to_state":      t.ToState,
			"description":   t.Description,
			"conditions":    t.Conditions,
			"allowed_roles": t.AllowedRoles,
			"active":        t.Active,
		}
	}
	return map[string]interface{}{
		"id":              def.ID,
		"organization_id": def.TenantID,
		"key":             def.Key,
		"name":            def.Name,
		"description":     def.Description,
		"version":         def.Version,
		"status":          def.Status,
		"initial_state":   def.InitialState,
		"states":          states,
		"transitions":     transitions,
		"metadata":        def.Metadata,
		"created_at":      def.CreatedAt,
		"updated_at":      def.UpdatedAt,
	}
}

func serializeWorkflowInstance(instance *domain.WorkflowInstance) map[string]interface{} {
	return map[string]interface{}{
		"id":                          instance.ID,
		"organization_id":             instance.TenantID,
		"workflow_definition_id":      instance.WorkflowDefID,
		"workflow_definition_version": instance.WorkflowDefVersion,
		"case_id":                     instance.CaseID,
		"current_state":               instance.CurrentState,
		"started_at":                  instance.StartedAt,
		"completed_at":                instance.CompletedAt,
		"metadata":                    instance.Metadata,
	}
}

func writeWorkflowError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrWorkflowInstanceNotFound{}):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "workflow instance not found")
	case errors.Is(err, domain.ErrWorkflowDefinitionNotFound{}):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "workflow definition not found")
	case errors.Is(err, domain.ErrTenantViolation{}):
		shared.WriteError(w, http.StatusForbidden, shared.CodeForbidden, "tenant violation")
	case errors.Is(err, domain.TerminalStateError{}):
		shared.WriteError(w, http.StatusConflict, shared.CodeStateTransition, err.Error())
	case errors.Is(err, domain.ErrTransitionNotFound{}):
		shared.WriteError(w, http.StatusConflict, shared.CodeStateTransition, err.Error())
	case errors.Is(err, domain.ErrUnauthorizedTransition{}):
		shared.WriteError(w, http.StatusForbidden, shared.CodeForbidden, err.Error())
	case errors.Is(err, domain.ErrWorkflowInstanceExists{}):
		shared.WriteError(w, http.StatusConflict, shared.CodeConflict, err.Error())
	default:
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "internal server error")
	}
}
