package application

import (
	"context"
	"testing"
	"time"

	"github.com/alrazihi/civora/internal/operations/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockAnalysisService struct {
	mock.Mock
}

func (m *mockAnalysisService) GetWorkflowAnalysisReport(ctx context.Context, orgID uuid.UUID, thresholds domain.AnalysisThresholds) (*domain.WorkflowAnalysisReport, error) {
	args := m.Called(ctx, orgID, thresholds)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.WorkflowAnalysisReport), args.Error(1)
}

type mockImpactService struct {
	mock.Mock
}

func (m *mockImpactService) GetImpactIntelligenceReport(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) (*domain.ImpactIntelligenceReport, error) {
	args := m.Called(ctx, orgID, period, bucket)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ImpactIntelligenceReport), args.Error(1)
}

func TestExportService_GenerateExport_JSON(t *testing.T) {
	repo := new(mockMetricsRepo)
	analysisSvc := new(mockAnalysisService)
	impactSvc := new(mockImpactService)
	svc := NewExportService(NewOperationsService(repo), analysisSvc, impactSvc)

	orgID := uuid.New()
	period := domain.MetricPeriodDaily
	bucket := time.Now().UTC()

	repo.On("GetCaseVolume", mock.Anything, orgID, period, bucket).Return(&domain.CaseVolumeMetric{
		OrganizationID: orgID,
		Period:         period,
		Bucket:         bucket,
		TotalCases:     10,
		CalculatedAt:   bucket,
	}, nil)
	repo.On("GetWorkflowThroughput", mock.Anything, orgID, period, bucket).Return([]*domain.WorkflowThroughputMetric{{WorkflowKey: "wf", TotalTransitions: 5, CalculatedAt: bucket}}, nil)
	repo.On("GetStateDuration", mock.Anything, orgID, period, bucket).Return([]*domain.StateDurationMetric{{StateKey: "NEW", EntryCount: 10, CalculatedAt: bucket}}, nil)
	repo.On("GetCaseCycleTime", mock.Anything, orgID, period, bucket).Return([]*domain.CaseCycleTimeMetric{{CompletedCases: 5, CalculatedAt: bucket}}, nil)
	repo.On("GetAgingCases", mock.Anything, orgID, float64(720)).Return([]*domain.AgingCaseMetric{{CaseNumber: "C1", AgeHours: 48, CalculatedAt: bucket}}, nil)
	repo.On("GetPendingReviews", mock.Anything, orgID).Return(&domain.PendingReviewMetric{PendingReviews: 3, CalculatedAt: bucket}, nil)
	repo.On("GetDecisions", mock.Anything, orgID, period, bucket).Return(&domain.DecisionMetric{TotalDecisions: 10, CalculatedAt: bucket}, nil)
	repo.On("GetAssistanceOutcomes", mock.Anything, orgID, period, bucket).Return(&domain.AssistanceOutcomeMetric{TotalAssistance: 5, CalculatedAt: bucket}, nil)
	repo.On("GetEvidenceVerification", mock.Anything, orgID, period, bucket).Return(&domain.EvidenceVerificationMetric{TotalEvidence: 20, CalculatedAt: bucket}, nil)
	repo.On("GetInformationRequired", mock.Anything, orgID, period, bucket).Return(&domain.InformationRequiredMetric{TotalCases: 2, CalculatedAt: bucket}, nil)

	analysisSvc.On("GetWorkflowAnalysisReport", mock.Anything, orgID, domain.DefaultAnalysisThresholds).Return(&domain.WorkflowAnalysisReport{Period: "daily", CalculatedAt: bucket}, nil)
	impactSvc.On("GetImpactIntelligenceReport", mock.Anything, orgID, period, bucket).Return(&domain.ImpactIntelligenceReport{Period: period, Bucket: bucket, Metrics: []*domain.OutcomeMetric{{Name: "case_completion_rate", ImpactValue: 0.6}}}, nil)

	result, err := svc.GenerateExport(context.Background(), orgID, domain.ExportRequest{
		Format: domain.ExportFormatJSON,
		Scope:  domain.ExportScopeAll,
		Period: period,
		Bucket: bucket,
	})

	require.NoError(t, err)
	assert.Equal(t, "application/json", result.ContentType)
	assert.Contains(t, result.Filename, ".json")
	assert.NotEmpty(t, result.Body)
	assert.Equal(t, orgID, result.Metadata.OrganizationID)
	assert.Equal(t, domain.ExportMetricVersion, result.Metadata.MetricVersion)
	assert.Contains(t, result.Metadata.DataSources, "operations_metrics")
	assert.Contains(t, result.Metadata.DataSources, "workflow_analysis")
	assert.Contains(t, result.Metadata.DataSources, "impact_intelligence")
}

