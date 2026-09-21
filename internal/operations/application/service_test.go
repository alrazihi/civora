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

type mockMetricsRepo struct {
	mock.Mock
}

func (m *mockMetricsRepo) GetCaseVolume(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) (*domain.CaseVolumeMetric, error) {
	args := m.Called(ctx, orgID, period, bucket)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.CaseVolumeMetric), args.Error(1)
}

func (m *mockMetricsRepo) GetCaseVolumeByWorkflow(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) ([]*domain.CaseVolumeMetric, error) {
	args := m.Called(ctx, orgID, period, bucket)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.CaseVolumeMetric), args.Error(1)
}

func (m *mockMetricsRepo) GetCaseVolumeByState(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) ([]*domain.CaseVolumeMetric, error) {
	args := m.Called(ctx, orgID, period, bucket)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.CaseVolumeMetric), args.Error(1)
}

func (m *mockMetricsRepo) GetCaseVolumeByServiceType(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) ([]*domain.CaseVolumeMetric, error) {
	args := m.Called(ctx, orgID, period, bucket)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.CaseVolumeMetric), args.Error(1)
}

func (m *mockMetricsRepo) GetWorkflowThroughput(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) ([]*domain.WorkflowThroughputMetric, error) {
	args := m.Called(ctx, orgID, period, bucket)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.WorkflowThroughputMetric), args.Error(1)
}

func (m *mockMetricsRepo) GetStateDuration(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) ([]*domain.StateDurationMetric, error) {
	args := m.Called(ctx, orgID, period, bucket)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.StateDurationMetric), args.Error(1)
}

func (m *mockMetricsRepo) GetCaseCycleTime(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) ([]*domain.CaseCycleTimeMetric, error) {
	args := m.Called(ctx, orgID, period, bucket)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.CaseCycleTimeMetric), args.Error(1)
}

func (m *mockMetricsRepo) GetAgingCases(ctx context.Context, orgID uuid.UUID, thresholdHours float64) ([]*domain.AgingCaseMetric, error) {
	args := m.Called(ctx, orgID, thresholdHours)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AgingCaseMetric), args.Error(1)
}

func (m *mockMetricsRepo) GetPendingReviews(ctx context.Context, orgID uuid.UUID) (*domain.PendingReviewMetric, error) {
	args := m.Called(ctx, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PendingReviewMetric), args.Error(1)
}

func (m *mockMetricsRepo) GetDecisions(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) (*domain.DecisionMetric, error) {
	args := m.Called(ctx, orgID, period, bucket)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.DecisionMetric), args.Error(1)
}

func (m *mockMetricsRepo) GetAssistanceOutcomes(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) (*domain.AssistanceOutcomeMetric, error) {
	args := m.Called(ctx, orgID, period, bucket)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AssistanceOutcomeMetric), args.Error(1)
}

func (m *mockMetricsRepo) GetEvidenceVerification(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) (*domain.EvidenceVerificationMetric, error) {
	args := m.Called(ctx, orgID, period, bucket)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EvidenceVerificationMetric), args.Error(1)
}

func (m *mockMetricsRepo) GetInformationRequired(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) (*domain.InformationRequiredMetric, error) {
	args := m.Called(ctx, orgID, period, bucket)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.InformationRequiredMetric), args.Error(1)
}

func (m *mockMetricsRepo) GetDashboardMetrics(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time, workflowKey string, status string) (*domain.OperationsMetrics, error) {
	args := m.Called(ctx, orgID, period, bucket, workflowKey, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.OperationsMetrics), args.Error(1)
}

func TestOperationsService_GetCaseVolume(t *testing.T) {
	repo := new(mockMetricsRepo)
	svc := NewOperationsService(repo)

	orgID := uuid.New()
	period := domain.MetricPeriodDaily
	bucket := time.Now().UTC()
	expected := &domain.CaseVolumeMetric{
		OrganizationID: orgID,
		Period:         period,
		Bucket:         bucket,
		TotalCases:     10,
		CalculatedAt:   time.Now().UTC(),
	}

	repo.On("GetCaseVolume", mock.Anything, orgID, period, bucket).Return(expected, nil)

	result, err := svc.GetCaseVolume(context.Background(), orgID, period, bucket)
	require.NoError(t, err)
	assert.Equal(t, expected, result)
	repo.AssertExpectations(t)
}

