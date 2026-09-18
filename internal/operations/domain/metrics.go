package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type MetricPeriod string

const (
	MetricPeriodDaily   MetricPeriod = "daily"
	MetricPeriodWeekly  MetricPeriod = "weekly"
	MetricPeriodMonthly MetricPeriod = "monthly"
)

func BucketStart(t time.Time, period MetricPeriod) time.Time {
	switch period {
	case MetricPeriodWeekly:
		// Start of week (Monday)
		weekday := t.Weekday()
		if weekday == time.Sunday {
			weekday = 7
		}
		diff := int(weekday - 1)
		return time.Date(t.Year(), t.Month(), t.Day()-diff, 0, 0, 0, 0, t.Location())
	case MetricPeriodMonthly:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	default:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	}
}

type CaseVolumeMetric struct {
	OrganizationID uuid.UUID      `json:"organization_id"`
	Period         MetricPeriod   `json:"period"`
	Bucket         time.Time      `json:"bucket"`
	TotalCases     int            `json:"total_cases"`
	NewCases       int            `json:"new_cases"`
	OpenCases      int            `json:"open_cases"`
	ClosedCases    int            `json:"closed_cases"`
	RejectedCases  int            `json:"rejected_cases"`
	ByStatus       map[string]int `json:"by_status,omitempty"`
	ByServiceType  map[string]int `json:"by_service_type,omitempty"`
	ByWorkflowKey  map[string]int `json:"by_workflow_key,omitempty"`
	ByState        map[string]int `json:"by_state,omitempty"`
	CalculatedAt   time.Time      `json:"calculated_at"`
}

type WorkflowThroughputMetric struct {
	OrganizationID        uuid.UUID    `json:"organization_id"`
	Period                MetricPeriod `json:"period"`
	Bucket                time.Time    `json:"bucket"`
	WorkflowKey           string       `json:"workflow_key"`
	TotalTransitions      int          `json:"total_transitions"`
	UniqueCases           int          `json:"unique_cases"`
	AvgTransitionsPerCase float64      `json:"avg_transitions_per_case"`
	CalculatedAt          time.Time    `json:"calculated_at"`
}

type StateDurationMetric struct {
	OrganizationID      uuid.UUID    `json:"organization_id"`
	Period              MetricPeriod `json:"period"`
	Bucket              time.Time    `json:"bucket"`
	WorkflowKey         string       `json:"workflow_key"`
	StateKey            string       `json:"state_key"`
	EntryCount          int          `json:"entry_count"`
	AvgDurationHours    float64      `json:"avg_duration_hours"`
	MedianDurationHours float64      `json:"median_duration_hours"`
	MaxDurationHours    float64      `json:"max_duration_hours"`
	CalculatedAt        time.Time    `json:"calculated_at"`
}

type CaseCycleTimeMetric struct {
	OrganizationID       uuid.UUID    `json:"organization_id"`
	Period               MetricPeriod `json:"period"`
	Bucket               time.Time    `json:"bucket"`
	WorkflowKey          string       `json:"workflow_key"`
	CompletedCases       int          `json:"completed_cases"`
	AvgCycleTimeHours    float64      `json:"avg_cycle_time_hours"`
	MedianCycleTimeHours float64      `json:"median_cycle_time_hours"`
	CalculatedAt         time.Time    `json:"calculated_at"`
}

