package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/alrazihi/civora/internal/casesummary/application"
	"github.com/alrazihi/civora/internal/casesummary/domain"
	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc *application.CaseSummaryService
}

type CaseSummaryService interface {
	GenerateCaseSummary(ctx context.Context, params application.GenerateCaseSummaryParams) (*application.GenerateCaseSummaryResult, error)
	ListCaseSummaries(ctx context.Context, params application.ListCaseSummariesParams) ([]*domain.CaseSummary, int, error)
	ReviewCaseSummary(ctx context.Context, params application.ReviewCaseSummaryParams) (*domain.CaseSummary, error)
}

func NewHandler(svc *application.CaseSummaryService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/organizations/{orgId}/cases/{caseId}/summary", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.RequireSameTenant)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAnyRole("admin", "staff"))
			r.Post("/generate", h.GenerateCaseSummary)
			r.Get("/", h.ListCaseSummaries)
		})
		r.Post("/{summaryId}/accept", h.AcceptSummary)
		r.Post("/{summaryId}/reject", h.RejectSummary)
	})
}

func (h *Handler) GenerateCaseSummary(w http.ResponseWriter, r *http.Request) {
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
		Types           []string `json:"types"`
		MaxTokens       int      `json:"max_tokens"`
		IncludeSections []string `json:"include_sections"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid JSON body")
		return
	}

	var types []domain.SummaryType
	for _, t := range req.Types {
		types = append(types, domain.SummaryType(t))
	}

	result, err := h.svc.GenerateCaseSummary(r.Context(), application.GenerateCaseSummaryParams{
		OrganizationID:  orgID,
		CaseID:          caseID,
		ActorID:         actorID,
		Types:           types,
		MaxTokens:       req.MaxTokens,
		IncludeSections: req.IncludeSections,
	})
	if err != nil {
		writeSummaryError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeSummary(result.Summary), nil)
}

func (h *Handler) ListCaseSummaries(w http.ResponseWriter, r *http.Request) {
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

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 200 {
		perPage = 200
	}
	offset := (page - 1) * perPage

	summaries, total, err := h.svc.ListCaseSummaries(r.Context(), application.ListCaseSummariesParams{
		OrganizationID: orgID,
		CaseID:         caseID,
		ActorID:        actorID,
		Limit:          perPage,
		Offset:         offset,
	})
	if err != nil {
		writeSummaryError(w, err)
		return
	}

	result := make([]map[string]interface{}, len(summaries))
	for i, s := range summaries {
		result[i] = serializeSummary(s)
	}

	shared.WritePaginatedSuccess(w, http.StatusOK, result, page, perPage, total)
}

func (h *Handler) AcceptSummary(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	summaryID, ok := parseUUID(r, "summaryId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid summary ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		Notes string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	summary, err := h.svc.ReviewCaseSummary(r.Context(), application.ReviewCaseSummaryParams{
		OrganizationID: orgID,
		SummaryID:      summaryID,
		ReviewerID:     actorID,
		Action:         domain.SummaryStatusAccepted,
		Notes:          req.Notes,
	})
	if err != nil {
		writeSummaryError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeSummary(summary), nil)
}

func (h *Handler) RejectSummary(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUID(r, "orgId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	summaryID, ok := parseUUID(r, "summaryId")
	if !ok {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid summary ID")
		return
	}

	actorID := getUserID(r)
	if actorID == uuid.Nil {
		shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
		return
	}

	var req struct {
		Notes string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid request body")
		return
	}

	summary, err := h.svc.ReviewCaseSummary(r.Context(), application.ReviewCaseSummaryParams{
		OrganizationID: orgID,
		SummaryID:      summaryID,
		ReviewerID:     actorID,
		Action:         domain.SummaryStatusRejected,
		Notes:          req.Notes,
	})
	if err != nil {
		writeSummaryError(w, err)
		return
	}

	shared.WriteSuccess(w, http.StatusOK, serializeSummary(summary), nil)
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

func serializeSummary(s *domain.CaseSummary) map[string]interface{} {
	return map[string]interface{}{
		"id":              s.ID,
		"organization_id": s.OrganizationID,
		"case_id":         s.CaseID,
		"type":            s.Type,
		"source":          s.Source,
		"status":          s.Status,
		"model":           s.Model,
		"content":         serializeSummaryContent(s.Content),
		"confidence":      s.Confidence,
		"input_hash":      s.InputHash,
		"output_hash":     s.OutputHash,
		"schema_version":  s.SchemaVersion,
		"token_estimate":  s.TokenEstimate,
		"created_at":      s.CreatedAt,
		"created_by":      s.CreatedBy,
		"reviewed_at":     s.ReviewedAt,
		"reviewed_by":     s.ReviewedBy,
		"review_notes":    s.ReviewNotes,
	}
}

func serializeSummaryContent(c domain.SummaryContent) map[string]interface{} {
	return map[string]interface{}{
		"situation":                 serializeClaims(c.Situation),
		"relevant_information":      serializeClaims(c.RelevantInformation),
		"important_evidence":        serializeClaims(c.ImportantEvidence),
		"missing_information":       serializeClaims(c.MissingInformation),
		"potential_inconsistencies": serializeClaims(c.PotentialInconsistencies),
		"rule_evaluation_results":   serializeClaims(c.RuleEvaluationResults),
		"workflow_history":          serializeClaims(c.WorkflowHistory),
		"previous_actions":          serializeClaims(c.PreviousActions),
		"ai_observations":           serializeClaims(c.AIObservations),
	}
}

func serializeClaims(claims []domain.SummaryClaim) []map[string]interface{} {
	result := make([]map[string]interface{}, len(claims))
	for i, c := range claims {
		refs := make([]map[string]interface{}, len(c.References))
		for j, r := range c.References {
			refs[j] = map[string]interface{}{
				"type":  r.Type,
				"id":    r.ID,
				"label": r.Label,
			}
		}
		result[i] = map[string]interface{}{
			"text":       c.Text,
			"provenance": c.Provenance,
			"references": refs,
			"confidence": c.Confidence,
		}
	}
	return result
}

func writeSummaryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrCaseNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "case not found")
	case errors.Is(err, application.ErrSummaryNotFound):
		shared.WriteError(w, http.StatusNotFound, shared.CodeNotFound, "case summary not found")
	case errors.Is(err, application.ErrAIProviderUnavailable):
		shared.WriteError(w, http.StatusServiceUnavailable, shared.CodeInternalError, "AI provider unavailable")
	case errors.Is(err, application.ErrInvalidInput):
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid input")
	default:
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "internal server error")
	}
}
