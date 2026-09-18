package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/alrazihi/civora/internal/operations/domain"
	"github.com/google/uuid"
)

func (r *PostgresMetricsRepository) GetStateAccumulations(ctx context.Context, orgID uuid.UUID, threshold int) ([]*domain.StateAccumulationObservation, error) {
	if threshold < domain.MinimumAggregationGroupSize {
		threshold = domain.MinimumAggregationGroupSize
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			c.workflow_key,
			c.workflow_state,
			COUNT(*) AS case_count
		FROM cases c
		WHERE c.organization_id = $1
		  AND c.status NOT IN ('CLOSED', 'REJECTED')
		GROUP BY c.workflow_key, c.workflow_state
		HAVING COUNT(*) >= $2
		ORDER BY case_count DESC
	`, orgID, threshold)
	if err != nil {
		return nil, fmt.Errorf("failed to query state accumulations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []*domain.StateAccumulationObservation
	for rows.Next() {
		var workflowKey, stateKey string
		var caseCount int
		if err := rows.Scan(&workflowKey, &stateKey, &caseCount); err != nil {
			return nil, fmt.Errorf("failed to scan state accumulation row: %w", err)
		}
		results = append(results, &domain.StateAccumulationObservation{
			OrganizationID: orgID,
			WorkflowKey:    workflowKey,
			StateKey:       stateKey,
			CaseCount:      caseCount,
			Threshold:      threshold,
			Observation:    fmt.Sprintf("high volume in state %s", stateKey),
			Language:       string(domain.ObservationLanguageHighVolume),
			CalculatedAt:   time.Now().UTC(),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return results, nil
}

func (r *PostgresMetricsRepository) GetStateDurationAnomalies(ctx context.Context, orgID uuid.UUID, thresholdHours float64) ([]*domain.StateDurationAnomaly, error) {
	rows, err := r.db.QueryContext(ctx, `
		WITH state_durations AS (
			SELECT
				c.workflow_key,
				wth.to_state_id AS state_id,
				ws.name AS state_key,
				AVG(EXTRACT(EPOCH FROM (wth2.created_at - wth.created_at)) / 3600) AS avg_duration_hours,
				PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (wth2.created_at - wth.created_at)) / 3600) AS median_duration_hours,
				MAX(EXTRACT(EPOCH FROM (wth2.created_at - wth.created_at)) / 3600) AS max_duration_hours
			FROM workflow_transition_history wth
			JOIN workflow_instances wi ON wi.id = wth.workflow_instance_id
			JOIN workflow_states ws ON ws.id = wth.to_state_id
			JOIN workflow_transition_history wth2 ON wth2.workflow_instance_id = wth.workflow_instance_id
				AND wth2.id > wth.id
			WHERE wi.organization_id = $1
			GROUP BY c.workflow_key, wth.to_state_id, ws.name
		)
		SELECT workflow_key, state_key, avg_duration_hours, median_duration_hours, max_duration_hours
		FROM state_durations
		WHERE avg_duration_hours > $2
	`, orgID, thresholdHours)
	if err != nil {
		return nil, fmt.Errorf("failed to query state duration anomalies: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []*domain.StateDurationAnomaly
	for rows.Next() {
		var workflowKey, stateKey string
		var avgDuration, medianDuration, maxDuration float64
		if err := rows.Scan(&workflowKey, &stateKey, &avgDuration, &medianDuration, &maxDuration); err != nil {
			return nil, fmt.Errorf("failed to scan state duration anomaly row: %w", err)
		}
		results = append(results, &domain.StateDurationAnomaly{
			OrganizationID:      orgID,
			WorkflowKey:         workflowKey,
			StateKey:            stateKey,
			AvgDurationHours:    avgDuration,
			MedianDurationHours: medianDuration,
			MaxDurationHours:    maxDuration,
			ThresholdHours:      thresholdHours,
			Observation:         fmt.Sprintf("longer observed duration in state %s", stateKey),
			Language:            string(domain.ObservationLanguageLongerDuration),
			CalculatedAt:        time.Now().UTC(),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return results, nil
}

func (r *PostgresMetricsRepository) GetWorkflowClosurePatterns(ctx context.Context, orgID uuid.UUID) ([]*domain.WorkflowClosurePattern, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			workflow_key,
			COUNT(*) AS total_cases,
			COUNT(*) FILTER (WHERE status = 'CLOSED') AS closed_cases,
			COUNT(*) FILTER (WHERE status = 'REJECTED') AS rejected_cases,
			CASE WHEN COUNT(*) > 0 THEN COUNT(*) FILTER (WHERE status = 'CLOSED')::float / COUNT(*) ELSE 0 END AS closure_rate,
			CASE WHEN COUNT(*) > 0 THEN COUNT(*) FILTER (WHERE status = 'REJECTED')::float / COUNT(*) ELSE 0 END AS rejection_rate
		FROM cases
		WHERE organization_id = $1
		GROUP BY workflow_key
		ORDER BY total_cases DESC
	`, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to query workflow closure patterns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []*domain.WorkflowClosurePattern
	for rows.Next() {
		var workflowKey string
		var totalCases, closedCases, rejectedCases int
		var closureRate, rejectionRate float64
		if err := rows.Scan(&workflowKey, &totalCases, &closedCases, &rejectedCases, &closureRate, &rejectionRate); err != nil {
			return nil, fmt.Errorf("failed to scan workflow closure pattern row: %w", err)
		}
		results = append(results, &domain.WorkflowClosurePattern{
			OrganizationID: orgID,
			WorkflowKey:    workflowKey,
			TotalCases:     totalCases,
			ClosedCases:    closedCases,
			RejectedCases:  rejectedCases,
			ClosureRate:    closureRate,
			RejectionRate:  rejectionRate,
			Observation:    "workflow closure pattern observed",
			Language:       string(domain.ObservationLanguageHighVolume),
			CalculatedAt:   time.Now().UTC(),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return results, nil
}

func (r *PostgresMetricsRepository) GetInformationRequestPatterns(ctx context.Context, orgID uuid.UUID, thresholdPerCase float64) ([]*domain.InformationRequestPattern, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			c.workflow_key,
			COUNT(DISTINCT c.id) AS total_cases,
			COUNT(fu.id) AS info_request_count,
			CASE WHEN COUNT(DISTINCT c.id) > 0 THEN COUNT(fu.id)::float / COUNT(DISTINCT c.id) ELSE 0 END AS frequency_per_case
		FROM cases c
		JOIN follow_ups fu ON fu.organization_id = c.organization_id
			AND fu.service_request_id = c.id
			AND fu.type = 'INFORMATION_REQUIRED'
		WHERE c.organization_id = $1
		GROUP BY c.workflow_key
		HAVING COUNT(fu.id)::float / COUNT(DISTINCT c.id) >= $2
		ORDER BY frequency_per_case DESC
	`, orgID, thresholdPerCase)
	if err != nil {
		return nil, fmt.Errorf("failed to query information request patterns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []*domain.InformationRequestPattern
	for rows.Next() {
		var workflowKey string
		var totalCases, infoRequestCount int
		var frequencyPerCase float64
		if err := rows.Scan(&workflowKey, &totalCases, &infoRequestCount, &frequencyPerCase); err != nil {
			return nil, fmt.Errorf("failed to scan information request pattern row: %w", err)
		}
		results = append(results, &domain.InformationRequestPattern{
			OrganizationID:   orgID,
			WorkflowKey:      workflowKey,
			TotalCases:       totalCases,
			InfoRequestCount: infoRequestCount,
			FrequencyPerCase: frequencyPerCase,
			ThresholdPerCase: thresholdPerCase,
			Observation:      "higher frequency of information requests",
			Language:         string(domain.ObservationLanguageHigherFrequency),
			CalculatedAt:     time.Now().UTC(),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return results, nil
}

func (r *PostgresMetricsRepository) GetReviewBacklogObservations(ctx context.Context, orgID uuid.UUID, threshold int) ([]*domain.ReviewBacklogObservation, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'PENDING') AS pending_reviews,
			COUNT(*) FILTER (WHERE status = 'ASSIGNED') AS assigned_reviews,
			COUNT(*) FILTER (WHERE status = 'IN_REVIEW') AS in_review_reviews,
			AVG(EXTRACT(EPOCH FROM (NOW() - created_at)) / 3600) AS avg_wait_time_hours
		FROM review_queue
		WHERE organization_id = $1
		  AND status IN ('PENDING', 'ASSIGNED', 'IN_REVIEW')
		GROUP BY organization_id
		HAVING COUNT(*) FILTER (WHERE status = 'PENDING') >= $2
	`, orgID, threshold)
	if err != nil {
		return nil, fmt.Errorf("failed to query review backlog observations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []*domain.ReviewBacklogObservation
	for rows.Next() {
		var pendingReviews, assignedReviews, inReviewReviews int
		var avgWaitTimeHours float64
		if err := rows.Scan(&pendingReviews, &assignedReviews, &inReviewReviews, &avgWaitTimeHours); err != nil {
			return nil, fmt.Errorf("failed to scan review backlog observation row: %w", err)
		}
		results = append(results, &domain.ReviewBacklogObservation{
			OrganizationID:   orgID,
			PendingReviews:   pendingReviews,
			AssignedReviews:  assignedReviews,
			InReviewReviews:  inReviewReviews,
			AvgWaitTimeHours: avgWaitTimeHours,
			Threshold:        threshold,
			Observation:      "increased backlog in review queue",
			Language:         string(domain.ObservationLanguageIncreasedBacklog),
			CalculatedAt:     time.Now().UTC(),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return results, nil
}

func (r *PostgresMetricsRepository) GetThresholdExceedances(ctx context.Context, orgID uuid.UUID, agingThresholdHours float64, cycleTimeThresholdHours float64) ([]*domain.ThresholdExceedanceObservation, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			workflow_key,
			workflow_state,
			EXTRACT(EPOCH FROM (NOW() - created_at)) / 3600 AS age_hours,
			CASE WHEN closed_at IS NOT NULL THEN EXTRACT(EPOCH FROM (closed_at - created_at)) / 3600 ELSE NULL END AS cycle_time_hours
		FROM cases
		WHERE organization_id = $1
		  AND status NOT IN ('CLOSED', 'REJECTED')
		  AND (EXTRACT(EPOCH FROM (NOW() - created_at)) / 3600 > $2
		       OR (closed_at IS NOT NULL AND EXTRACT(EPOCH FROM (closed_at - created_at)) / 3600 > $3))
	`, orgID, agingThresholdHours, cycleTimeThresholdHours)
	if err != nil {
		return nil, fmt.Errorf("failed to query threshold exceedances: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []*domain.ThresholdExceedanceObservation
	for rows.Next() {
		var workflowKey, currentState string
		var ageHours, cycleTimeHours float64
		if err := rows.Scan(&workflowKey, &currentState, &ageHours, &cycleTimeHours); err != nil {
			return nil, fmt.Errorf("failed to scan threshold exceedance row: %w", err)
		}
		var thresholdType, observation string
		var thresholdValue, actualValue float64
		if ageHours > agingThresholdHours {
			thresholdType = "aging_threshold_hours"
			thresholdValue = agingThresholdHours
			actualValue = ageHours
			observation = fmt.Sprintf("case age %.1f hours exceeds threshold %.1f hours", ageHours, agingThresholdHours)
		}
		if cycleTimeHours > cycleTimeThresholdHours {
			thresholdType = "cycle_time_threshold_hours"
			thresholdValue = cycleTimeThresholdHours
			actualValue = cycleTimeHours
			observation = fmt.Sprintf("case cycle time %.1f hours exceeds threshold %.1f hours", cycleTimeHours, cycleTimeThresholdHours)
		}
		if thresholdType != "" {
			results = append(results, &domain.ThresholdExceedanceObservation{
				OrganizationID: orgID,
				WorkflowKey:    workflowKey,
				CurrentState:   currentState,
				ThresholdType:  thresholdType,
				ThresholdValue: thresholdValue,
				ActualValue:    actualValue,
				Observation:    observation,
				Language:       string(domain.ObservationLanguageLongerDuration),
				CalculatedAt:   time.Now().UTC(),
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return results, nil
}

func (r *PostgresMetricsRepository) GetWorkflowAnalysisReport(ctx context.Context, orgID uuid.UUID, thresholds domain.AnalysisThresholds) (*domain.WorkflowAnalysisReport, error) {
	report := &domain.WorkflowAnalysisReport{
		OrganizationID: orgID,
		CalculatedAt:   time.Now().UTC(),
		Period:         domain.AnalysisPeriodDaily,
	}

	var err error
	if report.StateAccumulations, err = r.GetStateAccumulations(ctx, orgID, thresholds.StateAccumulationThreshold); err != nil {
		return nil, err
	}
	if report.StateDurationAnomalies, err = r.GetStateDurationAnomalies(ctx, orgID, thresholds.StateDurationThresholdHours); err != nil {
		return nil, err
	}
	if report.WorkflowClosurePatterns, err = r.GetWorkflowClosurePatterns(ctx, orgID); err != nil {
		return nil, err
	}
	if report.InformationRequestPatterns, err = r.GetInformationRequestPatterns(ctx, orgID, thresholds.InfoRequestFrequencyPerCase); err != nil {
		return nil, err
	}
	if report.ReviewBacklogObservations, err = r.GetReviewBacklogObservations(ctx, orgID, thresholds.ReviewBacklogThreshold); err != nil {
		return nil, err
	}
	if report.ThresholdExceedances, err = r.GetThresholdExceedances(ctx, orgID, thresholds.AgingThresholdHours, thresholds.CycleTimeThresholdHours); err != nil {
		return nil, err
	}

	return report, nil
}
