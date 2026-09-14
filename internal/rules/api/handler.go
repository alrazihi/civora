package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/rules/application"
	rulesdomain "github.com/alrazihi/civora/internal/rules/domain"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc RuleSetService
}

func NewHandler(svc RuleSetService) *Handler {
	return &Handler{svc: svc}
}

type RuleSetService interface {
	CreateRuleSet(ctx context.Context, params application.CreateRuleSetParams) (*rulesdomain.RuleSet, error)
	GetRuleSet(ctx context.Context, orgID, id uuid.UUID) (*rulesdomain.RuleSet, error)
	GetRuleSetByKey(ctx context.Context, orgID uuid.UUID, key string) (*rulesdomain.RuleSet, error)
	ListRuleSets(ctx context.Context, params application.ListRuleSetsParams) ([]*rulesdomain.RuleSet, int, error)
	UpdateRuleSet(ctx context.Context, params application.UpdateRuleSetParams) (*rulesdomain.RuleSet, error)
	CreateVersion(ctx context.Context, orgID, id uuid.UUID, actorID uuid.UUID) (*rulesdomain.RuleSet, error)
	PublishRuleSet(ctx context.Context, orgID, id uuid.UUID, actorID uuid.UUID) (*rulesdomain.RuleSet, error)
	ArchiveRuleSet(ctx context.Context, orgID, id uuid.UUID, actorID uuid.UUID) (*rulesdomain.RuleSet, error)
	DeleteRuleSet(ctx context.Context, orgID, id uuid.UUID) error
	ListVersions(ctx context.Context, orgID uuid.UUID, key string, limit, offset int) ([]*rulesdomain.RuleSet, int, error)
	EvaluateRuleSet(ctx context.Context, params application.EvaluateRuleSetParams) (*rulesdomain.Evaluation, error)
	GetEvaluation(ctx context.Context, orgID, id uuid.UUID) (*rulesdomain.Evaluation, error)
	ListEvaluationsByRuleSet(ctx context.Context, orgID, ruleSetID uuid.UUID, limit, offset int) ([]*rulesdomain.Evaluation, int, error)
	ListEvaluationsByCase(ctx context.Context, orgID, caseID uuid.UUID, limit, offset int) ([]*rulesdomain.Evaluation, int, error)
	ListDiscoverableFields(ctx context.Context, orgID uuid.UUID) ([]shared.FormFieldView, error)
	CreateRuleTemplate(ctx context.Context, params application.CreateRuleTemplateParams) (*rulesdomain.RuleTemplate, error)
	ListRuleTemplates(ctx context.Context, params application.ListRuleTemplatesParams) ([]*rulesdomain.RuleTemplate, int, error)
	GetRuleTemplate(ctx context.Context, orgID, id uuid.UUID) (*rulesdomain.RuleTemplate, error)
	GetRuleTemplateByKey(ctx context.Context, orgID uuid.UUID, key string) (*rulesdomain.RuleTemplate, error)
	DeleteRuleTemplate(ctx context.Context, orgID, id uuid.UUID, actorID uuid.UUID) error
	InstantiateRuleTemplate(ctx context.Context, orgID, templateID uuid.UUID, actorID uuid.UUID) (*rulesdomain.Rule, error)
}

