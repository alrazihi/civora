package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type AnalysisPeriod string

const (
	AnalysisPeriodDaily   AnalysisPeriod = "daily"
	AnalysisPeriodWeekly  AnalysisPeriod = "weekly"
	AnalysisPeriodMonthly AnalysisPeriod = "monthly"
)

type ObservationLanguage string

const (
	ObservationLanguageHighVolume       ObservationLanguage = "high_volume"
	ObservationLanguageLongerDuration   ObservationLanguage = "longer_duration"
	ObservationLanguageIncreasedBacklog ObservationLanguage = "increased_backlog"
	ObservationLanguageHigherFrequency  ObservationLanguage = "higher_frequency"
)

type StateAccumulationObservation struct {
	OrganizationID uuid.UUID `json:"organization_id"`
	WorkflowKey    string    `json:"workflow_key"`
	StateKey       string    `json:"state_key"`
	CaseCount      int       `json:"case_count"`
	Threshold      int       `json:"threshold"`
	Observation    string    `json:"observation"`
	Language       string    `json:"language"`
	CalculatedAt   time.Time `json:"calculated_at"`
}

type StateDurationAnomaly struct {
	OrganizationID      uuid.UUID `json:"organization_id"`
	WorkflowKey         string    `json:"workflow_key"`
	StateKey            string    `json:"state_key"`
	AvgDurationHours    float64   `json:"avg_duration_hours"`
	MedianDurationHours float64   `json:"median_duration_hours"`
	MaxDurationHours    float64   `json:"max_duration_hours"`
	ThresholdHours      float64   `json:"threshold_hours"`
	Observation         string    `json:"observation"`
	Language            string    `json:"language"`
	CalculatedAt        time.Time `json:"calculated_at"`
}

type WorkflowClosurePattern struct {
	OrganizationID uuid.UUID `json:"organization_id"`
	WorkflowKey    string    `json:"workflow_key"`
	TotalCases     int       `json:"total_cases"`
	ClosedCases    int       `json:"closed_cases"`
	RejectedCases  int       `json:"rejected_cases"`
	ClosureRate    float64   `json:"closure_rate"`
	RejectionRate  float64   `json:"rejection_rate"`
	Observation    string    `json:"observation"`
	Language       string    `json:"language"`
	CalculatedAt   time.Time `json:"calculated_at"`
}

type InformationRequestPattern struct {
	OrganizationID   uuid.UUID `json:"organization_id"`
	WorkflowKey      string    `json:"workflow_key"`
	TotalCases       int       `json:"total_cases"`
	InfoRequestCount int       `json:"info_request_count"`
	FrequencyPerCase float64   `json:"frequency_per_case"`
	ThresholdPerCase float64   `json:"threshold_per_case"`
	Observation      string    `json:"observation"`
	Language         string    `json:"language"`
	CalculatedAt     time.Time `json:"calculated_at"`
}

type ReviewBacklogObservation struct {
	OrganizationID   uuid.UUID `json:"organization_id"`
	PendingReviews   int       `json:"pending_reviews"`
	AssignedReviews  int       `json:"assigned_reviews"`
	InReviewReviews  int       `json:"in_review_reviews"`
	AvgWaitTimeHours float64   `json:"avg_wait_time_hours"`
	Threshold        int       `json:"threshold"`
	Observation      string    `json:"observation"`
	Language         string    `json:"language"`
	CalculatedAt     time.Time `json:"calculated_at"`
}

type ThresholdExceedanceObservation struct {
	OrganizationID uuid.UUID `json:"organization_id"`
	CaseID         uuid.UUID `json:"case_id"`
	CaseNumber     string    `json:"case_number"`
	WorkflowKey    string    `json:"workflow_key"`
	CurrentState   string    `json:"current_state"`
	ThresholdType  string    `json:"threshold_type"`
	ThresholdValue float64   `json:"threshold_value"`
	ActualValue    float64   `json:"actual_value"`
	Observation    string    `json:"observation"`
	Language       string    `json:"language"`
	CalculatedAt   time.Time `json:"calculated_at"`
}

type WorkflowAnalysisReport struct {
	OrganizationID             uuid.UUID                         `json:"organization_id"`
	CalculatedAt               time.Time                         `json:"calculated_at"`
	Period                     AnalysisPeriod                    `json:"period"`
	StateAccumulations         []*StateAccumulationObservation   `json:"state_accumulations,omitempty"`
	StateDurationAnomalies     []*StateDurationAnomaly           `json:"state_duration_anomalies,omitempty"`
	WorkflowClosurePatterns    []*WorkflowClosurePattern         `json:"workflow_closure_patterns,omitempty"`
	InformationRequestPatterns []*InformationRequestPattern      `json:"information_request_patterns,omitempty"`
	ReviewBacklogObservations  []*ReviewBacklogObservation       `json:"review_backlog_observations,omitempty"`
	ThresholdExceedances       []*ThresholdExceedanceObservation `json:"threshold_exceedances,omitempty"`
}

type AnalysisThresholds struct {
	StateAccumulationThreshold  int     `json:"state_accumulation_threshold"`
	StateDurationThresholdHours float64 `json:"state_duration_threshold_hours"`
	AgingThresholdHours         float64 `json:"aging_threshold_hours"`
	ReviewBacklogThreshold      int     `json:"review_backlog_threshold"`
	InfoRequestFrequencyPerCase float64 `json:"info_request_frequency_per_case"`
	CycleTimeThresholdHours     float64 `json:"cycle_time_threshold_hours"`
}

var DefaultAnalysisThresholds = AnalysisThresholds{
	StateAccumulationThreshold:  10,
	StateDurationThresholdHours: 24,
	AgingThresholdHours:         720,
	ReviewBacklogThreshold:      20,
	InfoRequestFrequencyPerCase: 1.5,
	CycleTimeThresholdHours:     72,
}

type AnalysisRepository interface {
	GetStateAccumulations(ctx context.Context, orgID uuid.UUID, threshold int) ([]*StateAccumulationObservation, error)
	GetStateDurationAnomalies(ctx context.Context, orgID uuid.UUID, thresholdHours float64) ([]*StateDurationAnomaly, error)
	GetWorkflowClosurePatterns(ctx context.Context, orgID uuid.UUID) ([]*WorkflowClosurePattern, error)
	GetInformationRequestPatterns(ctx context.Context, orgID uuid.UUID, thresholdPerCase float64) ([]*InformationRequestPattern, error)
	GetReviewBacklogObservations(ctx context.Context, orgID uuid.UUID, threshold int) ([]*ReviewBacklogObservation, error)
	GetThresholdExceedances(ctx context.Context, orgID uuid.UUID, agingThresholdHours float64, cycleTimeThresholdHours float64) ([]*ThresholdExceedanceObservation, error)
	GetWorkflowAnalysisReport(ctx context.Context, orgID uuid.UUID, thresholds AnalysisThresholds) (*WorkflowAnalysisReport, error)
}

type AnalysisService interface {
	GetWorkflowAnalysisReport(ctx context.Context, orgID uuid.UUID, thresholds AnalysisThresholds) (*WorkflowAnalysisReport, error)
}
