package application

import (
	"context"
	"time"

	"github.com/alrazihi/civora/internal/operations/domain"
	"github.com/google/uuid"
)

type OperationsService struct {
	repo domain.MetricsRepository
}

func NewOperationsService(repo domain.MetricsRepository) *OperationsService {
	return &OperationsService{repo: repo}
}

func (s *OperationsService) GetCaseVolume(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) (*domain.CaseVolumeMetric, error) {
	return s.repo.GetCaseVolume(ctx, orgID, period, bucket)
}

func (s *OperationsService) GetCaseVolumeByWorkflow(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) ([]*domain.CaseVolumeMetric, error) {
	return s.repo.GetCaseVolumeByWorkflow(ctx, orgID, period, bucket)
}

func (s *OperationsService) GetCaseVolumeByState(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) ([]*domain.CaseVolumeMetric, error) {
	return s.repo.GetCaseVolumeByState(ctx, orgID, period, bucket)
}

func (s *OperationsService) GetCaseVolumeByServiceType(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) ([]*domain.CaseVolumeMetric, error) {
	return s.repo.GetCaseVolumeByServiceType(ctx, orgID, period, bucket)
}

func (s *OperationsService) GetWorkflowThroughput(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) ([]*domain.WorkflowThroughputMetric, error) {
	return s.repo.GetWorkflowThroughput(ctx, orgID, period, bucket)
}

func (s *OperationsService) GetStateDuration(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) ([]*domain.StateDurationMetric, error) {
	return s.repo.GetStateDuration(ctx, orgID, period, bucket)
}

func (s *OperationsService) GetCaseCycleTime(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) ([]*domain.CaseCycleTimeMetric, error) {
	return s.repo.GetCaseCycleTime(ctx, orgID, period, bucket)
}

func (s *OperationsService) GetAgingCases(ctx context.Context, orgID uuid.UUID, thresholdHours float64) ([]*domain.AgingCaseMetric, error) {
	return s.repo.GetAgingCases(ctx, orgID, thresholdHours)
}

func (s *OperationsService) GetPendingReviews(ctx context.Context, orgID uuid.UUID) (*domain.PendingReviewMetric, error) {
	return s.repo.GetPendingReviews(ctx, orgID)
}

func (s *OperationsService) GetDecisions(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) (*domain.DecisionMetric, error) {
	return s.repo.GetDecisions(ctx, orgID, period, bucket)
}

func (s *OperationsService) GetAssistanceOutcomes(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) (*domain.AssistanceOutcomeMetric, error) {
	return s.repo.GetAssistanceOutcomes(ctx, orgID, period, bucket)
}

func (s *OperationsService) GetEvidenceVerification(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) (*domain.EvidenceVerificationMetric, error) {
	return s.repo.GetEvidenceVerification(ctx, orgID, period, bucket)
}

func (s *OperationsService) GetInformationRequired(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) (*domain.InformationRequiredMetric, error) {
	return s.repo.GetInformationRequired(ctx, orgID, period, bucket)
}