func TestExportService_GenerateExport_CSV(t *testing.T) {
	repo := new(mockMetricsRepo)
	analysisSvc := new(mockAnalysisService)
	impactSvc := new(mockImpactService)
	svc := NewExportService(NewOperationsService(repo), analysisSvc, impactSvc)

	orgID := uuid.New()
	period := domain.MetricPeriodDaily
	bucket := time.Now().UTC()

	repo.On("GetCaseVolume", mock.Anything, orgID, period, bucket).Return(&domain.CaseVolumeMetric{
		OrganizationID: orgID,
		Period:         period,
		Bucket:         bucket,
		TotalCases:     10,
		CalculatedAt:   bucket,
	}, nil)
	repo.On("GetWorkflowThroughput", mock.Anything, orgID, period, bucket).Return([]*domain.WorkflowThroughputMetric{{WorkflowKey: "wf", TotalTransitions: 5, CalculatedAt: bucket}}, nil)
	repo.On("GetStateDuration", mock.Anything, orgID, period, bucket).Return([]*domain.StateDurationMetric{{StateKey: "NEW", EntryCount: 10, CalculatedAt: bucket}}, nil)
	repo.On("GetCaseCycleTime", mock.Anything, orgID, period, bucket).Return([]*domain.CaseCycleTimeMetric{{CompletedCases: 5, CalculatedAt: bucket}}, nil)
	repo.On("GetAgingCases", mock.Anything, orgID, float64(720)).Return([]*domain.AgingCaseMetric{{CaseNumber: "C1", AgeHours: 48, CalculatedAt: bucket}}, nil)
	repo.On("GetPendingReviews", mock.Anything, orgID).Return(&domain.PendingReviewMetric{PendingReviews: 3, CalculatedAt: bucket}, nil)
	repo.On("GetDecisions", mock.Anything, orgID, period, bucket).Return(&domain.DecisionMetric{TotalDecisions: 10, CalculatedAt: bucket}, nil)
	repo.On("GetAssistanceOutcomes", mock.Anything, orgID, period, bucket).Return(&domain.AssistanceOutcomeMetric{TotalAssistance: 5, CalculatedAt: bucket}, nil)
	repo.On("GetEvidenceVerification", mock.Anything, orgID, period, bucket).Return(&domain.EvidenceVerificationMetric{TotalEvidence: 20, CalculatedAt: bucket}, nil)
	repo.On("GetInformationRequired", mock.Anything, orgID, period, bucket).Return(&domain.InformationRequiredMetric{TotalCases: 2, CalculatedAt: bucket}, nil)

	result, err := svc.GenerateExport(context.Background(), orgID, domain.ExportRequest{
		Format: domain.ExportFormatCSV,
		Scope:  domain.ExportScopeOperations,
		Period: period,
		Bucket: bucket,
	})

	require.NoError(t, err)
	assert.Equal(t, "text/csv", result.ContentType)
	assert.Contains(t, result.Filename, ".csv")
	assert.NotEmpty(t, result.Body)
	assert.Contains(t, string(result.Body), "section,metric,value,unit,category")
}