func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/organizations/{orgId}/rules", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.RequireSameTenant)

		// Rule set endpoints - read access for admin and staff
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAnyRole("admin", "staff"))
			r.Get("/rule-sets", h.ListRuleSets)
			r.Get("/rule-sets/{ruleSetId}", h.GetRuleSet)
			r.Get("/rule-sets/key/{key}", h.GetRuleSetByKey)
			r.Get("/rule-sets/{ruleSetId}/versions", h.ListVersions)
			r.Get("/rule-sets/{ruleSetId}/evaluations", h.ListEvaluationsByRuleSet)
			r.Get("/evaluations/{evaluationId}", h.GetEvaluation)
			r.Get("/cases/{caseId}/evaluations", h.ListEvaluationsByCase)
			r.Get("/fields", h.ListFields)

			// Rule template endpoints - read access for admin and staff
			r.Get("/templates", h.ListRuleTemplates)
			r.Get("/templates/{templateId}", h.GetRuleTemplate)
			r.Get("/templates/key/{key}", h.GetRuleTemplateByKey)
			r.Post("/templates/{templateId}/instantiate", h.InstantiateRuleTemplate)
		})

		// Rule set write endpoints - admin only
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAnyRole("admin"))
			r.Post("/rule-sets", h.CreateRuleSet)
			r.Patch("/rule-sets/{ruleSetId}", h.UpdateRuleSet)
			r.Post("/rule-sets/{ruleSetId}/version", h.CreateVersion)
			r.Post("/rule-sets/{ruleSetId}/publish", h.PublishRuleSet)
			r.Post("/rule-sets/{ruleSetId}/archive", h.ArchiveRuleSet)
			r.Delete("/rule-sets/{ruleSetId}", h.DeleteRuleSet)

			// Rule template write endpoints - admin only
			r.Post("/templates", h.CreateRuleTemplate)
			r.Delete("/templates/{templateId}", h.DeleteRuleTemplate)
		})

		// Evaluation endpoint - admin and staff can trigger evaluations
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAnyRole("admin", "staff"))
			r.Post("/rule-sets/{ruleSetId}/evaluate", h.EvaluateRuleSet)
		})
	})
}

func (h *Handler) CreateRuleSet(w http.ResponseWriter, r *http.Request) {
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
		CaseID         *string           `json:"case_id"`
		Key            string            `json:"key"`
		Name           string            `json:"name"`
		Description    string            `json:"description"`
		DefaultOutcome string            `json:"default_outcome"`
		Rules          []json.RawMessage `json:"rules"`
		Triggers       []string          `json:"triggers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	var caseID *uuid.UUID
	if req.CaseID != nil && *req.CaseID != "" {
		id, err := uuid.Parse(*req.CaseID)
		if err != nil {
			shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid case ID")
			return
		}
		caseID = &id
	}

	rules, err := parseRules(req.Rules)
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid rules: "+err.Error())
		return
	}

	rs, err := h.svc.CreateRuleSet(r.Context(), application.CreateRuleSetParams{
		OrganizationID: orgID,
		CaseID:         caseID,
		Key:            req.Key,
		Name:           req.Name,
		Description:    req.Description,
		DefaultOutcome: rulesdomain.Outcome(req.DefaultOutcome),
		Rules:          rules,
		Triggers:       req.Triggers,
		ActorID:        actorID,
	})
	if err != nil {
		writeRuleSetError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusCreated, serializeRuleSet(rs), nil)
}

func (h *Handler) GetRuleSet(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	ruleSetID, ok := parseUUID(r, "ruleSetId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid rule set ID")
		return
	}

	rs, err := h.svc.GetRuleSet(r.Context(), orgID, ruleSetID)
	if err != nil {
		writeRuleSetError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeRuleSet(rs), nil)
}

func (h *Handler) GetRuleSetByKey(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	key := chi.URLParam(r, "key")
	if key == "" {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "key is required")
		return
	}

	rs, err := h.svc.GetRuleSetByKey(r.Context(), orgID, key)
	if err != nil {
		writeRuleSetError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeRuleSet(rs), nil)
}

func (h *Handler) ListRuleSets(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	caseIDStr := r.URL.Query().Get("case_id")
	var caseID *uuid.UUID
	if caseIDStr != "" {
		id, err := uuid.Parse(caseIDStr)
		if err != nil {
			shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid case ID")
			return
		}
		caseID = &id
	}

	key := r.URL.Query().Get("key")
	status := r.URL.Query().Get("status")

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

	items, total, err := h.svc.ListRuleSets(r.Context(), application.ListRuleSetsParams{
		OrganizationID: orgID,
		CaseID:         caseID,
		Key:            key,
		Status:         rulesdomain.RuleSetStatus(status),
		Limit:          perPage,
		Offset:         offset,
	})
	if err != nil {
		writeRuleSetError(w, err)
		return
	}

	result := make([]map[string]interface{}, len(items))
	for i, rs := range items {
		result[i] = serializeRuleSet(rs)
	}

	shared.WritePaginatedSuccess(w, http.StatusOK, result, page, perPage, total)
}

func (h *Handler) ListVersions(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	ruleSetID, ok := parseUUID(r, "ruleSetId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid rule set ID")
		return
	}

	rs, err := h.svc.GetRuleSet(r.Context(), orgID, ruleSetID)
	if err != nil {
		writeRuleSetError(w, err)
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

	versions, total, err := h.svc.ListVersions(r.Context(), orgID, rs.Key, perPage, offset)
	if err != nil {
		writeRuleSetError(w, err)
		return
	}

	result := make([]map[string]interface{}, len(versions))
	for i, v := range versions {
		result[i] = serializeRuleSet(v)
	}

	shared.WritePaginatedSuccess(w, http.StatusOK, result, page, perPage, total)
}

func (h *Handler) UpdateRuleSet(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	ruleSetID, ok := parseUUID(r, "ruleSetId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid rule set ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		Name           string            `json:"name"`
		Description    string            `json:"description"`
		DefaultOutcome string            `json:"default_outcome"`
		Rules          []json.RawMessage `json:"rules"`
		Triggers       []string          `json:"triggers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	var rules []rulesdomain.Rule
	if req.Rules != nil {
		var err error
		rules, err = parseRules(req.Rules)
		if err != nil {
			shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid rules: "+err.Error())
			return
		}
	}

	var defaultOutcome rulesdomain.Outcome
	if req.DefaultOutcome != "" {
		defaultOutcome = rulesdomain.Outcome(req.DefaultOutcome)
	}

	rs, err := h.svc.UpdateRuleSet(r.Context(), application.UpdateRuleSetParams{
		OrganizationID: orgID,
		ID:             ruleSetID,
		Name:           req.Name,
		Description:    req.Description,
		DefaultOutcome: defaultOutcome,
		Rules:          rules,
		Triggers:       req.Triggers,
		ActorID:        actorID,
	})
	if err != nil {
		writeRuleSetError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeRuleSet(rs), nil)
}

