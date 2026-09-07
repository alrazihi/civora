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
	"github.com/alrazihi/civora/internal/shared"
	"github.com/google/uuid"
)

var (
	ErrAssessmentNotFound = errors.New("assessment not found")
	ErrAssessmentInput    = errors.New("invalid assessment input")
	ErrCaseNotFound       = errors.New("case not found")
	ErrUserNotFound       = errors.New("user not found")
)

type AssessmentService struct {
	repo        assessmentdomain.AssessmentRepository
	caseRepo    shared.CaseFinder
	userChecker shared.UserChecker
	auditor     auditdomain.EventRecorder
}

func NewAssessmentService(repo assessmentdomain.AssessmentRepository, caseRepo shared.CaseFinder, userChecker shared.UserChecker, auditor auditdomain.EventRecorder) *AssessmentService {
	return &AssessmentService{repo: repo, caseRepo: caseRepo, userChecker: userChecker, auditor: auditor}
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
	c, err := s.caseRepo.FindByID(ctx, params.OrganizationID, params.ServiceRequestID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCaseNotFound, err)
	}
	if c.OrganizationID != params.OrganizationID {
		return nil, ErrCaseNotFound
	}

	existing, err := s.repo.FindByServiceRequest(ctx, params.OrganizationID, params.ServiceRequestID)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("%w: assessment already exists for this service request", ErrAssessmentInput)
	}

	valid, err := s.userChecker.BelongsToOrganization(ctx, params.OrganizationID, params.ActorID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate actor: %w", err)
	}
	if !valid {
		return nil, ErrUserNotFound
	}

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
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: a.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "assessment.completed",
				Resource:       "assessment",
				ResourceID:     shared.StrPtr(a.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
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
