package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/database"
	evidencedomain "github.com/alrazihi/civora/internal/evidence/domain"
	intmid "github.com/alrazihi/civora/internal/middleware"
	"github.com/google/uuid"
)

var (
	ErrEvidenceNotFound = errors.New("evidence not found")
	ErrEvidenceInput    = errors.New("invalid evidence input")
)

type EvidenceService struct {
	repo    evidencedomain.EvidenceRepository
	auditor auditdomain.EventRecorder
}

type txEventRecorder interface {
	RecordEventInTx(ctx context.Context, tx *sql.Tx, params auditdomain.RecordEventParams) error
}

func NewEvidenceService(repo evidencedomain.EvidenceRepository, auditor auditdomain.EventRecorder) *EvidenceService {
	return &EvidenceService{repo: repo, auditor: auditor}
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
			if err := recordAuditEventInTx(ctx, tx, s.auditor, auditdomain.RecordEventParams{
				OrganizationID: e.OrganizationID,
				ActorID:        &params.ActorID,
				Action:         "evidence.added",
				Resource:       "evidence",
				ResourceID:     strPtr(e.ID.String()),
				Outcome:        "success",
				RequestID:      strPtr(intmid.RequestIDFromContext(ctx)),
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

func strPtr(s string) *string {
	return &s
}

func recordAuditEventInTx(ctx context.Context, tx *sql.Tx, auditor auditdomain.EventRecorder, params auditdomain.RecordEventParams) error {
	if txRecorder, ok := auditor.(txEventRecorder); ok {
		return txRecorder.RecordEventInTx(ctx, tx, params)
	}
	return auditor.RecordEvent(ctx, params)
}
