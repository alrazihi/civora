package api

import (
	"net/http"
	"strconv"

	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/operations/domain"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AnalysisHandler struct {
	svc domain.AnalysisService
}

func NewAnalysisHandler(svc domain.AnalysisService) *AnalysisHandler {
	return &AnalysisHandler{svc: svc}
}

func (h *AnalysisHandler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/organizations/{orgId}/operations/analysis", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.RequireSameTenant)
		r.Use(middleware.RequireAnyRole("admin", "staff"))

		r.Get("/workflow", h.GetWorkflowAnalysis)
		r.Get("/thresholds", h.GetDefaultThresholds)
	})
}

func (h *AnalysisHandler) GetDefaultThresholds(w http.ResponseWriter, r *http.Request) {
	shared.WriteSuccess(w, http.StatusOK, domain.DefaultAnalysisThresholds, nil)
}

func (h *AnalysisHandler) GetWorkflowAnalysis(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	thresholds := domain.DefaultAnalysisThresholds

	if stateAccum := r.URL.Query().Get("state_accumulation_threshold"); stateAccum != "" {
		if v, err := strconv.Atoi(stateAccum); err == nil && v > 0 {
			thresholds.StateAccumulationThreshold = v
		}
	}
	if stateDuration := r.URL.Query().Get("state_duration_threshold_hours"); stateDuration != "" {
		if v, err := strconv.ParseFloat(stateDuration, 64); err == nil && v > 0 {
			thresholds.StateDurationThresholdHours = v
		}
	}
	if aging := r.URL.Query().Get("aging_threshold_hours"); aging != "" {
		if v, err := strconv.ParseFloat(aging, 64); err == nil && v > 0 {
			thresholds.AgingThresholdHours = v
		}
	}
	if reviewBacklog := r.URL.Query().Get("review_backlog_threshold"); reviewBacklog != "" {
		if v, err := strconv.Atoi(reviewBacklog); err == nil && v > 0 {
			thresholds.ReviewBacklogThreshold = v
		}
	}
	if infoFreq := r.URL.Query().Get("info_request_frequency_per_case"); infoFreq != "" {
		if v, err := strconv.ParseFloat(infoFreq, 64); err == nil && v > 0 {
			thresholds.InfoRequestFrequencyPerCase = v
		}
	}
	if cycleTime := r.URL.Query().Get("cycle_time_threshold_hours"); cycleTime != "" {
		if v, err := strconv.ParseFloat(cycleTime, 64); err == nil && v > 0 {
			thresholds.CycleTimeThresholdHours = v
		}
	}

	report, err := h.svc.GetWorkflowAnalysisReport(r.Context(), orgID, thresholds)
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "failed to generate workflow analysis")
		return
	}
	shared.WriteSuccess(w, http.StatusOK, report, nil)
}
