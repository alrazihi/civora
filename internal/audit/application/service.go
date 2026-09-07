package application

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/config"
	"github.com/google/uuid"
)

type AuditService struct {
	repo   domain.AuditRepository
	config config.AuditConfig
}

func NewAuditService(repo domain.AuditRepository, cfg config.AuditConfig) *AuditService {
	return &AuditService{
		repo:   repo,
		config: cfg,
	}
}

var _ domain.EventRecorder = (*AuditService)(nil)

func (s *AuditService) RecordEvent(ctx context.Context, params domain.RecordEventParams) error {
	return s.recordEvent(ctx, nil, params)
}

func (s *AuditService) RecordEventInTx(ctx context.Context, tx *sql.Tx, params domain.RecordEventParams) error {
	return s.recordEvent(ctx, tx, params)
}

func (s *AuditService) recordEvent(ctx context.Context, tx *sql.Tx, params domain.RecordEventParams) error {
	if !s.config.Enabled {
		return nil
	}

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

	if actorID == uuid.Nil {
		event.ActorID = nil
	}

	var err error
	if tx != nil {
		err = s.repo.RecordEventTx(ctx, tx, params.OrganizationID, event)
	} else {
		err = s.repo.RecordEvent(ctx, params.OrganizationID, event)
	}
	if err != nil {
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

func (s *AuditService) PurgeOld(ctx context.Context) (int, error) {
	if s.config.RetentionDays <= 0 {
		return 0, nil
	}
	cutoff := time.Now().AddDate(0, 0, -s.config.RetentionDays)
	return s.repo.PurgeOld(ctx, cutoff)
}
