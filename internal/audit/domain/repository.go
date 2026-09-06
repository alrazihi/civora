package domain

import (
	"context"

	"github.com/google/uuid"
)

type AuditRepository interface {
	Save(ctx context.Context, event *AuditEvent) error
	GetLastHash(ctx context.Context, orgID uuid.UUID) (*string, error)
	FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*AuditEvent, error)
}
