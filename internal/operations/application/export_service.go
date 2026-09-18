package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/alrazihi/civora/internal/operations/domain"
	"github.com/google/uuid"
)

type ExportService struct {
	operationsService *OperationsService
	analysisService   domain.AnalysisService
	impactService     domain.ImpactService
}

func NewExportService(
	operationsService *OperationsService,
	analysisService domain.AnalysisService,
	impactService domain.ImpactService,
) *ExportService {
	return &ExportService{
		operationsService: operationsService,
		analysisService:   analysisService,
		impactService:     impactService,
	}
}

func (s *ExportService) GenerateExport(ctx context.Context, orgID uuid.UUID, req domain.ExportRequest) (*domain.ExportResult, error) {
	switch req.Scope {
	case domain.ExportScopeOperations, domain.ExportScopeAnalysis, domain.ExportScopeImpact, domain.ExportScopeAll:
	default:
		return nil, fmt.Errorf("invalid export scope: %s", req.Scope)
	}

	bucketStart := domain.BucketStart(req.Bucket, req.Period)
	dataSources := []string{}

	exportData := map[string]interface{}{}

	if req.Scope == domain.ExportScopeOperations || req.Scope == domain.ExportScopeAll {
		metrics, err := s.operationsService.GetAllMetrics(ctx, orgID, req.Period, bucketStart)
		if err != nil {
			return nil, err
		}
		exportData["operations_metrics"] = sanitizeMetricsForExport(metrics)
		dataSources = append(dataSources, "operations_metrics")
	}

	if req.Scope == domain.ExportScopeAnalysis || req.Scope == domain.ExportScopeAll {
		thresholds := domain.DefaultAnalysisThresholds
		report, err := s.analysisService.GetWorkflowAnalysisReport(ctx, orgID, thresholds)
		if err != nil {
			return nil, err
		}
		exportData["workflow_analysis"] = sanitizeAnalysisForExport(report)
		dataSources = append(dataSources, "workflow_analysis")
	}

	if req.Scope == domain.ExportScopeImpact || req.Scope == domain.ExportScopeAll {
		report, err := s.impactService.GetImpactIntelligenceReport(ctx, orgID, req.Period, bucketStart)
		if err != nil {
			return nil, err
		}
		exportData["impact_intelligence"] = sanitizeImpactForExport(report)
		dataSources = append(dataSources, "impact_intelligence")
	}

	metadata := domain.BuildExportMetadata(orgID, req.Scope, req.Period, bucketStart, req.WorkflowKey, req.Status, dataSources)

	rawBody, err := json.Marshal(exportData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal export data: %w", err)
	}

	result := &domain.ExportResult{
		Metadata: metadata,
		Body:     rawBody,
	}

	switch req.Format {
	case domain.ExportFormatCSV:
		result.ContentType = "text/csv"
		result.Filename = buildFilename("csv", req.Scope, req.Period, bucketStart)
		body, err := result.ToCSV()
		if err != nil {
			return nil, err
		}
		result.Body = body
	case domain.ExportFormatReport:
		result.ContentType = "text/markdown; charset=utf-8"
		result.Filename = buildFilename("md", req.Scope, req.Period, bucketStart)
		body, err := result.ToReport()
		if err != nil {
			return nil, err
		}
		result.Body = body
	default:
		result.ContentType = "application/json"
		result.Filename = buildFilename("json", req.Scope, req.Period, bucketStart)
		body, err := result.ToJSON()
		if err != nil {
			return nil, err
		}
		result.Body = body
	}

	return result, nil
}