func (h *Handler) CreateVersion(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	ruleSetID, ok := parseUUID(r, "ruleSetId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid rule set ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	rs, err := h.svc.CreateVersion(r.Context(), orgID, ruleSetID, actorID)
	if err != nil {
		writeRuleSetError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusCreated, serializeRuleSet(rs), nil)
}

func (h *Handler) PublishRuleSet(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	ruleSetID, ok := parseUUID(r, "ruleSetId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid rule set ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	rs, err := h.svc.PublishRuleSet(r.Context(), orgID, ruleSetID, actorID)
	if err != nil {
		writeRuleSetError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeRuleSet(rs), nil)
}

func (h *Handler) ArchiveRuleSet(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	ruleSetID, ok := parseUUID(r, "ruleSetId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid rule set ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	rs, err := h.svc.ArchiveRuleSet(r.Context(), orgID, ruleSetID, actorID)
	if err != nil {
		writeRuleSetError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeRuleSet(rs), nil)
}

func (h *Handler) DeleteRuleSet(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	ruleSetID, ok := parseUUID(r, "ruleSetId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid rule set ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	err := h.svc.DeleteRuleSet(r.Context(), orgID, ruleSetID)
	if err != nil {
		writeRuleSetError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, map[string]interface{}{"deleted": true}, nil)
}

func (h *Handler) EvaluateRuleSet(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	ruleSetID, ok := parseUUID(r, "ruleSetId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid rule set ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		CaseID  *string                `json:"case_id"`
		Facts   map[string]interface{} `json:"facts"`
		Trigger string                 `json:"trigger"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	var caseID *uuid.UUID
	if req.CaseID != nil && *req.CaseID != "" {
		id, err := uuid.Parse(*req.CaseID)
		if err != nil {
			shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid case ID")
			return
		}
		caseID = &id
	}

	trigger := rulesdomain.Trigger(req.Trigger)
	if trigger == "" {
		trigger = rulesdomain.TriggerManual
	}

	facts := req.Facts
	if facts == nil {
		facts = map[string]interface{}{}
	}

	ev, err := h.svc.EvaluateRuleSet(r.Context(), application.EvaluateRuleSetParams{
		OrganizationID: orgID,
		ID:             ruleSetID,
		CaseID:         caseID,
		Facts:          facts,
		Trigger:        trigger,
		ActorID:        &actorID,
	})
	if err != nil {
		writeEvaluationError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeEvaluation(ev), nil)
}

func (h *Handler) GetEvaluation(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	evaluationID, ok := parseUUID(r, "evaluationId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid evaluation ID")
		return
	}

	ev, err := h.svc.GetEvaluation(r.Context(), orgID, evaluationID)
	if err != nil {
		writeEvaluationError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeEvaluation(ev), nil)
}

func (h *Handler) ListEvaluationsByRuleSet(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	ruleSetID, ok := parseUUID(r, "ruleSetId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid rule set ID")
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

	items, total, err := h.svc.ListEvaluationsByRuleSet(r.Context(), orgID, ruleSetID, perPage, offset)
	if err != nil {
		writeEvaluationError(w, err)
		return
	}

	result := make([]map[string]interface{}, len(items))
	for i, ev := range items {
		result[i] = serializeEvaluation(ev)
	}

	shared.WritePaginatedSuccess(w, http.StatusOK, result, page, perPage, total)
}

func (h *Handler) ListEvaluationsByCase(w http.ResponseWriter, r *http.Request) {
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

	items, total, err := h.svc.ListEvaluationsByCase(r.Context(), orgID, caseID, perPage, offset)
	if err != nil {
		writeEvaluationError(w, err)
		return
	}

	result := make([]map[string]interface{}, len(items))
	for i, ev := range items {
		result[i] = serializeEvaluation(ev)
	}

	shared.WritePaginatedSuccess(w, http.StatusOK, result, page, perPage, total)
}

func (h *Handler) ListFields(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	fields, err := h.svc.ListDiscoverableFields(r.Context(), orgID)
	if err != nil {
		writeRuleSetError(w, err)
		return
	}

	result := make([]map[string]interface{}, len(fields))
	for i, f := range fields {
		result[i] = map[string]interface{}{
			"key":      f.Key,
			"label":    f.Label,
			"type":     f.Type,
			"required": f.Required,
			"options":  f.Options,
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

func parseRules(rawRules []json.RawMessage) ([]rulesdomain.Rule, error) {
	rules := make([]rulesdomain.Rule, len(rawRules))
	for i, raw := range rawRules {
		var rule rulesdomain.Rule
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		if err := dec.Decode(&rule); err != nil {
			return nil, fmt.Errorf("rule %d: %w", i, err)
		}
		rules[i] = rule
	}
	return rules, nil
}

func serializeRuleSet(rs *rulesdomain.RuleSet) map[string]interface{} {
	caseID := ""
	if rs.CaseID != nil {
		caseID = rs.CaseID.String()
	}

	return map[string]interface{}{
		"id":              rs.ID,
		"organization_id": rs.OrganizationID,
		"case_id":         caseID,
		"key":             rs.Key,
		"name":            rs.Name,
		"description":     rs.Description,
		"version":         rs.Version,
		"status":          string(rs.Status),
		"default_outcome": string(rs.DefaultOutcome),
		"rules":           rs.Rules,
		"triggers":        rs.Triggers,
		"created_by":      rs.CreatedBy,
		"created_at":      rs.CreatedAt,
		"updated_at":      rs.UpdatedAt,
	}
}

func serializeEvaluation(ev *rulesdomain.Evaluation) map[string]interface{} {
	caseID := ""
	if ev.CaseID != nil {
		caseID = ev.CaseID.String()
	}
	matchedRuleID := ""
	if ev.MatchedRuleID != nil {
		matchedRuleID = ev.MatchedRuleID.String()
	}
	evaluatedBy := ""
	if ev.EvaluatedBy != nil {
		evaluatedBy = ev.EvaluatedBy.String()
	}
	reason := ""
	if ev.Reason != nil {
		reason = *ev.Reason
	}

	return map[string]interface{}{
		"id":               ev.ID,
		"rule_set_id":      ev.RuleSetID,
		"rule_set_version": ev.RuleSetVersion,
		"organization_id":  ev.OrganizationID,
		"case_id":          caseID,
		"status":           string(ev.Status),
		"outcome":          string(ev.Outcome),
		"reason":           reason,
		"matched_rule_id":  matchedRuleID,
		"trace":            ev.Trace,
		"trigger":          string(ev.Trigger),
		"evaluated_by":     evaluatedBy,
		"evaluated_at":     ev.EvaluatedAt,
		"facts_snapshot":   ev.FactsSnapshot,
	}
}

func writeRuleSetError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrRuleSetNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "rule set not found")
	case errors.Is(err, application.ErrRuleSetInvalid):
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, err.Error())
	case errors.Is(err, application.ErrRuleSetNotDraft):
		shared.WriteError(w, http.StatusConflict, shared.CodeConflict, err.Error())
	case errors.Is(err, application.ErrRuleSetNotPublished):
		shared.WriteError(w, http.StatusConflict, shared.CodeConflict, err.Error())
	case errors.Is(err, application.ErrRuleSetArchived):
		shared.WriteError(w, http.StatusConflict, shared.CodeConflict, err.Error())
	case errors.Is(err, application.ErrRuleSetKeyExists):
		shared.WriteError(w, http.StatusConflict, shared.CodeConflict, err.Error())
	case errors.Is(err, application.ErrRuleSetVersionExists):
		shared.WriteError(w, http.StatusConflict, shared.CodeConflict, err.Error())
	case errors.Is(err, application.ErrInvalidOutcome):
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, err.Error())
	case errors.Is(err, application.ErrDuplicatePriority):
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, err.Error())
	case errors.Is(err, application.ErrMaxRulesExceeded):
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, err.Error())
	case errors.Is(err, application.ErrInvalidCondition):
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, err.Error())
	case errors.Is(err, application.ErrUserNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, err.Error())
	default:
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "internal server error")
	}
}

func writeEvaluationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrEvaluationNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "evaluation not found")
	case errors.Is(err, application.ErrInvalidTrigger):
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, err.Error())
	case errors.Is(err, application.ErrRuleSetNotPublished):
		shared.WriteError(w, http.StatusConflict, shared.CodeConflict, err.Error())
	case errors.Is(err, application.ErrRuleSetArchived):
		shared.WriteError(w, http.StatusConflict, shared.CodeConflict, err.Error())
	default:
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "internal server error")
	}
}

func writeRuleTemplateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrRuleTemplateNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "rule template not found")
	case errors.Is(err, application.ErrRuleTemplateInvalid):
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, err.Error())
	case errors.Is(err, application.ErrRuleTemplateKeyExists):
		shared.WriteError(w, http.StatusConflict, shared.CodeConflict, err.Error())
	case errors.Is(err, application.ErrMaxTemplatesExceeded):
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, err.Error())
	default:
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "internal server error")
	}
}

func (h *Handler) CreateRuleTemplate(w http.ResponseWriter, r *http.Request) {
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
		Scope       string            `json:"scope"`
		Key         string            `json:"key"`
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Category    string            `json:"category"`
		Rule        json.RawMessage   `json:"rule"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	scope := rulesdomain.RuleTemplateScope(req.Scope)
	if scope != rulesdomain.RuleTemplateScopeOrg && scope != rulesdomain.RuleTemplateScopeGlobal {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid scope: must be ORG or GLOBAL")
		return
	}

	var rule rulesdomain.Rule
	dec := json.NewDecoder(bytes.NewReader(req.Rule))
	dec.UseNumber()
	if err := dec.Decode(&rule); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid rule: "+err.Error())
		return
	}

	rt, err := h.svc.CreateRuleTemplate(r.Context(), application.CreateRuleTemplateParams{
		OrganizationID: orgID,
		Scope:          scope,
		Key:            req.Key,
		Name:           req.Name,
		Description:    req.Description,
		Category:       req.Category,
		Rule:           rule,
		ActorID:        actorID,
	})
	if err != nil {
		writeRuleTemplateError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusCreated, serializeRuleTemplate(rt), nil)
}

func (h *Handler) ListRuleTemplates(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	scopeStr := r.URL.Query().Get("scope")
	scope := rulesdomain.RuleTemplateScope(scopeStr)
	category := r.URL.Query().Get("category")

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

	items, total, err := h.svc.ListRuleTemplates(r.Context(), application.ListRuleTemplatesParams{
		OrganizationID: orgID,
		Scope:          scope,
		Category:       category,
		Limit:          perPage,
		Offset:         offset,
	})
	if err != nil {
		writeRuleTemplateError(w, err)
		return
	}

	result := make([]map[string]interface{}, len(items))
	for i, rt := range items {
		result[i] = serializeRuleTemplate(rt)
	}

	shared.WritePaginatedSuccess(w, http.StatusOK, result, page, perPage, total)
}

func (h *Handler) GetRuleTemplate(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	templateID, ok := parseUUID(r, "templateId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid template ID")
		return
	}

	rt, err := h.svc.GetRuleTemplate(r.Context(), orgID, templateID)
	if err != nil {
		writeRuleTemplateError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeRuleTemplate(rt), nil)
}

func (h *Handler) GetRuleTemplateByKey(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	key := chi.URLParam(r, "key")
	if key == "" {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "key is required")
		return
	}

	rt, err := h.svc.GetRuleTemplateByKey(r.Context(), orgID, key)
	if err != nil {
		writeRuleTemplateError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeRuleTemplate(rt), nil)
}

func (h *Handler) DeleteRuleTemplate(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	templateID, ok := parseUUID(r, "templateId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid template ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	err := h.svc.DeleteRuleTemplate(r.Context(), orgID, templateID, actorID)
	if err != nil {
		writeRuleTemplateError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, map[string]interface{}{"deleted": true}, nil)
}

func (h *Handler) InstantiateRuleTemplate(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	templateID, ok := parseUUID(r, "templateId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid template ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	rule, err := h.svc.InstantiateRuleTemplate(r.Context(), orgID, templateID, actorID)
	if err != nil {
		writeRuleTemplateError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeRule(rule), nil)
}

func serializeRuleTemplate(rt *rulesdomain.RuleTemplate) map[string]interface{} {
	orgID := ""
	if rt.OrganizationID != nil {
		orgID = rt.OrganizationID.String()
	}
	return map[string]interface{}{
		"id":               rt.ID,
		"scope":            string(rt.Scope),
		"organization_id":  orgID,
		"key":              rt.Key,
		"name":             rt.Name,
		"description":      rt.Description,
		"category":         rt.Category,
		"rule":             rt.Rule,
		"created_by":       rt.CreatedBy,
		"created_at":       rt.CreatedAt,
		"updated_at":       rt.UpdatedAt,
	}
}

func serializeRule(rule *rulesdomain.Rule) map[string]interface{} {
	return map[string]interface{}{
		"id":          rule.ID,
		"priority":    rule.Priority,
		"outcome":     string(rule.Outcome),
		"conditions":  rule.Conditions,
		"active":      rule.Active,
		"created_at":  rule.CreatedAt,
	}
}

var _ = bytes.NewReader
var _ = fmt.Errorf
