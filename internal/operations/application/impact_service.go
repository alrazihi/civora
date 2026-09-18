package application

import (
	"context"
	"time"

	"github.com/alrazihi/civora/internal/operations/domain"
	"github.com/google/uuid"
)

type ImpactService struct {
	repo domain.ImpactRepository
}

func NewImpactService(repo domain.ImpactRepository) *ImpactService {
	return &ImpactService{repo: repo}
}

func (s *ImpactService) GetImpactIntelligenceReport(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) (*domain.ImpactIntelligenceReport, error) {
	report, err := s.repo.GetImpactIntelligenceReport(ctx, orgID, period, bucket)
	if err != nil {
		return nil, err
	}
	report.CalculatedAt = time.Now().UTC()
	return report, nil
}