func sanitizeMetricsForExport(metrics *domain.OperationsMetrics) map[string]interface{} {
	if metrics == nil {
		return nil
	}
	data := map[string]interface{}{
		"organization_id": metrics.OrganizationID,
		"period":          metrics.Period,
		"calculated_at":   metrics.CalculatedAt,
	}
	if metrics.CaseVolume != nil {
		data["case_volume"] = sanitizeCaseVolumeForExport(metrics.CaseVolume)
	}
	if metrics.WorkflowThroughput != nil {
		data["workflow_throughput"] = sanitizeWorkflowThroughputForExport(metrics.WorkflowThroughput)
	}
	if metrics.StateDuration != nil {
		data["state_duration"] = sanitizeStateDurationForExport(metrics.StateDuration)
	}
	if metrics.CaseCycleTime != nil {
		data["case_cycle_time"] = sanitizeCaseCycleTimeForExport(metrics.CaseCycleTime)
	}
	if metrics.AgingCases != nil {
		data["aging_cases"] = sanitizeAgingCasesForExport(metrics.AgingCases)
	}
	if metrics.PendingReviews != nil {
		data["pending_reviews"] = sanitizePendingReviewsForExport(metrics.PendingReviews)
	}
	if metrics.Decisions != nil {
		data["decisions"] = sanitizeDecisionsForExport(metrics.Decisions)
	}
	if metrics.AssistanceOutcomes != nil {
		data["assistance_outcomes"] = sanitizeAssistanceOutcomesForExport(metrics.AssistanceOutcomes)
	}
	if metrics.EvidenceVerification != nil {
		data["evidence_verification"] = sanitizeEvidenceVerificationForExport(metrics.EvidenceVerification)
	}
	if metrics.InformationRequired != nil {
		data["information_required"] = sanitizeInformationRequiredForExport(metrics.InformationRequired)
	}
	return data
}

func sanitizeCaseVolumeForExport(m *domain.CaseVolumeMetric) map[string]interface{} {
	return map[string]interface{}{
		"total_cases":     m.TotalCases,
		"new_cases":       m.NewCases,
		"open_cases":      m.OpenCases,
		"closed_cases":    m.ClosedCases,
		"rejected_cases":  m.RejectedCases,
		"by_status":       m.ByStatus,
		"by_service_type": m.ByServiceType,
		"by_workflow_key": m.ByWorkflowKey,
		"by_state":        m.ByState,
		"calculated_at":   m.CalculatedAt,
	}
}

func sanitizeWorkflowThroughputForExport(m *domain.WorkflowThroughputMetric) map[string]interface{} {
	return map[string]interface{}{
		"workflow_key":             m.WorkflowKey,
		"total_transitions":        m.TotalTransitions,
		"unique_cases":             m.UniqueCases,
		"avg_transitions_per_case": m.AvgTransitionsPerCase,
		"calculated_at":            m.CalculatedAt,
	}
}

func sanitizeStateDurationForExport(m *domain.StateDurationMetric) map[string]interface{} {
	return map[string]interface{}{
		"workflow_key":          m.WorkflowKey,
		"state_key":             m.StateKey,
		"entry_count":           m.EntryCount,
		"avg_duration_hours":    m.AvgDurationHours,
		"median_duration_hours": m.MedianDurationHours,
		"max_duration_hours":    m.MaxDurationHours,
		"calculated_at":         m.CalculatedAt,
	}
}

func sanitizeCaseCycleTimeForExport(m *domain.CaseCycleTimeMetric) map[string]interface{} {
	return map[string]interface{}{
		"workflow_key":            m.WorkflowKey,
		"completed_cases":         m.CompletedCases,
		"avg_cycle_time_hours":    m.AvgCycleTimeHours,
		"median_cycle_time_hours": m.MedianCycleTimeHours,
		"calculated_at":           m.CalculatedAt,
	}
}

func sanitizeAgingCasesForExport(cases []*domain.AgingCaseMetric) []map[string]interface{} {
	result := make([]map[string]interface{}, len(cases))
	for i, c := range cases {
		result[i] = map[string]interface{}{
			"workflow_key":           c.WorkflowKey,
			"current_state":          c.CurrentState,
			"service_type":           c.ServiceType,
			"priority":               c.Priority,
			"age_hours":              c.AgeHours,
			"in_current_state_hours": c.InCurrentStateHours,
			"calculated_at":          c.CalculatedAt,
		}
	}
	return result
}