type AgingCaseMetric struct {
	OrganizationID      uuid.UUID `json:"organization_id"`
	CaseID              uuid.UUID `json:"case_id"`
	CaseNumber          string    `json:"case_number"`
	WorkflowKey         string    `json:"workflow_key"`
	CurrentState        string    `json:"current_state"`
	ServiceType         string    `json:"service_type"`
	Priority            string    `json:"priority"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	AgeHours            float64   `json:"age_hours"`
	InCurrentStateHours float64   `json:"in_current_state_hours"`
	CalculatedAt        time.Time `json:"calculated_at"`
}

type PendingReviewMetric struct {
	OrganizationID     uuid.UUID `json:"organization_id"`
	PendingReviews     int       `json:"pending_reviews"`
	AssignedReviews    int       `json:"assigned_reviews"`
	InReviewReviews    int       `json:"in_review_reviews"`
	WaitingInfoReviews int       `json:"waiting_info_reviews"`
	AvgWaitTimeHours   float64   `json:"avg_wait_time_hours"`
	CalculatedAt       time.Time `json:"calculated_at"`
}

type DecisionMetric struct {
	OrganizationID          uuid.UUID    `json:"organization_id"`
	Period                  MetricPeriod `json:"period"`
	Bucket                  time.Time    `json:"bucket"`
	TotalDecisions          int          `json:"total_decisions"`
	Approved                int          `json:"approved"`
	Rejected                int          `json:"rejected"`
	NeedsMoreInfo           int          `json:"needs_more_info"`
	Escalated               int          `json:"escalated"`
	ApprovalRate            float64      `json:"approval_rate"`
	AvgDecisionsPerReviewer float64      `json:"avg_decisions_per_reviewer"`
	CalculatedAt            time.Time    `json:"calculated_at"`
}

type AssistanceOutcomeMetric struct {
	OrganizationID  uuid.UUID    `json:"organization_id"`
	Period          MetricPeriod `json:"period"`
	Bucket          time.Time    `json:"bucket"`
	TotalAssistance int          `json:"total_assistance"`
	Planned         int          `json:"planned"`
	InProgress      int          `json:"in_progress"`
	Completed       int          `json:"completed"`
	Cancelled       int          `json:"cancelled"`
	CompletionRate  float64      `json:"completion_rate"`
	CalculatedAt    time.Time    `json:"calculated_at"`
}

type EvidenceVerificationMetric struct {
	OrganizationID   uuid.UUID    `json:"organization_id"`
	Period           MetricPeriod `json:"period"`
	Bucket           time.Time    `json:"bucket"`
	TotalEvidence    int          `json:"total_evidence"`
	Verified         int          `json:"verified"`
	Rejected         int          `json:"rejected"`
	NeedsReview      int          `json:"needs_review"`
	Unverified       int          `json:"unverified"`
	VerificationRate float64      `json:"verification_rate"`
	CalculatedAt     time.Time    `json:"calculated_at"`
}

type InformationRequiredMetric struct {
	OrganizationID      uuid.UUID    `json:"organization_id"`
	Period              MetricPeriod `json:"period"`
	Bucket              time.Time    `json:"bucket"`
	TotalCases          int          `json:"total_cases"`
	InformationRequired int          `json:"information_required"`
	Escalated           int          `json:"escalated"`
	AwaitingInfoReviews int          `json:"awaiting_info_reviews"`
	CalculatedAt        time.Time    `json:"calculated_at"`
}

type OperationsMetrics struct {
	OrganizationID       uuid.UUID                   `json:"organization_id"`
	CalculatedAt         time.Time                   `json:"calculated_at"`
	Period               MetricPeriod                `json:"period"`
	CaseVolume           *CaseVolumeMetric           `json:"case_volume,omitempty"`
	WorkflowThroughput   *WorkflowThroughputMetric   `json:"workflow_throughput,omitempty"`
	StateDuration        *StateDurationMetric        `json:"state_duration,omitempty"`
	CaseCycleTime        *CaseCycleTimeMetric        `json:"case_cycle_time,omitempty"`
	AgingCases           []*AgingCaseMetric          `json:"aging_cases,omitempty"`
	PendingReviews       *PendingReviewMetric        `json:"pending_reviews,omitempty"`
	Decisions            *DecisionMetric             `json:"decisions,omitempty"`
	AssistanceOutcomes   *AssistanceOutcomeMetric    `json:"assistance_outcomes,omitempty"`
	EvidenceVerification *EvidenceVerificationMetric `json:"evidence_verification,omitempty"`
	InformationRequired  *InformationRequiredMetric  `json:"information_required,omitempty"`
}

type MetricsRepository interface {
	GetCaseVolume(ctx context.Context, orgID uuid.UUID, period MetricPeriod, bucket time.Time) (*CaseVolumeMetric, error)
	GetCaseVolumeByWorkflow(ctx context.Context, orgID uuid.UUID, period MetricPeriod, bucket time.Time) ([]*CaseVolumeMetric, error)
	GetCaseVolumeByState(ctx context.Context, orgID uuid.UUID, period MetricPeriod, bucket time.Time) ([]*CaseVolumeMetric, error)
	GetCaseVolumeByServiceType(ctx context.Context, orgID uuid.UUID, period MetricPeriod, bucket time.Time) ([]*CaseVolumeMetric, error)
	GetWorkflowThroughput(ctx context.Context, orgID uuid.UUID, period MetricPeriod, bucket time.Time) ([]*WorkflowThroughputMetric, error)
	GetStateDuration(ctx context.Context, orgID uuid.UUID, period MetricPeriod, bucket time.Time) ([]*StateDurationMetric, error)
	GetCaseCycleTime(ctx context.Context, orgID uuid.UUID, period MetricPeriod, bucket time.Time) ([]*CaseCycleTimeMetric, error)
	GetAgingCases(ctx context.Context, orgID uuid.UUID, thresholdHours float64) ([]*AgingCaseMetric, error)
	GetPendingReviews(ctx context.Context, orgID uuid.UUID) (*PendingReviewMetric, error)
	GetDecisions(ctx context.Context, orgID uuid.UUID, period MetricPeriod, bucket time.Time) (*DecisionMetric, error)
	GetAssistanceOutcomes(ctx context.Context, orgID uuid.UUID, period MetricPeriod, bucket time.Time) (*AssistanceOutcomeMetric, error)
	GetEvidenceVerification(ctx context.Context, orgID uuid.UUID, period MetricPeriod, bucket time.Time) (*EvidenceVerificationMetric, error)
	GetInformationRequired(ctx context.Context, orgID uuid.UUID, period MetricPeriod, bucket time.Time) (*InformationRequiredMetric, error)
}

type MetricsService interface {
	GetCaseVolume(ctx context.Context, orgID uuid.UUID, period MetricPeriod, bucket time.Time) (*CaseVolumeMetric, error)
	GetCaseVolumeByWorkflow(ctx context.Context, orgID uuid.UUID, period MetricPeriod, bucket time.Time) ([]*CaseVolumeMetric, error)
	GetCaseVolumeByState(ctx context.Context, orgID uuid.UUID, period MetricPeriod, bucket time.Time) ([]*CaseVolumeMetric, error)
	GetCaseVolumeByServiceType(ctx context.Context, orgID uuid.UUID, period MetricPeriod, bucket time.Time) ([]*CaseVolumeMetric, error)
	GetWorkflowThroughput(ctx context.Context, orgID uuid.UUID, period MetricPeriod, bucket time.Time) ([]*WorkflowThroughputMetric, error)
	GetStateDuration(ctx context.Context, orgID uuid.UUID, period MetricPeriod, bucket time.Time) ([]*StateDurationMetric, error)
	GetCaseCycleTime(ctx context.Context, orgID uuid.UUID, period MetricPeriod, bucket time.Time) ([]*CaseCycleTimeMetric, error)
	GetAgingCases(ctx context.Context, orgID uuid.UUID, thresholdHours float64) ([]*AgingCaseMetric, error)
	GetPendingReviews(ctx context.Context, orgID uuid.UUID) (*PendingReviewMetric, error)
	GetDecisions(ctx context.Context, orgID uuid.UUID, period MetricPeriod, bucket time.Time) (*DecisionMetric, error)
	GetAssistanceOutcomes(ctx context.Context, orgID uuid.UUID, period MetricPeriod, bucket time.Time) (*AssistanceOutcomeMetric, error)
	GetEvidenceVerification(ctx context.Context, orgID uuid.UUID, period MetricPeriod, bucket time.Time) (*EvidenceVerificationMetric, error)
	GetInformationRequired(ctx context.Context, orgID uuid.UUID, period MetricPeriod, bucket time.Time) (*InformationRequiredMetric, error)
	GetAllMetrics(ctx context.Context, orgID uuid.UUID, period MetricPeriod, bucket time.Time) (*OperationsMetrics, error)
	GetDashboardMetrics(ctx context.Context, orgID uuid.UUID, period MetricPeriod, bucket time.Time, workflowKey string, status string) (*OperationsMetrics, error)
}
