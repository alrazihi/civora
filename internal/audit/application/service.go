package application

import (
	"context"
	"fmt"

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

	lastHash, err := s.repo.GetLastHash(ctx, params.OrganizationID)
	if err != nil {
		return fmt.Errorf("failed to get last hash: %w", err)
	}

	actorID := uuid.Nil
	if params.ActorID != nil {
		actorID = *params.ActorID
	}

	event := domain.NewAuditEvent(
		params.OrganizationID,
		actorID,
		params.Action,
		params.Resource,
		params.ResourceID,
		params.Outcome,
		params.RequestID,
		params.Metadata,
		lastHash,
	)

	if err := s.repo.Save(ctx, event); err != nil {
		return fmt.Errorf("failed to save audit event: %w", err)
	}

	return nil
}

func (s *AuditService) FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.AuditEvent, error) {
	return s.repo.FindByOrganization(ctx, orgID, limit, offset)
}
