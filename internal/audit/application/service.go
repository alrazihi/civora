package application

import (
	"context"
	"fmt"
	"time"

	"github.com/alrazihi/civora/internal/audit/domain"
	"github.com/google/uuid"
)

type AuditService struct {
	repo domain.AuditRepository
}

func NewAuditService(repo domain.AuditRepository) *AuditService {
	return &AuditService{repo: repo}
}

var _ domain.EventRecorder = (*AuditService)(nil)

func (s *AuditService) RecordEvent(ctx context.Context, params domain.RecordEventParams) error {
	if !domain.IsValidOutcome(params.Outcome) {
		return fmt.Errorf("invalid outcome: %s", params.Outcome)
	}

	actorID := uuid.Nil
	if params.ActorID != nil {
		actorID = *params.ActorID
	}

	event := &domain.AuditEvent{
		ID:             uuid.New(),
		OrganizationID: params.OrganizationID,
		ActorID:        &actorID,
		Action:         params.Action,
		Resource:       params.Resource,
		ResourceID:     params.ResourceID,
		Outcome:        params.Outcome,
		RequestID:      params.RequestID,
		Metadata:       params.Metadata,
		Timestamp:      time.Now().UTC(),
	}

	if err := s.repo.RecordEvent(ctx, params.OrganizationID, event); err != nil {
		return fmt.Errorf("failed to record audit event: %w", err)
	}

	return nil
}

func (s *AuditService) FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.AuditEvent, error) {
	return s.repo.FindByOrganization(ctx, orgID, limit, offset)
}

func (s *AuditService) CountByOrganization(ctx context.Context, orgID uuid.UUID) (int, error) {
	return s.repo.CountByOrganization(ctx, orgID)
}
