package api

import (
	"net/http"

	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/operations/domain"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type ImpactHandler struct {
	svc domain.ImpactService
}

func NewImpactHandler(svc domain.ImpactService) *ImpactHandler {
	return &ImpactHandler{svc: svc}
}

func (h *ImpactHandler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/organizations/{orgId}/operations/impact", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.RequireSameTenant)

		r.Get("/report", h.GetImpactReport)
	})
}

func (h *ImpactHandler) GetImpactReport(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	period := domain.MetricPeriodDaily
	if p := r.URL.Query().Get("period"); p != "" {
		switch p {
		case "weekly":
			period = domain.MetricPeriodWeekly
		case "monthly":
			period = domain.MetricPeriodMonthly
		}
	}

	bucket, _ := parseBucket(r)
	report, err := h.svc.GetImpactIntelligenceReport(r.Context(), orgID, period, bucket)
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "failed to generate impact report")
		return
	}
	shared.WriteSuccess(w, http.StatusOK, report, nil)
}