func TestOperationsService_GetAllMetrics(t *testing.T) {
	repo := new(mockMetricsRepo)
	svc := NewOperationsService(repo)

	orgID := uuid.New()
	period := domain.MetricPeriodDaily
	bucket := time.Now().UTC()

	repo.On("GetCaseVolume", mock.Anything, orgID, period, bucket).Return(&domain.CaseVolumeMetric{OrganizationID: orgID, TotalCases: 10, CalculatedAt: time.Now().UTC()}, nil)
	repo.On("GetWorkflowThroughput", mock.Anything, orgID, period, bucket).Return([]*domain.WorkflowThroughputMetric{{WorkflowKey: "emergency", TotalTransitions: 5, CalculatedAt: time.Now().UTC()}}, nil)
	repo.On("GetStateDuration", mock.Anything, orgID, period, bucket).Return([]*domain.StateDurationMetric{{StateKey: "NEW", EntryCount: 10, CalculatedAt: time.Now().UTC()}}, nil)
	repo.On("GetCaseCycleTime", mock.Anything, orgID, period, bucket).Return([]*domain.CaseCycleTimeMetric{{CompletedCases: 5, CalculatedAt: time.Now().UTC()}}, nil)
	repo.On("GetAgingCases", mock.Anything, orgID, float64(720)).Return([]*domain.AgingCaseMetric{{AgeHours: 48, CalculatedAt: time.Now().UTC()}}, nil)
	repo.On("GetPendingReviews", mock.Anything, orgID).Return(&domain.PendingReviewMetric{PendingReviews: 3, CalculatedAt: time.Now().UTC()}, nil)
	repo.On("GetDecisions", mock.Anything, orgID, period, bucket).Return(&domain.DecisionMetric{TotalDecisions: 10, CalculatedAt: time.Now().UTC()}, nil)
	repo.On("GetAssistanceOutcomes", mock.Anything, orgID, period, bucket).Return(&domain.AssistanceOutcomeMetric{TotalAssistance: 5, CalculatedAt: time.Now().UTC()}, nil)
	repo.On("GetEvidenceVerification", mock.Anything, orgID, period, bucket).Return(&domain.EvidenceVerificationMetric{TotalEvidence: 20, CalculatedAt: time.Now().UTC()}, nil)
	repo.On("GetInformationRequired", mock.Anything, orgID, period, bucket).Return(&domain.InformationRequiredMetric{TotalCases: 2, CalculatedAt: time.Now().UTC()}, nil)

	result, err := svc.GetAllMetrics(context.Background(), orgID, period, bucket)
	require.NoError(t, err)
	assert.Equal(t, orgID, result.OrganizationID)
	assert.NotNil(t, result.CaseVolume)
	assert.Equal(t, 10, result.CaseVolume.TotalCases)
	assert.NotNil(t, result.WorkflowThroughput)
	assert.Equal(t, "emergency", result.WorkflowThroughput.WorkflowKey)
	assert.NotNil(t, result.PendingReviews)
	assert.Equal(t, 3, result.PendingReviews.PendingReviews)
	assert.NotNil(t, result.Decisions)
	assert.Equal(t, 10, result.Decisions.TotalDecisions)
	repo.AssertExpectations(t)
}

func TestOperationsService_GetAllMetrics_Error(t *testing.T) {
	repo := new(mockMetricsRepo)
	svc := NewOperationsService(repo)

	repo.On("GetCaseVolume", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)

	result, err := svc.GetAllMetrics(context.Background(), uuid.New(), domain.MetricPeriodDaily, time.Now().UTC())
	assert.Error(t, err)
	assert.Nil(t, result)
}
