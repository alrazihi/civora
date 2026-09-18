package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/operations/domain"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type OperationsService interface {
	GetCaseVolume(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) (*domain.CaseVolumeMetric, error)
	GetCaseVolumeByWorkflow(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) ([]*domain.CaseVolumeMetric, error)
	GetCaseVolumeByState(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) ([]*domain.CaseVolumeMetric, error)
	GetCaseVolumeByServiceType(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) ([]*domain.CaseVolumeMetric, error)
	GetWorkflowThroughput(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) ([]*domain.WorkflowThroughputMetric, error)
	GetStateDuration(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) ([]*domain.StateDurationMetric, error)
	GetCaseCycleTime(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) ([]*domain.CaseCycleTimeMetric, error)
	GetAgingCases(ctx context.Context, orgID uuid.UUID, thresholdHours float64) ([]*domain.AgingCaseMetric, error)
	GetPendingReviews(ctx context.Context, orgID uuid.UUID) (*domain.PendingReviewMetric, error)
	GetDecisions(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) (*domain.DecisionMetric, error)
	GetAssistanceOutcomes(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) (*domain.AssistanceOutcomeMetric, error)
	GetEvidenceVerification(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) (*domain.EvidenceVerificationMetric, error)
	GetInformationRequired(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) (*domain.InformationRequiredMetric, error)
	GetAllMetrics(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) (*domain.OperationsMetrics, error)
	GetDashboardMetrics(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time, workflowKey string, status string) (*domain.OperationsMetrics, error)
}

type Handler struct {
	svc OperationsService
}

func NewHandler(svc OperationsService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/organizations/{orgId}/operations", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.RequireSameTenant)

		r.Get("/metrics/cases", h.GetCaseVolume)
		r.Get("/metrics/cases/by-workflow", h.GetCaseVolumeByWorkflow)
		r.Get("/metrics/cases/by-state", h.GetCaseVolumeByState)
		r.Get("/metrics/cases/by-service-type", h.GetCaseVolumeByServiceType)
		r.Get("/metrics/workflow-throughput", h.GetWorkflowThroughput)
		r.Get("/metrics/state-duration", h.GetStateDuration)
		r.Get("/metrics/case-cycle-time", h.GetCaseCycleTime)
		r.Get("/metrics/aging-cases", h.GetAgingCases)
		r.Get("/metrics/pending-reviews", h.GetPendingReviews)
		r.Get("/metrics/decisions", h.GetDecisions)
		r.Get("/metrics/assistance-outcomes", h.GetAssistanceOutcomes)
		r.Get("/metrics/evidence-verification", h.GetEvidenceVerification)
		r.Get("/metrics/information-required", h.GetInformationRequired)
		r.Get("/metrics", h.GetAllMetrics)
		r.Get("/metrics/dashboard", h.GetDashboardMetrics)
	})
}

func parsePeriod(r *http.Request) domain.MetricPeriod {
	period := r.URL.Query().Get("period")
	switch period {
	case "weekly":
		return domain.MetricPeriodWeekly
	case "monthly":
		return domain.MetricPeriodMonthly
	default:
		return domain.MetricPeriodDaily
	}
}

func parseBucket(r *http.Request) (time.Time, bool) {
	bucketStr := r.URL.Query().Get("bucket")
	if bucketStr == "" {
		return time.Now().UTC(), false
	}
	bucket, err := time.Parse(time.RFC3339, bucketStr)
	if err != nil {
		return time.Time{}, false
	}
	return bucket, true
}

func (h *Handler) GetCaseVolume(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}
	period := parsePeriod(r)
	bucket, _ := parseBucket(r)
	metric, err := h.svc.GetCaseVolume(r.Context(), orgID, period, bucket)
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "failed to calculate case volume")
		return
	}
	shared.WriteSuccess(w, http.StatusOK, metric, nil)
}

func (h *Handler) GetCaseVolumeByWorkflow(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}
	period := parsePeriod(r)
	bucket, _ := parseBucket(r)
	metrics, err := h.svc.GetCaseVolumeByWorkflow(r.Context(), orgID, period, bucket)
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "failed to calculate case volume by workflow")
		return
	}
	shared.WriteSuccess(w, http.StatusOK, metrics, nil)
}

func (h *Handler) GetCaseVolumeByState(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}
	period := parsePeriod(r)
	bucket, _ := parseBucket(r)
	metrics, err := h.svc.GetCaseVolumeByState(r.Context(), orgID, period, bucket)
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "failed to calculate case volume by state")
		return
	}
	shared.WriteSuccess(w, http.StatusOK, metrics, nil)
}

