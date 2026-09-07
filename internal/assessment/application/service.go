package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	assessmentdomain "github.com/alrazihi/civora/internal/assessment/domain"
	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/database"
	intmid "github.com/alrazihi/civora/internal/middleware"
	"github.com/google/uuid"
)

var (
	ErrAssessmentNotFound = errors.New("assessment not found")
	ErrAssessmentInput    = errors.New("invalid assessment input")
)

type AssessmentService struct {
	repo    assessmentdomain.AssessmentRepository
	auditor auditdomain.EventRecorder
}

type txEventRecorder interface {
	RecordEventInTx(ctx context.Context, tx *sql.Tx, params auditdomain.RecordEventParams) error
}

func NewAssessmentService(repo assessmentdomain.AssessmentRepository, auditor auditdomain.EventRecorder) *AssessmentService {
	return &AssessmentService{repo: repo, auditor: auditor}
}

type CreateAssessmentParams struct {
	OrganizationID   uuid.UUID
	ServiceRequestID uuid.UUID
	Findings         string
	NeedsIdentified  string
	Recommendation   string
	ActorID          uuid.UUID
}

func (s *AssessmentService) CreateAssessment(ctx context.Context, params CreateAssessmentParams) (*assessmentdomain.Assessment, error) {
	a, err := assessmentdomain.NewAssessment(params.OrganizationID, params.ServiceRequestID, params.ActorID, params.Findings, params.NeedsIdentified, params.Recommendation)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAssessmentInput, err)
	}

	var result *assessmentdomain.Assessment
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.SaveTx(ctx, tx, a); err != nil {
			return fmt.Errorf("failed to save assessment: %w", err)
		}

		if s.auditor != nil {
			if err := recordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: a.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "assessment.completed",
				Resource:       "assessment",
				ResourceID:     strPtr(a.ID.String()),
				Outcome:        "success",
				RequestID:      strPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"service_request_id": a.ServiceRequestID.String(),
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		result = a
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *AssessmentService) GetAssessment(ctx context.Context, orgID, id uuid.UUID) (*assessmentdomain.Assessment, error) {
	a, err := s.repo.FindByID(ctx, orgID, id)
	if err != nil {
		return nil, ErrAssessmentNotFound
	}
	return a, nil
}

func (s *AssessmentService) GetAssessmentByServiceRequest(ctx context.Context, orgID, serviceRequestID uuid.UUID) (*assessmentdomain.Assessment, error) {
	a, err := s.repo.FindByServiceRequest(ctx, orgID, serviceRequestID)
	if err != nil {
		return nil, ErrAssessmentNotFound
	}
	return a, nil
}

func (s *AssessmentService) ListAssessments(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*assessmentdomain.Assessment, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	total, err := s.repo.CountByOrganization(ctx, orgID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count assessments: %w", err)
	}

	items, err := s.repo.FindByOrganization(ctx, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list assessments: %w", err)
	}

	return items, total, nil
}

func strPtr(s string) *string {
	return &s
}

func recordAuditEventInTx(ctx context.Context, tx *sql.Tx, auditor auditdomain.EventRecorder, params auditdomain.RecordEventParams) error {
	if txRecorder, ok := auditor.(txEventRecorder); ok {
		return txRecorder.RecordEventInTx(ctx, tx, params)
	}
	return auditor.RecordEvent(ctx, params)
}