func sanitizePendingReviewsForExport(m *domain.PendingReviewMetric) map[string]interface{} {
	return map[string]interface{}{
		"pending_reviews":      m.PendingReviews,
		"assigned_reviews":     m.AssignedReviews,
		"in_review_reviews":    m.InReviewReviews,
		"waiting_info_reviews": m.WaitingInfoReviews,
		"avg_wait_time_hours":  m.AvgWaitTimeHours,
		"calculated_at":        m.CalculatedAt,
	}
}

func sanitizeDecisionsForExport(m *domain.DecisionMetric) map[string]interface{} {
	return map[string]interface{}{
		"total_decisions":            m.TotalDecisions,
		"approved":                   m.Approved,
		"rejected":                   m.Rejected,
		"needs_more_info":            m.NeedsMoreInfo,
		"escalated":                  m.Escalated,
		"approval_rate":              m.ApprovalRate,
		"avg_decisions_per_reviewer": m.AvgDecisionsPerReviewer,
		"calculated_at":              m.CalculatedAt,
	}
}

func sanitizeAssistanceOutcomesForExport(m *domain.AssistanceOutcomeMetric) map[string]interface{} {
	return map[string]interface{}{
		"total_assistance": m.TotalAssistance,
		"planned":          m.Planned,
		"in_progress":      m.InProgress,
		"completed":        m.Completed,
		"cancelled":        m.Cancelled,
		"completion_rate":  m.CompletionRate,
		"calculated_at":    m.CalculatedAt,
	}
}

func sanitizeEvidenceVerificationForExport(m *domain.EvidenceVerificationMetric) map[string]interface{} {
	return map[string]interface{}{
		"total_evidence":    m.TotalEvidence,
		"verified":          m.Verified,
		"rejected":          m.Rejected,
		"needs_review":      m.NeedsReview,
		"unverified":        m.Unverified,
		"verification_rate": m.VerificationRate,
		"calculated_at":     m.CalculatedAt,
	}
}

func sanitizeInformationRequiredForExport(m *domain.InformationRequiredMetric) map[string]interface{} {
	return map[string]interface{}{
		"total_cases":           m.TotalCases,
		"information_required":  m.InformationRequired,
		"escalated":             m.Escalated,
		"awaiting_info_reviews": m.AwaitingInfoReviews,
		"calculated_at":         m.CalculatedAt,
	}
}

func sanitizeAnalysisForExport(report *domain.WorkflowAnalysisReport) map[string]interface{} {
	if report == nil {
		return nil
	}
	return map[string]interface{}{
		"organization_id":              report.OrganizationID,
		"calculated_at":                report.CalculatedAt,
		"period":                       report.Period,
		"state_accumulations":          report.StateAccumulations,
		"state_duration_anomalies":     report.StateDurationAnomalies,
		"workflow_closure_patterns":    report.WorkflowClosurePatterns,
		"information_request_patterns": report.InformationRequestPatterns,
		"review_backlog_observations":  report.ReviewBacklogObservations,
		"threshold_exceedances":        report.ThresholdExceedances,
	}
}

func sanitizeImpactForExport(report *domain.ImpactIntelligenceReport) map[string]interface{} {
	if report == nil {
		return nil
	}
	return map[string]interface{}{
		"organization_id": report.OrganizationID,
		"calculated_at":   report.CalculatedAt,
		"period":          report.Period,
		"bucket":          report.Bucket,
		"metrics":         report.Metrics,
	}
}

func buildFilename(ext string, scope domain.ExportScope, period domain.MetricPeriod, bucket time.Time) string {
	bucketStr := bucket.Format("2006-01-02")
	return fmt.Sprintf("civora-%s-%s-%s.%s", scope, period, bucketStr, ext)
}
