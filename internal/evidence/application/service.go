package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	casesdomain "github.com/alrazihi/civora/internal/cases/domain"
	"github.com/alrazihi/civora/internal/database"
	evidencedomain "github.com/alrazihi/civora/internal/evidence/domain"
	intmid "github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/google/uuid"
)

var (
	ErrEvidenceNotFound = errors.New("evidence not found")
	ErrEvidenceInput    = errors.New("invalid evidence input")
	ErrCaseNotFound     = errors.New("case not found")
	ErrCaseClosed       = errors.New("case is closed")
	ErrUserNotFound     = errors.New("user not found")
)

type EvidenceService struct {
	repo        evidencedomain.EvidenceRepository
	caseRepo    shared.CaseFinder
	userChecker shared.UserChecker
	auditor     auditdomain.EventRecorder
}

func NewEvidenceService(repo evidencedomain.EvidenceRepository, caseRepo shared.CaseFinder, userChecker shared.UserChecker, auditor auditdomain.EventRecorder) *EvidenceService {
	return &EvidenceService{repo: repo, caseRepo: caseRepo, userChecker: userChecker, auditor: auditor}
}

type AddEvidenceParams struct {
	OrganizationID   uuid.UUID
	ServiceRequestID uuid.UUID
	Type             evidencedomain.EvidenceType
	Description      string
	StorageReference string
	ActorID          uuid.UUID
}

func (s *EvidenceService) AddEvidence(ctx context.Context, params AddEvidenceParams) (*evidencedomain.Evidence, error) {
	c, err := s.caseRepo.FindByID(ctx, params.OrganizationID, params.ServiceRequestID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCaseNotFound, err)
	}
	if c.OrganizationID != params.OrganizationID {
		return nil, ErrCaseNotFound
	}
	if casesdomain.IsClosed(c.Status) {
		return nil, ErrCaseClosed
	}

	valid, err := s.userChecker.BelongsToOrganization(ctx, params.OrganizationID, params.ActorID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate actor: %w", err)
	}
	if !valid {
		return nil, ErrUserNotFound
	}

	e, err := evidencedomain.NewEvidence(params.OrganizationID, params.ServiceRequestID, params.ActorID, params.Type, params.Description, params.StorageReference)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEvidenceInput, err)
	}

	var result *evidencedomain.Evidence
	err = database.InTransaction(ctx, s.repo.DB(), func(tx *sql.Tx) error {
		if err := s.repo.SaveTx(ctx, tx, e); err != nil {
			return fmt.Errorf("failed to save evidence: %w", err)
		}

		if s.auditor != nil {
			if err := shared.RecordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: e.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "evidence.added",
				Resource:       "evidence",
				ResourceID:     shared.StrPtr(e.ID.String()),
				Outcome:        "success",
				RequestID:      shared.StrPtr(intmid.RequestIDFromContext(ctx)),
				Metadata: map[string]interface{}{
					"service_request_id": e.ServiceRequestID.String(),
					"evidence_type":      string(e.Type),
				},
			}); err != nil {
				return fmt.Errorf("failed to record audit event: %w", err)
			}
		}

		result = e
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *EvidenceService) GetEvidence(ctx context.Context, orgID, id uuid.UUID) (*evidencedomain.Evidence, error) {
	e, err := s.repo.FindByID(ctx, orgID, id)
	if err != nil {
		return nil, ErrEvidenceNotFound
	}
	return e, nil
}

func (s *EvidenceService) ListEvidence(ctx context.Context, orgID, serviceRequestID uuid.UUID, limit, offset int) ([]*evidencedomain.Evidence, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	total, err := s.repo.CountByServiceRequest(ctx, orgID, serviceRequestID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count evidence: %w", err)
	}

	items, err := s.repo.FindByServiceRequest(ctx, orgID, serviceRequestID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list evidence: %w", err)
	}

	return items, total, nil
}