func TestExportService_GenerateExport_Report(t *testing.T) {
	repo := new(mockMetricsRepo)
	analysisSvc := new(mockAnalysisService)
	impactSvc := new(mockImpactService)
	svc := NewExportService(NewOperationsService(repo), analysisSvc, impactSvc)

	orgID := uuid.New()
	period := domain.MetricPeriodDaily
	bucket := time.Now().UTC()

	repo.On("GetCaseVolume", mock.Anything, orgID, period, bucket).Return(&domain.CaseVolumeMetric{
		OrganizationID: orgID,
		Period:         period,
		Bucket:         bucket,
		TotalCases:     10,
		CalculatedAt:   bucket,
	}, nil)
	repo.On("GetWorkflowThroughput", mock.Anything, orgID, period, bucket).Return([]*domain.WorkflowThroughputMetric{{WorkflowKey: "wf", TotalTransitions: 5, CalculatedAt: bucket}}, nil)
	repo.On("GetStateDuration", mock.Anything, orgID, period, bucket).Return([]*domain.StateDurationMetric{{StateKey: "NEW", EntryCount: 10, CalculatedAt: bucket}}, nil)
	repo.On("GetCaseCycleTime", mock.Anything, orgID, period, bucket).Return([]*domain.CaseCycleTimeMetric{{CompletedCases: 5, CalculatedAt: bucket}}, nil)
	repo.On("GetAgingCases", mock.Anything, orgID, float64(720)).Return([]*domain.AgingCaseMetric{{CaseNumber: "C1", AgeHours: 48, CalculatedAt: bucket}}, nil)
	repo.On("GetPendingReviews", mock.Anything, orgID).Return(&domain.PendingReviewMetric{PendingReviews: 3, CalculatedAt: bucket}, nil)
	repo.On("GetDecisions", mock.Anything, orgID, period, bucket).Return(&domain.DecisionMetric{TotalDecisions: 10, CalculatedAt: bucket}, nil)
	repo.On("GetAssistanceOutcomes", mock.Anything, orgID, period, bucket).Return(&domain.AssistanceOutcomeMetric{TotalAssistance: 5, CalculatedAt: bucket}, nil)
	repo.On("GetEvidenceVerification", mock.Anything, orgID, period, bucket).Return(&domain.EvidenceVerificationMetric{TotalEvidence: 20, CalculatedAt: bucket}, nil)
	repo.On("GetInformationRequired", mock.Anything, orgID, period, bucket).Return(&domain.InformationRequiredMetric{TotalCases: 2, CalculatedAt: bucket}, nil)

	result, err := svc.GenerateExport(context.Background(), orgID, domain.ExportRequest{
		Format: domain.ExportFormatReport,
		Scope:  domain.ExportScopeAll,
		Period: period,
		Bucket: bucket,
	})

	require.NoError(t, err)
	assert.Equal(t, "text/markdown; charset=utf-8", result.ContentType)
	assert.Contains(t, result.Filename, ".md")
	assert.NotEmpty(t, result.Body)
	assert.Contains(t, string(result.Body), "# CIVORA")
	assert.Contains(t, string(result.Body), "## Filters")
}

func TestExportService_SanitizeSensitiveFields(t *testing.T) {
	input := map[string]interface{}{
		"password":     "secret123",
		"api_key":      "key123",
		"database_url": "postgres://localhost/db",
		"home_dir":     "/home/user",
		"total_cases":  10,
		"workflow_key": "emergency",
		"nested": map[string]interface{}{
			"token": "tok123",
			"label": "Cases",
			"path":  "/tmp",
			"count": 5,
		},
	}

	sanitized := domain.SanitizeExportData(input).(map[string]interface{})
	assert.Equal(t, "[REDACTED]", sanitized["password"])
	assert.Equal(t, "[REDACTED]", sanitized["api_key"])
	assert.Equal(t, "[REDACTED]", sanitized["database_url"])
	assert.Equal(t, "[REDACTED]", sanitized["home_dir"])
	assert.Equal(t, float64(10), sanitized["total_cases"])
	assert.Equal(t, "emergency", sanitized["workflow_key"])

	nested := sanitized["nested"].(map[string]interface{})
	assert.Equal(t, "[REDACTED]", nested["token"])
	assert.Equal(t, "Cases", nested["label"])
	assert.Equal(t, "[REDACTED]", nested["path"])
	assert.Equal(t, float64(5), nested["count"])
}

