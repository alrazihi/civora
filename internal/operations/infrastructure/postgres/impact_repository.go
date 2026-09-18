package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/alrazihi/civora/internal/operations/domain"
	"github.com/google/uuid"
)

func (r *PostgresMetricsRepository) GetImpactIntelligenceReport(ctx context.Context, orgID uuid.UUID, period domain.MetricPeriod, bucket time.Time) (*domain.ImpactIntelligenceReport, error) {
	bucketStart := bucketStart(bucket, period)
	bucketEnd := bucketStart.AddDate(0, 0, 1)
	if period == domain.MetricPeriodWeekly {
		bucketEnd = bucketStart.AddDate(0, 0, 7)
	}
	if period == domain.MetricPeriodMonthly {
		bucketEnd = bucketStart.AddDate(0, 1, 0)
	}

	report := &domain.ImpactIntelligenceReport{
		OrganizationID: orgID,
		Period:         period,
		Bucket:         bucketStart,
		CalculatedAt:   time.Now().UTC(),
	}

	var casesCreated, casesClosed, casesRejected, casesNeedsInfo int
	if err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE created_at >= $2 AND created_at < $3),
			COUNT(*) FILTER (WHERE status = 'CLOSED' AND updated_at >= $2 AND updated_at < $3),
			COUNT(*) FILTER (WHERE status = 'REJECTED' AND updated_at >= $2 AND updated_at < $3),
			COUNT(*) FILTER (WHERE EXISTS (
				SELECT 1 FROM decisions d
				WHERE d.service_request_id = cases.id
				  AND d.decision = 'NEEDS_MORE_INFORMATION'
				  AND d.decided_at >= $2 AND d.decided_at < $3
			))
		FROM cases
		WHERE organization_id = $1
	`, orgID, bucketStart, bucketEnd).Scan(&casesCreated, &casesClosed, &casesRejected, &casesNeedsInfo); err != nil {
		return nil, fmt.Errorf("failed to query case outcomes: %w", err)
	}

	var assistanceCreated, assistanceCompleted int
	if err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE created_at >= $2 AND created_at < $3),
			COUNT(*) FILTER (WHERE status = 'COMPLETED' AND completed_at >= $2 AND completed_at < $3)
		FROM assistance
		WHERE organization_id = $1
	`, orgID, bucketStart, bucketEnd).Scan(&assistanceCreated, &assistanceCompleted); err != nil {
		return nil, fmt.Errorf("failed to query assistance outcomes: %w", err)
	}

	var followUpsScheduled, followUpsCompleted int
	if err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE scheduled_date >= $2 AND scheduled_date < $3),
			COUNT(*) FILTER (WHERE completed_date IS NOT NULL AND completed_date >= $2 AND completed_date < $3)
		FROM follow_ups
		WHERE organization_id = $1
	`, orgID, bucketStart, bucketEnd).Scan(&followUpsScheduled, &followUpsCompleted); err != nil {
		return nil, fmt.Errorf("failed to query follow-up outcomes: %w", err)
	}

	var evidenceSubmitted, evidenceVerified int
	if err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE created_at >= $2 AND created_at < $3),
			COUNT(*) FILTER (WHERE verification_status = 'VERIFIED' AND verified_at >= $2 AND verified_at < $3)
		FROM evidence
		WHERE organization_id = $1
	`, orgID, bucketStart, bucketEnd).Scan(&evidenceSubmitted, &evidenceVerified); err != nil {
		return nil, fmt.Errorf("failed to query evidence outcomes: %w", err)
	}

	var decisionsMade, decisionsApproved int
	if err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE decided_at >= $2 AND decided_at < $3),
			COUNT(*) FILTER (WHERE decision = 'APPROVED' AND decided_at >= $2 AND decided_at < $3)
		FROM decisions
		WHERE organization_id = $1
	`, orgID, bucketStart, bucketEnd).Scan(&decisionsMade, &decisionsApproved); err != nil {
		return nil, fmt.Errorf("failed to query decision outcomes: %w", err)
	}

	now := time.Now().UTC()
	metrics := []*domain.OutcomeMetric{
		{
			OrganizationID: orgID,
			Period:         period,
			Bucket:         bucketStart,
			Category:       domain.OutcomeCategoryActivity,
			Name:           "cases_created",
			Label:          "Cases Created",
			Description:    "Total cases opened in the period",
			ActivityCount:  casesCreated,
			Unit:           "cases",
			CalculatedAt:   now,
		},
		{
			OrganizationID: orgID,
			Period:         period,
			Bucket:         bucketStart,
			Category:       domain.OutcomeCategoryOutcome,
			Name:           "cases_completed",
			Label:          "Cases Completed",
			Description:    "Cases moved to a closed terminal state",
			OutcomeCount:   casesClosed,
			Unit:           "cases",
			CalculatedAt:   now,
		},
		{
			OrganizationID: orgID,
			Period:         period,
			Bucket:         bucketStart,
			Category:       domain.OutcomeCategoryOutcome,
			Name:           "cases_rejected",
			Label:          "Cases Rejected",
			Description:    "Cases closed with a rejected status",
			OutcomeCount:   casesRejected,
			Unit:           "cases",
			CalculatedAt:   now,
		},
		{
			OrganizationID: orgID,
			Period:         period,
			Bucket:         bucketStart,
			Category:       domain.OutcomeCategoryOutcome,
			Name:           "cases_needing_information",
			Label:          "Cases Needing Additional Information",
			Description:    "Cases with a decision requesting more information",
			OutcomeCount:   casesNeedsInfo,
			Unit:           "cases",
			CalculatedAt:   now,
		},
		{
			OrganizationID: orgID,
			Period:         period,
			Bucket:         bucketStart,
			Category:       domain.OutcomeCategoryActivity,
			Name:           "assistance_created",
			Label:          "Assistance Created",
			Description:    "Assistance records opened",
			ActivityCount:  assistanceCreated,
			Unit:           "records",
			CalculatedAt:   now,
		},
		{
			OrganizationID: orgID,
			Period:         period,
			Bucket:         bucketStart,
			Category:       domain.OutcomeCategoryOutcome,
			Name:           "assistance_completed",
			Label:          "Assistance Completed",
			Description:    "Assistance records marked as completed",
			OutcomeCount:   assistanceCompleted,
			Unit:           "records",
			CalculatedAt:   now,
		},
		{
			OrganizationID: orgID,
			Period:         period,
			Bucket:         bucketStart,
			Category:       domain.OutcomeCategoryActivity,
			Name:           "follow_ups_scheduled",
			Label:          "Follow-ups Scheduled",
			Description:    "Follow-up appointments scheduled",
			ActivityCount:  followUpsScheduled,
			Unit:           "records",
			CalculatedAt:   now,
		},
		{
			OrganizationID: orgID,
			Period:         period,
			Bucket:         bucketStart,
			Category:       domain.OutcomeCategoryOutcome,
			Name:           "follow_ups_completed",
			Label:          "Follow-ups Completed",
			Description:    "Follow-up appointments marked as completed",
			OutcomeCount:   followUpsCompleted,
			Unit:           "records",
			CalculatedAt:   now,
		},
		{
			OrganizationID: orgID,
			Period:         period,
			Bucket:         bucketStart,
			Category:       domain.OutcomeCategoryActivity,
			Name:           "evidence_submitted",
			Label:          "Evidence Submitted",
			Description:    "Evidence items uploaded to cases",
			ActivityCount:  evidenceSubmitted,
			Unit:           "items",
			CalculatedAt:   now,
		},
		{
			OrganizationID: orgID,
			Period:         period,
			Bucket:         bucketStart,
			Category:       domain.OutcomeCategoryOutcome,
			Name:           "evidence_verified",
			Label:          "Evidence Verified",
			Description:    "Evidence items verified by staff",
			OutcomeCount:   evidenceVerified,
			Unit:           "items",
			CalculatedAt:   now,
		},
		{
			OrganizationID: orgID,
			Period:         period,
			Bucket:         bucketStart,
			Category:       domain.OutcomeCategoryActivity,
			Name:           "decisions_made",
			Label:          "Decisions Made",
			Description:    "Total human decisions recorded",
			ActivityCount:  decisionsMade,
			Unit:           "decisions",
			CalculatedAt:   now,
		},
		{
			OrganizationID: orgID,
			Period:         period,
			Bucket:         bucketStart,
			Category:       domain.OutcomeCategoryOutcome,
			Name:           "decisions_approved",
			Label:          "Decisions Approved",
			Description:    "Decisions with an approved outcome",
			OutcomeCount:   decisionsApproved,
			Unit:           "decisions",
			CalculatedAt:   now,
		},
	}

	if casesCreated > 0 {
		metrics = append(metrics, &domain.OutcomeMetric{
			OrganizationID: orgID,
			Period:         period,
			Bucket:         bucketStart,
			Category:       domain.OutcomeCategoryImpact,
			Name:           "case_completion_rate",
			Label:          "Case Completion Rate",
			Description:    "Share of opened cases that were completed in the period",
			ActivityCount:  casesCreated,
			OutcomeCount:   casesClosed,
			ImpactValue:    float64(casesClosed) / float64(casesCreated),
			Unit:           "ratio",
			CalculatedAt:   now,
		})
		metrics = append(metrics, &domain.OutcomeMetric{
			OrganizationID: orgID,
			Period:         period,
			Bucket:         bucketStart,
			Category:       domain.OutcomeCategoryImpact,
			Name:           "case_rejection_rate",
			Label:          "Case Rejection Rate",
			Description:    "Share of opened cases that were rejected in the period",
			ActivityCount:  casesCreated,
			OutcomeCount:   casesRejected,
			ImpactValue:    float64(casesRejected) / float64(casesCreated),
			Unit:           "ratio",
			CalculatedAt:   now,
		})
		metrics = append(metrics, &domain.OutcomeMetric{
			OrganizationID: orgID,
			Period:         period,
			Bucket:         bucketStart,
			Category:       domain.OutcomeCategoryImpact,
			Name:           "information_request_rate",
			Label:          "Information Request Rate",
			Description:    "Share of opened cases that needed more information",
			ActivityCount:  casesCreated,
			OutcomeCount:   casesNeedsInfo,
			ImpactValue:    float64(casesNeedsInfo) / float64(casesCreated),
			Unit:           "ratio",
			CalculatedAt:   now,
		})
	}

	if assistanceCreated > 0 {
		metrics = append(metrics, &domain.OutcomeMetric{
			OrganizationID: orgID,
			Period:         period,
			Bucket:         bucketStart,
			Category:       domain.OutcomeCategoryImpact,
			Name:           "assistance_completion_rate",
			Label:          "Assistance Completion Rate",
			Description:    "Share of assistance records completed in the period",
			ActivityCount:  assistanceCreated,
			OutcomeCount:   assistanceCompleted,
			ImpactValue:    float64(assistanceCompleted) / float64(assistanceCreated),
			Unit:           "ratio",
			CalculatedAt:   now,
		})
	}

	if followUpsScheduled > 0 {
		metrics = append(metrics, &domain.OutcomeMetric{
			OrganizationID: orgID,
			Period:         period,
			Bucket:         bucketStart,
			Category:       domain.OutcomeCategoryImpact,
			Name:           "follow_up_completion_rate",
			Label:          "Follow-up Completion Rate",
			Description:    "Share of scheduled follow-ups completed in the period",
			ActivityCount:  followUpsScheduled,
			OutcomeCount:   followUpsCompleted,
			ImpactValue:    float64(followUpsCompleted) / float64(followUpsScheduled),
			Unit:           "ratio",
			CalculatedAt:   now,
		})
	}

	if evidenceSubmitted > 0 {
		metrics = append(metrics, &domain.OutcomeMetric{
			OrganizationID: orgID,
			Period:         period,
			Bucket:         bucketStart,
			Category:       domain.OutcomeCategoryImpact,
			Name:           "evidence_verification_rate",
			Label:          "Evidence Verification Rate",
			Description:    "Share of submitted evidence verified in the period",
			ActivityCount:  evidenceSubmitted,
			OutcomeCount:   evidenceVerified,
			ImpactValue:    float64(evidenceVerified) / float64(evidenceSubmitted),
			Unit:           "ratio",
			CalculatedAt:   now,
		})
	}

	if decisionsMade > 0 {
		metrics = append(metrics, &domain.OutcomeMetric{
			OrganizationID: orgID,
			Period:         period,
			Bucket:         bucketStart,
			Category:       domain.OutcomeCategoryImpact,
			Name:           "decision_approval_rate",
			Label:          "Decision Approval Rate",
			Description:    "Share of decisions that were approved",
			ActivityCount:  decisionsMade,
			OutcomeCount:   decisionsApproved,
			ImpactValue:    float64(decisionsApproved) / float64(decisionsMade),
			Unit:           "ratio",
			CalculatedAt:   now,
		})
	}

	report.Metrics = metrics
	return report, nil
}
