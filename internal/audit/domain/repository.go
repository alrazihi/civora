package domain

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type AuditRepository interface {
	DB() *sql.DB
	Save(ctx context.Context, event *AuditEvent) error
	GetLastHash(ctx context.Context, orgID uuid.UUID) (*string, error)
	FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*AuditEvent, error)
	CountByOrganization(ctx context.Context, orgID uuid.UUID) (int, error)
	RecordEvent(ctx context.Context, orgID uuid.UUID, event *AuditEvent) error
	RecordEventTx(ctx context.Context, tx *sql.Tx, orgID uuid.UUID, event *AuditEvent) error
	PurgeOld(ctx context.Context, olderThan time.Time) (int, error)
}
