package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/operations/domain"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type ExportHandler struct {
	exportService domain.ExportService
}

func NewExportHandler(exportService domain.ExportService) *ExportHandler {
	return &ExportHandler{exportService: exportService}
}

func (h *ExportHandler) RegisterRoutes(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/organizations/{orgId}/operations/export", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.RequireSameTenant)
		r.Use(middleware.RequireAnyRole("admin", "staff"))

		r.Get("/", h.GetExport)
	})
}

func (h *ExportHandler) GetExport(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "invalid organization ID")
		return
	}

	format := domain.ExportFormatJSON
	if f := r.URL.Query().Get("format"); f != "" {
		switch strings.ToLower(f) {
		case "csv":
			format = domain.ExportFormatCSV
		case "report":
			format = domain.ExportFormatReport
		}
	}

	scope := domain.ExportScopeAll
	if s := r.URL.Query().Get("scope"); s != "" {
		switch strings.ToLower(s) {
		case "operations":
			scope = domain.ExportScopeOperations
		case "analysis":
			scope = domain.ExportScopeAnalysis
		case "impact":
			scope = domain.ExportScopeImpact
		}
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

	bucket := time.Now().UTC()
	if b := r.URL.Query().Get("bucket"); b != "" {
		if parsed, err := time.Parse(time.RFC3339, b); err == nil {
			bucket = parsed
		}
	}

	workflowKey := r.URL.Query().Get("workflow_key")
	status := r.URL.Query().Get("status")

	exportReq := domain.ExportRequest{
		Format:      format,
		Scope:       scope,
		Period:      period,
		Bucket:      bucket,
		WorkflowKey: workflowKey,
		Status:      status,
	}

	result, err := h.exportService.GenerateExport(r.Context(), orgID, exportReq)
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "failed to generate export")
		return
	}

	var body []byte
	switch exportReq.Format {
	case domain.ExportFormatCSV:
		body, err = result.ToCSV()
		if err != nil {
			shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "failed to generate CSV export")
			return
		}
	case domain.ExportFormatReport:
		body, err = result.ToReport()
		if err != nil {
			shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "failed to generate report export")
			return
		}
	default:
		body, err = result.ToJSON()
		if err != nil {
			shared.WriteError(w, http.StatusInternalServerError, shared.CodeInternalError, "failed to generate JSON export")
			return
		}
	}

	w.Header().Set("Content-Type", result.ContentType)
	w.Header().Set("Content-Disposition", "attachment; filename="+result.Filename)
	w.Header().Set("X-Export-Generated-At", result.Metadata.GeneratedAt.Format(time.RFC3339))
	w.Header().Set("X-Export-Scope", string(result.Metadata.OrganizationScope))
	w.Header().Set("X-Export-Metric-Version", result.Metadata.MetricVersion)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
