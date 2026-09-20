package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/alrazihi/civora/internal/operations/domain"
	"github.com/google/uuid"
)

type PostgresMetricsRepository struct {
	db *sql.DB
}

func NewPostgresMetricsRepository(db *sql.DB) *PostgresMetricsRepository {
	return &PostgresMetricsRepository{db: db}
}

func (r *PostgresMetricsRepository) DB() *sql.DB {
	return r.db
}

func bucketStart(t time.Time, period domain.MetricPeriod) time.Time {
	switch period {
	case domain.MetricPeriodDaily:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	case domain.MetricPeriodWeekly:
		// Start of week (Monday)
		weekday := int(t.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		return t.AddDate(0, 0, -weekday+1).Truncate(24 * time.Hour)
	case domain.MetricPeriodMonthly:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	default:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	}
}

func (r *PostgresMetricsRepository) GetCaseVolume(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) (*domain.CaseVolumeMetric, error) {
	bucketStart := bucketStart(bucket, period)
	bucketEnd := bucketStart.AddDate(0, 0, 1)
	if period == domain.MetricPeriodWeekly {
		bucketEnd = bucketStart.AddDate(0, 0, 7)
	}
	if period == domain.MetricPeriodMonthly {
		bucketEnd = bucketStart.AddDate(0, 1, 0)
	}

	metric := &domain.CaseVolumeMetric{
		OrganizationID: orgID,
		Period:         period,
		Bucket:         bucketStart,
		ByStatus:       make(map[string]int),
		ByServiceType:  make(map[string]int),
		ByWorkflowKey:  make(map[string]int),
		ByState:        make(map[string]int),
		CalculatedAt:   time.Now().UTC(),
	}

	err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE created_at >= $2 AND created_at < $3),
			COUNT(*) FILTER (WHERE status NOT IN ('CLOSED', 'REJECTED') AND created_at >= $2 AND created_at < $3),
			COUNT(*) FILTER (WHERE status IN ('CLOSED') AND created_at >= $2 AND created_at < $3),
			COUNT(*) FILTER (WHERE status IN ('REJECTED') AND created_at >= $2 AND created_at < $3)
		FROM cases
		WHERE organization_id = $1
	`, orgID, bucketStart, bucketEnd).Scan(
		&metric.NewCases,
		&metric.OpenCases,
		&metric.ClosedCases,
		&metric.RejectedCases,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query case volume: %w", err)
	}

	metric.TotalCases = metric.NewCases

	rows, err := r.db.QueryContext(ctx, `
		SELECT status, service_type, COALESCE(workflow_key, ''), COALESCE(workflow_state, ''), COUNT(*)
		FROM cases
		WHERE organization_id = $1 AND created_at >= $2 AND created_at < $3
		GROUP BY status, service_type, workflow_key, workflow_state
	`, orgID, bucketStart, bucketEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to query case volume breakdown: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var status, serviceType, workflowKey, workflowState string
		var count int
		if err := rows.Scan(&status, &serviceType, &workflowKey, &workflowState, &count); err != nil {
			return nil, fmt.Errorf("failed to scan case volume row: %w", err)
		}
		if count < domain.MinimumAggregationGroupSize {
			continue
		}
		metric.ByStatus[status] += count
		metric.ByServiceType[serviceType] += count
		if workflowKey != "" {
			metric.ByWorkflowKey[workflowKey] += count
		}
		if workflowState != "" {
			metric.ByState[workflowState] += count
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return metric, nil
}

func (r *PostgresMetricsRepository) GetCaseVolumeByWorkflow(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) ([]*domain.CaseVolumeMetric, error) {
	bucketStart := bucketStart(bucket, period)
	bucketEnd := bucketStart.AddDate(0, 0, 1)
	if period == domain.MetricPeriodWeekly {
		bucketEnd = bucketStart.AddDate(0, 0, 7)
	}
	if period == domain.MetricPeriodMonthly {
		bucketEnd = bucketStart.AddDate(0, 1, 0)
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT
			COALESCE(workflow_key, 'unknown'),
			COUNT(*) FILTER (WHERE created_at >= $2 AND created_at < $3),
			COUNT(*) FILTER (WHERE status NOT IN ('CLOSED', 'REJECTED') AND created_at >= $2 AND created_at < $3),
			COUNT(*) FILTER (WHERE status = 'CLOSED' AND created_at >= $2 AND created_at < $3),
			COUNT(*) FILTER (WHERE status = 'REJECTED' AND created_at >= $2 AND created_at < $3)
		FROM cases
		WHERE organization_id = $1
		GROUP BY COALESCE(workflow_key, 'unknown')
		HAVING COUNT(*) >= $4
	`, orgID, bucketStart, bucketEnd, domain.MinimumAggregationGroupSize)
	if err != nil {
		return nil, fmt.Errorf("failed to query case volume by workflow: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []*domain.CaseVolumeMetric
	for rows.Next() {
		var workflowKey string
		var total, open, closed, rejected int
		if err := rows.Scan(&workflowKey, &total, &open, &closed, &rejected); err != nil {
			return nil, fmt.Errorf("failed to scan workflow volume row: %w", err)
		}
		results = append(results, &domain.CaseVolumeMetric{
			OrganizationID: orgID,
			Period:         period,
			Bucket:         bucketStart,
			ByWorkflowKey:  map[string]int{workflowKey: total},
			TotalCases:     total,
			NewCases:       total,
			OpenCases:      open,
			ClosedCases:    closed,
			RejectedCases:  rejected,
			CalculatedAt:   time.Now().UTC(),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return results, nil
}

func (r *PostgresMetricsRepository) GetCaseVolumeByState(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) ([]*domain.CaseVolumeMetric, error) {
	bucketStart := bucketStart(bucket, period)
	bucketEnd := bucketStart.AddDate(0, 0, 1)
	if period == domain.MetricPeriodWeekly {
		bucketEnd = bucketStart.AddDate(0, 0, 7)
	}
	if period == domain.MetricPeriodMonthly {
		bucketEnd = bucketStart.AddDate(0, 1, 0)
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT workflow_state, COUNT(*)
		FROM cases
		WHERE organization_id = $1 AND created_at >= $2 AND created_at < $3
		GROUP BY workflow_state
		HAVING COUNT(*) >= $4
	`, orgID, bucketStart, bucketEnd, domain.MinimumAggregationGroupSize)
	if err != nil {
		return nil, fmt.Errorf("failed to query case volume by state: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []*domain.CaseVolumeMetric
	for rows.Next() {
		var state string
		var count int
		if err := rows.Scan(&state, &count); err != nil {
			return nil, fmt.Errorf("failed to scan state volume row: %w", err)
		}
		results = append(results, &domain.CaseVolumeMetric{
			OrganizationID: orgID,
			Period:         period,
			Bucket:         bucketStart,
			ByState:        map[string]int{state: count},
			TotalCases:     count,
			CalculatedAt:   time.Now().UTC(),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return results, nil
}

func (r *PostgresMetricsRepository) GetCaseVolumeByServiceType(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) ([]*domain.CaseVolumeMetric, error) {
	bucketStart := bucketStart(bucket, period)
	bucketEnd := bucketStart.AddDate(0, 0, 1)
	if period == domain.MetricPeriodWeekly {
		bucketEnd = bucketStart.AddDate(0, 0, 7)
	}
	if period == domain.MetricPeriodMonthly {
		bucketEnd = bucketStart.AddDate(0, 1, 0)
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT service_type, COUNT(*)
		FROM cases
		WHERE organization_id = $1 AND created_at >= $2 AND created_at < $3
		GROUP BY service_type
		HAVING COUNT(*) >= $4
	`, orgID, bucketStart, bucketEnd, domain.MinimumAggregationGroupSize)
	if err != nil {
		return nil, fmt.Errorf("failed to query case volume by service type: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []*domain.CaseVolumeMetric
	for rows.Next() {
		var serviceType string
		var count int
		if err := rows.Scan(&serviceType, &count); err != nil {
			return nil, fmt.Errorf("failed to scan service type volume row: %w", err)
		}
		results = append(results, &domain.CaseVolumeMetric{
			OrganizationID: orgID,
			Period:         period,
			Bucket:         bucketStart,
			ByServiceType:  map[string]int{serviceType: count},
			TotalCases:     count,
			CalculatedAt:   time.Now().UTC(),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return results, nil
}

func (r *PostgresMetricsRepository) GetWorkflowThroughput(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) ([]*domain.WorkflowThroughputMetric, error) {
	bucketStart := bucketStart(bucket, period)
	bucketEnd := bucketStart.AddDate(0, 0, 1)
	if period == domain.MetricPeriodWeekly {
		bucketEnd = bucketStart.AddDate(0, 0, 7)
	}
	if period == domain.MetricPeriodMonthly {
		bucketEnd = bucketStart.AddDate(0, 1, 0)
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT
			wd.key,
			COUNT(*)::int,
			COUNT(DISTINCT wth.case_id)::int
		FROM workflow_transition_history wth
		JOIN workflow_instances wi ON wi.id = wth.workflow_instance_id
		JOIN workflow_definitions wd ON wd.id = wi.workflow_definition_id
		WHERE wth.organization_id = $1
		  AND wth.occurred_at >= $2
		  AND wth.occurred_at < $3
		GROUP BY wd.key
	`, orgID, bucketStart, bucketEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to query workflow throughput: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []*domain.WorkflowThroughputMetric
	for rows.Next() {
		var workflowKey string
		var totalTransitions, uniqueCases int
		if err := rows.Scan(&workflowKey, &totalTransitions, &uniqueCases); err != nil {
			return nil, fmt.Errorf("failed to scan throughput row: %w", err)
		}
		avg := 0.0
		if uniqueCases > 0 {
			avg = float64(totalTransitions) / float64(uniqueCases)
		}
		results = append(results, &domain.WorkflowThroughputMetric{
			OrganizationID:        orgID,
			Period:                period,
			Bucket:                bucketStart,
			WorkflowKey:           workflowKey,
			TotalTransitions:      totalTransitions,
			UniqueCases:           uniqueCases,
			AvgTransitionsPerCase: avg,
			CalculatedAt:          time.Now().UTC(),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return results, nil
}

func (r *PostgresMetricsRepository) GetStateDuration(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) ([]*domain.StateDurationMetric, error) {
	bucketStart := bucketStart(bucket, period)
	bucketEnd := bucketStart.AddDate(0, 0, 1)
	if period == domain.MetricPeriodWeekly {
		bucketEnd = bucketStart.AddDate(0, 0, 7)
	}
	if period == domain.MetricPeriodMonthly {
		bucketEnd = bucketStart.AddDate(0, 1, 0)
	}

	rows, err := r.db.QueryContext(ctx, `
		WITH state_durations AS (
			SELECT
				wd.key AS workflow_key,
				wth.from_state AS state_key,
				EXTRACT(EPOCH FROM (
					COALESCE(
						LEAD(wth.occurred_at) OVER (
							PARTITION BY wth.workflow_instance_id
							ORDER BY wth.occurred_at
						),
						LEAST($3, now())
					) - wth.occurred_at
				)) / 3600.0 AS duration_hours
			FROM workflow_transition_history wth
			JOIN workflow_instances wi ON wi.id = wth.workflow_instance_id
			JOIN workflow_definitions wd ON wd.id = wi.workflow_definition_id
			WHERE wth.organization_id = $1
			  AND wth.occurred_at >= $2
			  AND wth.occurred_at < $3
		)
		SELECT workflow_key, state_key,
			COUNT(*)::int,
			AVG(duration_hours),
			AVG(duration_hours),
			MAX(duration_hours)
		FROM state_durations
		GROUP BY workflow_key, state_key
	`, orgID, bucketStart, bucketEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to query state duration: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []*domain.StateDurationMetric
	for rows.Next() {
		var workflowKey, stateKey string
		var count int
		var avg, median, max float64
		if err := rows.Scan(&workflowKey, &stateKey, &count, &avg, &median, &max); err != nil {
			return nil, fmt.Errorf("failed to scan state duration row: %w", err)
		}
		results = append(results, &domain.StateDurationMetric{
			OrganizationID:      orgID,
			Period:              period,
			Bucket:              bucketStart,
			WorkflowKey:         workflowKey,
			StateKey:            stateKey,
			EntryCount:          count,
			AvgDurationHours:    avg,
			MedianDurationHours: median,
			MaxDurationHours:    max,
			CalculatedAt:        time.Now().UTC(),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return results, nil
}

func (r *PostgresMetricsRepository) GetCaseCycleTime(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) ([]*domain.CaseCycleTimeMetric, error) {
	bucketStart := bucketStart(bucket, period)
	bucketEnd := bucketStart.AddDate(0, 0, 1)
	if period == domain.MetricPeriodWeekly {
		bucketEnd = bucketStart.AddDate(0, 0, 7)
	}
	if period == domain.MetricPeriodMonthly {
		bucketEnd = bucketStart.AddDate(0, 1, 0)
	}

	rows, err := r.db.QueryContext(ctx, `
		WITH closed_cases AS (
			SELECT
				COALESCE(wd.key, 'unknown') AS workflow_key,
				EXTRACT(EPOCH FROM (cases.closed_at - cases.created_at)) / 3600.0 AS cycle_hours
			FROM cases
			LEFT JOIN workflow_instances wi ON wi.case_id = cases.id
			LEFT JOIN workflow_definitions wd ON wd.id = wi.workflow_definition_id
			WHERE cases.organization_id = $1
			  AND cases.closed_at >= $2
			  AND cases.closed_at < $3
		)
		SELECT workflow_key,
			COUNT(*)::int,
			AVG(cycle_hours),
			AVG(cycle_hours),
			MAX(cycle_hours)
		FROM closed_cases
		GROUP BY workflow_key
	`, orgID, bucketStart, bucketEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to query case cycle time: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []*domain.CaseCycleTimeMetric
	for rows.Next() {
		var workflowKey string
		var completedCases int
		var avg, median, max float64
		if err := rows.Scan(&workflowKey, &completedCases, &avg, &median, &max); err != nil {
			return nil, fmt.Errorf("failed to scan cycle time row: %w", err)
		}
		results = append(results, &domain.CaseCycleTimeMetric{
			OrganizationID:       orgID,
			Period:               period,
			Bucket:               bucketStart,
			WorkflowKey:          workflowKey,
			CompletedCases:       completedCases,
			AvgCycleTimeHours:    avg,
			MedianCycleTimeHours: median,
			CalculatedAt:         time.Now().UTC(),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return results, nil
}

func (r *PostgresMetricsRepository) GetAgingCases(ctx context.Context, orgID uuid.UUID, thresholdHours float64) ([]*domain.AgingCaseMetric, error) {
	rows, err := r.db.QueryContext(ctx, `
		WITH workflow_state AS (
			SELECT
				COALESCE(wd.key, 'unknown') AS workflow_key,
				COALESCE(wi.current_state, c.status) AS current_state,
				c.service_type,
				c.priority,
				EXTRACT(EPOCH FROM (now() - c.created_at)) / 3600.0 AS age_hours,
				EXTRACT(EPOCH FROM (now() - COALESCE(
					(SELECT wth.occurred_at
					 FROM workflow_transition_history wth
					 WHERE wth.workflow_instance_id = wi.id
					 ORDER BY wth.occurred_at DESC
					 LIMIT 1),
					c.updated_at
				))) / 3600.0 AS in_current_state_hours
			FROM cases c
			LEFT JOIN workflow_instances wi ON wi.case_id = c.id
			LEFT JOIN workflow_definitions wd ON wd.id = wi.workflow_definition_id
			WHERE c.organization_id = $1
			  AND c.status NOT IN ('CLOSED', 'REJECTED')
		)
		SELECT workflow_key, current_state, service_type, priority,
			   age_hours, in_current_state_hours
		FROM workflow_state
		WHERE age_hours >= $2
		ORDER BY age_hours DESC
	`, orgID, thresholdHours)
	if err != nil {
		return nil, fmt.Errorf("failed to query aging cases: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []*domain.AgingCaseMetric
	now := time.Now().UTC()
	for rows.Next() {
		var m domain.AgingCaseMetric
		if err := rows.Scan(
			&m.WorkflowKey, &m.CurrentState,
			&m.ServiceType, &m.Priority,
			&m.AgeHours, &m.InCurrentStateHours,
		); err != nil {
			return nil, fmt.Errorf("failed to scan aging case row: %w", err)
		}
		m.OrganizationID = orgID
		m.CalculatedAt = now
		results = append(results, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return results, nil
}

func (r *PostgresMetricsRepository) GetPendingReviews(ctx context.Context, orgID uuid.UUID) (*domain.PendingReviewMetric, error) {
	metric := &domain.PendingReviewMetric{
		OrganizationID: orgID,
		CalculatedAt:   time.Now().UTC(),
	}

	err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'PENDING'),
			COUNT(*) FILTER (WHERE status = 'ASSIGNED'),
			COUNT(*) FILTER (WHERE status = 'IN_REVIEW'),
			COUNT(*) FILTER (WHERE status = 'WAITING_INFORMATION'),
			COALESCE(AVG(EXTRACT(EPOCH FROM (now() - created_at)) / 3600.0), 0)
		FROM review_queue
		WHERE organization_id = $1
		  AND status NOT IN ('COMPLETED', 'ESCALATED')
	`, orgID).Scan(
		&metric.PendingReviews,
		&metric.AssignedReviews,
		&metric.InReviewReviews,
		&metric.WaitingInfoReviews,
		&metric.AvgWaitTimeHours,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending reviews: %w", err)
	}

	return metric, nil
}

func (r *PostgresMetricsRepository) GetDecisions(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) (*domain.DecisionMetric, error) {
	bucketStart := bucketStart(bucket, period)
	bucketEnd := bucketStart.AddDate(0, 0, 1)
	if period == domain.MetricPeriodWeekly {
		bucketEnd = bucketStart.AddDate(0, 0, 7)
	}
	if period == domain.MetricPeriodMonthly {
		bucketEnd = bucketStart.AddDate(0, 1, 0)
	}

	metric := &domain.DecisionMetric{
		OrganizationID: orgID,
		Period:         period,
		Bucket:         bucketStart,
		CalculatedAt:   time.Now().UTC(),
	}

	err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE decided_at >= $2 AND decided_at < $3),
			COUNT(*) FILTER (WHERE decision = 'APPROVED' AND decided_at >= $2 AND decided_at < $3),
			COUNT(*) FILTER (WHERE decision = 'REJECTED' AND decided_at >= $2 AND decided_at < $3),
			COUNT(*) FILTER (WHERE decision = 'NEEDS_MORE_INFORMATION' AND decided_at >= $2 AND decided_at < $3),
			COUNT(*) FILTER (WHERE decision = 'ESCALATE' AND decided_at >= $2 AND decided_at < $3),
			COUNT(DISTINCT decision_maker) FILTER (WHERE decided_at >= $2 AND decided_at < $3)
		FROM decisions
		WHERE organization_id = $1
	`, orgID, bucketStart, bucketEnd).Scan(
		&metric.TotalDecisions,
		&metric.Approved,
		&metric.Rejected,
		&metric.NeedsMoreInfo,
		&metric.Escalated,
		&metric.AvgDecisionsPerReviewer,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query decisions: %w", err)
	}

	if metric.TotalDecisions > 0 {
		metric.ApprovalRate = float64(metric.Approved) / float64(metric.TotalDecisions)
	}

	return metric, nil
}

func (r *PostgresMetricsRepository) GetAssistanceOutcomes(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) (*domain.AssistanceOutcomeMetric, error) {
	bucketStart := bucketStart(bucket, period)
	bucketEnd := bucketStart.AddDate(0, 0, 1)
	if period == domain.MetricPeriodWeekly {
		bucketEnd = bucketStart.AddDate(0, 0, 7)
	}
	if period == domain.MetricPeriodMonthly {
		bucketEnd = bucketStart.AddDate(0, 1, 0)
	}

	metric := &domain.AssistanceOutcomeMetric{
		OrganizationID: orgID,
		Period:         period,
		Bucket:         bucketStart,
		CalculatedAt:   time.Now().UTC(),
	}

	err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE created_at >= $2 AND created_at < $3),
			COUNT(*) FILTER (WHERE status = 'PLANNED' AND created_at >= $2 AND created_at < $3),
			COUNT(*) FILTER (WHERE status = 'IN_PROGRESS' AND created_at >= $2 AND created_at < $3),
			COUNT(*) FILTER (WHERE status = 'COMPLETED' AND created_at >= $2 AND created_at < $3),
			COUNT(*) FILTER (WHERE status = 'CANCELLED' AND created_at >= $2 AND created_at < $3)
		FROM assistance
		WHERE organization_id = $1
	`, orgID, bucketStart, bucketEnd).Scan(
		&metric.TotalAssistance,
		&metric.Planned,
		&metric.InProgress,
		&metric.Completed,
		&metric.Cancelled,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query assistance outcomes: %w", err)
	}

	if metric.TotalAssistance > 0 {
		metric.CompletionRate = float64(metric.Completed) / float64(metric.TotalAssistance)
	}

	return metric, nil
}

func (r *PostgresMetricsRepository) GetEvidenceVerification(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) (*domain.EvidenceVerificationMetric, error) {
	bucketStart := bucketStart(bucket, period)
	bucketEnd := bucketStart.AddDate(0, 0, 1)
	if period == domain.MetricPeriodWeekly {
		bucketEnd = bucketStart.AddDate(0, 0, 7)
	}
	if period == domain.MetricPeriodMonthly {
		bucketEnd = bucketStart.AddDate(0, 1, 0)
	}

	metric := &domain.EvidenceVerificationMetric{
		OrganizationID: orgID,
		Period:         period,
		Bucket:         bucketStart,
		CalculatedAt:   time.Now().UTC(),
	}

	err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE created_at >= $2 AND created_at < $3),
			COUNT(*) FILTER (WHERE verification_status = 'VERIFIED' AND created_at >= $2 AND created_at < $3),
			COUNT(*) FILTER (WHERE verification_status = 'REJECTED' AND created_at >= $2 AND created_at < $3),
			COUNT(*) FILTER (WHERE verification_status = 'NEEDS_REVIEW' AND created_at >= $2 AND created_at < $3),
			COUNT(*) FILTER (WHERE verification_status = 'UNVERIFIED' AND created_at >= $2 AND created_at < $3)
		FROM evidence
		WHERE organization_id = $1
	`, orgID, bucketStart, bucketEnd).Scan(
		&metric.TotalEvidence,
		&metric.Verified,
		&metric.Rejected,
		&metric.NeedsReview,
		&metric.Unverified,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query evidence verification: %w", err)
	}

	if metric.TotalEvidence > 0 {
		metric.VerificationRate = float64(metric.Verified) / float64(metric.TotalEvidence)
	}

	return metric, nil
}

func (r *PostgresMetricsRepository) GetInformationRequired(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) (*domain.InformationRequiredMetric, error) {
	bucketStart := bucketStart(bucket, period)
	bucketEnd := bucketStart.AddDate(0, 0, 1)
	if period == domain.MetricPeriodWeekly {
		bucketEnd = bucketStart.AddDate(0, 0, 7)
	}
	if period == domain.MetricPeriodMonthly {
		bucketEnd = bucketStart.AddDate(0, 1, 0)
	}

	metric := &domain.InformationRequiredMetric{
		OrganizationID: orgID,
		Period:         period,
		Bucket:         bucketStart,
		CalculatedAt:   time.Now().UTC(),
	}

	err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE created_at >= $2 AND created_at < $3),
			COUNT(*) FILTER (WHERE status = 'DECISION_PENDING' AND workflow_state = 'DECISION_PENDING' AND created_at >= $2 AND created_at < $3),
			COUNT(*) FILTER (WHERE status = 'ESCALATED' AND created_at >= $2 AND created_at < $3),
			COUNT(*) FILTER (WHERE status IN ('PENDING', 'ASSIGNED', 'IN_REVIEW', 'WAITING_INFORMATION') AND created_at >= $2 AND created_at < $3)
		FROM review_queue
		WHERE organization_id = $1
	`, orgID, bucketStart, bucketEnd).Scan(
		&metric.TotalCases,
		&metric.InformationRequired,
		&metric.Escalated,
		&metric.AwaitingInfoReviews,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query information required: %w", err)
	}

	return metric, nil
}
