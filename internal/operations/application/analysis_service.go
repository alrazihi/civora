package application

import (
	"context"
	"time"

	"github.com/alrazihi/civora/internal/operations/domain"
	"github.com/google/uuid"
)

type AnalysisService struct {
	repo domain.AnalysisRepository
}

func NewAnalysisService(repo domain.AnalysisRepository) *AnalysisService {
	return &AnalysisService{repo: repo}
}

func (s *AnalysisService) GetWorkflowAnalysisReport(ctx context.Context, orgID uuid.UUID, thresholds domain.AnalysisThresholds) (*domain.WorkflowAnalysisReport, error) {
	if thresholds.StateAccumulationThreshold == 0 {
		thresholds.StateAccumulationThreshold = domain.DefaultAnalysisThresholds.StateAccumulationThreshold
	}
	if thresholds.StateDurationThresholdHours == 0 {
		thresholds.StateDurationThresholdHours = domain.DefaultAnalysisThresholds.StateDurationThresholdHours
	}
	if thresholds.AgingThresholdHours == 0 {
		thresholds.AgingThresholdHours = domain.DefaultAnalysisThresholds.AgingThresholdHours
	}
	if thresholds.ReviewBacklogThreshold == 0 {
		thresholds.ReviewBacklogThreshold = domain.DefaultAnalysisThresholds.ReviewBacklogThreshold
	}
	if thresholds.InfoRequestFrequencyPerCase == 0 {
		thresholds.InfoRequestFrequencyPerCase = domain.DefaultAnalysisThresholds.InfoRequestFrequencyPerCase
	}
	if thresholds.CycleTimeThresholdHours == 0 {
		thresholds.CycleTimeThresholdHours = domain.DefaultAnalysisThresholds.CycleTimeThresholdHours
	}

	report, err := s.repo.GetWorkflowAnalysisReport(ctx, orgID, thresholds)
	if err != nil {
		return nil, err
	}
	report.CalculatedAt = time.Now().UTC()
	return report, nil
}
