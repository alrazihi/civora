package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type OutcomeCategory string

const (
	OutcomeCategoryActivity OutcomeCategory = "activity"
	OutcomeCategoryOutcome  OutcomeCategory = "outcome"
	OutcomeCategoryImpact   OutcomeCategory = "impact"
)

type OutcomeMetric struct {
	OrganizationID uuid.UUID       `json:"organization_id"`
	Period         MetricPeriod    `json:"period"`
	Bucket         time.Time       `json:"bucket"`
	Category       OutcomeCategory `json:"category"`
	Name           string          `json:"name"`
	Label          string          `json:"label"`
	Description    string          `json:"description"`
	ActivityCount  int             `json:"activity_count,omitempty"`
	OutcomeCount   int             `json:"outcome_count,omitempty"`
	ImpactValue    float64         `json:"impact_value,omitempty"`
	Unit           string          `json:"unit"`
	CalculatedAt   time.Time       `json:"calculated_at"`
}

type ImpactIntelligenceReport struct {
	OrganizationID uuid.UUID        `json:"organization_id"`
	CalculatedAt   time.Time        `json:"calculated_at"`
	Period         MetricPeriod     `json:"period"`
	Bucket         time.Time        `json:"bucket"`
	Metrics        []*OutcomeMetric `json:"metrics"`
}

type ImpactRepository interface {
	GetImpactIntelligenceReport(ctx context.Context, orgID uuid.UUID, period MetricPeriod, bucket time.Time) (*ImpactIntelligenceReport, error)
}

type ImpactService interface {
	GetImpactIntelligenceReport(ctx context.Context, orgID uuid.UUID, period MetricPeriod, bucket time.Time) (*ImpactIntelligenceReport, error)
}
