package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/alrazihi/civora/internal/casecontext/application"
	"github.com/alrazihi/civora/internal/casecontext/domain"
	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc CaseContextService
}

type CaseContextService interface {
	BuildContext(ctx context.Context, params application.BuildContextParams) (*application.BuildContextResult, error)
	GetContext(ctx context.Context, orgID, contextID uuid.UUID) (*domain.CaseContext, error)
	GetLatestContext(ctx context.Context, orgID, caseID uuid.UUID) (*domain.CaseContext, error)
}

func NewHandler(svc CaseContextService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/organizations/{orgId}/cases/{caseId}/context", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.RequireSameTenant)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAnyRole("admin", "staff"))
			r.Post("/build", h.BuildContext)
			r.Get("/latest", h.GetLatestContext)
			r.Get("/{contextId}", h.GetContext)
		})
	})
}

func (h *Handler) BuildContext(w http.ResponseWriter, r *http.Request) {
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
		IncludeCaseMetadata     bool `json:"include_case_metadata,omitempty"`
		IncludePerson           bool `json:"include_person,omitempty"`
		IncludeEvidence         bool `json:"include_evidence,omitempty"`
		IncludeVerifiedEvidence bool `json:"include_verified_evidence,omitempty"`
		IncludeDocuments        bool `json:"include_documents,omitempty"`
		IncludeFormSubmissions  bool `json:"include_form_submissions,omitempty"`
		IncludeRuleEvaluations  bool `json:"include_rule_evaluations,omitempty"`
		IncludeWorkflowHistory  bool `json:"include_workflow_history,omitempty"`
		IncludeDecisions        bool `json:"include_decisions,omitempty"`
		IncludeAIObservations   bool `json:"include_ai_observations,omitempty"`
		MaxTokens               int  `json:"max_tokens,omitempty"`
		MaxFacts                int  `json:"max_facts,omitempty"`
		OnlyVerified            bool `json:"only_verified,omitempty"`
		OnlyHumanVerified       bool `json:"only_human_verified,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	options := domain.DefaultContextOptions()
	options.IncludeCaseMetadata = req.IncludeCaseMetadata
	options.IncludePerson = req.IncludePerson
	options.IncludeEvidence = req.IncludeEvidence
	options.IncludeVerifiedEvidence = req.IncludeVerifiedEvidence
	options.IncludeDocuments = req.IncludeDocuments
	options.IncludeFormSubmissions = req.IncludeFormSubmissions
	options.IncludeRuleEvaluations = req.IncludeRuleEvaluations
	options.IncludeWorkflowHistory = req.IncludeWorkflowHistory
	options.IncludeDecisions = req.IncludeDecisions
	options.IncludeAIObservations = req.IncludeAIObservations
	options.MaxTokens = req.MaxTokens
	options.MaxFacts = req.MaxFacts
	options.OnlyVerified = req.OnlyVerified
	options.OnlyHumanVerified = req.OnlyHumanVerified

	if options.MaxTokens <= 0 {
		options.MaxTokens = 8000
	}
	if options.MaxFacts <= 0 {
		options.MaxFacts = 200
	}

	result, err := h.svc.BuildContext(r.Context(), application.BuildContextParams{
		OrganizationID: orgID,
		CaseID:         caseID,
		ActorID:        actorID,
		Options:        options,
	})
	if err != nil {
		writeContextError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeContext(result.Context), nil)
}

func (h *Handler) GetLatestContext(w http.ResponseWriter, r *http.Request) {
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

	ctxObj, err := h.svc.GetLatestContext(r.Context(), orgID, caseID)
	if err != nil {
		writeContextError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeContext(ctxObj), nil)
}

func (h *Handler) GetContext(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	contextID, ok := parseUUID(r, "contextId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid context ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	ctxObj, err := h.svc.GetContext(r.Context(), orgID, contextID)
	if err != nil {
		writeContextError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeContext(ctxObj), nil)
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

func serializeContext(ctx *domain.CaseContext) map[string]interface{} {
	auth := make(map[string]interface{}, len(ctx.Authorization))
	for k, v := range ctx.Authorization {
		auth[k] = map[string]interface{}{
			"resource_type": v.ResourceType,
			"resource_ids":  v.ResourceIDs,
			"granted":       v.Granted,
			"reason":        v.Reason,
		}
	}

	facts := make([]map[string]interface{}, len(ctx.Facts))
	for i, f := range ctx.Facts {
		facts[i] = map[string]interface{}{
			"id":           f.ID,
			"case_id":      f.CaseID,
			"source_type":  f.SourceType,
			"source_id":    f.SourceID,
			"provenance":   f.Provenance,
			"key":          f.Key,
			"value":        f.Value,
			"value_type":   f.ValueType,
			"confidence":   f.Confidence,
			"retrieved_at": f.RetrievedAt,
			"expires_at":   f.ExpiresAt,
			"metadata":     f.Metadata,
			"references":   uuidSliceToString(f.References),
		}
	}

	return map[string]interface{}{
		"id":              ctx.ID,
		"organization_id": ctx.OrganizationID,
		"case_id":         ctx.CaseID,
		"actor_id":        ctx.ActorID,
		"facts":           facts,
		"token_estimate":  ctx.TokenEstimate,
		"created_at":      ctx.CreatedAt,
		"expires_at":      ctx.ExpiresAt,
		"authorization":   auth,
	}
}

func uuidSliceToString(ids []uuid.UUID) []string {
	result := make([]string, len(ids))
	for i, id := range ids {
		result[i] = id.String()
	}
	return result
}

func writeContextError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrContextNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "context not found")
	case errors.Is(err, application.ErrAuthorizationFailed):
		shared.WriteError(w, http.StatusForbidden, shared.CodeForbidden, "authorization failed")
	case errors.Is(err, application.ErrTenantViolation):
		shared.WriteError(w, http.StatusForbidden, shared.CodeForbidden, "tenant violation")
	case errors.Is(err, application.ErrCaseAccessDenied):
		shared.WriteError(w, http.StatusForbidden, shared.CodeForbidden, "case access denied")
	case errors.Is(err, application.ErrEvidenceAccessDenied):
		shared.WriteError(w, http.StatusForbidden, shared.CodeForbidden, "evidence access denied")
	case errors.Is(err, application.ErrDocumentAccessDenied):
		shared.WriteError(w, http.StatusForbidden, shared.CodeForbidden, "document access denied")
	case errors.Is(err, application.ErrPersonAccessDenied):
		shared.WriteError(w, http.StatusForbidden, shared.CodeForbidden, "person access denied")
	case errors.Is(err, application.ErrInsufficientContext):
		shared.WriteError(w, http.StatusUnprocessableEntity, shared.CodeInvalidInput, "insufficient information to build context")
	default:
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "internal server error")
	}
}
