package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestMetricPeriod_Values(t *testing.T) {
	assert.Equal(t, MetricPeriodDaily, MetricPeriod("daily"))
	assert.Equal(t, MetricPeriodWeekly, MetricPeriod("weekly"))
	assert.Equal(t, MetricPeriodMonthly, MetricPeriod("monthly"))
}

func TestCaseVolumeMetric_New(t *testing.T) {
	orgID := uuid.New()
	metric := &CaseVolumeMetric{
		OrganizationID: orgID,
		Period:         MetricPeriodDaily,
		Bucket:         time.Now().UTC(),
		ByStatus:       map[string]int{"NEW": 5},
		ByServiceType:  map[string]int{"EMERGENCY": 5},
		ByWorkflowKey:  map[string]int{"emergency": 5},
		ByState:        map[string]int{"NEW": 5},
		CalculatedAt:   time.Now().UTC(),
	}

	assert.Equal(t, orgID, metric.OrganizationID)
	assert.Equal(t, MetricPeriodDaily, metric.Period)
	assert.Equal(t, 5, metric.ByStatus["NEW"])
	assert.Equal(t, 5, metric.ByServiceType["EMERGENCY"])
}

func TestWorkflowThroughputMetric_RawValues(t *testing.T) {
	metric := &WorkflowThroughputMetric{
		TotalTransitions: 10,
		UniqueCases:      5,
	}

	assert.Equal(t, 10, metric.TotalTransitions)
	assert.Equal(t, 5, metric.UniqueCases)
	assert.Equal(t, 0.0, metric.AvgTransitionsPerCase)
}

func TestDecisionMetric_RawValues(t *testing.T) {
	metric := &DecisionMetric{
		TotalDecisions: 10,
		Approved:       7,
		Rejected:       2,
		NeedsMoreInfo:  1,
		Escalated:      0,
	}

	assert.Equal(t, 10, metric.TotalDecisions)
	assert.Equal(t, 7, metric.Approved)
	assert.Equal(t, 0.0, metric.ApprovalRate)
}

func TestAssistanceOutcomeMetric_RawValues(t *testing.T) {
	metric := &AssistanceOutcomeMetric{
		TotalAssistance: 10,
		Completed:       8,
		InProgress:      1,
		Planned:         1,
		Cancelled:       0,
	}

	assert.Equal(t, 10, metric.TotalAssistance)
	assert.Equal(t, 8, metric.Completed)
	assert.Equal(t, 0.0, metric.CompletionRate)
}

func TestEvidenceVerificationMetric_RawValues(t *testing.T) {
	metric := &EvidenceVerificationMetric{
		TotalEvidence: 100,
		Verified:      75,
		Rejected:      10,
		NeedsReview:   5,
		Unverified:    10,
	}

	assert.Equal(t, 100, metric.TotalEvidence)
	assert.Equal(t, 75, metric.Verified)
	assert.Equal(t, 0.0, metric.VerificationRate)
}

func TestAgingCaseMetric_Age(t *testing.T) {
	now := time.Now().UTC()
	metric := &AgingCaseMetric{
		CaseID:       uuid.New(),
		AgeHours:     48.5,
		CalculatedAt: now,
	}

	assert.True(t, metric.AgeHours > 0)
	assert.True(t, metric.CalculatedAt.Before(now.Add(time.Minute)))
}

func TestOperationsMetrics_Aggregation(t *testing.T) {
	orgID := uuid.New()
	now := time.Now().UTC()
	metrics := &OperationsMetrics{
		OrganizationID: orgID,
		Period:         MetricPeriodDaily,
		CalculatedAt:   now,
		CaseVolume: &CaseVolumeMetric{
			OrganizationID: orgID,
			TotalCases:     10,
			ByStatus:       map[string]int{},
			ByServiceType:  map[string]int{},
			ByWorkflowKey:  map[string]int{},
			ByState:        map[string]int{},
		},
		PendingReviews: &PendingReviewMetric{
			OrganizationID: orgID,
			PendingReviews: 5,
		},
	}

	assert.Equal(t, orgID, metrics.OrganizationID)
	assert.NotNil(t, metrics.CaseVolume)
	assert.Equal(t, 10, metrics.CaseVolume.TotalCases)
	assert.NotNil(t, metrics.PendingReviews)
	assert.Equal(t, 5, metrics.PendingReviews.PendingReviews)
}