func TestExportService_CrossTenantIsolation(t *testing.T) {
	repo := new(mockMetricsRepo)
	analysisSvc := new(mockAnalysisService)
	impactSvc := new(mockImpactService)
	svc := NewExportService(NewOperationsService(repo), analysisSvc, impactSvc)

	orgA := uuid.New()
	orgB := uuid.New()
	period := domain.MetricPeriodDaily
	bucket := time.Now().UTC()

	repo.On("GetCaseVolume", mock.Anything, orgA, period, bucket).Return(&domain.CaseVolumeMetric{
		OrganizationID: orgA, Period: period, Bucket: bucket, TotalCases: 10, CalculatedAt: bucket,
	}, nil)
	repo.On("GetWorkflowThroughput", mock.Anything, orgA, period, bucket).Return([]*domain.WorkflowThroughputMetric{{WorkflowKey: "wf", TotalTransitions: 5, CalculatedAt: bucket}}, nil)
	repo.On("GetStateDuration", mock.Anything, orgA, period, bucket).Return([]*domain.StateDurationMetric{{StateKey: "NEW", EntryCount: 10, CalculatedAt: bucket}}, nil)
	repo.On("GetCaseCycleTime", mock.Anything, orgA, period, bucket).Return([]*domain.CaseCycleTimeMetric{{CompletedCases: 5, CalculatedAt: bucket}}, nil)
	repo.On("GetAgingCases", mock.Anything, orgA, float64(720)).Return([]*domain.AgingCaseMetric{{CaseNumber: "C1", AgeHours: 48, CalculatedAt: bucket}}, nil)
	repo.On("GetPendingReviews", mock.Anything, orgA).Return(&domain.PendingReviewMetric{PendingReviews: 3, CalculatedAt: bucket}, nil)
	repo.On("GetDecisions", mock.Anything, orgA, period, bucket).Return(&domain.DecisionMetric{TotalDecisions: 10, CalculatedAt: bucket}, nil)
	repo.On("GetAssistanceOutcomes", mock.Anything, orgA, period, bucket).Return(&domain.AssistanceOutcomeMetric{TotalAssistance: 5, CalculatedAt: bucket}, nil)
	repo.On("GetEvidenceVerification", mock.Anything, orgA, period, bucket).Return(&domain.EvidenceVerificationMetric{TotalEvidence: 20, CalculatedAt: bucket}, nil)
	repo.On("GetInformationRequired", mock.Anything, orgA, period, bucket).Return(&domain.InformationRequiredMetric{TotalCases: 2, CalculatedAt: bucket}, nil)

	resultA, err := svc.GenerateExport(context.Background(), orgA, domain.ExportRequest{
		Format: domain.ExportFormatJSON, Scope: domain.ExportScopeOperations, Period: period, Bucket: bucket,
	})
	require.NoError(t, err)
	assert.Equal(t, orgA, resultA.Metadata.OrganizationID)

	repo.On("GetCaseVolume", mock.Anything, orgB, period, bucket).Return(&domain.CaseVolumeMetric{
		OrganizationID: orgB, Period: period, Bucket: bucket, TotalCases: 20, CalculatedAt: bucket,
	}, nil)
	repo.On("GetWorkflowThroughput", mock.Anything, orgB, period, bucket).Return([]*domain.WorkflowThroughputMetric{{WorkflowKey: "wf", TotalTransitions: 15, CalculatedAt: bucket}}, nil)
	repo.On("GetStateDuration", mock.Anything, orgB, period, bucket).Return([]*domain.StateDurationMetric{{StateKey: "NEW", EntryCount: 20, CalculatedAt: bucket}}, nil)
	repo.On("GetCaseCycleTime", mock.Anything, orgB, period, bucket).Return([]*domain.CaseCycleTimeMetric{{CompletedCases: 15, CalculatedAt: bucket}}, nil)
	repo.On("GetAgingCases", mock.Anything, orgB, float64(720)).Return([]*domain.AgingCaseMetric{{CaseNumber: "C2", AgeHours: 24, CalculatedAt: bucket}}, nil)
	repo.On("GetPendingReviews", mock.Anything, orgB).Return(&domain.PendingReviewMetric{PendingReviews: 5, CalculatedAt: bucket}, nil)
	repo.On("GetDecisions", mock.Anything, orgB, period, bucket).Return(&domain.DecisionMetric{TotalDecisions: 20, CalculatedAt: bucket}, nil)
	repo.On("GetAssistanceOutcomes", mock.Anything, orgB, period, bucket).Return(&domain.AssistanceOutcomeMetric{TotalAssistance: 10, CalculatedAt: bucket}, nil)
	repo.On("GetEvidenceVerification", mock.Anything, orgB, period, bucket).Return(&domain.EvidenceVerificationMetric{TotalEvidence: 40, CalculatedAt: bucket}, nil)
	repo.On("GetInformationRequired", mock.Anything, orgB, period, bucket).Return(&domain.InformationRequiredMetric{TotalCases: 4, CalculatedAt: bucket}, nil)

	resultB, err := svc.GenerateExport(context.Background(), orgB, domain.ExportRequest{
		Format: domain.ExportFormatJSON, Scope: domain.ExportScopeOperations, Period: period, Bucket: bucket,
	})
	require.NoError(t, err)
	assert.Equal(t, orgB, resultB.Metadata.OrganizationID)
	assert.Contains(t, string(resultB.Body), `"total_cases":20`)
	assert.NotContains(t, string(resultB.Body), `"total_cases":10`)
}

