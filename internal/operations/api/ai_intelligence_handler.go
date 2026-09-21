package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/operations/application"
	"github.com/alrazihi/civora/internal/operations/domain"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const MaxIntelligenceQuestionLength = 2000

func sanitizeQuestion(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) > MaxIntelligenceQuestionLength {
		trimmed = trimmed[:MaxIntelligenceQuestionLength]
	}
	var b strings.Builder
	b.Grow(len(trimmed))
	for _, r := range trimmed {
		if r == '\n' || r == '\r' || r == '\t' || r >= 0x20 && r != 0x7f {
			b.WriteRune(r)
		}
	}
	return b.String()
}

type OperationsIntelligenceHandler struct {
	intelligenceService *application.OperationsIntelligenceService
}

func NewOperationsIntelligenceHandler(intelligenceService *application.OperationsIntelligenceService) *OperationsIntelligenceHandler {
	return &OperationsIntelligenceHandler{intelligenceService: intelligenceService}
}

func (h *OperationsIntelligenceHandler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/organizations/{orgId}/operations/intelligence", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.RequireSameTenant)
		r.Use(middleware.RequireAnyRole("admin", "staff"))

		r.Post("/", h.PostIntelligence)
		r.Get("/suggested-questions", h.GetSuggestedQuestions)
	})
}

func (h *OperationsIntelligenceHandler) PostIntelligence(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	var req domain.IntelligenceRequest
	req.Type = domain.IntelligenceRequestType(r.URL.Query().Get("type"))
	if req.Type == "" {
		req.Type = domain.IntelligenceRequestTypeSummarizeTrends
	}
	req.Question = sanitizeQuestion(r.URL.Query().Get("question"))
	req.Period = domain.MetricPeriod(r.URL.Query().Get("period"))
	if req.Period == "" {
		req.Period = domain.MetricPeriodDaily
	}
	req.WorkflowKey = r.URL.Query().Get("workflow_key")
	req.Status = r.URL.Query().Get("status")
	req.Language = r.URL.Query().Get("language")

	if b := r.URL.Query().Get("bucket"); b != "" {
		if parsed, err := time.Parse(time.RFC3339, b); err == nil {
			if _, ok := validateBucket(parsed); ok {
				req.Bucket = parsed
			} else {
				shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid bucket")
				return
			}
		}
	} else {
		req.Bucket = time.Now().UTC()
	}

	result, err := h.intelligenceService.GenerateIntelligence(r.Context(), orgID, req)
	if err != nil {
		if err.Error() == "AI provider is not configured" {
			shared.WriteError(w, http.StatusServiceUnavailable, shared.CodeInternalError, "AI provider is not configured or is disabled")
			return
		}
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "failed to generate intelligence")
		return
	}

	shared.WriteSuccess(w, http.StatusOK, result, nil)
}

func (h *OperationsIntelligenceHandler) GetSuggestedQuestions(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	req := domain.IntelligenceRequest{
		Type:   domain.IntelligenceRequestTypeSuggestQuestions,
		Period: domain.MetricPeriodDaily,
		Bucket: time.Now().UTC(),
	}

	result, err := h.intelligenceService.GenerateIntelligence(r.Context(), orgID, req)
	if err != nil {
		if err.Error() == "AI provider is not configured" {
			shared.WriteError(w, http.StatusServiceUnavailable, shared.CodeInternalError, "AI provider is not configured or is disabled")
			return
		}
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "failed to generate suggested questions")
		return
	}

	shared.WriteSuccess(w, http.StatusOK, map[string]interface{}{
		"suggested_questions": result.SuggestedQuestions,
		"generated_at":        result.GeneratedAt,
	}, nil)
}