func (h *Handler) GetCaseVolumeByServiceType(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}
	period := parsePeriod(r)
	bucket, _ := parseBucket(r)
	metrics, err := h.svc.GetCaseVolumeByServiceType(r.Context(), orgID, period, bucket)
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "failed to calculate case volume by service type")
		return
	}
	shared.WriteSuccess(w, http.StatusOK, metrics, nil)
}

func (h *Handler) GetWorkflowThroughput(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}
	period := parsePeriod(r)
	bucket, _ := parseBucket(r)
	metrics, err := h.svc.GetWorkflowThroughput(r.Context(), orgID, period, bucket)
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "failed to calculate workflow throughput")
		return
	}
	shared.WriteSuccess(w, http.StatusOK, metrics, nil)
}

func (h *Handler) GetStateDuration(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}
	period := parsePeriod(r)
	bucket, _ := parseBucket(r)
	metrics, err := h.svc.GetStateDuration(r.Context(), orgID, period, bucket)
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "failed to calculate state duration")
		return
	}
	shared.WriteSuccess(w, http.StatusOK, metrics, nil)
}

func (h *Handler) GetCaseCycleTime(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}
	period := parsePeriod(r)
	bucket, _ := parseBucket(r)
	metrics, err := h.svc.GetCaseCycleTime(r.Context(), orgID, period, bucket)
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "failed to calculate case cycle time")
		return
	}
	shared.WriteSuccess(w, http.StatusOK, metrics, nil)
}

func (h *Handler) GetAgingCases(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}
	thresholdStr := r.URL.Query().Get("threshold_hours")
	if thresholdStr == "" {
		thresholdStr = "720"
	}
	thresholdHours, err := strconv.ParseFloat(thresholdStr, 64)
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid threshold_hours")
		return
	}
	metrics, err := h.svc.GetAgingCases(r.Context(), orgID, thresholdHours)
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "failed to query aging cases")
		return
	}
	shared.WriteSuccess(w, http.StatusOK, metrics, nil)
}

func (h *Handler) GetPendingReviews(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}
	metric, err := h.svc.GetPendingReviews(r.Context(), orgID)
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "failed to calculate pending reviews")
		return
	}
	shared.WriteSuccess(w, http.StatusOK, metric, nil)
}

func (h *Handler) GetDecisions(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}
	period := parsePeriod(r)
	bucket, _ := parseBucket(r)
	metric, err := h.svc.GetDecisions(r.Context(), orgID, period, bucket)
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "failed to calculate decisions")
		return
	}
	shared.WriteSuccess(w, http.StatusOK, metric, nil)
}

func (h *Handler) GetAssistanceOutcomes(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}
	period := parsePeriod(r)
	bucket, _ := parseBucket(r)
	metric, err := h.svc.GetAssistanceOutcomes(r.Context(), orgID, period, bucket)
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "failed to calculate assistance outcomes")
		return
	}
	shared.WriteSuccess(w, http.StatusOK, metric, nil)
}

func (h *Handler) GetEvidenceVerification(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}
	period := parsePeriod(r)
	bucket, _ := parseBucket(r)
	metric, err := h.svc.GetEvidenceVerification(r.Context(), orgID, period, bucket)
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "failed to calculate evidence verification")
		return
	}
	shared.WriteSuccess(w, http.StatusOK, metric, nil)
}

func (h *Handler) GetInformationRequired(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}
	period := parsePeriod(r)
	bucket, _ := parseBucket(r)
	metric, err := h.svc.GetInformationRequired(r.Context(), orgID, period, bucket)
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "failed to calculate information required")
		return
	}
	shared.WriteSuccess(w, http.StatusOK, metric, nil)
}

func (h *Handler) GetAllMetrics(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}
	period := parsePeriod(r)
	bucket, _ := parseBucket(r)
	metrics, err := h.svc.GetAllMetrics(r.Context(), orgID, period, bucket)
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "failed to calculate metrics")
		return
	}
	shared.WriteSuccess(w, http.StatusOK, metrics, nil)
}

func (h *Handler) GetDashboardMetrics(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}
	period := parsePeriod(r)
	bucket, _ := parseBucket(r)
	workflowKey := r.URL.Query().Get("workflow_key")
	status := r.URL.Query().Get("status")
	metrics, err := h.svc.GetDashboardMetrics(r.Context(), orgID, period, bucket, workflowKey, status)
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "failed to calculate dashboard metrics")
		return
	}
	shared.WriteSuccess(w, http.StatusOK, metrics, nil)
}