func TestExportService_LargeExportBehavior(t *testing.T) {
	repo := new(mockMetricsRepo)
	analysisSvc := new(mockAnalysisService)
	impactSvc := new(mockImpactService)
	svc := NewExportService(NewOperationsService(repo), analysisSvc, impactSvc)

	orgID := uuid.New()
	period := domain.MetricPeriodDaily
	bucket := time.Now().UTC()

	largeAgingCases := make([]*domain.AgingCaseMetric, 1000)
	for i := 0; i < 1000; i++ {
		largeAgingCases[i] = &domain.AgingCaseMetric{
			CaseNumber:          "C" + string(rune('A'+i%26)) + string(rune('0'+i/26%10)),
			WorkflowKey:         "wf",
			CurrentState:        "IN_PROGRESS",
			ServiceType:         "EMERGENCY",
			AgeHours:            float64(i),
			InCurrentStateHours: float64(i % 100),
			CalculatedAt:        bucket,
		}
	}

	repo.On("GetCaseVolume", mock.Anything, orgID, period, bucket).Return(&domain.CaseVolumeMetric{
		OrganizationID: orgID, Period: period, Bucket: bucket, TotalCases: 1000, CalculatedAt: bucket,
	}, nil)
	repo.On("GetWorkflowThroughput", mock.Anything, orgID, period, bucket).Return([]*domain.WorkflowThroughputMetric{{WorkflowKey: "wf", TotalTransitions: 5000, CalculatedAt: bucket}}, nil)
	repo.On("GetStateDuration", mock.Anything, orgID, period, bucket).Return([]*domain.StateDurationMetric{{StateKey: "NEW", EntryCount: 1000, CalculatedAt: bucket}}, nil)
	repo.On("GetCaseCycleTime", mock.Anything, orgID, period, bucket).Return([]*domain.CaseCycleTimeMetric{{CompletedCases: 500, CalculatedAt: bucket}}, nil)
	repo.On("GetAgingCases", mock.Anything, orgID, float64(720)).Return(largeAgingCases, nil)
	repo.On("GetPendingReviews", mock.Anything, orgID).Return(&domain.PendingReviewMetric{PendingReviews: 100, CalculatedAt: bucket}, nil)
	repo.On("GetDecisions", mock.Anything, orgID, period, bucket).Return(&domain.DecisionMetric{TotalDecisions: 1000, CalculatedAt: bucket}, nil)
	repo.On("GetAssistanceOutcomes", mock.Anything, orgID, period, bucket).Return(&domain.AssistanceOutcomeMetric{TotalAssistance: 500, CalculatedAt: bucket}, nil)
	repo.On("GetEvidenceVerification", mock.Anything, orgID, period, bucket).Return(&domain.EvidenceVerificationMetric{TotalEvidence: 2000, CalculatedAt: bucket}, nil)
	repo.On("GetInformationRequired", mock.Anything, orgID, period, bucket).Return(&domain.InformationRequiredMetric{TotalCases: 200, CalculatedAt: bucket}, nil)

	result, err := svc.GenerateExport(context.Background(), orgID, domain.ExportRequest{
		Format: domain.ExportFormatJSON, Scope: domain.ExportScopeOperations, Period: period, Bucket: bucket,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, result.Body)
	assert.Contains(t, string(result.Body), `"aging_cases"`)
}

func TestExportService_UnauthorizedScope(t *testing.T) {
	repo := new(mockMetricsRepo)
	analysisSvc := new(mockAnalysisService)
	impactSvc := new(mockImpactService)
	svc := NewExportService(NewOperationsService(repo), analysisSvc, impactSvc)

	orgID := uuid.New()
	period := domain.MetricPeriodDaily
	bucket := time.Now().UTC()

	result, err := svc.GenerateExport(context.Background(), orgID, domain.ExportRequest{
		Format: domain.ExportFormatJSON,
		Scope:  "invalid",
		Period: period,
		Bucket: bucket,
	})

	assert.Error(t, err)
	assert.Nil(t, result)
}