func (s *OperationsService) GetAllMetrics(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) (*domain.OperationsMetrics, error) {
	result := &domain.OperationsMetrics{
		OrganizationID: orgID,
		Period:         period,
		CalculatedAt:   time.Now().UTC(),
	}

	caseVolume, err := s.repo.GetCaseVolume(ctx, orgID, period, bucket)
	if err != nil {
		return nil, err
	}
	result.CaseVolume = caseVolume

	workflowThroughput, err := s.repo.GetWorkflowThroughput(ctx, orgID, period, bucket)
	if err != nil {
		return nil, err
	}
	if len(workflowThroughput) > 0 {
		result.WorkflowThroughput = workflowThroughput[0]
	}

	stateDuration, err := s.repo.GetStateDuration(ctx, orgID, period, bucket)
	if err != nil {
		return nil, err
	}
	if len(stateDuration) > 0 {
		result.StateDuration = stateDuration[0]
	}

	caseCycleTime, err := s.repo.GetCaseCycleTime(ctx, orgID, period, bucket)
	if err != nil {
		return nil, err
	}
	if len(caseCycleTime) > 0 {
		result.CaseCycleTime = caseCycleTime[0]
	}

	agingCases, err := s.repo.GetAgingCases(ctx, orgID, 720)
	if err != nil {
		return nil, err
	}
	result.AgingCases = agingCases

	pendingReviews, err := s.repo.GetPendingReviews(ctx, orgID)
	if err != nil {
		return nil, err
	}
	result.PendingReviews = pendingReviews

	decisions, err := s.repo.GetDecisions(ctx, orgID, period, bucket)
	if err != nil {
		return nil, err
	}
	result.Decisions = decisions

	assistanceOutcomes, err := s.repo.GetAssistanceOutcomes(ctx, orgID, period, bucket)
	if err != nil {
		return nil, err
	}
	result.AssistanceOutcomes = assistanceOutcomes

	evidenceVerification, err := s.repo.GetEvidenceVerification(ctx, orgID, period, bucket)
	if err != nil {
		return nil, err
	}
	result.EvidenceVerification = evidenceVerification

	informationRequired, err := s.repo.GetInformationRequired(ctx, orgID, period, bucket)
	if err != nil {
		return nil, err
	}
	result.InformationRequired = informationRequired

	return result, nil
}

func (s *OperationsService) GetDashboardMetrics(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time, workflowKey string, status string) (*domain.OperationsMetrics, error) {
	result := &domain.OperationsMetrics{
		OrganizationID: orgID,
		Period:         period,
		CalculatedAt:   time.Now().UTC(),
	}

	caseVolume, err := s.repo.GetCaseVolume(ctx, orgID, period, bucket)
	if err != nil {
		return nil, err
	}
	result.CaseVolume = caseVolume

	workflowThroughput, err := s.repo.GetWorkflowThroughput(ctx, orgID, period, bucket)
	if err != nil {
		return nil, err
	}
	if len(workflowThroughput) > 0 {
		result.WorkflowThroughput = workflowThroughput[0]
	}

	stateDuration, err := s.repo.GetStateDuration(ctx, orgID, period, bucket)
	if err != nil {
		return nil, err
	}
	if len(stateDuration) > 0 {
		result.StateDuration = stateDuration[0]
	}

	caseCycleTime, err := s.repo.GetCaseCycleTime(ctx, orgID, period, bucket)
	if err != nil {
		return nil, err
	}
	if len(caseCycleTime) > 0 {
		result.CaseCycleTime = caseCycleTime[0]
	}

	agingCases, err := s.repo.GetAgingCases(ctx, orgID, 720)
	if err != nil {
		return nil, err
	}
	if workflowKey != "" || status != "" {
		filtered := make([]*domain.AgingCaseMetric, 0)
		for _, c := range agingCases {
			if workflowKey != "" && c.WorkflowKey != workflowKey {
				continue
			}
			if status != "" && c.CurrentState != status {
				continue
			}
			filtered = append(filtered, c)
		}
		result.AgingCases = filtered
	} else {
		result.AgingCases = agingCases
	}

	pendingReviews, err := s.repo.GetPendingReviews(ctx, orgID)
	if err != nil {
		return nil, err
	}
	result.PendingReviews = pendingReviews

	decisions, err := s.repo.GetDecisions(ctx, orgID, period, bucket)
	if err != nil {
		return nil, err
	}
	result.Decisions = decisions

	assistanceOutcomes, err := s.repo.GetAssistanceOutcomes(ctx, orgID, period, bucket)
	if err != nil {
		return nil, err
	}
	result.AssistanceOutcomes = assistanceOutcomes

	evidenceVerification, err := s.repo.GetEvidenceVerification(ctx, orgID, period, bucket)
	if err != nil {
		return nil, err
	}
	result.EvidenceVerification = evidenceVerification

	informationRequired, err := s.repo.GetInformationRequired(ctx, orgID, period, bucket)
	if err != nil {
		return nil, err
	}
	result.InformationRequired = informationRequired

	return result, nil
}
